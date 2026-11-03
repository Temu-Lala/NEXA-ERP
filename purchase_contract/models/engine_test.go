package models

import (
	"testing"
	"time"
)

func TestResolveTierPrice(t *testing.T) {
	tiers := []priceTier{
		{QtyFrom: 0, QtyTo: 10, UnitPrice: 5},
		{QtyFrom: 10, QtyTo: 50, UnitPrice: 4},
		{QtyFrom: 50, QtyTo: 0, UnitPrice: 3},
	}
	cases := []struct {
		qty  float64
		want float64
	}{
		{5, 5},   // first band
		{10, 4},  // overlapping bound picks the higher bulk band
		{25, 4},  // middle band
		{100, 3}, // open-ended top band
	}
	for _, c := range cases {
		got, ok := resolveTierPrice(tiers, c.qty)
		if !ok {
			t.Fatalf("qty %v expected a match", c.qty)
		}
		if got != c.want {
			t.Errorf("qty %v price = %v want %v", c.qty, got, c.want)
		}
	}
}

func TestResolveTierPriceIndexation(t *testing.T) {
	tiers := []priceTier{{QtyFrom: 0, QtyTo: 0, UnitPrice: 10, IndexPct: 10}}
	got, ok := resolveTierPrice(tiers, 5)
	if !ok || got != 11 {
		t.Fatalf("indexed price = %v (ok=%v) want 11", got, ok)
	}
}

func TestResolveTierPriceNoMatch(t *testing.T) {
	tiers := []priceTier{{QtyFrom: 10, QtyTo: 0, UnitPrice: 4}}
	if _, ok := resolveTierPrice(tiers, 5); ok {
		t.Fatalf("qty below every band must not match")
	}
}

func TestPriceWithinTolerance(t *testing.T) {
	if !priceWithinTolerance(100, 90, 0) {
		t.Errorf("paying below contract is always fine")
	}
	if !priceWithinTolerance(100, 105, 10) {
		t.Errorf("105 is within 10%% of 100")
	}
	if priceWithinTolerance(100, 115, 10) {
		t.Errorf("115 exceeds 10%% tolerance of 100")
	}
	if priceWithinTolerance(0, 10, 5) {
		t.Errorf("no contract rate but a positive price must fail")
	}
}

func TestContractUsage(t *testing.T) {
	if got := contractUsage(1000, 400, 150); got != 1250 {
		t.Errorf("usage = %v want 1250", got)
	}
	if got := contractUsage(0, 0, 0); got != 0 {
		t.Errorf("empty usage = %v want 0", got)
	}
}

func TestCeilingStatus(t *testing.T) {
	if got := ceilingStatus(500, 0, 90); got != "unlimited" {
		t.Errorf("no ceiling -> %v want unlimited", got)
	}
	if got := ceilingStatus(500, 1000, 90); got != "within" {
		t.Errorf("500/1000 -> %v want within", got)
	}
	if got := ceilingStatus(950, 1000, 90); got != "warning" {
		t.Errorf("950/1000 -> %v want warning", got)
	}
	if got := ceilingStatus(1100, 1000, 90); got != "exceeded" {
		t.Errorf("1100/1000 -> %v want exceeded", got)
	}
}

func TestAvailableCeiling(t *testing.T) {
	if got := availableCeiling(400, 1000); got != 600 {
		t.Errorf("available = %v want 600", got)
	}
	if got := availableCeiling(1200, 1000); got != 0 {
		t.Errorf("over-ceiling available = %v want 0", got)
	}
	if got := availableCeiling(100, 0); got != 0 {
		t.Errorf("unlimited available = %v want 0", got)
	}
}

func TestWithinRenewalWindow(t *testing.T) {
	if !withinRenewalWindow(30, 60) {
		t.Errorf("30 days left with 60-day notice should be inside window")
	}
	if withinRenewalWindow(70, 60) {
		t.Errorf("70 days left is outside a 60-day notice window")
	}
	if withinRenewalWindow(-1, 60) {
		t.Errorf("expired (negative) is not inside the notice window")
	}
}

func TestDaysUntil(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if d, ok := daysUntil(from, "2026-01-11"); !ok || d != 10 {
		t.Errorf("days to 2026-01-11 = %d (ok=%v) want 10", d, ok)
	}
	if d, ok := daysUntil(from, "2025-12-22"); !ok || d != -10 {
		t.Errorf("days to past date = %d (ok=%v) want -10", d, ok)
	}
	if _, ok := daysUntil(from, "not-a-date"); ok {
		t.Errorf("invalid date must not parse")
	}
}
