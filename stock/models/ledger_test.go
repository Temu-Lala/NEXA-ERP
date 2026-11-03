package models

import (
	"testing"
	"time"
)

func q(id int, qty, rsvd float64, days int) QuantSnap {
	var t time.Time
	if days >= 0 {
		t = time.Date(2026, 10, 1+days, 0, 0, 0, 0, time.UTC)
	}
	return QuantSnap{ID: id, Qty: qty, Rsvd: rsvd, InDate: t}
}

func sumAllocs(allocs []QuantAlloc) float64 {
	var s float64
	for _, a := range allocs {
		s += a.Qty
	}
	return s
}

func TestPlanAllocationFIFOOrder(t *testing.T) {
	snaps := []QuantSnap{
		q(3, 60, 0, 20),  // newest
		q(1, 100, 40, 0), // oldest, 40 reserved → 60 available
		q(2, 80, 0, 10),
	}
	allocs, short := planAllocation(120, snaps)
	if short != 0 {
		t.Fatalf("expected full coverage, short=%v", short)
	}
	if len(allocs) != 2 {
		t.Fatalf("expected 2 slices, got %d", len(allocs))
	}
	if allocs[0].QuantID != 1 || allocs[0].Qty != 60 {
		t.Fatalf("oldest quant must be consumed first, got %+v", allocs[0])
	}
	if allocs[1].QuantID != 2 || allocs[1].Qty != 60 {
		t.Fatalf("second slice wrong, got %+v", allocs[1])
	}
}

func TestPlanAllocationShortfall(t *testing.T) {
	allocs, short := planAllocation(100, []QuantSnap{q(1, 30, 10, 0), q(2, 20, 0, 5)})
	if got := sumAllocs(allocs); got != 40 {
		t.Fatalf("allocated %v, want 40", got)
	}
	if short != 60 {
		t.Fatalf("short %v, want 60", short)
	}
}

func TestPlanAllocationSkipsUnreservedNone(t *testing.T) {
	allocs, short := planAllocation(10, []QuantSnap{q(1, 50, 50, 0)})
	if len(allocs) != 0 || short != 10 {
		t.Fatalf("fully reserved quant must not allocate: allocs=%v short=%v", allocs, short)
	}
}

func TestSplitCompletion(t *testing.T) {
	tests := []struct {
		demand, done         float64
		wantPosted, wantBack float64
	}{
		{100, 100, 100, 0},
		{100, 60, 60, 40},              // partial → backorder
		{100, 120, 120, 0},             // over-pick posts fully
		{100, 0, 0, 100},               // nothing done
		{100, 99.999999, 99.999999, 0}, // within epsilon
	}
	for _, tc := range tests {
		posted, back := splitCompletion(tc.demand, tc.done)
		if diff := posted - tc.wantPosted; diff > 1e-6 || diff < -1e-6 {
			t.Fatalf("demand=%v done=%v posted=%v want %v", tc.demand, tc.done, posted, tc.wantPosted)
		}
		if diff := back - tc.wantBack; diff > 1e-6 || diff < -1e-6 {
			t.Fatalf("demand=%v done=%v backorder=%v want %v", tc.demand, tc.done, back, tc.wantBack)
		}
	}
}

func TestQuantSnapAvailable(t *testing.T) {
	if got := q(1, 10, 4, 0).Available(); got != 6 {
		t.Fatalf("available %v, want 6", got)
	}
}

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

func TestPlanAllocationFEFOOrder(t *testing.T) {
	// Two lots: B received earlier but expires later; A expires sooner,
	// so FEFO must pick A first even though FIFO-by-receipt would pick B.
	snaps := []QuantSnap{
		{ID: 2, Qty: 50, InDate: d(2026, 9, 1), Expiry: d(2026, 12, 31)},
		{ID: 1, Qty: 50, InDate: d(2026, 10, 1), Expiry: d(2026, 11, 1)},
	}
	allocs, short := planAllocation(60, snaps)
	if short != 0 {
		t.Fatalf("expected full coverage, short=%v", short)
	}
	if allocs[0].QuantID != 1 || allocs[0].Qty != 50 {
		t.Fatalf("earliest-expiry lot must be consumed first, got %+v", allocs[0])
	}
	if allocs[1].QuantID != 2 || allocs[1].Qty != 10 {
		t.Fatalf("second slice wrong, got %+v", allocs[1])
	}
}

func TestPlanAllocationDatedBeforeUndated(t *testing.T) {
	// Dated lots are consumed before undated stock, then FIFO among the rest.
	snaps := []QuantSnap{
		{ID: 1, Qty: 10, InDate: d(2026, 8, 1)},
		{ID: 2, Qty: 10, InDate: d(2026, 10, 1), Expiry: d(2026, 11, 1)},
	}
	allocs, _ := planAllocation(15, snaps)
	if allocs[0].QuantID != 2 {
		t.Fatalf("dated lot should win over undated, got %+v", allocs[0])
	}
	if allocs[1].QuantID != 1 || allocs[1].Qty != 5 {
		t.Fatalf("undated remainder wrong, got %+v", allocs[1])
	}
}

func TestEligibleForReservation(t *testing.T) {
	today := d(2026, 10, 20)
	cases := []struct {
		name string
		g    lotGuard
		want bool
	}{
		{"released undated", lotGuard{State: "released"}, true},
		{"expires today still usable", lotGuard{State: "released", Expiry: today}, true},
		{"expired yesterday", lotGuard{State: "released", Expiry: d(2026, 10, 19)}, false},
		{"past removal date", lotGuard{State: "released", Removal: d(2026, 10, 10)}, false},
		{"quarantined", lotGuard{State: "quarantined"}, false},
		{"recalled", lotGuard{State: "recalled"}, false},
	}
	for _, tc := range cases {
		if got := eligibleForReservation(tc.g, today); got != tc.want {
			t.Fatalf("%s: eligible=%v want %v", tc.name, got, tc.want)
		}
	}
}
