package models

import (
	"context"
	"time"

	"nexa/core/orm"
)

// Behaviour layer for staff. The two pieces of arithmetic worth testing — turning an
// observation score into a rating band, and deciding whether a leave range covers a given
// day — are pure. Everything else projects the profile's live signals: weekly teaching
// load summed from school.teaching.assignment, and leave read straight from the reused HR
// hr.leave records (no new leave engine).

const (
	memberModel     = "school.staff.member"
	qualModel       = "school.staff.qualification"
	substModel      = "school.staff.substitution"
	perfModel       = "school.staff.performance.review"
	assignmentModel = "school.teaching.assignment"
	leaveModel      = "hr.leave"

	dateLayout = "2006-01-02"
)

// performanceBand maps a 0-100 observation score to a rating label. Out-of-range scores
// still classify (negative => Unsatisfactory; >=90 => Outstanding).
func performanceBand(score float64) string {
	switch {
	case score >= 90:
		return "Outstanding"
	case score >= 75:
		return "Exceeds Expectations"
	case score >= 60:
		return "Meets Expectations"
	case score >= 40:
		return "Developing"
	default:
		return "Unsatisfactory"
	}
}

// coversDate reports whether ISO date `on` falls within [from, to]. A blank `to` is
// treated as a single-day range; a blank `from` never matches.
func coversDate(from, to, on string) bool {
	if from == "" {
		return false
	}
	if to == "" {
		to = from
	}
	return on >= from && on <= to
}

func init() {
	orm.RegisterCompute(memberModel, "qualification_count", []string{"id"}, computeQualCount)
	orm.RegisterCompute(memberModel, "weekly_periods", []string{"teacher_id"}, computeWeeklyPeriods)
	orm.RegisterCompute(memberModel, "open_leave_requests", []string{"employee_id"}, computeOpenLeave)
	orm.RegisterCompute(memberModel, "on_leave", []string{"employee_id"}, computeOnLeave)
	orm.RegisterCompute(perfModel, "overall_rating", []string{"observation_score"}, computePerfRating)
}

func computeQualCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), qualModel, [][]interface{}{
		{"staff_id", "=", intOr(rec["id"])},
	})
	if err != nil {
		return 0, nil
	}
	return n, nil
}

func computeWeeklyPeriods(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	userID := intOr(rec["teacher_id"])
	if userID <= 0 {
		return 0, nil
	}
	rows, err := orm.Search(orm.ContextWithBypass(ctx, true), assignmentModel, [][]interface{}{
		{"teacher_id", "=", userID},
		{"active", "=", true},
	})
	if err != nil {
		return 0, nil
	}
	total := 0
	for _, r := range rows {
		total += intOr(r["periods_per_week"])
	}
	return total, nil
}

func computeOpenLeave(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	empID := intOr(rec["employee_id"])
	if empID <= 0 {
		return 0, nil
	}
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), leaveModel, [][]interface{}{
		{"employee_id", "=", empID},
		{"state", "in", []string{"draft", "confirm"}},
	})
	if err != nil {
		return 0, nil
	}
	return n, nil
}

func computeOnLeave(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	empID := intOr(rec["employee_id"])
	if empID <= 0 {
		return false, nil
	}
	today := time.Now().UTC().Format(dateLayout)
	rows, err := orm.Search(orm.ContextWithBypass(ctx, true), leaveModel, [][]interface{}{
		{"employee_id", "=", empID},
		{"state", "=", "validate"},
	})
	if err != nil {
		return false, nil
	}
	for _, r := range rows {
		if coversDate(strOr(r["date_from"]), strOr(r["date_to"]), today) {
			return true, nil
		}
	}
	return false, nil
}

func computePerfRating(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return performanceBand(floatOr(rec["observation_score"])), nil
}

// ---------------------------------------------------------------------------
// Shared coercion helpers (self-contained to this module)
// ---------------------------------------------------------------------------

func intOr(v interface{}) int {
	n, ok := orm.CoerceInt64(v)
	if !ok {
		return 0
	}
	return int(n)
}

func strOr(v interface{}) string { return orm.AsString(v) }

func boolOr(v interface{}) bool { return orm.AsBool(v) }

func floatOr(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	}
	if i, ok := orm.CoerceInt64(v); ok {
		return float64(i)
	}
	return 0
}
