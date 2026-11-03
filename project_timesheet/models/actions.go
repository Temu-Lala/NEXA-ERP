package models

import (
	"context"
	"fmt"
	"time"

	"nexa/core/orm"
)

// Timesheet actions (blueprint §9.2, §20). A sheet is submitted for review, then
// approved or rejected by an independent reviewer (a submitter cannot approve
// their own time). Approval derives the task's spent hours and locks the sheet;
// rejection or a controlled reopen recomputes them so task effort never drifts.

func init() {
	orm.RegisterObjectAction(tsModel, "action_compute", tsCompute)
	orm.RegisterObjectAction(tsModel, "action_submit", tsSubmit)
	orm.RegisterObjectAction(tsModel, "action_approve", tsApprove)
	orm.RegisterObjectAction(tsModel, "action_reject", tsReject)
	orm.RegisterObjectAction(tsModel, "action_reopen", tsReopen)
}

// tsCompute refreshes the sheet's total and billable hours from its lines.
func tsCompute(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, tsModel, id)
	if rec == nil {
		return "", fmt.Errorf("timesheet not found")
	}
	total, billable := totals(bypass, id)
	if err := orm.UpdateRecordByID(bypass, tsModel, id, map[string]interface{}{
		"total_hours": total, "billable_hours": billable,
	}); err != nil {
		return "", err
	}
	return fmt.Sprintf("Total %.2f h, billable %.2f h.", total, billable), nil
}

func tsSubmit(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, tsModel, id)
	if rec == nil {
		return "", fmt.Errorf("timesheet not found")
	}
	if s(rec["state"]) != "draft" {
		return "", fmt.Errorf("only draft timesheets can be submitted")
	}
	lines, _ := orm.SearchCount(bypass, tsLine, [][]interface{}{{"timesheet_id", "=", id}})
	if lines == 0 {
		return "", fmt.Errorf("a timesheet needs at least one entry before it can be submitted")
	}
	total, billable := totals(bypass, id)
	if err := orm.UpdateRecordByID(bypass, tsModel, id, map[string]interface{}{
		"state": "submitted", "total_hours": total, "billable_hours": billable,
	}); err != nil {
		return "", err
	}
	return "Timesheet submitted for approval.", nil
}

// tsApprove enforces independent approval, records the approver, locks the sheet
// (active=false) and re-derives the affected tasks' spent hours.
func tsApprove(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, tsModel, id)
	if rec == nil {
		return "", fmt.Errorf("timesheet not found")
	}
	if s(rec["state"]) != "submitted" {
		return "", fmt.Errorf("only submitted timesheets can be approved")
	}
	uid := orm.UIDFromContext(ctx)
	owner := idOf(rec["user_id"])
	if uid > 0 && owner > 0 && uid == owner {
		return "", fmt.Errorf("a timesheet must be approved by someone other than its owner")
	}
	total, billable := totals(bypass, id)
	upd := map[string]interface{}{
		"state": "approved", "total_hours": total, "billable_hours": billable,
		"approved_date": time.Now().Format(dateLayout), "active": false,
	}
	if uid > 0 {
		upd["approved_by"] = uid
	}
	if err := orm.UpdateRecordByID(bypass, tsModel, id, upd); err != nil {
		return "", err
	}
	refreshSheetTasks(bypass, id)
	return "Timesheet approved and locked.", nil
}

func tsReject(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, tsModel, id)
	if rec == nil {
		return "", fmt.Errorf("timesheet not found")
	}
	if s(rec["state"]) != "submitted" {
		return "", fmt.Errorf("only submitted timesheets can be rejected")
	}
	if err := orm.UpdateRecordByID(bypass, tsModel, id, map[string]interface{}{"state": "rejected"}); err != nil {
		return "", err
	}
	refreshSheetTasks(bypass, id)
	return "Timesheet rejected; returned for correction.", nil
}

// tsReopen unlocks an approved sheet back to draft through a controlled act, then
// re-derives task hours (now excluding this sheet).
func tsReopen(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, tsModel, id)
	if rec == nil {
		return "", fmt.Errorf("timesheet not found")
	}
	if s(rec["state"]) != "approved" {
		return "", fmt.Errorf("only approved timesheets can be reopened")
	}
	if err := orm.UpdateRecordByID(bypass, tsModel, id, map[string]interface{}{
		"state": "draft", "active": true,
	}); err != nil {
		return "", err
	}
	refreshSheetTasks(bypass, id)
	return "Timesheet reopened as draft.", nil
}

// refreshSheetTasks re-derives spent hours for every distinct task referenced by
// the sheet's lines.
func refreshSheetTasks(ctx context.Context, timesheetID int) {
	rows, err := orm.SearchPage(ctx, tsLine, [][]interface{}{
		{"timesheet_id", "=", timesheetID},
	}, 5000, 0, "id asc")
	if err != nil {
		return
	}
	seen := map[int]bool{}
	for _, r := range rows {
		if tid := idOf(r["task_id"]); tid > 0 && !seen[tid] {
			seen[tid] = true
			rollupTaskHours(ctx, tid)
		}
	}
}
