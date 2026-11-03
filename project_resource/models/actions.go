package models

import (
	"context"
	"fmt"

	"nexa/core/orm"
)

// Resource actions (blueprint §8.3, §20). Confirming an allocation refreshes the
// employee's over-allocation flag immediately; releasing frees capacity. A
// resource request only becomes a confirmed allocation through an approval, and
// the approved request writes the allocation record it stood for — the request is
// a controlled demand, not a silent assignment.

func init() {
	orm.RegisterObjectAction(allocModel, "action_confirm", allocConfirm)
	orm.RegisterObjectAction(allocModel, "action_release", allocRelease)
	orm.RegisterObjectAction(allocModel, "action_check", allocCheck)

	orm.RegisterObjectAction(reqModel, "action_submit", reqSubmit)
	orm.RegisterObjectAction(reqModel, "action_approve", reqApprove)
	orm.RegisterObjectAction(reqModel, "action_reject", reqReject)
}

func allocConfirm(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, allocModel, id)
	if rec == nil {
		return "", fmt.Errorf("allocation not found")
	}
	if s(rec["state"]) != "draft" {
		return "", fmt.Errorf("only draft allocations can be confirmed")
	}
	if err := orm.UpdateRecordByID(bypass, allocModel, id, map[string]interface{}{"state": "confirmed"}); err != nil {
		return "", err
	}
	committed := userCommittedPct(bypass, idOf(rec["user_id"]))
	over := isOverAllocated(committed)
	_ = orm.UpdateRecordByID(bypass, allocModel, id, map[string]interface{}{"overalloc": over})
	if over {
		return fmt.Sprintf("Allocation confirmed; employee now at %.0f%% (over-allocated).", committed), nil
	}
	return fmt.Sprintf("Allocation confirmed; employee at %.0f%%.", committed), nil
}

func allocRelease(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, allocModel, id)
	if rec == nil {
		return "", fmt.Errorf("allocation not found")
	}
	if s(rec["state"]) != "confirmed" {
		return "", fmt.Errorf("only confirmed allocations can be released")
	}
	uid := idOf(rec["user_id"])
	if err := orm.UpdateRecordByID(bypass, allocModel, id, map[string]interface{}{
		"state": "released", "overalloc": false,
	}); err != nil {
		return "", err
	}
	// Refresh remaining allocations' flags for this employee.
	rows, _ := orm.SearchPage(bypass, allocModel, [][]interface{}{
		{"user_id", "=", uid}, {"state", "=", "confirmed"},
	}, 5000, 0, "id asc")
	over := isOverAllocated(userCommittedPct(bypass, uid))
	for _, r := range rows {
		_ = orm.UpdateRecordByID(bypass, allocModel, idOf(r["id"]), map[string]interface{}{"overalloc": over})
	}
	return "Allocation released and capacity freed.", nil
}

// allocCheck recomputes and reports the employee's committed capacity.
func allocCheck(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, allocModel, id)
	if rec == nil {
		return "", fmt.Errorf("allocation not found")
	}
	uid := idOf(rec["user_id"])
	committed := userCommittedPct(bypass, uid)
	over := isOverAllocated(committed)
	rows, _ := orm.SearchPage(bypass, allocModel, [][]interface{}{
		{"user_id", "=", uid}, {"state", "=", "confirmed"},
	}, 5000, 0, "id asc")
	for _, r := range rows {
		_ = orm.UpdateRecordByID(bypass, allocModel, idOf(r["id"]), map[string]interface{}{"overalloc": over})
	}
	if over {
		return fmt.Sprintf("Employee is over-allocated at %.0f%%; review assignments.", committed), nil
	}
	return fmt.Sprintf("Employee at %.0f%% of capacity.", committed), nil
}

func reqSubmit(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, reqModel, id)
	if rec == nil {
		return "", fmt.Errorf("resource request not found")
	}
	if s(rec["state"]) != "draft" {
		return "", fmt.Errorf("only draft requests can be submitted")
	}
	uid := orm.UIDFromContext(ctx)
	upd := map[string]interface{}{"state": "submitted"}
	// Capture the raiser so approval can enforce independence.
	if uid > 0 && idOf(rec["requested_by"]) <= 0 {
		upd["requested_by"] = uid
	}
	if err := orm.UpdateRecordByID(bypass, reqModel, id, upd); err != nil {
		return "", err
	}
	return "Resource request submitted for approval.", nil
}

// reqApprove enforces independent approval, records the approver and materialises
// the request into a confirmed allocation when a specific employee is named, then
// refreshes that employee's over-allocation flag. A role-only request (no named
// employee) is simply approved and awaits a later assignment.
func reqApprove(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, reqModel, id)
	if rec == nil {
		return "", fmt.Errorf("resource request not found")
	}
	if s(rec["state"]) != "submitted" {
		return "", fmt.Errorf("only submitted requests can be approved")
	}
	uid := orm.UIDFromContext(ctx)
	requester := idOf(rec["requested_by"])
	if uid > 0 && requester > 0 && uid == requester {
		return "", fmt.Errorf("a resource request must be approved by someone other than its raiser")
	}
	upd := map[string]interface{}{"state": "approved"}
	if uid > 0 {
		upd["approved_by"] = uid
	}
	if err := orm.UpdateRecordByID(bypass, reqModel, id, upd); err != nil {
		return "", err
	}
	emp := idOf(rec["user_id"])
	if emp <= 0 {
		return "Request approved; assign an employee to create the allocation.", nil
	}
	inst, ok := orm.Registry[allocModel]
	if !ok {
		return "", fmt.Errorf("allocation model unavailable")
	}
	if _, err := orm.Create(bypass, inst, map[string]interface{}{
		"project_id": idOf(rec["project_id"]),
		"user_id":    emp,
		"role":       s(rec["role"]),
		"alloc_pct":  flt(rec["needed_pct"]),
		"start_date": s(rec["start_date"]),
		"end_date":   s(rec["end_date"]),
		"state":      "confirmed",
		"note":       "Created from approved resource request.",
	}); err != nil {
		return "", err
	}
	over := isOverAllocated(userCommittedPct(bypass, emp))
	rows, _ := orm.SearchPage(bypass, allocModel, [][]interface{}{
		{"user_id", "=", emp}, {"state", "=", "confirmed"},
	}, 5000, 0, "id asc")
	for _, r := range rows {
		_ = orm.UpdateRecordByID(bypass, allocModel, idOf(r["id"]), map[string]interface{}{"overalloc": over})
	}
	if over {
		return "Request approved; confirmed allocation created (employee now over-allocated).", nil
	}
	return "Request approved; confirmed allocation created.", nil
}

func reqReject(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	return reqSet(ctx, id, []string{"submitted"}, "rejected", "Resource request rejected.")
}

func reqSet(ctx context.Context, id int, from []string, to, msg string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, reqModel, id)
	if rec == nil {
		return "", fmt.Errorf("resource request not found")
	}
	cur := s(rec["state"])
	allowed := false
	for _, f := range from {
		if cur == f {
			allowed = true
			break
		}
	}
	if !allowed {
		return "", fmt.Errorf("transition to %s not allowed from %s", to, cur)
	}
	if err := orm.UpdateRecordByID(bypass, reqModel, id, map[string]interface{}{"state": to}); err != nil {
		return "", err
	}
	return msg, nil
}
