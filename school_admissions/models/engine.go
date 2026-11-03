package models

import (
	"context"
	"time"

	"nexa/core/orm"
	"nexa/core/scheduler"
)

// Behaviour layer for admissions. The transition table, seat arithmetic and offer
// expiry predicate are pure and unit-tested (no DB); the computes project live
// pipeline signals and the cron flips stale offers to expired.

const (
	applicantModel = "school.applicant"
	docModel       = "school.application.document"
	decisionModel  = "school.admission.decision"

	studentModel = "school.student"
	sectionModel = "school.section"

	expireCronCode = "school.applicant.expire"

	dateLayout   = "2006-01-02"
	datetimeFull = "2006-01-02 15:04:05"
)

// transition is one named pipeline move: the set of states it is legal to leave
// from, and the single state it lands in. Centralising the map here is what keeps
// the workflow honest — the actions in actions.go just delegate to allowed().
type transition struct {
	from []string
	to   string
}

var transitions = map[string]transition{
	"submit":   {from: []string{string(StateDraft)}, to: string(StateSubmitted)},
	"review":   {from: []string{string(StateSubmitted)}, to: string(StateReview)},
	"assess":   {from: []string{string(StateReview)}, to: string(StateAssessment)},
	"offer":    {from: []string{string(StateReview), string(StateAssessment)}, to: string(StateOffered)},
	"waitlist": {from: []string{string(StateReview), string(StateAssessment)}, to: string(StateWaitlisted)},
	"reject":   {from: []string{string(StateReview), string(StateAssessment), string(StateWaitlisted)}, to: string(StateRejected)},
	"accept":   {from: []string{string(StateOffered)}, to: string(StateAccepted)},
	"withdraw": {from: []string{string(StateSubmitted), string(StateReview), string(StateAssessment), string(StateOffered), string(StateWaitlisted), string(StateAccepted)}, to: string(StateCancelled)},
	"enroll":   {from: []string{string(StateAccepted)}, to: string(StateEnrolled)},
}

// allowed reports whether `action` may fire from state `from`.
func allowed(action, from string) bool {
	t, ok := transitions[action]
	if !ok {
		return false
	}
	for _, s := range t.from {
		if s == from {
			return true
		}
	}
	return false
}

// targetState returns the state `action` lands in (empty if unknown action).
func targetState(action string) string {
	if t, ok := transitions[action]; ok {
		return t.to
	}
	return ""
}

// seatsAvailable reports whether a place can be taken. A capacity of 0 (or less)
// means the section is uncapped.
func seatsAvailable(capacity, enrolled int) bool {
	if capacity <= 0 {
		return true
	}
	return enrolled < capacity
}

// offerExpired reports whether an expiry date has passed relative to `today`
// (both YYYY-MM-DD, so a lexical compare is a correct date compare). A blank
// expiry never expires.
func offerExpired(today, expiry string) bool {
	if expiry == "" {
		return false
	}
	return expiry < today
}

// rankKey builds the duplicate-detection fingerprint: trimmed lowercase
// "lastname|firstname|dob". Empty on any missing component so it never matches.
func rankKey(last, first, dob string) string {
	if trim(last) == "" || trim(dob) == "" {
		return ""
	}
	return lower(trim(last)) + "|" + lower(trim(first)) + "|" + trim(dob)
}

func init() {
	orm.RegisterCompute(applicantModel, "docs_complete", []string{"id"}, computeDocs)
	orm.RegisterCompute(applicantModel, "decision_count", []string{"id"}, computeDecisions)
	orm.RegisterCompute(applicantModel, "duplicate_suspect", []string{"last_name", "first_name", "date_birth", "id"}, computeDuplicate)
	orm.RegisterCompute(applicantModel, "seat_available", []string{"section_id"}, computeSeat)
	scheduler.RegisterCronHandler(expireCronCode, runOfferExpiry)
}

// ---------------------------------------------------------------------------
// Computes
// ---------------------------------------------------------------------------

