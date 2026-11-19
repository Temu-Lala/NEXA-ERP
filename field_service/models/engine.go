package models

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"nexa/core/orm"
)

// Field service engine: dispatch with skill matching, on-site state flow,
// time tracking, completion with signature + service report, parts amounts,
// and technician utilization analytics.

const (
	techModel = "fs.technician"
	woModel   = "fs.workorder"
	partModel = "fs.workorder.part"
)

func init() {
	orm.RegisterCompute(partModel, "amount", []string{"quantity", "unit_cost"}, computePartAmount)
	orm.RegisterCompute(techModel, "open_orders", []string{"id"}, computeTechOpenOrders)
	orm.RegisterObjectAction(woModel, "action_fs_dispatch", woDispatch)
	orm.RegisterObjectAction(woModel, "action_fs_arrive", woArrive)
	orm.RegisterObjectAction(woModel, "action_fs_complete", woComplete)
}

func fnum(v interface{}) float64 {
	n, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(v)), 64)
	return n
}

func oid(v interface{}) int {
	if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
		n, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprint(arr[0])))
		return n
	}
	n, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprint(v)))
	return n
}

func loadRec(ctx context.Context, model string, id int) map[string]interface{} {
	rows, err := orm.SearchPage(ctx, model, [][]interface{}{{"id", "=", id}}, 1, 0, "id asc")
	if err != nil || len(rows) == 0 {
		return nil
	}
	return rows[0]
}

func computePartAmount(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	return fnum(rec["quantity"]) * fnum(rec["unit_cost"]), nil
}

// computeTechOpenOrders = active work orders assigned to this technician.
func computeTechOpenOrders(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	rows, err := orm.SearchPage(orm.ContextWithBypass(ctx, true), woModel,
		[][]interface{}{{"technician_id", "=", oid(rec["id"])},
			{"state", "in", []interface{}{"dispatched", "en_route", "on_site", "in_progress"}}}, 500, 0, "id asc")
	if err != nil {
		return 0, nil
	}
	return len(rows), nil
}

// woDispatch assigns a technician — checks skill match if required_skill set.
func woDispatch(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	wo := loadRec(bypass, woModel, id)
	if wo == nil {
		return "", fmt.Errorf("work order not found")
	}
	if st := orm.AsString(wo["state"]); st != "draft" {
		return "", fmt.Errorf("only draft work orders can be dispatched (state=%s)", st)
	}
	techID := oid(args["technician_id"])
	if techID <= 0 {
		return "", fmt.Errorf("technician_id required")
	}
	tech := loadRec(bypass, techModel, techID)
	if tech == nil {
		return "", fmt.Errorf("technician not found")
	}
	// skill match: warn (not block) if required skill isn't in technician skills
	need := strings.ToLower(strings.TrimSpace(orm.AsString(wo["required_skill"])))
	have := strings.ToLower(orm.AsString(tech["skills"]))
	warn := ""
	if need != "" && !strings.Contains(have, need) {
		warn = fmt.Sprintf(" (warning: technician lacks '%s' skill)", need)
	}
	upd := map[string]interface{}{
		"state":         "dispatched",
		"technician_id": techID,
	}
	if t := strings.TrimSpace(args["scheduled_at"]); t != "" {
		upd["scheduled_at"] = t
	}
	_ = orm.UpdateRecordByID(bypass, woModel, id, upd)
	return fmt.Sprintf("Dispatched to %s%s", orm.AsString(tech["name"]), warn), nil
}

// woArrive marks arrival on site — dispatched/en_route → on_site.
func woArrive(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	wo := loadRec(bypass, woModel, id)
	if wo == nil {
		return "", fmt.Errorf("work order not found")
	}
	st := orm.AsString(wo["state"])
	if st != "dispatched" && st != "en_route" {
		return "", fmt.Errorf("cannot arrive from state %s", st)
	}
	_ = orm.UpdateRecordByID(bypass, woModel, id, map[string]interface{}{
		"state":      "on_site",
		"arrived_at": time.Now().UTC().Format(time.RFC3339),
	})
	return "Technician on site", nil
}

// woComplete closes the work order: on_site/in_progress → completed with
// labor time, signature, report, first-time-fix flag.
func woComplete(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	wo := loadRec(bypass, woModel, id)
	if wo == nil {
		return "", fmt.Errorf("work order not found")
	}
	st := orm.AsString(wo["state"])
	if st != "on_site" && st != "in_progress" {
		return "", fmt.Errorf("cannot complete from state %s", st)
	}
	upd := map[string]interface{}{
		"state":        "completed",
		"completed_at": time.Now().UTC().Format(time.RFC3339),
	}
	if l := strings.TrimSpace(args["labor_min"]); l != "" {
		n, _ := strconv.Atoi(l)
		upd["labor_min"] = n
	}
	if t := strings.TrimSpace(args["travel_min"]); t != "" {
		n, _ := strconv.Atoi(t)
		upd["travel_min"] = n
	}
	if s := strings.TrimSpace(args["signature"]); s != "" {
		upd["signature"] = s
	}
	if r := strings.TrimSpace(args["report"]); r != "" {
		upd["report"] = r
	}
	if ftf := strings.TrimSpace(args["first_time_fix"]); ftf == "1" || ftf == "true" {
		upd["first_time_fix"] = true
	}
	_ = orm.UpdateRecordByID(bypass, woModel, id, upd)
	return "Work order completed — service report filed", nil
}
