package models

import (
	"context"
	"fmt"
	"time"

	"nexa/core/orm"
)

// HR service-desk + self-service workflow actions. Case state transitions are
// officer-gated; a self-service request is a maker-checker approval (the person
// who raised it can never approve it) and approving records only a decision - it
// never posts a cost, creates a leave / attendance record or mutates pay. The SLA
// window is snapshotted from the category when a case is first assigned so later
// category edits never rewrite a running clock, and every meaningful change is
// mirrored into the append-only case history.

func init() {
	orm.RegisterObjectAction(caseModel, "action_case_assign", caseAssign)
	orm.RegisterObjectAction(caseModel, "action_case_start", caseStart)
	orm.RegisterObjectAction(caseModel, "action_case_pending", casePending)
	orm.RegisterObjectAction(caseModel, "action_case_resolve", caseResolve)
	orm.RegisterObjectAction(caseModel, "action_case_close", caseClose)
	orm.RegisterObjectAction(caseModel, "action_case_reopen", caseReopen)
	orm.RegisterObjectAction(caseModel, "action_case_cancel", caseCancel)

	orm.RegisterObjectAction(requestModel, "action_request_submit", requestSubmit)
	orm.RegisterObjectAction(requestModel, "action_request_approve", requestApprove)
	orm.RegisterObjectAction(requestModel, "action_request_reject", requestReject)
	orm.RegisterObjectAction(requestModel, "action_request_cancel", requestCancel)
}

// ---------------------------------------------------------------------------
// Case workflow
// ---------------------------------------------------------------------------

func caseAssign(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("only an HR Officer or Manager can assign a case")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, caseModel, id)
	if err != nil {
		return "", err
	}
	if CaseState(orm.AsString(rec["state"])) != CaseNew {
		return "", fmt.Errorf("only a new case can be assigned")
	}
	vals := map[string]interface{}{
		"state":       string(CaseAssigned),
		"assigned_to": uid,
	}
	if intOr(rec["team_id"]) <= 0 {
		if team := categoryTeam(bypass, intOr(rec["category_id"])); team > 0 {
			vals["team_id"] = team
		}
	}
	// Snapshot the SLA window from the category the first time it is assigned.
	if resp, res, ok := categoryWindows(bypass, intOr(rec["category_id"])); ok {
		if orm.AsString(rec["response_due"]) == "" {
			vals["response_due"] = time.Now().UTC().Add(time.Duration(resp) * time.Hour).Format(datetimeFull)
		}
		if orm.AsString(rec["resolution_due"]) == "" {
			vals["resolution_due"] = time.Now().UTC().Add(time.Duration(res) * time.Hour).Format(datetimeFull)
		}
	}
	if err := orm.UpdateRecordByID(bypass, caseModel, id, vals); err != nil {
		return "", err
	}
	writeHistory(bypass, id, HistAssignment, "assigned_to", "", "self", uid)
	return "", nil
}

func caseStart(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("only an HR Officer or Manager can start a case")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, caseModel, id)
	if err != nil {
		return "", err
	}
	st := CaseState(orm.AsString(rec["state"]))
	if st != CaseAssigned && st != CasePending {
		return "", fmt.Errorf("only an assigned or pending case can be started")
	}
	if err := orm.UpdateRecordByID(bypass, caseModel, id, map[string]interface{}{"state": string(CaseInProgress)}); err != nil {
		return "", err
	}
	writeHistory(bypass, id, HistState, "state", string(st), string(CaseInProgress), uid)
	return "", nil
}

func casePending(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("only an HR Officer or Manager can hold a case")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, caseModel, id)
	if err != nil {
		return "", err
	}
	if CaseState(orm.AsString(rec["state"])) != CaseInProgress {
		return "", fmt.Errorf("only an in-progress case can be held pending")
	}
	if err := orm.UpdateRecordByID(bypass, caseModel, id, map[string]interface{}{"state": string(CasePending)}); err != nil {
		return "", err
	}
	writeHistory(bypass, id, HistState, "state", string(CaseInProgress), string(CasePending), uid)
	return "", nil
}

