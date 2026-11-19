package models

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"nexa/core/orm"
)

// Earned Value Management (spec ERP differentiation). The classic EVM trio:
//
//	PV — Planned Value:  budgeted cost of work scheduled to date
//	EV — Earned Value:   budgeted cost of work actually performed (% complete)
//	AC — Actual Cost:    what has actually been spent
//
//	CPI = EV / AC  (cost performance — >1 under budget)
//	SPI = EV / PV  (schedule performance — >1 ahead of plan)
//
// PV uses a linear time-phase approximation between planned start and planned
// end — honest and dependency-free; a future enhancement can read budget lines
// for true S-curve phasing. All values derive from real project fields; nothing
// is stored, so they can never drift from the underlying budget/progress/cost.

func init() {
	deps := []string{"budget", "actual_cost", "progress", "planned_start", "planned_end", "actual_start", "actual_end", "state"}
	orm.RegisterCompute(projModel, "ev_pv", deps, computeEVPlannedValue)
	orm.RegisterCompute(projModel, "ev_ev", deps, computeEVEarnedValue)
	orm.RegisterCompute(projModel, "ev_ac", deps, computeEVActualCost)
	orm.RegisterCompute(projModel, "ev_cpi", deps, computeEVCostPerformance)
	orm.RegisterCompute(projModel, "ev_spi", deps, computeEVSchedulePerformance)
}

func projectBudget(rec map[string]interface{}) float64 {
	return numOf(rec["budget"])
}

func numOf(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	}
	// sdk.Numeric and friends arrive as strings or decimal structs — parse.
	if f, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(v)), 64); err == nil {
		return f
	}
	return 0
}

func projectProgressPct(rec map[string]interface{}) float64 {
	return numOf(rec["progress"]) // stored as 0-100
}

func parseProjectDate(v interface{}) time.Time {
	s := str(v)
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse("2006-01-02", s[:min(10, len(s))]); err == nil {
		return t
	}
	return time.Time{}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// timePhasePct is the fraction of the planned duration elapsed, clamped 0..1.
func timePhasePct(rec map[string]interface{}, now time.Time) float64 {
	start := parseProjectDate(rec["actual_start"])
	if start.IsZero() {
		start = parseProjectDate(rec["planned_start"])
	}
	end := parseProjectDate(rec["planned_end"])
	if end.IsZero() {
		end = parseProjectDate(rec["target_end"])
	}
	if start.IsZero() || end.IsZero() || !end.After(start) {
		return 0
	}
	if state := str(rec["state"]); state == "completed" {
		return 1
	}
	elapsed := now.Sub(start).Hours() / 24
	total := end.Sub(start).Hours() / 24
	if total <= 0 {
		return 0
	}
	pct := elapsed / total
	if pct < 0 {
		return 0
	}
	if pct > 1 {
		return 1
	}
	return pct
}

func computeEVPlannedValue(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	// Preferred: real phasing — sum budget-line amounts whose Period (YYYY-MM)
	// has arrived. Falls back to the linear time-phase when no lines carry a
	// period (project_finance absent or unscheduled budget).
	if pv, ok := phasedPlannedValue(ctx, rec); ok {
		return pv, nil
	}
	return projectBudget(rec) * timePhasePct(rec, time.Now()), nil
}

// phasedPlannedValue sums project.budget.line amounts scheduled on or before
// the current month. Returns ok=false when the finance addon isn't installed
// or no line carries a period — the caller then uses the linear approximation.
func phasedPlannedValue(ctx context.Context, rec map[string]interface{}) (float64, bool) {
	pid := intID(rec["id"])
	if pid <= 0 {
		return 0, false
	}
	bypass := orm.ContextWithBypass(ctx, true)
	budgets, err := orm.SearchPage(bypass, "project.budget",
		[][]interface{}{{"project_id", "=", pid}}, 100, 0, "id asc")
	if err != nil || len(budgets) == 0 {
		return 0, false
	}
	var bids []interface{}
	for _, b := range budgets {
		bids = append(bids, intID(b["id"]))
	}
	lines, err := orm.SearchPage(bypass, "project.budget.line",
		[][]interface{}{{"budget_id", "in", bids}}, 2000, 0, "id asc")
	if err != nil {
		return 0, false
	}
	now := time.Now().Format("2006-01")
	pv, scheduled := 0.0, false
	for _, l := range lines {
		period := str(l["period"])
		if len(period) >= 7 && period[:7] <= now {
			pv += numOf(l["amount"])
			scheduled = true
		}
	}
	if !scheduled {
		return 0, false
	}
	// PV can never exceed the approved budget (BAC).
	if bac := projectBudget(rec); bac > 0 && pv > bac {
		pv = bac
	}
	return pv, true
}

func computeEVEarnedValue(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	return projectBudget(rec) * projectProgressPct(rec) / 100, nil
}

func computeEVActualCost(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	return numOf(rec["actual_cost"]), nil
}

func computeEVCostPerformance(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	ev := projectBudget(rec) * projectProgressPct(rec) / 100
	ac := numOf(rec["actual_cost"])
	if ac <= 0 {
		if ev <= 0 {
			return 0, nil
		}
		return 99.99, nil // spent nothing yet but earned value — treat as excellent
	}
	return ev / ac, nil
}

func computeEVSchedulePerformance(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	pv := projectBudget(rec) * timePhasePct(rec, time.Now())
	ev := projectBudget(rec) * projectProgressPct(rec) / 100
	if pv <= 0 {
		return 0, nil
	}
	return ev / pv, nil
}
