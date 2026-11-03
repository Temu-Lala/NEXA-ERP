package models

import (
	"context"
	"strconv"
	"strings"

	"nexa/core/orm"
)

// Governance engine (blueprint §7.3, §10, §14.3). The invariant that matters is
// that an approved plan is never overwritten: approving a new baseline marks the
// project's previously-approved baseline as superseded, so schedule variance can
// still be measured against the original commitment. Financial snapshots read the
// project's reconciled cost measures and write derived trend lines (actual,
// committed, forecast, margin) rather than trusting hand-entered totals.

const (
	baselineModel = "project.baseline"
	decisionModel = "project.decision"
	snapModel     = "project.financial.snapshot"
	projModel     = "project.project"
	dateLayout    = "2006-01-02"
)

// supersedePriorBaselines marks every currently-approved baseline of the project
// (other than keepID) as superseded, so exactly one approved baseline is active.
func supersedePriorBaselines(ctx context.Context, projectID, keepID int) int {
	if projectID <= 0 {
		return 0
	}
	rows, err := orm.SearchPage(ctx, baselineModel, [][]interface{}{
		{"project_id", "=", projectID},
		{"state", "=", "approved"},
		{"id", "!=", keepID},
	}, 1000, 0, "id asc")
	if err != nil {
		return 0
	}
	n := 0
	for _, r := range rows {
		if err := orm.UpdateRecordByID(ctx, baselineModel, idOf(r["id"]), map[string]interface{}{"state": "superseded"}); err == nil {
			n++
		}
	}
	return n
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