func caseResolve(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("only an HR Officer or Manager can resolve a case")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, caseModel, id)
	if err != nil {
		return "", err
	}
	st := CaseState(orm.AsString(rec["state"]))
	if st != CaseAssigned && st != CaseInProgress && st != CasePending {
		return "", fmt.Errorf("the case is not in a state that can be resolved")
	}
	if orm.AsString(rec["resolution"]) == "" {
		return "", fmt.Errorf("record a resolution before resolving the case")
	}
	if err := orm.UpdateRecordByID(bypass, caseModel, id, map[string]interface{}{"state": string(CaseResolved)}); err != nil {
		return "", err
	}
	writeHistory(bypass, id, HistResolve, "state", string(st), string(CaseResolved), uid)
	return "", nil
}

func caseClose(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("only an HR Officer or Manager can close a case")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, caseModel, id)
	if err != nil {
		return "", err
	}
	if CaseState(orm.AsString(rec["state"])) != CaseResolved {
		return "", fmt.Errorf("only a resolved case can be closed")
	}
	vals := map[string]interface{}{
		"state":       string(CaseClosed),
		"closed_date": time.Now().UTC().Format(datetimeFull),
	}
	if err := orm.UpdateRecordByID(bypass, caseModel, id, vals); err != nil {
		return "", err
	}
	writeHistory(bypass, id, HistState, "state", string(CaseResolved), string(CaseClosed), uid)
	return "", nil
}

func caseReopen(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("only an HR Officer or Manager can reopen a case")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, caseModel, id)
	if err != nil {
		return "", err
	}
	st := CaseState(orm.AsString(rec["state"]))
	if st != CaseResolved && st != CaseClosed {
		return "", fmt.Errorf("only a resolved or closed case can be reopened")
	}
	vals := map[string]interface{}{
		"state":    string(CaseInProgress),
		"breached": false,
	}
	if err := orm.UpdateRecordByID(bypass, caseModel, id, vals); err != nil {
		return "", err
	}
	writeHistory(bypass, id, HistState, "state", string(st), string(CaseInProgress), uid)
	return "", nil
}

func caseCancel(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, caseModel, id)
	if err != nil {
		return "", err
	}
	st := CaseState(orm.AsString(rec["state"]))
	if st == CaseClosed || st == CaseCancelled {
		return "", fmt.Errorf("the case is already finished")
	}
	// An officer, or the case's own employee, may cancel it.
	if !requireOfficer(ctx, uid) && intOr(rec["employee_user_id"]) != uid {
		return "", fmt.Errorf("only an officer or the requesting employee can cancel this case")
	}
	if err := orm.UpdateRecordByID(bypass, caseModel, id, map[string]interface{}{"state": string(CaseCancelled)}); err != nil {
		return "", err
	}
	writeHistory(bypass, id, HistState, "state", string(st), string(CaseCancelled), uid)
	return "", nil
}

// ---------------------------------------------------------------------------
// Self-service requests (approval inbox)
// ---------------------------------------------------------------------------

func requestSubmit(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, requestModel, id)
	if err != nil {
		return "", err
	}
	if RequestState(orm.AsString(rec["state"])) != ReqDraft {
		return "", fmt.Errorf("only a draft request can be submitted")
	}
	employeeID := intOr(rec["employee_id"])
	if employeeID <= 0 {
		return "", fmt.Errorf("the request has no employee")
	}
	// An employee may only submit their own request.
	if myUserEmployeeID(ctx, uid) != 0 && employeeUser(bypass, employeeID) != uid && !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("you can only submit a request for yourself")
	}
	approver := managerEmployeeID(bypass, employeeID)
	if approver <= 0 {
		return "", fmt.Errorf("no reporting manager is configured to approve this request")
	}
	vals := map[string]interface{}{
		"state":        string(ReqSubmitted),
		"requested_by": uid,
		"approver_id":  approver,
	}
	return "", orm.UpdateRecordByID(bypass, requestModel, id, vals)
}

func requestApprove(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	return decideRequest(ctx, id, ReqApproved)
}

func requestReject(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	return decideRequest(ctx, id, ReqRejected)
}

