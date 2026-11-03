package models

import (
	"context"
	"fmt"

	"nexa/core/orm"
)

// Operator-facing actions for the catalog companion: catalog-item publication
// lifecycle and the governed service-request workflow (submit → approval →
// fulfilment → completion) plus fulfilment-task progression. Every state change is
// guarded by the pure transition rules in engine.go, and consequential request
// moves stamp timestamps. Actions mirror the anchor's workflow.go style.

func init() {
	// Catalog item publication.
	orm.RegisterObjectAction("helpdesk.catalog.item", "action_submit_review", itemSubmitReview)
	orm.RegisterObjectAction("helpdesk.catalog.item", "action_publish", itemPublish)
	orm.RegisterObjectAction("helpdesk.catalog.item", "action_archive", itemArchive)

	// Service request workflow.
	orm.RegisterObjectAction("helpdesk.service.request", "action_submit", requestSubmit)
	orm.RegisterObjectAction("helpdesk.service.request", "action_approve", requestApprove)
	orm.RegisterObjectAction("helpdesk.service.request", "action_reject", requestReject)
	orm.RegisterObjectAction("helpdesk.service.request", "action_complete", requestComplete)
	orm.RegisterObjectAction("helpdesk.service.request", "action_cancel", requestCancel)

	// Fulfilment task progression.
	orm.RegisterObjectAction("helpdesk.fulfillment.task", "action_task_start", taskStart)
	orm.RegisterObjectAction("helpdesk.fulfillment.task", "action_task_done", taskDone)
	orm.RegisterObjectAction("helpdesk.fulfillment.task", "action_task_skip", taskSkip)
}

// ---------------------------------------------------------------------------
// Catalog item lifecycle
// ---------------------------------------------------------------------------

func itemTransition(model string, id int, from []ItemState, to ItemState) func(context.Context, string, int, map[string]string) (string, error) {
	return func(ctx context.Context, _ string, _ int, _ map[string]string) (string, error) {
		bypass := orm.ContextWithBypass(ctx, true)
		rec, err := orm.SearchOne(bypass, model, map[string]interface{}{"id": id})
		if err != nil || rec == nil {
			return "", fmt.Errorf("catalog item not found")
		}
		current := ItemState(str(rec["state"]))
		allowed := false
		for _, s := range from {
			if current == s {
				allowed = true
				break
			}
		}
		if !allowed {
			return "", fmt.Errorf("cannot move item from %q to %q", current, to)
		}
		if err := orm.UpdateRecordByID(bypass, model, id, map[string]interface{}{"state": string(to)}); err != nil {
			return "", err
		}
		return "", nil
	}
}

func itemSubmitReview(ctx context.Context, model string, id int, _ map[string]string) (string, error) {
	return itemTransition(model, id, []ItemState{ItemDraft}, ItemInReview)(ctx, model, id, nil)
}

func itemPublish(ctx context.Context, model string, id int, _ map[string]string) (string, error) {
	return itemTransition(model, id, []ItemState{ItemDraft, ItemInReview, ItemArchived}, ItemPublished)(ctx, model, id, nil)
}

func itemArchive(ctx context.Context, model string, id int, _ map[string]string) (string, error) {
	return itemTransition(model, id, []ItemState{ItemPublished, ItemInReview, ItemDraft}, ItemArchived)(ctx, model, id, nil)
}

// ---------------------------------------------------------------------------
// Service request workflow
// ---------------------------------------------------------------------------

func requestSubmit(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, "helpdesk.service.request", map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("service request not found")
	}
	current := RequestState(str(rec["state"]))
	approvalRequired := itemApprovalRequired(bypass, str(rec["item_id"]))
	to, ok := requestAllowedTransition(current, "submit", approvalRequired)
	if !ok {
		return "", fmt.Errorf("cannot submit a request in state %q", current)
	}
	upd := map[string]interface{}{"state": string(to), "submitted_at": nowStamp()}
	if err := orm.UpdateRecordByID(bypass, "helpdesk.service.request", id, upd); err != nil {
		return "", err
	}
	if to == ReqInApproval {
		ensurePendingApproval(bypass, id, str(rec["item_id"]))
		return "Request submitted for approval.", nil
	}
	return "Request submitted and routed to fulfilment.", nil
}

