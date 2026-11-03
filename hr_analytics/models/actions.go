package models

import (
	"context"
	"fmt"

	"nexa/core/orm"
)

// HR-F11 actions. This module is read-oriented: the only writes are (a) rebuilding
// the derived department snapshots, which never mutates a source record, and
// (b) an explicit HR-Manager approval flag on a position plan (headcount / budget
// governance). Neither ever posts money, edits a payslip, or changes an employee.

func init() {
	orm.RegisterObjectAction(snapshotModel, "action_refresh_snapshot", snapshotRefresh)
	orm.RegisterObjectAction(positionPlanMod, "action_plan_approve", planApprove)
	orm.RegisterObjectAction(positionPlanMod, "action_plan_unapprove", planUnapprove)
}

// snapshotRefresh re-derives every department snapshot for the current period.
// Gated to an HR Officer / Manager; it only recomputes this module's own rows.
func snapshotRefresh(ctx context.Context, _ string, _ int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("only an HR Officer or Manager can refresh workforce snapshots")
	}
	if err := runWorkforceSnapshot(ctx, nil); err != nil {
		return "", err
	}
	return "Workforce snapshots refreshed", nil
}

// planApprove records an HR-Manager approval of a position plan. It is a decision
// flag only; the plan's budget never flows into payroll from here.
func planApprove(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireManager(ctx, uid) {
		return "", fmt.Errorf("only an HR Manager can approve a position plan")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, positionPlanMod, id)
	if err != nil {
		return "", err
	}
	if orm.AsBool(rec["approved"]) {
		return "", fmt.Errorf("this position plan is already approved")
	}
	return "", orm.UpdateRecordByID(bypass, positionPlanMod, id, map[string]interface{}{
		"approved":    true,
		"approver_id": uid,
	})
}

func planUnapprove(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireManager(ctx, uid) {
		return "", fmt.Errorf("only an HR Manager can revoke a position plan approval")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	if _, err := getRecord(bypass, positionPlanMod, id); err != nil {
		return "", err
	}
	return "", orm.UpdateRecordByID(bypass, positionPlanMod, id, map[string]interface{}{
		"approved":    false,
		"approver_id": 0,
	})
}

// ---------------------------------------------------------------------------
// Permission / record helpers (self-contained to this module)
// ---------------------------------------------------------------------------

func requireOfficer(ctx context.Context, uid int) bool {
	return orm.UserHasGroupXML(ctx, uid, "hr.group_hr_user") ||
		orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager")
}

func requireManager(ctx context.Context, uid int) bool {
	return orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager")
}

func getRecord(ctx context.Context, model string, id int) (map[string]interface{}, error) {
	rec, err := orm.SearchOne(ctx, model, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return nil, fmt.Errorf("record not found")
	}
	return rec, nil
}
