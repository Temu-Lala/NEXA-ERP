package models

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nexa/core/event"
	"nexa/core/orm"
)

// Recruitment-plus behaviour layer: two idempotent event handlers (duplicate
// detection on applicants, weighted-score roll-up on scorecards) plus the pure
// helpers used by the workflow actions. Both handlers only write when a value
// actually changes, so re-firing the same event is a no-op (no update loop).

const (
	applicantModel   = "hr.applicant"
	interviewModel   = "hr.recruitment.interview"
	scorecardModel   = "hr.recruitment.scorecard"
	offerModel       = "hr.recruitment.offer"
	openingModel     = "hr.job.opening"
	planModel        = "hr.onboarding.plan"
	activityModel    = "hr.onboarding.activity"
	employeeModel    = "hr.employee"
	recommendPctGate = 70.0
	dateLayout       = "2006-01-02"
)

func init() {
	event.Subscribe(orm.EventRecordCreated, onApplicantChanged)
	event.Subscribe(orm.EventRecordUpdated, onApplicantChanged)
	event.Subscribe(orm.EventRecordCreated, onScorecardChanged)
	event.Subscribe(orm.EventRecordUpdated, onScorecardChanged)
}

// ---------------------------------------------------------------------------
// Pure helpers
// ---------------------------------------------------------------------------

// weightedScore reduces scorecard lines to a weighted mean and a percentage of the
// weighted maximum. recommend is true only when there are scored lines and the
// percentage clears the gate.
func weightedScore(lines []map[string]interface{}) (avg, pct float64, recommend bool) {
	var sumScoreWeighted, sumMaxWeighted, sumWeight, sumScore float64
	for _, l := range lines {
		score := floatOr(l["score"])
		maxScore := floatOr(l["max_score"])
		weight := floatOr(l["weight"])
		if weight <= 0 {
			weight = 1
		}
		sumScore += score
		sumWeight += weight
		sumScoreWeighted += score * weight
		sumMaxWeighted += maxScore * weight
	}
	if sumWeight <= 0 {
		return 0, 0, false
	}
	avg = round2(sumScore / sumWeight)
	if sumMaxWeighted > 0 {
		pct = round2(sumScoreWeighted / sumMaxWeighted * 100)
	}
	recommend = len(lines) > 0 && pct >= recommendPctGate
	return avg, pct, recommend
}

// renderTemplate substitutes {{key}} tokens in a template body with values.
func renderTemplate(body string, vars map[string]string) string {
	out := body
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}
	return out
}

func normalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func intOr(v interface{}) int {
	n, ok := orm.CoerceInt64(v)
	if !ok {
		return 0
	}
	return int(n)
}

func floatOr(v interface{}) float64 {
	f, ok := orm.CoerceFloat64(v)
	if !ok {
		return 0
	}
	return f
}

func round2(f float64) float64 {
	scaled := f * 100
	if scaled >= 0 {
		return float64(int64(scaled+0.5)) / 100
	}
	return float64(int64(scaled-0.5)) / 100
}

func dateStr(v interface{}) string {
	s := strings.TrimSpace(orm.AsString(v))
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func parseDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if len(s) >= 10 {
		s = s[:10]
	}
	if t, err := time.Parse(dateLayout, s); err == nil {
		return t
	}
	return time.Time{}
}

func addDays(s string, n int) string {
	t := parseDate(s)
	if t.IsZero() {
		return ""
	}
	return t.AddDate(0, 0, n).Format(dateLayout)
}

func createRecord(ctx context.Context, model string, vals map[string]interface{}) (int, error) {
	inst, ok := orm.Registry[model]
	if !ok {
		return 0, fmt.Errorf("model %s is not registered", model)
	}
	return orm.Create(ctx, inst, vals)
}

// ---------------------------------------------------------------------------
// Duplicate detection
// ---------------------------------------------------------------------------

func onApplicantChanged(ctx context.Context, ev event.Event) error {
	model, _ := ev.Payload["model"].(string)
	if model != applicantModel {
		return nil
	}
	id, ok := orm.CoerceInt64(ev.Payload["id"])
	if !ok || id <= 0 {
		return nil
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, applicantModel, map[string]interface{}{"id": int(id)})
	if err != nil || rec == nil {
		return nil
	}
	email := normalizeEmail(orm.AsString(rec["email"]))
	// Flag the earliest other applicant with the same email as the duplicate source.
	dup := 0
	if email != "" {
		cands, err := orm.SearchPage(bypass, applicantModel, [][]interface{}{
			{"email", "=", orm.AsString(rec["email"])},
			{"id", "<", int(id)},
		}, 1, 0, "id ASC")
		if err == nil && len(cands) > 0 {
			dup = intOr(cands[0]["id"])
		}
	}
	want := dup > 0
	vals := map[string]interface{}{}
	if orm.AsBool(rec["duplicate_detected"]) != want {
		vals["duplicate_detected"] = want
	}
	if intOr(rec["duplicate_of_id"]) != dup {
		vals["duplicate_of_id"] = dup
	}
	if len(vals) == 0 {
		return nil
	}
	_ = orm.UpdateRecordByID(bypass, applicantModel, int(id), vals)
	return nil
}

// ---------------------------------------------------------------------------
// Scorecard roll-up
// ---------------------------------------------------------------------------

func onScorecardChanged(ctx context.Context, ev event.Event) error {
	model, _ := ev.Payload["model"].(string)
	if model != scorecardModel {
		return nil
	}
	id, ok := orm.CoerceInt64(ev.Payload["id"])
	if !ok || id <= 0 {
		return nil
	}
	bypass := orm.ContextWithBypass(ctx, true)
	card, err := orm.SearchOne(bypass, scorecardModel, map[string]interface{}{"id": int(id)})
	if err != nil || card == nil {
		return nil
	}
	recomputeInterview(bypass, intOr(card["interview_id"]))
	return nil
}

func recomputeInterview(ctx context.Context, interviewID int) {
	if interviewID <= 0 {
		return
	}
	interview, err := orm.SearchOne(ctx, interviewModel, map[string]interface{}{"id": interviewID})
	if err != nil || interview == nil {
		return
	}
	lines, err := orm.Search(ctx, scorecardModel, [][]interface{}{{"interview_id", "=", interviewID}})
	if err != nil {
		return
	}
	avg, pct, recommend := weightedScore(lines)
	vals := map[string]interface{}{}
	if round2(floatOr(interview["avg_score"])) != avg {
		vals["avg_score"] = avg
	}
	if round2(floatOr(interview["score_pct"])) != pct {
		vals["score_pct"] = pct
	}
	if orm.AsBool(interview["recommend"]) != recommend {
		vals["recommend"] = recommend
	}
	if len(vals) == 0 {
		return
	}
	_ = orm.UpdateRecordByID(ctx, interviewModel, interviewID, vals)
}
