package models

import (
	"context"
	"fmt"

	"nexa/core/orm"
)

// Finance actions (blueprint §10.4, §20). A budget's total is computed from its
// lines; confirmation locks that derived total and approval projects it onto the
// project's approved budget as a controlled act. Costs roll up into the project's
// actual and committed measures whenever they are posted or refreshed, so the
// portfolio dashboard reads reconciled figures. No accounting posting happens
// here — Finance only mirrors source-linked snapshots onto the project.

func init() {
	orm.RegisterObjectAction(budgetModel, "action_compute", budgetCompute)
	orm.RegisterObjectAction(budgetModel, "action_confirm", budgetConfirm)
	orm.RegisterObjectAction(budgetModel, "action_approve", budgetApprove)

	orm.RegisterObjectAction(costModel, "action_post", costPost)
	orm.RegisterObjectAction(costModel, "action_refresh", costRefresh)
}

// budgetCompute refreshes total_amount from the sum of the budget's lines.
func budgetCompute(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, budgetModel, id)
	if rec == nil {
		return "", fmt.Errorf("budget not found")
	}
	total := sumBudgetLines(bypass, id)
	if err := orm.UpdateRecordByID(bypass, budgetModel, id, map[string]interface{}{"total_amount": total}); err != nil {
		return "", err
	}
	return fmt.Sprintf("Budget total computed: %.2f.", total), nil
}

// budgetConfirm derives and locks the total on a draft budget.
func budgetConfirm(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, budgetModel, id)
	if rec == nil {
		return "", fmt.Errorf("budget not found")
	}
	if s(rec["state"]) != "draft" {
		return "", fmt.Errorf("only draft budgets can be confirmed")
	}
	total := sumBudgetLines(bypass, id)
	if total <= 0 {
		return "", fmt.Errorf("a budget needs at least one positive line before it can be confirmed")
	}
	if err := orm.UpdateRecordByID(bypass, budgetModel, id, map[string]interface{}{
		"state": "confirmed", "total_amount": total,
	}); err != nil {
		return "", err
	}
	return fmt.Sprintf("Budget confirmed at %.2f.", total), nil
}

// budgetApprove promotes a confirmed budget and projects its total onto the
// project's approved budget.
func budgetApprove(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, budgetModel, id)
	if rec == nil {
		return "", fmt.Errorf("budget not found")
	}
	if s(rec["state"]) != "confirmed" {
		return "", fmt.Errorf("only confirmed budgets can be approved")
	}
	if err := orm.UpdateRecordByID(bypass, budgetModel, id, map[string]interface{}{"state": "approved"}); err != nil {
		return "", err
	}
	total := sumBudgetLines(bypass, id)
	if pid := idOf(rec["project_id"]); pid > 0 {
		_ = orm.UpdateRecordByID(bypass, projModel, pid, map[string]interface{}{"budget": total})
	}
	return fmt.Sprintf("Budget approved and applied to the project (%.2f).", total), nil
}

// costPost rolls a cost's project totals up, marking the cost as consumed by the
// project measures.
func costPost(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, costModel, id)
	if rec == nil {
		return "", fmt.Errorf("cost not found")
	}
	actual, committed, err := projectCostRollup(bypass, idOf(rec["project_id"]))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Cost rolled up: actual %.2f, committed %.2f.", actual, committed), nil
}

// costRefresh recomputes the project's cost rollup without changing the cost.
func costRefresh(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, costModel, id)
	if rec == nil {
		return "", fmt.Errorf("cost not found")
	}
	actual, committed, err := projectCostRollup(bypass, idOf(rec["project_id"]))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Project totals refreshed: actual %.2f, committed %.2f.", actual, committed), nil
}