func decideRequest(ctx context.Context, id int, to RequestState) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("only an HR Officer or Manager can decide a request")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, requestModel, id)
	if err != nil {
		return "", err
	}
	if RequestState(orm.AsString(rec["state"])) != ReqSubmitted {
		return "", fmt.Errorf("only a submitted request can be decided")
	}
	if requester := intOr(rec["requested_by"]); requester != 0 && requester == uid {
		return "", fmt.Errorf("maker-checker: the requester cannot decide their own request")
	}
	// Approving records a decision only - it never creates leave / attendance or
	// posts a cost.
	vals := map[string]interface{}{
		"state":       string(to),
		"decide_date": time.Now().UTC().Format(dateLayout),
	}
	return "", orm.UpdateRecordByID(bypass, requestModel, id, vals)
}

func requestCancel(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, requestModel, id)
	if err != nil {
		return "", err
	}
	st := RequestState(orm.AsString(rec["state"]))
	if st == ReqApproved || st == ReqRejected || st == ReqCancelled {
		return "", fmt.Errorf("the request is already decided")
	}
	if !requireOfficer(ctx, uid) && intOr(rec["employee_user_id"]) != uid && intOr(rec["requested_by"]) != uid {
		return "", fmt.Errorf("only the requester or an officer can cancel this request")
	}
	return "", orm.UpdateRecordByID(bypass, requestModel, id, map[string]interface{}{"state": string(ReqCancelled)})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func requireOfficer(ctx context.Context, uid int) bool {
	return orm.UserHasGroupXML(ctx, uid, "hr.group_hr_user") ||
		orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager")
}

func getRecord(ctx context.Context, model string, id int) (map[string]interface{}, error) {
	rec, err := orm.SearchOne(ctx, model, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return nil, fmt.Errorf("record not found")
	}
	return rec, nil
}

// categoryWindows returns the response / resolution hour targets for a category.
func categoryWindows(ctx context.Context, categoryID int) (resp, res int, ok bool) {
	if categoryID <= 0 {
		return 0, 0, false
	}
	cat, err := orm.SearchOne(ctx, categoryModel, map[string]interface{}{"id": categoryID})
	if err != nil || cat == nil {
		return 0, 0, false
	}
	return intOr(cat["response_hours"]), intOr(cat["resolution_hours"]), true
}

func categoryTeam(ctx context.Context, categoryID int) int {
	if categoryID <= 0 {
		return 0
	}
	cat, err := orm.SearchOne(ctx, categoryModel, map[string]interface{}{"id": categoryID})
	if err != nil || cat == nil {
		return 0
	}
	return intOr(cat["default_team_id"])
}

// managerEmployeeID resolves the reporting manager (parent_id) of an employee.
func managerEmployeeID(ctx context.Context, employeeID int) int {
	if employeeID <= 0 {
		return 0
	}
	emp, err := orm.SearchOne(ctx, employeeMod, map[string]interface{}{"id": employeeID})
	if err != nil || emp == nil {
		return 0
	}
	return intOr(emp["parent_id"])
}

// myUserEmployeeID finds the employee whose related user is uid (0 if none).
func myUserEmployeeID(ctx context.Context, uid int) int {
	if uid <= 0 {
		return 0
	}
	bypass := orm.ContextWithBypass(ctx, true)
	emp, err := orm.SearchOne(bypass, employeeMod, map[string]interface{}{"user_id": uid})
	if err != nil || emp == nil {
		return 0
	}
	return intOr(emp["id"])
}

// writeHistory appends one immutable audit row to a case's history.
func writeHistory(ctx context.Context, caseID int, action CaseHistoryAction, field, oldVal, newVal string, byUID int) {
	vals := map[string]interface{}{
		"case_id":      caseID,
		"action":       string(action),
		"field":        field,
		"old_value":    oldVal,
		"new_value":    newVal,
		"performed_at": time.Now().UTC().Format(datetimeFull),
	}
	if byUID > 0 {
		vals["performed_by"] = byUID
	}
	createRecord(ctx, caseHistModel, vals)
}

func createRecord(ctx context.Context, model string, vals map[string]interface{}) (int, error) {
	inst, ok := orm.Registry[model]
	if !ok {
		return 0, fmt.Errorf("model %s is not registered", model)
	}
	return orm.Create(ctx, inst, vals)
}
