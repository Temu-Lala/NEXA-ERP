package models

import (
	"math"
	"testing"
)

// Planning maths tests. The reorder point case is the blueprint §9 worked
// example: 20 units/day × 7-day lead time + 40 safety = 180.

func TestReorderPointBlueprintExample(t *testing.T) {
	if got := reorderPoint(20, 7, 40); got != 180 {
		t.Errorf("reorderPoint(20,7,40) = %v, want 180", got)
	}
	if got := reorderPoint(-5, 7, 40); got != 40 {
		t.Errorf("negative demand must clamp: got %v, want 40", got)
	}
}

func TestInventoryPosition(t *testing.T) {
	if got := inventoryPosition(50, 20, 15); got != 55 {
		t.Errorf("position = %v, want 55", got)
	}
	if got := inventoryPosition(5, 0, 12); got != -7 {
		t.Errorf("position = %v, want -7", got)
	}
}

func TestReplenishTrigger(t *testing.T) {
	if fired, trig := replenishTrigger(MethodMinMax, 90, 100, 180); !fired || trig != TrigMinMax {
		t.Errorf("min/max below min must fire: fired=%v trig=%v", fired, trig)
	}
	if fired, _ := replenishTrigger(MethodMinMax, 120, 100, 180); fired {
		t.Error("min/max above min must not fire even when under ROP")
	}
	if fired, trig := replenishTrigger(MethodRop, 179.9, 0, 180); !fired || trig != TrigRop {
		t.Errorf("rop trigger at 180 must fire: fired=%v trig=%v", fired, trig)
	}
	if fired, trig := replenishTrigger(MethodRop, -1, 100, 180); !fired || trig != TrigStockout {
		t.Errorf("non-positive position escalates to stockout: fired=%v trig=%v", fired, trig)
	}
	if fired, _ := replenishTrigger(MethodMinMax, 10, 0, 0); fired {
		t.Error("unconfigured levels must never fire")
	}
}

func TestOrderQuantity(t *testing.T) {
	if got := orderQuantity(30, 100, 0); math.Abs(got-70) > qtyEps {
		t.Errorf("top-up = %v, want 70", got)
	}
	if got := orderQuantity(120, 100, 0); got != 0 {
		t.Errorf("above target must order nothing, got %v", got)
	}
	if got := orderQuantity(30, 100, 25); math.Abs(got-25) > qtyEps {
		t.Errorf("fixed batch wins: got %v, want 25", got)
	}
}

func TestApplyPurchaseConstraints(t *testing.T) {
	// MOQ lifts, multiple rounds up, cap truncates — each noted.
	qty, notes := applyPurchaseConstraints(8, 10, 5, 0)
	if qty != 10 || len(notes) != 1 {
		t.Errorf("MOQ: got %v %v, want 10 with one note", qty, notes)
	}
	qty, notes = applyPurchaseConstraints(10, 0, 6, 0)
	if math.Abs(qty-12) > qtyEps || len(notes) != 1 {
		t.Errorf("multiple: got %v %v, want 12 with one note", qty, notes)
	}
	qty, notes = applyPurchaseConstraints(10, 0, 6, 11.5)
	if math.Abs(qty-11.5) > qtyEps || len(notes) != 2 {
		t.Errorf("cap: got %v %v, want 11.5 with two notes", qty, notes)
	}
	qty, notes = applyPurchaseConstraints(0, 10, 5, 0)
	if qty != 0 || len(notes) != 0 {
		t.Errorf("zero demand stays zero: got %v %v", qty, notes)
	}
	// Exact multiples must not be rounded up a full step (epsilon guard).
	qty, notes = applyPurchaseConstraints(12, 0, 6, 0)
	if math.Abs(qty-12) > qtyEps || len(notes) != 0 {
		t.Errorf("exact multiple: got %v %v, want 12 note-free", qty, notes)
	}
}

func TestProposalKeyStable(t *testing.T) {
	a := proposalKey(7, 2, "cron")
	b := proposalKey(7, 2, "cron")
	if a != b {
		t.Errorf("key must be stable: %s vs %s", a, b)
	}
	if a == proposalKey(7, 3, "cron") || a == proposalKey(7, 2, "manual") {
		t.Error("warehouse and origin must participate in the key")
	}
}

func TestABCClassesCumulativeThresholds(t *testing.T) {
	// 70 + 15 + 10 + 5: A up to 80 %, B up to 95 % (inclusive), C the tail.
	got := abcClasses([]float64{70, 15, 10, 5})
	want := []ABClass{ABCA, ABCB, ABCB, ABCC}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("class[%d] = %v, want %v (full %v)", i, got[i], want[i], got)
		}
	}
	for _, c := range abcClasses([]float64{0, 0}) {
		if c != ABCNA {
			t.Error("zero total consumption classifies as unc")
		}
	}
}

func TestXYZClassNeedsHistory(t *testing.T) {
	if got := xyzClass(0.2, 2); got != XYZNA {
		t.Errorf("two months must stay unc, got %v", got)
	}
	if got := xyzClass(0.2, 6); got != XYXX {
		t.Errorf("cv 0.2 -> x, got %v", got)
	}
	if got := xyzClass(0.7, 6); got != XYXY {
		t.Errorf("cv 0.7 -> y, got %v", got)
	}
	if got := xyzClass(1.4, 6); got != XYZZ {
		t.Errorf("cv 1.4 -> z, got %v", got)
	}
}
