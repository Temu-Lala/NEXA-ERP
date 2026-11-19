package models

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"nexa/core/orm"
)

// Facilities engine: booking conflict detection on confirm, request workflow.

const (
	roomModel    = "facilities.room"
	bookingModel = "facilities.booking"
	reqModel     = "facilities.request"
)

func init() {
	orm.RegisterObjectAction(bookingModel, "action_booking_confirm", bookingConfirm)
	orm.RegisterObjectAction(reqModel, "action_req_assign", reqAssign)
	orm.RegisterObjectAction(reqModel, "action_req_resolve", reqResolve)
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

// bookingConfirm refuses double-booking: checks overlapping confirmed/
// tentative bookings on the same room.
func bookingConfirm(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	b := loadRec(bypass, bookingModel, id)
	if b == nil {
		return "", fmt.Errorf("booking not found")
	}
	if st := orm.AsString(b["state"]); st != "tentative" {
		return "", fmt.Errorf("only tentative bookings can be confirmed (state=%s)", st)
	}
	roomID := oid(b["room_id"])
	start := strings.TrimSpace(fmt.Sprint(b["start_at"]))
	end := strings.TrimSpace(fmt.Sprint(b["end_at"]))
	others, _ := orm.SearchPage(bypass, bookingModel,
		[][]interface{}{{"room_id", "=", roomID}, {"id", "!=", id},
			{"state", "in", []interface{}{"tentative", "confirmed"}}}, 200, 0, "id asc")
	for _, o := range others {
		os := strings.TrimSpace(fmt.Sprint(o["start_at"]))
		oe := strings.TrimSpace(fmt.Sprint(o["end_at"]))
		// overlap iff o.start < end && o.end > start (string compare works for ISO)
		if os < end && oe > start {
			return "", fmt.Errorf("room already booked %s → %s", os, oe)
		}
	}
	_ = orm.UpdateRecordByID(bypass, bookingModel, id, map[string]interface{}{"state": "confirmed"})
	return "Booking confirmed — no conflicts", nil
}

// reqAssign assigns a facility request to a user.
func reqAssign(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	r := loadRec(bypass, reqModel, id)
	if r == nil {
		return "", fmt.Errorf("request not found")
	}
	if st := orm.AsString(r["state"]); st == "resolved" || st == "cancelled" {
		return "", fmt.Errorf("cannot assign a %s request", st)
	}
	uid := oid(args["user_id"])
	upd := map[string]interface{}{"state": "assigned"}
	if uid > 0 {
		upd["assigned_to_id"] = uid
	}
	_ = orm.UpdateRecordByID(bypass, reqModel, id, upd)
	return "Request assigned", nil
}

// reqResolve marks resolved with timestamp.
func reqResolve(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	r := loadRec(bypass, reqModel, id)
	if r == nil {
		return "", fmt.Errorf("request not found")
	}
	if st := orm.AsString(r["state"]); st == "resolved" || st == "cancelled" {
		return "", fmt.Errorf("request already %s", st)
	}
	_ = orm.UpdateRecordByID(bypass, reqModel, id, map[string]interface{}{
		"state":       "resolved",
		"resolved_at": time.Now().UTC().Format(time.RFC3339),
	})
	return "Request resolved", nil
}
