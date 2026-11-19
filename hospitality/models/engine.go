package models

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"nexa/core/orm"
)

// Hospitality engine: reservation math (nights/total), double-booking
// prevention on confirm, check-in/out workflow, folio posting, and the
// ADR / RevPAR / occupancy analytics on each property.

const (
	propModel   = "hospitality.property"
	roomModel   = "hospitality.room"
	resModel    = "hospitality.reservation"
	chargeModel = "hospitality.charge"
)

func init() {
	orm.RegisterCompute(resModel, "nights", []string{"check_in", "check_out"}, computeResNights)
	orm.RegisterCompute(resModel, "total_amount", []string{"rate_per_night", "nights"}, computeResTotal)
	orm.RegisterCompute(chargeModel, "amount", []string{"quantity", "unit_price"}, computeChargeAmount)
	orm.RegisterCompute(propModel, "occupancy_rate", []string{"id"}, computePropOccupancy)
	orm.RegisterCompute(propModel, "adr", []string{"id"}, computePropADR)
	orm.RegisterCompute(propModel, "revpar", []string{"id"}, computePropRevPAR)
	orm.RegisterObjectAction(resModel, "action_res_confirm", resConfirm)
	orm.RegisterObjectAction(resModel, "action_res_checkin", resCheckIn)
	orm.RegisterObjectAction(resModel, "action_res_checkout", resCheckOut)
	orm.RegisterObjectAction(chargeModel, "action_charge_post", chargePost)
}

func fnum(v interface{}) float64 {
	n, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(v)), 64)
	return n
}

func fint(v interface{}) int {
	n, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprint(v)))
	return n
}

func oid(v interface{}) int {
	if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
		n, _ := strconv.Atoi(fmt.Sprint(arr[0]))
		return n
	}
	n, _ := strconv.Atoi(fmt.Sprint(v))
	return n
}

func loadRec(ctx context.Context, model string, id int) map[string]interface{} {
	rows, err := orm.SearchPage(ctx, model, [][]interface{}{{"id", "=", id}}, 1, 0, "id asc")
	if err != nil || len(rows) == 0 {
		return nil
	}
	return rows[0]
}

func parseDate(s string) (time.Time, bool) {
	if len(s) < 10 {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02", s[:10])
	return t, err == nil
}

func computeResNights(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	ci, _ := parseDate(fmt.Sprint(rec["check_in"]))
	co, _ := parseDate(fmt.Sprint(rec["check_out"]))
	if ci.IsZero() || co.IsZero() || !co.After(ci) {
		return 1, nil
	}
	return int(co.Sub(ci).Hours() / 24), nil
}

func computeResTotal(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	n, _ := computeResNights(context.Background(), rec)
	return fnum(rec["rate_per_night"]) * float64(fint(n)), nil
}

func computeChargeAmount(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	return fnum(rec["quantity"]) * fnum(rec["unit_price"]), nil
}

// overlap: another active reservation on the same room whose [in,out) range
// intersects [checkIn,checkOut).
func roomHasOverlap(ctx context.Context, roomID, selfID int, checkIn, checkOut string) bool {
	ci, ok1 := parseDate(checkIn)
	co, ok2 := parseDate(checkOut)
	if !ok1 || !ok2 {
		return false
	}
	rows, err := orm.SearchPage(orm.ContextWithBypass(ctx, true), resModel,
		[][]interface{}{{"room_id", "=", roomID}, {"id", "!=", selfID},
			{"state", "in", []interface{}{"tentative", "confirmed", "checked_in"}}}, 200, 0, "id asc")
	if err != nil {
		return false
	}
	for _, r := range rows {
		ri, _ := parseDate(fmt.Sprint(r["check_in"]))
		ro, _ := parseDate(fmt.Sprint(r["check_out"]))
		// overlap iff r.in < co && r.out > ci
		if !ri.IsZero() && !ro.IsZero() && ri.Before(co) && ro.After(ci) {
			return true
		}
	}
	return false
}

// resConfirm refuses double-booking: tentative → confirmed only if the room
// is free for the whole stay.
func resConfirm(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	r := loadRec(bypass, resModel, id)
	if r == nil {
		return "", fmt.Errorf("reservation not found")
	}
	if st := orm.AsString(r["state"]); st != "tentative" {
		return "", fmt.Errorf("only tentative reservations can be confirmed (state=%s)", st)
	}
	if roomHasOverlap(bypass, oid(r["room_id"]), id,
		fmt.Sprint(r["check_in"]), fmt.Sprint(r["check_out"])) {
		return "", fmt.Errorf("room is already booked for those dates — pick another room or dates")
	}
	_ = orm.UpdateRecordByID(bypass, resModel, id, map[string]interface{}{"state": "confirmed"})
	return "Reservation confirmed", nil
}

func resCheckIn(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	r := loadRec(bypass, resModel, id)
	if r == nil {
		return "", fmt.Errorf("reservation not found")
	}
	if st := orm.AsString(r["state"]); st != "confirmed" {
		return "", fmt.Errorf("only confirmed reservations can check in (state=%s)", st)
	}
	_ = orm.UpdateRecordByID(bypass, resModel, id, map[string]interface{}{"state": "checked_in"})
	return "Guest checked in", nil
}

func resCheckOut(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	r := loadRec(bypass, resModel, id)
	if r == nil {
		return "", fmt.Errorf("reservation not found")
	}
	if st := orm.AsString(r["state"]); st != "checked_in" {
		return "", fmt.Errorf("only checked-in reservations can check out (state=%s)", st)
	}
	// post the room-night folio charge automatically
	resID := id
	_, _ = orm.Create(bypass, mustRegistry(chargeModel), map[string]interface{}{
		"reservation_id": resID,
		"department":     "room",
		"description":    "Room charge",
		"quantity":       fnum(r["nights"]),
		"unit_price":     fnum(r["rate_per_night"]),
		"charge_date":    time.Now().UTC().Format("2006-01-02"),
		"posted":         true,
	})
	_ = orm.UpdateRecordByID(bypass, resModel, id, map[string]interface{}{"state": "checked_out"})
	return "Checked out — room charge posted to folio", nil
}

func chargePost(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	_ = orm.UpdateRecordByID(bypass, chargeModel, id, map[string]interface{}{"posted": true})
	return "Charge posted", nil
}

func mustRegistry(model string) orm.Model {
	inst, _ := orm.Registry[model]
	return inst
}

// ---- Analytics: current-month occupancy / ADR / RevPAR per property ----

func propMonthStats(ctx context.Context, propID int) (rooms, occupiedNights int, roomRevenue float64) {
	bypass := orm.ContextWithBypass(ctx, true)
	rooms = 0
	roomRows, _ := orm.SearchPage(bypass, roomModel,
		[][]interface{}{{"property_id", "=", propID}, {"state", "=", "active"}}, 2000, 0, "id asc")
	rooms = len(roomRows)
	// reservations overlapping the current month
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, -1)
	res, _ := orm.SearchPage(bypass, resModel,
		[][]interface{}{{"property_id", "=", propID},
			{"state", "in", []interface{}{"confirmed", "checked_in", "checked_out"}}}, 5000, 0, "id asc")
	for _, r := range res {
		ci, ok1 := parseDate(fmt.Sprint(r["check_in"]))
		co, ok2 := parseDate(fmt.Sprint(r["check_out"]))
		if !ok1 || !ok2 {
			continue
		}
		// clip stay to the month
		start := ci
		if start.Before(monthStart) {
			start = monthStart
		}
		end := co
		if end.After(monthEnd.AddDate(0, 0, 1)) {
			end = monthEnd.AddDate(0, 0, 1)
		}
		if end.After(start) {
			nights := int(end.Sub(start).Hours() / 24)
			occupiedNights += nights
			roomRevenue += float64(nights) * fnum(r["rate_per_night"])
		}
	}
	return rooms, occupiedNights, roomRevenue
}

