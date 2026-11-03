package models

import (
	"context"
	"strings"
	"time"

	"nexa/core/orm"
	"nexa/core/scheduler"
)

// Project portfolio engine (blueprint §2, §5, §17). Progress and health are
// derived from real execution signals — completed work, overdue milestones and
// tasks, and consumed budget — never hand-typed. The daily recompute is the
// authoritative periodic refresh; a manager may still override health with a
// recorded rationale, which the engine then preserves instead of recomputing.
// This module writes only its own records; costs and commitments are consumed
// as references from Finance/Procurement, not invented here.

const (
	healthCron = "project.health.recompute"

	projModel  = "project.project"
	taskModel  = "project.task"
	mileModel  = "project.milestone"
	costModel  = "project.cost"
	histModel  = "project.status.history"
	dateLayout = "2006-01-02"
	dtLayout   = "2006-01-02 15:04:05"
	// postgresTimeLayout is fmt.Sprint of a scanned time.Time ("2006-01-02 15:04:05 +0000 UTC").
	postgresTimeLayout = "2006-01-02 15:04:05.999999999 -0700 MST"
	openTaskFrom       = 30 // percent under which a project is not yet "closing out"
)

func init() {
	scheduler.RegisterCronHandler(healthCron, runProjectHealthRecompute)
}

// ---------------------------------------------------------------------------
// Pure maths (unit tested)
// ---------------------------------------------------------------------------

// progressPct is the task-completion rollup: done tasks over total eligible
// tasks, as a percentage (0..100). A project with no tasks has no measurable
// progress and reports 0 rather than a misleading 100.
func progressPct(done, total int) float64 {
	if total <= 0 {
		return 0
	}
	p := float64(done) / float64(total) * 100
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}

// budgetPressure is actual cost as a fraction of the approved budget. A zero or
// missing budget yields 0 (no pressure signal) rather than dividing by zero.
func budgetPressure(actual, budget float64) float64 {
	if budget <= 0 {
		return 0
	}
	return actual / budget
}

// classifyHealth maps execution pressure to an explainable health level.
//   - offtrack: over budget (>=120%) with open work, or 4+ overdue items
//   - atrisk:   over 90% budget, or any overdue milestone/task
//   - ontrack:  otherwise
func classifyHealth(overdueItems int, pressure float64, hasOpenWork bool) ProjectHealth {
	if (pressure >= 1.2 && hasOpenWork) || overdueItems >= 4 {
		return HealthOffTrack
	}
	if pressure >= 0.9 || overdueItems >= 1 {
		return HealthAtRisk
	}
	return HealthOnTrack
}

// ---------------------------------------------------------------------------
// Daily health + progress recompute
// ---------------------------------------------------------------------------

func runProjectHealthRecompute(ctx context.Context, _ map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	now := time.Now()

	projects, err := orm.SearchPage(bypass, projModel, [][]interface{}{
		{"state", "in", []interface{}{string(ProjectStateActive), string(ProjectStatePlanned), string(ProjectStateOnHold)}},
		{"active", "=", true},
	}, 5000, 0, "id asc")
	if err != nil {
		return err
	}

	for _, p := range projects {
		id := intID(p["id"])
		if id <= 0 {
			continue
		}

		total, _ := orm.SearchCount(bypass, taskModel, [][]interface{}{{"project_id", "=", id}})
		done, _ := orm.SearchCount(bypass, taskModel, [][]interface{}{
			{"project_id", "=", id}, {"state", "in", []interface{}{string(TaskStateDone)}},
		})
		overdueTasks, _ := orm.SearchCount(bypass, taskModel, [][]interface{}{
			{"project_id", "=", id},
			{"deadline", "!=", ""},
			{"deadline", "<", now.Format(dateLayout)},
			{"state", "not in", []interface{}{string(TaskStateDone), string(TaskStateCancelled)}},
		})
		overdueMiles, _ := orm.SearchCount(bypass, mileModel, [][]interface{}{
			{"project_id", "=", id},
			{"due_date", "!=", ""},
			{"due_date", "<", now.Format(dateLayout)},
			{"state", "!=", string(MilestoneDone)},
		})

		actual := sumCosts(bypass, costModel, id, "actual")
		committed := sumCosts(bypass, costModel, id, "committed")
		pressure := budgetPressure(actual, flt(p["budget"]))

		upd := map[string]interface{}{
			"progress":       progressPct(done, total),
			"actual_cost":    actual,
			"committed_cost": committed,
		}
		// Respect an explicit manager override when one is recorded; otherwise
		// derive health from the live signals.
		if str(p["health_note"]) == "" {
			hasOpen := (total - done) > 0
			upd["health"] = string(classifyHealth(overdueTasks+overdueMiles, pressure, hasOpen))
		}

		_ = orm.UpdateRecordByID(bypass, projModel, id, upd)
	}
	return nil
}

// sumCosts totals project.cost amounts for a project, optionally filtered to a
// source-type bucket ("" = all, "actual", "committed"). Costs live in
// project_finance; the engine only reads them, keeping Finance authoritative.
func sumCosts(ctx context.Context, model string, projectID int, sourceType string) float64 {
	domain := [][]interface{}{{"project_id", "=", projectID}}
	if t := strings.TrimSpace(sourceType); t != "" {
		domain = append(domain, []interface{}{"source_type", "=", t})
	}
	rows, err := orm.SearchPage(ctx, model, domain, 5000, 0, "id asc")
	if err != nil {
		return 0
	}
	var sum float64
	for _, r := range rows {
		sum += flt(r["amount"])
	}
	return sum
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

func loadRec(ctx context.Context, model string, id int) map[string]interface{} {
	if id <= 0 {
		return nil
	}
	rec, err := orm.SearchOne(ctx, model, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return nil
	}
	return rec
}

func str(v interface{}) string { return orm.AsString(v) }

func flt(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int64:
		return float64(t)
	case int:
		return float64(t)
	default:
		n, _ := orm.CoerceInt64(v)
		return float64(n)
	}
}

func intID(v interface{}) int {
	n, _ := orm.CoerceInt64(v)
	return int(n)
}

func parseDate(v interface{}) (time.Time, bool) {
	// Date columns are scanned as time.Time; handle them before stringifying so a
	// record read straight from the database parses like a date typed into a form.
	if t, ok := v.(time.Time); ok {
		return t, !t.IsZero()
	}
	s := strings.TrimSpace(str(v))
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{dateLayout, dtLayout, time.RFC3339, postgresTimeLayout} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// dateValue normalizes a stored date to the form layout used in writes and messages.
// A time.Time column must not be rendered with fmt.Sprint (which appends the zone
// name), or the value would fail validation on the next read.
func dateValue(v interface{}) string {
	if t, ok := v.(time.Time); ok {
		if t.IsZero() {
			return ""
		}
		return t.Format(dateLayout)
	}
	return strings.TrimSpace(str(v))
}
