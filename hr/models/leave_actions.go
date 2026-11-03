package models

import (
	"context"
	"fmt"

	"nexa/core/orm"
)

// Leave / allocation approval workflow. Both hr.leave and hr.leave.allocation
// share the LeaveState machine (draft -> confirm -> validate/refuse). Confirm is
// self-service; validate and refuse are guarded to HR Managers. Invalid
// transitions return a clear message instead of changing data.
func init() {
	for _, model := range []string{"hr.leave", "hr.leave.allocation"} {
		orm.RegisterObjectAction(model, "action_confirm", leaveTransition(model, "submit", false,
			[]LeaveState{LeaveDraft}, LeaveConfirm))
		orm.RegisterObjectAction(model, "action_validate", leaveTransition(model, "approve", true,
			[]LeaveState{LeaveConfirm}, LeaveValidate))
		orm.RegisterObjectAction(model, "action_refuse", leaveTransition(model, "refuse", true,
			[]LeaveState{LeaveDraft, LeaveConfirm}, LeaveRefuse))
		orm.RegisterObjectAction(model, "action_reset", leaveTransition(model, "reset to draft", false,
			[]LeaveState{LeaveConfirm, LeaveRefuse}, LeaveDraft))
	}
}

// leaveTransition builds an object-action handler that moves a leave/allocation
// record from one of the allowed states to a target state.
func leaveTransition(model, verb string, managerOnly bool, from []LeaveState, to LeaveState) func(context.Context, string, int, map[string]string) (string, error) {
	return func(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
		uid := orm.SecurityUID(ctx)
		if err := orm.CheckModelAccess(ctx, uid, model, "write"); err != nil {
			return "", err
		}
		if managerOnly && !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
			return "", fmt.Errorf("only an HR Manager can %s this request", verb)
		}
		bypass := orm.ContextWithBypass(ctx, true)
		rec, err := orm.SearchOne(bypass, model, map[string]interface{}{"id": id})
		if err != nil {
			return "", fmt.Errorf("record not found")
		}
		current := LeaveState(orm.AsString(rec["state"]))
		if !leaveStateAllowed(current, from) {
			return "", fmt.Errorf("cannot %s a request in the %q stage", verb, leaveStageName(current))
		}
		if err := orm.UpdateRecordByID(bypass, model, id, map[string]interface{}{"state": string(to)}); err != nil {
			return "", err
		}
		return "", nil
	}
}

func leaveStateAllowed(current LeaveState, allowed []LeaveState) bool {
	for _, s := range allowed {
		if current == s {
			return true
		}
	}
	return false
}

func leaveStageName(s LeaveState) string {
	switch s {
	case LeaveDraft:
		return "Draft"
	case LeaveConfirm:
		return "Waiting Approval"
	case LeaveValidate:
		return "Approved"
	case LeaveRefuse:
		return "Refused"
	default:
		return "Unset"
	}
}
