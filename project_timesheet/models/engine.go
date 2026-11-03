package models

import (
	"context"
	"strconv"
	"strings"

	"nexa/core/orm"
)

// Timesheet engine (blueprint §9). Approved effort is the authoritative source
// for a task's spent hours, so approval triggers a roll-up onto the task; a
// rejected or reopened sheet subtracts back out to keep the numbers honest. An
// approved timesheet is locked: corrections must go through a controlled reopen
// rather than a silent edit, preserving the approval history.

const (
	tsModel    = "project.timesheet"
	tsLine     = "project.timesheet.line"
	taskModel  = "project.task"
	projModel  = "project.project"
	dateLayout = "2006-01-02"
)

// totals sums a timesheet's line hours and its billable hours.
func totals(ctx context.Context, timesheetID int) (total, billable float64) {
	rows, err := orm.SearchPage(ctx, tsLine, [][]interface{}{
		{"timesheet_id", "=", timesheetID},
	}, 5000, 0, "id asc")
	if err != nil {
		return 0, 0
	}
	for _, r := range rows {
		h := flt(r["hours"])
		total += h
		if boolOf(r["billable"]) {
			billable += h
		}
	}
	return total, billable
}

// rollupTaskHours recomputes a task's spent hours from all approved timesheet
// lines that reference it. Keeping task hours derived (not incremental) avoids
// drift when sheets are re-approved or reopened.
func rollupTaskHours(ctx context.Context, taskID int) float64 {
	if taskID <= 0 {
		return 0
	}
	// Only lines on approved sheets count as spent time.
	sheets, err := orm.SearchPage(ctx, tsModel, [][]interface{}{
		{"state", "=", "approved"},
	}, 100000, 0, "id asc")
	if err != nil {
		return 0
	}
	approved := map[int]bool{}
	for _, s := range sheets {
		approved[idOf(s["id"])] = true
	}
	rows, err := orm.SearchPage(ctx, tsLine, [][]interface{}{
		{"task_id", "=", taskID},
	}, 100000, 0, "id asc")
	if err != nil {
		return 0
	}
	var sum float64
	for _, r := range rows {
		if approved[idOf(r["timesheet_id"])] {
			sum += flt(r["hours"])
		}
	}
	bypass := orm.ContextWithBypass(ctx, true)
	_ = orm.UpdateRecordByID(bypass, taskModel, taskID, map[string]interface{}{"actual_hours": sum})
	return sum
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

func boolOf(v interface{}) bool {
	b, _ := v.(bool)
	return b
}

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
