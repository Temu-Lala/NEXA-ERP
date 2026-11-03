package models

import (
	"context"
	"fmt"
	"time"

	"nexa/core/orm"
)

// Object actions drive the self-service request lifecycle and portal access control.
// Every status move is validated against the pure state machine, applied with access
// bypass, and recorded in the append-only request history; the resulting data touch is
// mirrored into the immutable access log so ownership-isolation compliance can be
// audited. Raising a request (submit) and withdrawing one (cancel) only require a signed-
// in user, whereas triage and resolution (review, approve, reject, fulfil) and revoking a
// portal profile require a school officer. Rejection captures a reason from the caller.

func init() {
	orm.RegisterObjectAction(requestModel, "request_submit", actRequestSubmit)
	orm.RegisterObjectAction(requestModel, "request_review", actRequestReview)
	orm.RegisterObjectAction(requestModel, "request_approve", actRequestApprove)
	orm.RegisterObjectAction(requestModel, "request_reject", actRequestReject)
	orm.RegisterObjectAction(requestModel, "request_fulfil", actRequestFulfil)
	orm.RegisterObjectAction(requestModel, "request_cancel", actRequestCancel)
	orm.RegisterObjectAction(profileModel, "profile_deactivate", actProfileDeactivate)
}

func nowStamp() string { return time.Now().UTC().Format(datetimeFull) }

// transition validates and applies one request state move, then writes the append-only
// history row and an access-log audit entry.
func transition(ctx context.Context, id int, to, action string, note string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, requestModel, id)
	if err != nil {
		return "", err
	}
	from := strOr(rec["state"])
	if !allowedTransition(from, to) {
		return "", fmt.Errorf("a request cannot move from %s to %s", label(from), label(to))
	}
	patch := map[string]interface{}{"state": to}
	switch to {
	case string(ReqSubmitted):
		patch["submitted_date"] = time.Now().UTC().Format(dateLayout)
	case string(ReqApproved), string(ReqRejected), string(ReqFulfilled):
		patch["resolved_date"] = time.Now().UTC().Format(dateLayout)
	}
	if note != "" {
		if to == string(ReqRejected) {
			patch["reject_reason"] = note
		}
		patch["resolution_note"] = note
	}
	if err := orm.UpdateRecordByID(bypass, requestModel, id, patch); err != nil {
		return "", err
	}
	writeHistory(bypass, id, from, to, action, orm.SecurityUID(ctx), note)
	logAccess(bypass, rec)
	return fmt.Sprintf("Request %s", label(to)), nil
}

func actRequestSubmit(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	if orm.SecurityUID(ctx) <= 0 {
		return "", fmt.Errorf("a signed-in user is required to submit a request")
	}
	return transition(ctx, id, string(ReqSubmitted), "submit", "")
}

func actRequestReview(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	if err := requireOfficerErr(ctx, "triage a portal request"); err != nil {
		return "", err
	}
	return transition(ctx, id, string(ReqUnderReview), "review", "")
}

func actRequestApprove(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	if err := requireOfficerErr(ctx, "approve a portal request"); err != nil {
		return "", err
	}
	return transition(ctx, id, string(ReqApproved), "approve", args["note"])
}

func actRequestReject(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	if err := requireOfficerErr(ctx, "reject a portal request"); err != nil {
		return "", err
	}
	reason := args["reject_reason"]
	if reason == "" {
		reason = args["note"]
	}
	if reason == "" {
		return "", fmt.Errorf("a rejection reason is required")
	}
	return transition(ctx, id, string(ReqRejected), "reject", reason)
}

func actRequestFulfil(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	if err := requireOfficerErr(ctx, "fulfil a portal request"); err != nil {
		return "", err
	}
	return transition(ctx, id, string(ReqFulfilled), "fulfil", "")
}

func actRequestCancel(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	if orm.SecurityUID(ctx) <= 0 {
		return "", fmt.Errorf("a signed-in user is required to cancel a request")
	}
	return transition(ctx, id, string(ReqCancelled), "cancel", args["note"])
}

// actProfileDeactivate revokes a portal profile's access. The row is retained for the
// audit trail; only the active flag is cleared.
func actProfileDeactivate(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	if err := requireOfficerErr(ctx, "deactivate a portal profile"); err != nil {
		return "", err
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, profileModel, id)
	if err != nil {
		return "", err
	}
	if !boolOr(rec["active"]) {
		return "", fmt.Errorf("this portal profile is already inactive")
	}
	if err := orm.UpdateRecordByID(bypass, profileModel, id, map[string]interface{}{"active": false}); err != nil {
		return "", err
	}
	return "Portal access deactivated", nil
}

// ---------------------------------------------------------------------------
// Shared action helpers
// ---------------------------------------------------------------------------

func writeHistory(bypass context.Context, requestID int, from, to, action string, actor int, note string) {
	_, _ = orm.Create(bypass, orm.RegistryModel(requestHistoryModel), map[string]interface{}{
		"request_id": requestID,
		"from_state": from,
		"to_state":   to,
		"action":     action,
		"actor_id":   actor,
		"note":       note,
		"timestamp":  nowStamp(),
	})
}

// logAccess mirrors a request touch into the immutable audit ledger using the request's
// owning profile and learner. Financial is not asserted by request actions themselves.
func logAccess(bypass context.Context, rec map[string]interface{}) {
	_, _ = orm.Create(bypass, orm.RegistryModel(accessLogModel), map[string]interface{}{
		"profile_id":   intOr(rec["profile_id"]),
		"student_id":   intOr(rec["student_id"]),
		"target_model": requestModel,
		"record_id":    intOr(rec["id"]),
		"action":       string(ActUpdate),
		"accessed_at":  nowStamp(),
	})
}

func getRecord(ctx context.Context, model string, id int) (map[string]interface{}, error) {
	rec, err := orm.SearchOne(ctx, model, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return nil, fmt.Errorf("record not found")
	}
	return rec, nil
}

func requireOfficer(ctx context.Context, uid int) bool {
	return orm.UserHasGroupXML(ctx, uid, "school.group_school_clerk") ||
		orm.UserHasGroupXML(ctx, uid, "school.group_school_manager")
}

func requireOfficerErr(ctx context.Context, what string) error {
	uid := orm.SecurityUID(ctx)
	if uid <= 0 || !requireOfficer(ctx, uid) {
		return fmt.Errorf("insufficient permission to %s", what)
	}
	return nil
}

var stateLabels = map[string]string{
	"draft": "Draft", "submitted": "Submitted", "under_review": "Under Review",
	"approved": "Approved", "rejected": "Rejected", "fulfilled": "Fulfilled", "cancelled": "Cancelled",
}

func label(state string) string {
	if l, ok := stateLabels[state]; ok {
		return l
	}
	return state
}
