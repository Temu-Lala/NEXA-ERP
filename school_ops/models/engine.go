package models

import (
	"context"
	"math"
	"time"

	"nexa/core/orm"
)

// Behaviour layer for campus operations. The arithmetic that decides outcomes — overdue
// day counts and library fines, inventory movement application (a receipt adds, an issue
// subtracts but never below zero), low-stock detection against a reorder level, seat/bed
// availability and date addition — is pure and unit-tested. The computes project counts
// (route stops and assigned learners, a book's on-loan count, a house's dormitories, a
// dormitory's occupancy) and derived figures (a dormitory's free beds and an item's
// low-stock flag). Loan, boarding, transport and stock mutations run their guards through
// these helpers inside the actions so the rules stay in one tested place.

const (
	routeModel        = "school.transport.route"
	stopModel         = "school.transport.stop"
	vehicleModel      = "school.transport.vehicle"
	transportAssign   = "school.transport.assignment"
	bookModel         = "school.library.book"
	loanModel         = "school.library.loan"
	houseModel        = "school.boarding.house"
	dormModel         = "school.boarding.dormitory"
	boarderModel      = "school.boarding.boarder"
	itemModel         = "school.inventory.item"
	movementModel     = "school.inventory.movement"
	menuModel         = "school.cafeteria.menu"
	subscriptionModel = "school.cafeteria.subscription"

	dateLayout = "2006-01-02"
)

func round2(f float64) float64 { return math.Round(f*100) / 100 }

// parseDate reads an ISO yyyy-mm-dd date; ok is false for a blank or malformed value.
func parseDate(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// addDays shifts an ISO date by n days. A blank/unparsable base date returns "".
func addDays(date string, n int) string {
	t, ok := parseDate(date)
	if !ok {
		return ""
	}
	return t.AddDate(0, 0, n).Format(dateLayout)
}

// overdueDays counts whole days a loan is past its due date as of the given day; zero
// when not late or when either date is missing.
func overdueDays(due, on string) int {
	d, ok1 := parseDate(due)
	o, ok2 := parseDate(on)
	if !ok1 || !ok2 {
		return 0
	}
	diff := int(o.Sub(d).Hours() / 24)
	if diff < 0 {
		return 0
	}
	return diff
}

// loanFine is the overdue-day count times the per-day fine rate, rounded to 2dp.
func loanFine(days int, perDay float64) float64 {
	if days <= 0 || perDay <= 0 {
		return 0
	}
	return round2(float64(days) * perDay)
}

// applyMovement folds a movement into the on-hand quantity. A receipt ("in") always
// adds; an issue ("out") subtracts but reports not-ok when that would drive stock
// negative, leaving the balance untouched.
func applyMovement(onHand, qty float64, direction string) (float64, bool) {
	if qty < 0 {
		return onHand, false
	}
	if direction == "out" {
		if qty > onHand {
			return onHand, false
		}
		return round2(onHand - qty), true
	}
	return round2(onHand + qty), true
}

// lowStock reports whether an on-hand quantity has fallen to or below its reorder level.
func lowStock(onHand, reorder float64) bool {
	if reorder <= 0 {
		return false
	}
	return onHand <= reorder
}

// seatAvailable reports whether one more learner/rider fits given capacity and current
// occupancy. A non-positive capacity means unspecified and is treated as unavailable.
func seatAvailable(capacity, used int) bool {
	if capacity <= 0 {
		return false
	}
	return used < capacity
}

// ---------------------------------------------------------------------------
// Computes
// ---------------------------------------------------------------------------

func init() {
	orm.RegisterCompute(routeModel, "stop_count", []string{"id"}, computeRouteStopCount)
	orm.RegisterCompute(routeModel, "assigned_count", []string{"id"}, computeRouteAssignedCount)
	orm.RegisterCompute(bookModel, "borrowed_count", []string{"id"}, computeBookBorrowedCount)
	orm.RegisterCompute(houseModel, "dorm_count", []string{"id"}, computeHouseDormCount)
	orm.RegisterCompute(dormModel, "boarder_count", []string{"id"}, computeDormBoarderCount)
	orm.RegisterCompute(dormModel, "available_beds", []string{"capacity", "id"}, computeDormAvailableBeds)
	orm.RegisterCompute(itemModel, "is_low", []string{"on_hand_qty", "reorder_level"}, computeItemIsLow)
}

func computeRouteStopCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), stopModel, [][]interface{}{{"route_id", "=", intOr(rec["id"])}})
	if err != nil {
		return 0, nil
	}
	return n, nil
}

func computeRouteAssignedCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), transportAssign, [][]interface{}{
		{"route_id", "=", intOr(rec["id"])},
		{"state", "=", string(TAssignActive)},
	})
	if err != nil {
		return 0, nil
	}
	return n, nil
}

func computeBookBorrowedCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), loanModel, [][]interface{}{
		{"book_id", "=", intOr(rec["id"])},
		{"state", "in", []interface{}{string(LoanBorrowed), string(LoanOverdue)}},
	})
	if err != nil {
		return 0, nil
	}
	return n, nil
}

func computeHouseDormCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), dormModel, [][]interface{}{{"house_id", "=", intOr(rec["id"])}})
	if err != nil {
		return 0, nil
	}
	return n, nil
}

func computeDormBoarderCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), boarderModel, [][]interface{}{
		{"dormitory_id", "=", intOr(rec["id"])},
		{"state", "=", string(BoardActive)},
	})
	if err != nil {
		return 0, nil
	}
	return n, nil
}

func computeDormAvailableBeds(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	id := intOr(rec["id"])
	active, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), boarderModel, [][]interface{}{
		{"dormitory_id", "=", id},
		{"state", "=", string(BoardActive)},
	})
	if err != nil {
		return 0, nil
	}
	avail := intOr(rec["capacity"]) - active
	if avail < 0 {
		avail = 0
	}
	return avail, nil
}

func computeItemIsLow(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return lowStock(floatOr(rec["on_hand_qty"]), floatOr(rec["reorder_level"])), nil
}

// ---------------------------------------------------------------------------
// Shared coercion helpers (ORM values arrive as interface{})
// ---------------------------------------------------------------------------

func intOr(v interface{}) int {
	n, ok := orm.CoerceInt64(v)
	if !ok {
		return 0
	}
	return int(n)
}

func strOr(v interface{}) string { return orm.AsString(v) }

func boolOr(v interface{}) bool { return orm.AsBool(v) }

func floatOr(v interface{}) float64 {
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
	return 0
}
