package models

import (
	"math"
	"testing"
)

func TestReconcileMatchLineThreeWayExample(t *testing.T) {
	// Blueprint §12 worked example: PO 100 @ 500, only 80 accepted, but the
	// invoice bills for 100 -> a quantity exception and a 40,000 approvable
	// amount (we pay for what was accepted, not what was billed).
	v := reconcileMatchLine(80, 100, 500, 500, 0, 0, 0.05)
	if !v.QtyExc {
		t.Fatalf("expected quantity exception, got %+v", v)
	}
	if v.PriceExc {
		t.Fatalf("did not expect a price exception, got %+v", v)
	}
	if v.Approvable != 40000 {
		t.Errorf("approvable = %v want 40000", v.Approvable)
	}
	if v.AmountVar != 10000 {
		t.Errorf("amount variance = %v want 10000", v.AmountVar)
	}
	if v.QtyVar != 20 {
		t.Errorf("qty variance = %v want 20", v.QtyVar)
	}
}

func TestReconcileMatchLineClean(t *testing.T) {
	v := reconcileMatchLine(100, 100, 500, 500, 0, 0, 0.05)
	if v.QtyExc || v.PriceExc {
		t.Fatalf("clean line should raise nothing, got %+v", v)
	}
	if v.Approvable != 50000 {
		t.Errorf("approvable = %v want 50000", v.Approvable)
	}
}

func TestReconcileMatchLineUnderInvoiceAllowed(t *testing.T) {
	// Billing less than accepted is a legitimate partial invoice.
	v := reconcileMatchLine(100, 50, 500, 500, 0, 0, 0.05)
	if v.QtyExc {
		t.Fatalf("under-invoicing must not raise a quantity exception, got %+v", v)
	}
	if v.Approvable != 25000 {
		t.Errorf("approvable = %v want 25000", v.Approvable)
	}
}

func TestReconcileMatchLinePriceTolerance(t *testing.T) {
	// 520 vs 500 is a 20 unit (4%) variance: breaches at 2% but clears at 5%.
	if v := reconcileMatchLine(100, 100, 500, 520, 0, 2, 0.05); !v.PriceExc {
		t.Fatalf("expected price exception at 2%% tolerance, got %+v", v)
	}
	if v := reconcileMatchLine(100, 100, 500, 520, 0, 5, 0.05); v.PriceExc {
		t.Fatalf("price 520 within 5%% tolerance should pass, got %+v", v)
	}
}

func TestApprovableAmount(t *testing.T) {
	lines := []lineVar{
		reconcileMatchLine(80, 100, 500, 500, 0, 0, 0.05), // 40000
		reconcileMatchLine(10, 10, 20, 20, 0, 0, 0.05),    // 200
	}
	if got := approvableAmount(lines); got != 40200 {
		t.Errorf("approvable total = %v want 40200", got)
	}
}

func TestTaxVarianceOk(t *testing.T) {
	if !taxVarianceOk(100, 102, 5) {
		t.Errorf("2%% tax variance should be within 5%% tolerance")
	}
	if taxVarianceOk(100, 110, 5) {
		t.Errorf("10%% tax variance should breach 5%% tolerance")
	}
}

func TestMatchOutcome(t *testing.T) {
	if got := matchOutcome(0, false); got != ResultMatched {
		t.Errorf("no exceptions -> %v want matched", got)
	}
	if got := matchOutcome(2, false); got != ResultException {
		t.Errorf("2 exceptions -> %v want exception", got)
	}
	if got := matchOutcome(0, true); got != ResultException {
		t.Errorf("duplicate with no exceptions -> %v want exception", got)
	}
}

func TestDuplicateFingerprintDeterministic(t *testing.T) {
	a := duplicateFingerprint(7, "INV-001", 1234.5, "usd", "2026-01-02")
	b := duplicateFingerprint(7, "inv-001", 1234.50, "USD", "2026-01-02")
	if a != b {
		t.Errorf("fingerprint must be stable across case/format: %s vs %s", a, b)
	}
	c := duplicateFingerprint(7, "INV-002", 1234.5, "USD", "2026-01-02")
	if a == c {
		t.Errorf("different invoice number must change fingerprint")
	}
	d := duplicateFingerprint(8, "INV-001", 1234.5, "USD", "2026-01-02")
	if a == d {
		t.Errorf("different supplier must change fingerprint")
	}
}

func TestRoundHelpers(t *testing.T) {
	if got := round2(1.234); math.Abs(got-1.23) > 1e-9 {
		t.Errorf("round2(1.234) = %v want 1.23", got)
	}
	if got := round2(1.236); math.Abs(got-1.24) > 1e-9 {
		t.Errorf("round2(1.236) = %v want 1.24", got)
	}
	if got := round3(1.2344); math.Abs(got-1.234) > 1e-9 {
		t.Errorf("round3(1.2344) = %v want 1.234", got)
	}
	if got := round3(1.2346); math.Abs(got-1.235) > 1e-9 {
		t.Errorf("round3(1.2346) = %v want 1.235", got)
	}
	if got := round2(-1.004); math.Abs(got-(-1.0)) > 1e-9 {
		t.Errorf("round2(-1.004) = %v want -1.0", got)
	}
}
