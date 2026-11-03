package models

import "testing"

func TestAddDays(t *testing.T) {
	if got := addDays("2026-01-01", 14); got != "2026-01-15" {
		t.Fatalf("add 14 want 2026-01-15 got %q", got)
	}
	if got := addDays("2026-01-31", 1); got != "2026-02-01" {
		t.Fatalf("month rollup want 2026-02-01 got %q", got)
	}
	if got := addDays("", 5); got != "" {
		t.Fatalf("blank base should stay blank got %q", got)
	}
}

func TestOverdueDays(t *testing.T) {
	if got := overdueDays("2026-05-01", "2026-05-04"); got != 3 {
		t.Fatalf("want 3 got %d", got)
	}
	if got := overdueDays("2026-05-04", "2026-05-01"); got != 0 {
		t.Fatalf("not-yet-due want 0 got %d", got)
	}
	if got := overdueDays("2026-05-04", "2026-05-04"); got != 0 {
		t.Fatalf("same day want 0 got %d", got)
	}
	if got := overdueDays("", "2026-05-04"); got != 0 {
		t.Fatalf("blank due want 0 got %d", got)
	}
}

func TestLoanFine(t *testing.T) {
	if got := loanFine(4, 0.5); got != 2 {
		t.Fatalf("4 days @0.5 want 2 got %v", got)
	}
	if got := loanFine(0, 0.5); got != 0 {
		t.Fatalf("no overdue want 0 got %v", got)
	}
	if got := loanFine(10, 0); got != 0 {
		t.Fatalf("zero rate want 0 got %v", got)
	}
}

func TestApplyMovement(t *testing.T) {
	if q, ok := applyMovement(10, 5, "in"); !ok || q != 15 {
		t.Fatalf("receipt want 15/ok got %v/%v", q, ok)
	}
	if q, ok := applyMovement(10, 4, "out"); !ok || q != 6 {
		t.Fatalf("issue want 6/ok got %v/%v", q, ok)
	}
	// over-issue refused, balance untouched
	if q, ok := applyMovement(10, 12, "out"); ok || q != 10 {
		t.Fatalf("over-issue want refused/unchanged got ok=%v q=%v", ok, q)
	}
	// negative quantity refused
	if q, ok := applyMovement(10, -1, "in"); ok || q != 10 {
		t.Fatalf("negative qty want refused got ok=%v q=%v", ok, q)
	}
}

func TestLowStock(t *testing.T) {
	if !lowStock(2, 5) {
		t.Fatal("2 below reorder 5 should be low")
	}
	if !lowStock(5, 5) {
		t.Fatal("equal to reorder should be low")
	}
	if lowStock(6, 5) {
		t.Fatal("6 above reorder 5 should not be low")
	}
	if lowStock(0, 0) {
		t.Fatal("zero reorder never low")
	}
}

func TestSeatAvailable(t *testing.T) {
	if !seatAvailable(10, 9) {
		t.Fatal("9 of 10 should have room")
	}
	if seatAvailable(10, 10) {
		t.Fatal("full should have no room")
	}
	if seatAvailable(0, 0) {
		t.Fatal("unspecified capacity should be unavailable")
	}
}
