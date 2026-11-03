package models

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"nexa/core/orm"
)

// Finance engine (blueprint §10). Budget totals are derived from their lines, not
// hand-typed, and an approved budget is what projects up to the project's approved
// budget. Costs are bucketed strictly as actual or committed so the project never
// mixes realised expenditure with open commitments (the §10.3 double-count guard).
// Accounting and Procurement remain authoritative; this module only records
// source-linked snapshots and rolls them up by reference.

const (
	budgetModel = "project.budget"
	lineModel   = "project.budget.line"
	costModel   = "project.cost"
	projModel   = "project.project"
)

// sumBudgetLines totals the planned amounts of a budget's lines.
func sumBudgetLines(ctx context.Context, budgetID int) float64 {
	rows, err := orm.SearchPage(ctx, lineModel, [][]interface{}{
		{"budget_id", "=", budgetID},
	}, 5000, 0, "id asc")
	if err != nil {
		return 0
	}
	var sum float64
	for _, r := range rows {
		sum += flt(r["amount"])
	}
	return sum
}

// rollupCosts totals a project's costs into actual and committed buckets keyed on
// source_type. Any source other than "committed" is treated as realised actual,
// keeping the two measures disjoint.
func rollupCosts(ctx context.Context, projectID int) (actual, committed float64) {
	rows, err := orm.SearchPage(ctx, costModel, [][]interface{}{
		{"project_id", "=", projectID},
	}, 10000, 0, "id asc")
	if err != nil {
		return 0, 0
	}
	for _, r := range rows {
		a := flt(r["amount"])
		if strings.EqualFold(strings.TrimSpace(s(r["source_type"])), "committed") {
			committed += a
		} else {
			actual += a
		}
	}
	return actual, committed
}

// projectCostRollup writes the derived actual/committed totals back onto the
// project so the core health engine reads consistent figures.
func projectCostRollup(ctx context.Context, projectID int) (float64, float64, error) {
	if projectID <= 0 {
		return 0, 0, fmt.Errorf("cost must reference a project")
	}
	actual, committed := rollupCosts(ctx, projectID)
	bypass := orm.ContextWithBypass(ctx, true)
	if err := orm.UpdateRecordByID(bypass, projModel, projectID, map[string]interface{}{
		"actual_cost":    actual,
		"committed_cost": committed,
	}); err != nil {
		return actual, committed, err
	}
	return actual, committed, nil
}

// ---------------------------------------------------------------------------
// Helpers (package-local)
// ---------------------------------------------------------------------------

func load(ctx context.Context, model string, id int) map[string]interface{} {
	if id <= 0 {
		return nil
	}
	rec, err := orm.SearchOne(ctx, model, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return nil
	}
	return rec
}

func idOf(v interface{}) int {
	n, _ := orm.CoerceInt64(v)
	return int(n)
}

func s(v interface{}) string { return orm.AsString(v) }

func flt(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int64:
		return float64(t)
	case int:
		return float64(t)
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f
	default:
		n, _ := orm.CoerceInt64(v)
		return float64(n)
	}
}