func computeDocs(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	id := intOr(rec["id"])
	if id <= 0 {
		return false, nil
	}
	bypass := orm.ContextWithBypass(ctx, true)
	// Incomplete if any required document is still not received.
	rows, err := orm.Search(bypass, docModel, [][]interface{}{
		{"applicant_id", "=", id},
		{"required", "=", true},
		{"received", "=", false},
	})
	if err != nil {
		return true, nil
	}
	return len(rows) == 0, nil
}

func computeDecisions(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	id := intOr(rec["id"])
	if id <= 0 {
		return 0, nil
	}
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), decisionModel, [][]interface{}{
		{"applicant_id", "=", id},
	})
	if err != nil {
		return 0, nil
	}
	return n, nil
}

func computeDuplicate(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	key := rankKey(strOr(rec["last_name"]), strOr(rec["first_name"]), strOr(rec["date_birth"]))
	id := intOr(rec["id"])
	if key == "" {
		return false, nil
	}
	// Compare against stored candidates in the same tenant; the fingerprint uses
	// trimmed lowercase last|first|dob so casing/whitespace differences still hit.
	rows, err := orm.Search(orm.ContextWithBypass(ctx, true), applicantModel, [][]interface{}{})
	if err != nil {
		return false, nil
	}
	for _, r := range rows {
		if intOr(r["id"]) == id {
			continue
		}
		if rankKey(strOr(r["last_name"]), strOr(r["first_name"]), strOr(r["date_birth"])) == key {
			return true, nil
		}
	}
	return false, nil
}

func computeSeat(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	secID := intOr(rec["section_id"])
	if secID <= 0 {
		return true, nil
	}
	return sectionHasSeat(orm.ContextWithBypass(ctx, true), secID), nil
}

// sectionHasSeat reads a section's capacity and its current active student count
// and reports whether a seat is still free. Shared by the compute and the action.
func sectionHasSeat(ctx context.Context, sectionID int) bool {
	sec, err := orm.SearchOne(ctx, sectionModel, map[string]interface{}{"id": sectionID})
	if err != nil || sec == nil {
		return true
	}
	capacity := intOr(sec["capacity"])
	enrolled, err := orm.SearchCount(ctx, studentModel, [][]interface{}{
		{"section_id", "=", sectionID},
		{"active", "=", true},
	})
	if err != nil {
		return true
	}
	return seatsAvailable(capacity, enrolled)
}

// ---------------------------------------------------------------------------
// Cron: expire stale offers / waitlist holds past their expiry date.
// ---------------------------------------------------------------------------

func runOfferExpiry(ctx context.Context, _ map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	today := time.Now().UTC().Format(dateLayout)
	rows, err := orm.Search(bypass, applicantModel, [][]interface{}{
		{"state", "in", []string{string(StateOffered), string(StateWaitlisted)}},
	})
	if err != nil {
		return err
	}
	for _, r := range rows {
		expiry := strOr(r["offer_expiry"])
		if !offerExpired(today, expiry) {
			continue
		}
		id := intOr(r["id"])
		if id <= 0 {
			continue
		}
		if err := orm.UpdateRecordByID(bypass, applicantModel, id, map[string]interface{}{
			"state": string(StateExpired),
		}); err != nil {
			return err
		}
		appendDecision(bypass, id, strOr(r["state"]), string(StateExpired), "expire", "Offer/waitlist hold expired automatically", 0)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Shared value helpers + append-only decision writer
// ---------------------------------------------------------------------------

func appendDecision(ctx context.Context, applicantID int, from, to, action, reason string, operator int) {
	_, _ = orm.Create(ctx, orm.RegistryModel(decisionModel), map[string]interface{}{
		"applicant_id": applicantID,
		"from_state":   from,
		"to_state":     to,
		"action":       action,
		"reason":       reason,
		"operator_id":  operator,
		"timestamp":    time.Now().UTC().Format(datetimeFull),
	})
}

func intOr(v interface{}) int {
	n, ok := orm.CoerceInt64(v)
	if !ok {
		return 0
	}
	return int(n)
}

func strOr(v interface{}) string {
	return orm.AsString(v)
}

func trim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && s[len(s)-1] == ' ' {
		s = s[:len(s)-1]
	}
	return s
}

func lower(s string) string {
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		out[i] = c
	}
	return string(out)
}