func computePropOccupancy(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	rooms, nights, _ := propMonthStats(ctx, oid(rec["id"]))
	if rooms <= 0 {
		return 0, nil
	}
	days := time.Now().Day()
	return float64(nights) / float64(rooms*days) * 100, nil
}

func computePropADR(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	_, nights, revenue := propMonthStats(ctx, oid(rec["id"]))
	if nights <= 0 {
		return 0, nil
	}
	return revenue / float64(nights), nil
}

func computePropRevPAR(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	rooms, _, revenue := propMonthStats(ctx, oid(rec["id"]))
	if rooms <= 0 {
		return 0, nil
	}
	return revenue / float64(rooms), nil
}

// ---------------------------------------------------------------------------
// Housekeeping: assign/start/complete workflow.
// ---------------------------------------------------------------------------

const hkModel = "hospitality.housekeeping"

func init() {
	orm.RegisterObjectAction(hkModel, "action_hk_assign", hkAssign)
	orm.RegisterObjectAction(hkModel, "action_hk_start", hkStart)
	orm.RegisterObjectAction(hkModel, "action_hk_done", hkDone)
}

func hkLoad(ctx context.Context, id int) map[string]interface{} {
	rows, err := orm.SearchPage(orm.ContextWithBypass(ctx, true), hkModel,
		[][]interface{}{{"id", "=", id}}, 1, 0, "id asc")
	if err != nil || len(rows) == 0 {
		return nil
	}
	return rows[0]
}

func hkAssign(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	t := hkLoad(ctx, id)
	if t == nil {
		return "", fmt.Errorf("task not found")
	}
	if st := orm.AsString(t["state"]); st != "pending" {
		return "", fmt.Errorf("only pending tasks can be assigned (state=%s)", st)
	}
	uid := 0
	if arr, ok := args["user_id"]; ok {
		fmt.Sscanf(arr, "%d", &uid)
	}
	upd := map[string]interface{}{"state": "assigned"}
	if uid > 0 {
		upd["assigned_to_id"] = uid
	}
	_ = orm.UpdateRecordByID(bypass, hkModel, id, upd)
	return "Housekeeping task assigned", nil
}

func hkStart(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	t := hkLoad(ctx, id)
	if t == nil {
		return "", fmt.Errorf("task not found")
	}
	if st := orm.AsString(t["state"]); st != "assigned" && st != "pending" {
		return "", fmt.Errorf("cannot start from state %s", st)
	}
	_ = orm.UpdateRecordByID(bypass, hkModel, id, map[string]interface{}{"state": "in_progress"})
	return "Task in progress", nil
}

func hkDone(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	t := hkLoad(ctx, id)
	if t == nil {
		return "", fmt.Errorf("task not found")
	}
	if st := orm.AsString(t["state"]); st == "done" || st == "cancelled" {
		return "", fmt.Errorf("task already %s", st)
	}
	_ = orm.UpdateRecordByID(bypass, hkModel, id, map[string]interface{}{
		"state":   "done",
		"done_at": time.Now().UTC().Format(time.RFC3339),
	})
	return "Housekeeping task completed", nil
}
