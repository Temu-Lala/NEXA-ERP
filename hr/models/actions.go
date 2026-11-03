package models

import (
	"context"
	"fmt"
	"time"

	"nexa/core/orm"
)

// Employee lifecycle workflow (HR & HCM SRS §2.2). Each button performs a
// single guarded transition: it checks write access, verifies the record is in
// an allowed "from" state, and then moves it forward. Approval and closing
// steps additionally require HR Manager membership. Invalid transitions return
// a clear message rather than silently changing data.
func init() {
	orm.RegisterObjectAction("hr.employee", "action_submit", employeeTransition("submit", false,
		[]EmployeeState{StateDraft}, StatePending, nil))
	orm.RegisterObjectAction("hr.employee", "action_approve", employeeTransition("approve", true,
		[]EmployeeState{StatePending}, StateActive, nil))
	orm.RegisterObjectAction("hr.employee", "action_set_on_leave", employeeTransition("go on leave", false,
		[]EmployeeState{StateActive}, StateOnLeave, nil))
	orm.RegisterObjectAction("hr.employee", "action_return_to_work", employeeTransition("return to work", false,
		[]EmployeeState{StateOnLeave}, StateActive, nil))
	orm.RegisterObjectAction("hr.employee", "action_suspend", employeeTransition("suspend", true,
		[]EmployeeState{StateActive, StateOnLeave}, StateSuspended, nil))
	orm.RegisterObjectAction("hr.employee", "action_reinstate", employeeTransition("reinstate", true,
		[]EmployeeState{StateSuspended}, StateActive, nil))
	orm.RegisterObjectAction("hr.employee", "action_to_notice", employeeTransition("move to notice", false,
		[]EmployeeState{StateActive, StateOnLeave, StateSuspended}, StateNotice, nil))
	orm.RegisterObjectAction("hr.employee", "action_terminate", employeeTransition("terminate", true,
		[]EmployeeState{StateNotice, StateActive, StateSuspended, StateOnLeave}, StateTerminated, func(updates map[string]interface{}) {
			if _, ok := updates["termination_date"]; !ok {
				updates["termination_date"] = time.Now().Format("2006-01-02")
			}
		}))
	orm.RegisterObjectAction("hr.employee", "action_retire", employeeTransition("retire", true,
		[]EmployeeState{StateNotice, StateActive}, StateRetired, func(updates map[string]interface{}) {
			if _, ok := updates["termination_date"]; !ok {
				updates["termination_date"] = time.Now().Format("2006-01-02")
			}
			updates["termination_reason"] = string(TerminationRetired)
		}))
	orm.RegisterObjectAction("hr.employee", "action_rehire", employeeTransition("rehire", true,
		[]EmployeeState{StateTerminated, StateRetired}, StateActive, func(updates map[string]interface{}) {
			updates["active"] = true
			updates["termination_date"] = nil
			updates["termination_reason"] = string(TerminationUnspecified)
		}))
}

// employeeTransition builds an object-action handler that moves an employee
// from one of the allowed states to a target state. When managerOnly is set the
// transition additionally requires HR Manager group membership.
func employeeTransition(verb string, managerOnly bool, from []EmployeeState, to EmployeeState, extra func(map[string]interface{})) func(context.Context, string, int, map[string]string) (string, error) {
	return func(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
		uid := orm.SecurityUID(ctx)
		if err := orm.CheckModelAccess(ctx, uid, "hr.employee", "write"); err != nil {
			return "", err
		}
		if managerOnly && !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
			return "", fmt.Errorf("only an HR Manager can %s an employee", verb)
		}
		bypass := orm.ContextWithBypass(ctx, true)
		emp, err := orm.SearchOne(bypass, "hr.employee", map[string]interface{}{"id": id})
		if err != nil {
			return "", fmt.Errorf("employee not found")
		}
		current := EmployeeState(orm.AsString(emp["state"]))
		if !stateAllowed(current, from) {
			return "", fmt.Errorf("cannot %s an employee in the %q stage", verb, stageName(current))
		}
		updates := map[string]interface{}{"state": string(to)}
		if extra != nil {
			extra(updates)
		}
		if err := orm.UpdateRecordByID(bypass, "hr.employee", id, updates); err != nil {
			return "", err
		}
		return "", nil
	}
}

func stateAllowed(current EmployeeState, allowed []EmployeeState) bool {
	for _, s := range allowed {
		if current == s {
			return true
		}
	}
	return false
}

// stageName returns a human-friendly stage name for the error message.
func stageName(s EmployeeState) string {
	switch s {
	case StateDraft:
		return "Draft"
	case StatePending:
		return "Pending Approval"
	case StateActive:
		return "Active"
	case StateOnLeave:
		return "On Leave"
	case StateSuspended:
		return "Suspended"
	case StateNotice:
		return "Notice Period"
	case StateTerminated:
		return "Terminated"
	case StateRetired:
		return "Retired"
	default:
		return "Unset"
	}
}
