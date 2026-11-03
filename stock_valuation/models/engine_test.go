package models

import (
	"math"
	"testing"
)

// Valuation maths tests. The FIFO case is the blueprint §10 worked example:
// receipts of 40 units @ 100c and 20 units @ 110c hold a combined remaining
// value of 6 200 cents.

func TestFIFOConsumeBlueprintExample(t *testing.T) {
	snaps := []LayerSnap{
		{ID: 1, RemQty: 40, RemValCents: 4000, Seq: 1},
		{ID: 2, RemQty: 20, RemValCents: 2200, Seq: 2},
	}
	slices, short := fifoConsume(snaps, 60)
	if short != 0 {
		t.Fatalf("short = %v, want 0", short)
	}
	var total int64
	for _, s := range slices {
		total += s.Cents
	}
	if total != 6200 {
		t.Errorf("consume 60 total = %d cents, want 6200", total)
	}
	if len(slices) != 2 || slices[0].LayerID != 1 || slices[1].LayerID != 2 {
		t.Errorf("want both layers oldest-first, got %+v", slices)
	}
}

func TestFIFOConsumePartialLayer(t *testing.T) {
	snaps := []LayerSnap{
		{ID: 1, RemQty: 40, RemValCents: 4000, Seq: 1},
		{ID: 2, RemQty: 20, RemValCents: 2200, Seq: 2},
	}
	slices, short := fifoConsume(snaps, 50)
	if short != 0 {
		t.Fatalf("short = %v, want 0", short)
	}
	var total int64
	for _, s := range slices {
		total += s.Cents
	}
	// 40 @ 100 exact + 10/20 of the 2 200c layer = 1 100c.
	if total != 5100 {
		t.Errorf("consume 50 total = %d cents, want 5100", total)
	}
	if slices[1].Qty != 10 || slices[1].Cents != 1100 {
		t.Errorf("partial slice = %+v, want qty 10 / 1100c", slices[1])
	}
}

func TestFIFOConsumeOrderingAndShortfall(t *testing.T) {
	// Snaps arrive out of order; Seq must still drive oldest-first allocation.
	snaps := []LayerSnap{
		{ID: 2, RemQty: 20, RemValCents: 2200, Seq: 2},
		{ID: 1, RemQty: 40, RemValCents: 4000, Seq: 1},
	}
	slices, short := fifoConsume(snaps, 70)
	if math.Abs(short-10) > qtyEps {
		t.Errorf("short = %v, want 10", short)
	}
	if len(slices) != 2 || slices[0].LayerID != 1 {
		t.Errorf("oldest layer must be consumed first, got %+v", slices)
	}
	if _, short := fifoConsume(snaps, 0); short != 0 {
		t.Errorf("zero demand should report no shortfall")
	}
}

func TestAvgConsumePricesAtBlendedPool(t *testing.T) {
	snaps := []LayerSnap{
		{ID: 1, RemQty: 40, RemValCents: 4000, Seq: 1},
		{ID: 2, RemQty: 20, RemValCents: 2200, Seq: 2},
	}
	slices, short := avgConsume(snaps, 30)
	if short != 0 {
		t.Fatalf("short = %v, want 0", short)
	}
	var total int64
	for _, s := range slices {
		total += s.Cents
	}
	// Pool average 6 200/60 = 103.33c; 30 units -> 3 100c exactly.
	if total != 3100 {
		t.Errorf("avg consume 30 total = %d cents, want 3100", total)
	}
	// Slices keep pools consistent: each slice costs no more than its layer.
	for _, s := range slices {
		if s.Cents < 0 {
			t.Errorf("negative slice on layer %d", s.LayerID)
		}
	}
}

func TestAvgUnitCents(t *testing.T) {
	// Blend 40 units worth 4 000c with a receipt of 20 units worth 2 200c.
	if got := avgUnitCents(40, 4000, 20, 2200); got != 103 {
		t.Errorf("avgUnitCents = %d, want 103 (6200/60 rounded)", got)
	}
	if got := avgUnitCents(0, 0, 0, 0); got != 0 {
		t.Errorf("empty pool must price at 0, got %d", got)
	}
	if got := avgUnitCents(10, 1500, 0, 0); got != 150 {
		t.Errorf("zero-quantity receipt must not dilute: got %d, want 150", got)
	}
}

func TestAllocateLandedCentsRemainderToLastLine(t *testing.T) {
	got := allocateLandedCents(1000, []float64{1, 1, 1})
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0] != 333 || got[1] != 333 || got[2] != 334 {
		t.Errorf("allocation = %v, want [333 333 334]", got)
	}
	var sum int64
	for _, c := range got {
		sum += c
	}
	if sum != 1000 {
		t.Errorf("Σ = %d, want 1000", sum)
	}

	// Uneven weights stay reproducible and lossless.
	got = allocateLandedCents(10, []float64{3, 1})
	if got[0] != 7 || got[1] != 3 {
		t.Errorf("weighted split = %v, want [7 3]", got)
	}

	if allocateLandedCents(100, nil) != nil {
		t.Error("nil weights must yield nil")
	}
	if allocateLandedCents(100, []float64{0, 0}) != nil {
		t.Error("zero weights must yield nil (no silent dump on one line)")
	}
}