func requestApprove(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, "helpdesk.service.request", map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("service request not found")
	}
	current := RequestState(str(rec["state"]))
	if current != ReqInApproval {
		return "", fmt.Errorf("can only approve a request in approval, not %q", current)
	}
	states := approvalStatesFor(bypass, id)
	approved, rejected := approvalsResolved(states)
	if rejected {
		return "", fmt.Errorf("request has a rejected approval and cannot be approved")
	}
	if !approved {
		markApprovals(bypass, id, ApprovalApproved, uid)
	} else {
		return "", fmt.Errorf("all approvals are already resolved")
	}
	if err := orm.UpdateRecordByID(bypass, "helpdesk.service.request", id,
		map[string]interface{}{"state": string(ReqFulfilling)}); err != nil {
		return "", err
	}
	ensureDefaultTasks(bypass, id, str(rec["item_id"]))
	return "Request approved and opened for fulfilment.", nil
}

func requestReject(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, "helpdesk.service.request", map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("service request not found")
	}
	if RequestState(str(rec["state"])) != ReqInApproval {
		return "", fmt.Errorf("can only reject a request in approval")
	}
	note := args["note"]
	if note == "" {
		return "", fmt.Errorf("a rejection reason is required")
	}
	markApprovals(bypass, id, ApprovalRejected, uid)
	_ = orm.UpdateRecordByID(bypass, "helpdesk.service.request", id,
		map[string]interface{}{"state": string(ReqRejected)})
	return "Request rejected.", nil
}

func requestComplete(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, "helpdesk.service.request", map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("service request not found")
	}
	if RequestState(str(rec["state"])) != ReqFulfilling {
		return "", fmt.Errorf("can only complete a request that is fulfilling")
	}
	tasks := fetchTasks(bypass, id)
	pairs := make([][2]string, 0, len(tasks))
	for _, t := range tasks {
		pairs = append(pairs, [2]string{str(t["required"]), str(t["state"])})
	}
	if !requiredTasksSatisfied(pairs) {
		return "", fmt.Errorf("required fulfilment tasks are not complete")
	}
	if err := orm.UpdateRecordByID(bypass, "helpdesk.service.request", id,
		map[string]interface{}{"state": string(ReqCompleted), "completed_at": nowStamp()}); err != nil {
		return "", err
	}
	return "Request completed.", nil
}

func requestCancel(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, "helpdesk.service.request", map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("service request not found")
	}
	current := RequestState(str(rec["state"]))
	approvalRequired := itemApprovalRequired(bypass, str(rec["item_id"]))
	to, ok := requestAllowedTransition(current, "cancel", approvalRequired)
	if !ok {
		return "", fmt.Errorf("cannot cancel a request in state %q", current)
	}
	if err := orm.UpdateRecordByID(bypass, "helpdesk.service.request", id,
		map[string]interface{}{"state": string(to)}); err != nil {
		return "", err
	}
	return "Request cancelled.", nil
}

// ---------------------------------------------------------------------------
// Fulfilment task progression
// ---------------------------------------------------------------------------

func taskStart(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	t, err := orm.SearchOne(bypass, "helpdesk.fulfillment.task", map[string]interface{}{"id": id})
	if err != nil || t == nil {
		return "", fmt.Errorf("task not found")
	}
	depID := atoi(str(t["depends_on_id"]))
	depState := ""
	if depID > 0 {
		if dep, e := orm.SearchOne(bypass, "helpdesk.fulfillment.task", map[string]interface{}{"id": depID}); e == nil && dep != nil {
			depState = str(dep["state"])
		}
	}
	if !taskIsReady(str(t["state"]), depState, depID > 0) {
		return "", fmt.Errorf("task is not ready (dependency not satisfied or already started)")
	}
	if err := orm.UpdateRecordByID(bypass, "helpdesk.fulfillment.task", id,
		map[string]interface{}{"state": string(TaskInProgress)}); err != nil {
		return "", err
	}
	return "", nil
}

func taskDone(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	return taskClose(ctx, id, TaskDone)
}

func taskSkip(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	return taskClose(ctx, id, TaskSkipped)
}

