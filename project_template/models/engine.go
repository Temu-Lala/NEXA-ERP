package models

import (
	"context"
	"strconv"
	"strings"

	"nexa/core/orm"
)

// Template engine (blueprint §16). A template is inert metadata; its value is
// realised by the instantiate action, which reproduces the phase/task blueprint
// into a concrete draft project. There is no background job here; the shared
// helpers support reading blueprint rows and building the new project tree.

const (
	tplModel    = "project.template"
	tplPhaseMdl = "project.template.phase"
	tplTaskMdl  = "project.template.task"

	// Destination models owned by the core project module.
	destProjectModel = "project.project"
	destPhaseModel   = "project.phase"
	destTaskModel    = "project.task"
)

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
