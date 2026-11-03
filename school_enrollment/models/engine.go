package models

import (
	"context"
	"time"

	"nexa/core/orm"
)

// Behaviour layer for enrollment. The transition table, the current-placement
// predicate and the duplicate-enrollment guard are pure and unit-tested; the compute
// projects a live "current" flag and the history writer keeps every placement move
// auditable. Transfer/graduation orchestration lives in actions.go.

const (
	enrollmentModel = "school.enrollment"
	historyModel    = "school.enrollment.history"
	alumniModel     = "school.alumni"

	dateLayout   = "2006-01-02"
	datetimeFull = "2006-01-02 15:04:05"
)

// transition is one named enrollment move: the states it may leave from and the single
// state it lands in. actions.go delegates legality to allowed().
type transition struct {
	from []string
	to   string
}

var transitions = map[string]transition{
	"confirm":  {from: []string{string(EnrDraft)}, to: string(EnrActive)},
	"transfer": {from: []string{string(EnrActive)}, to: string(EnrTransferred)},
	"withdraw": {from: []string{string(EnrActive)}, to: string(EnrWithdrawn)},
	"graduate": {from: []string{string(EnrActive)}, to: string(EnrGraduated)},
	"cancel":   {from: []string{string(EnrDraft)}, to: string(EnrCancelled)},
}

// allowed reports whether `action` may fire from enrollment state `from`.
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

// targetState returns the state `action` lands in (empty for an unknown action).
func targetState(action string) string {
	if t, ok := transitions[action]; ok {
		return t.to
	}
	return ""
}

// isCurrent reports whether an enrollment is the learner's live placement as of `today`:
// it must be active and, if it carries an exit date, not yet past it.
func isCurrent(state, exitDate, today string) bool {
	if state != string(EnrActive) {
		return false
	}
	if exitDate != "" && today > exitDate {
		return false
	}
	return true
}

// blocksDuplicate reports whether opening another active enrollment should be refused.
// The blueprint forbids two active enrollments for the same learner in the same
// academic year, so any existing active count blocks a new one.
func blocksDuplicate(existingActive int) bool {
	return existingActive > 0
}

func init() {
	orm.RegisterCompute(enrollmentModel, "is_current", []string{"state", "exit_date"}, computeCurrent)
}

func computeCurrent(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	state := strOr(rec["state"])
	exit := strOr(rec["exit_date"])
	today := time.Now().UTC().Format(dateLayout)
	return isCurrent(state, exit, today), nil
}

// appendHistory writes one immutable enrollment-history row (bypassing ACL; called
// from actions running under an authorised user).
func appendHistory(ctx context.Context, enrollmentID, studentID int, from, to, action string, fromSection, toSection int, reason string, operator int) {
	vals := map[string]interface{}{
		"enrollment_id": enrollmentID,
		"student_id":    studentID,
		"from_state":    from,
		"to_state":      to,
		"action":        action,
		"reason":        reason,
		"operator_id":   operator,
		"timestamp":     time.Now().UTC().Format(datetimeFull),
	}
	if fromSection > 0 {
		vals["from_section_id"] = fromSection
	}
	if toSection > 0 {
		vals["to_section_id"] = toSection
	}
	_, _ = orm.Create(ctx, orm.RegistryModel(historyModel), vals)
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
