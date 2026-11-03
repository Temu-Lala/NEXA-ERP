package models

import (
	"context"
	"time"

	"nexa/core/orm"
)

// Behaviour layer for the school portal. The security-critical pieces are pure and
// unit-tested: the self-service request state machine (allowedTransition), the actionable
// test (isActionable), the widget role-visibility test (widgetVisible), the elapsed-days
// arithmetic (ageDays) and, most importantly, the ownership-isolation predicate
// (studentInScope) that decides whether a learner is inside a profile's scope — a
// learner is in scope only if it is the profile's own student record or appears among
// the learners linked to the profile's guardian. The computes project a profile's
// in-scope learner count and role label, and a request's age and action-needed flag.

const (
	profileModel        = "school.portal.profile"
	widgetModel         = "school.portal.widget"
	requestModel        = "school.portal.request"
	requestHistoryModel = "school.portal.request.history"
	accessLogModel      = "school.portal.access.log"
	studentGuardianRef  = "school.student.guardian"

	dateLayout   = "2006-01-02"
	datetimeFull = "2006-01-02 15:04:05"
)

// allowedTransition reports whether a request may move from one state to another.
// Terminal states (approved only advances to fulfilled; rejected/fulfilled/cancelled)
// cannot regress.
func allowedTransition(from, to string) bool {
	switch from {
	case string(ReqDraft):
		return to == string(ReqSubmitted) || to == string(ReqCancelled)
	case string(ReqSubmitted):
		return to == string(ReqUnderReview) || to == string(ReqApproved) || to == string(ReqRejected) || to == string(ReqCancelled)
	case string(ReqUnderReview):
		return to == string(ReqApproved) || to == string(ReqRejected) || to == string(ReqCancelled)
	case string(ReqApproved):
		return to == string(ReqFulfilled)
	default: // rejected, fulfilled, cancelled are terminal
		return false
	}
}

// isActionable reports whether a request in the given state is awaiting staff action.
func isActionable(state string) bool {
	return state == string(ReqSubmitted) || state == string(ReqUnderReview)
}

// ageDays is the elapsed whole days between a start date and an end date (defaulting to
// "today" when the end is blank). Any parse failure or a negative span yields 0.
func ageDays(fromDate, toDate, today string) int {
	if fromDate == "" {
		return 0
	}
	start, err := time.Parse(dateLayout, fromDate)
	if err != nil {
		return 0
	}
	endStr := toDate
	if endStr == "" {
		endStr = today
	}
	end, err := time.Parse(dateLayout, endStr)
	if err != nil {
		return 0
	}
	d := int(end.Sub(start).Hours() / 24)
	if d < 0 {
		return 0
	}
	return d
}

// widgetVisible reports whether a dashboard tile is shown to a portal role.
func widgetVisible(showParent, showStudent, showStaff bool, role string) bool {
	switch role {
	case string(RoleParent):
		return showParent
	case string(RoleStudent):
		return showStudent
	case string(RoleStaff):
		return showStaff
	default:
		return false
	}
}

// studentInScope is the ownership-isolation predicate. A target learner is reachable by
// a profile only when it is the profile's own student record or one of the learners
// linked to the profile's guardian. A zero target is never in scope.
func studentInScope(targetStudent, ownStudent int, linkedStudents []int) bool {
	if targetStudent <= 0 {
		return false
	}
	if ownStudent > 0 && targetStudent == ownStudent {
		return true
	}
	for _, id := range linkedStudents {
		if id == targetStudent {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Computes
// ---------------------------------------------------------------------------

func init() {
	orm.RegisterCompute(profileModel, "accessible_learner_count", []string{"id", "role", "student_id", "guardian_id"}, computeProfileLearnerCount)
	orm.RegisterCompute(profileModel, "role_label", []string{"role"}, computeProfileRoleLabel)
	orm.RegisterCompute(requestModel, "age_days", []string{"submitted_date", "resolved_date"}, computeRequestAgeDays)
	orm.RegisterCompute(requestModel, "actionable", []string{"state"}, computeRequestActionable)
}

func computeProfileLearnerCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	role := strOr(rec["role"])
	if role == string(RoleStudent) && intOr(rec["student_id"]) > 0 {
		return 1, nil
	}
	if guardian := intOr(rec["guardian_id"]); guardian > 0 {
		n, err := orm.SearchCount(bypass, studentGuardianRef, [][]interface{}{
			{"guardian_id", "=", guardian},
			{"status", "=", "active"},
		})
		if err != nil {
			return 0, nil
		}
		return n, nil
	}
	return 0, nil
}

func computeProfileRoleLabel(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	switch strOr(rec["role"]) {
	case string(RoleParent):
		return "Parent / Guardian", nil
	case string(RoleStudent):
		return "Student", nil
	case string(RoleStaff):
		return "Staff", nil
	default:
		return "", nil
	}
}

func computeRequestAgeDays(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	submitted := strOr(rec["submitted_date"])
	if submitted == "" {
		submitted = strOr(rec["created_at"])
	}
	return ageDays(submitted, strOr(rec["resolved_date"]), time.Now().UTC().Format(dateLayout)), nil
}

func computeRequestActionable(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return isActionable(strOr(rec["state"])), nil
}

// ---------------------------------------------------------------------------
// Shared coercion helpers (ORM values arrive as interface{})
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