func taskClose(ctx context.Context, id int, to TaskState) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	t, err := orm.SearchOne(bypass, "helpdesk.fulfillment.task", map[string]interface{}{"id": id})
	if err != nil || t == nil {
		return "", fmt.Errorf("task not found")
	}
	cur := TaskState(str(t["state"]))
	if cur == TaskDone || cur == TaskSkipped {
		return "", fmt.Errorf("task is already closed")
	}
	upd := map[string]interface{}{"state": string(to)}
	if to == TaskDone {
		upd["completed_at"] = nowStamp()
	}
	if err := orm.UpdateRecordByID(bypass, "helpdesk.fulfillment.task", id, upd); err != nil {
		return "", err
	}
	return "", nil
}

// ---------------------------------------------------------------------------
// Request/approval/task helpers
// ---------------------------------------------------------------------------

func itemApprovalRequired(ctx context.Context, itemID string) bool {
	iid := atoi(itemID)
	if iid <= 0 {
		return false
	}
	item, err := orm.SearchOne(ctx, "helpdesk.catalog.item", map[string]interface{}{"id": iid})
	if err != nil || item == nil {
		return false
	}
	return orm.AsBool(item["approval_required"])
}

func approvalStatesFor(ctx context.Context, requestID int) []string {
	rows := fetchAll(ctx, "helpdesk.request.approval",
		[][]interface{}{{"request_id", "=", requestID}}, "sequence ASC, id ASC")
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, str(r["state"]))
	}
	return out
}

func ensurePendingApproval(ctx context.Context, requestID int, itemID string) {
	existing := fetchAll(ctx, "helpdesk.request.approval",
		[][]interface{}{{"request_id", "=", requestID}}, "id ASC")
	if len(existing) > 0 {
		return
	}
	approver := 0
	if iid := atoi(itemID); iid > 0 {
		if item, err := orm.SearchOne(ctx, "helpdesk.catalog.item", map[string]interface{}{"id": iid}); err == nil && item != nil {
			approver = atoi(str(item["approver_user_id"]))
		}
	}
	inst, ok := orm.Registry["helpdesk.request.approval"]
	if !ok {
		return
	}
	vals := map[string]interface{}{"request_id": requestID, "level": 1, "state": string(ApprovalPending)}
	if approver > 0 {
		vals["approver_id"] = approver
	}
	_, _ = orm.Create(ctx, inst, vals)
}

func markApprovals(ctx context.Context, requestID int, to ApprovalState, actor int) {
	rows := fetchAll(ctx, "helpdesk.request.approval",
		[][]interface{}{{"request_id", "=", requestID}, {"state", "=", string(ApprovalPending)}}, "id ASC")
	for _, r := range rows {
		upd := map[string]interface{}{"state": string(to), "decided_at": nowStamp()}
		if actor > 0 {
			upd["decided_by"] = actor
		}
		_ = orm.UpdateRecordByID(ctx, "helpdesk.request.approval", atoi(str(r["id"])), upd)
	}
}

// ensureDefaultTasks seeds a single fulfilment task when a request is approved and
// the item defines none, so fulfilment always has something to progress.
func ensureDefaultTasks(ctx context.Context, requestID int, itemID string) {
	existing := fetchAll(ctx, "helpdesk.fulfillment.task",
		[][]interface{}{{"request_id", "=", requestID}}, "id ASC")
	if len(existing) > 0 {
		return
	}
	inst, ok := orm.Registry["helpdesk.fulfillment.task"]
	if !ok {
		return
	}
	name := "Fulfil request"
	team := 0
	if iid := atoi(itemID); iid > 0 {
		if item, err := orm.SearchOne(ctx, "helpdesk.catalog.item", map[string]interface{}{"id": iid}); err == nil && item != nil {
			if nm := str(item["name"]); nm != "" {
				name = "Fulfil: " + nm
			}
			team = atoi(str(item["team_id"]))
		}
	}
	vals := map[string]interface{}{"request_id": requestID, "name": name, "required": true, "state": string(TaskPending)}
	if team > 0 {
		vals["team_id"] = team
	}
	_, _ = orm.Create(ctx, inst, vals)
}
