package models

import (
	"testing"
	"time"
)

func nearly(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 0.005
}

// bidScore recomputes one bid's composite score against the full field, using
// the same normalization as weightedScore, so a single number can be quoted.
func bidScore(all []normBid, c scoreCriteria, key string) float64 {
	minGoods, minLanded, minLead := mathHuge, mathHuge, mathHuge
	for _, b := range all {
		if b.goods > 0 && b.goods < minGoods {
			minGoods = b.goods
		}
		if b.landed > 0 && b.landed < minLanded {
			minLanded = b.landed
		}
		if b.delivery > 0 && float64(b.delivery) < minLead {
			minLead = float64(b.delivery)
		}
	}
	total := c.total()
	if total <= 0 {
		total = 1
	}
	for _, b := range all {
		if b.key != key {
			continue
		}
		s := c.wPrice * safeRatio(minGoods, b.goods) * 100
		s += c.wLanded * safeRatio(minLanded, b.landed) * 100
		s += c.wDelivery * safeRatio(minLead, float64(b.delivery)) * 100
		s += c.wQuality * clamp100(b.quality)
		s += c.wTerms * clamp100(b.terms)
		s += c.wRisk * (100 - clamp100(b.risk))
		return round2(s / total)
	}
	return 0
}

// TestLandedCostWorkedExample is blueprint §7.3: two bids with different goods
// / freight / tax splits normalize to the same landed cost (110k), so the award
// is decided on the secondary dimensions, not the sticker price.
func TestLandedCostWorkedExample(t *testing.T) {
	a := landedCost(100000, 8000, 2000) // 110000
	b := landedCost(94000, 15000, 1000) // 110000
	if !nearly(a, 110000) || !nearly(b, 110000) {
		t.Fatalf("landed = %v/%v, want both 110000", a, b)
	}
	if !nearly(a, b) {
		t.Errorf("expected cost parity, got %v vs %v", a, b)
	}
	// Equal landed cost makes the primary commercial dimensions a tie; the
	// ranking must fall through to a deterministic secondary key.
	bids := []normBid{
		{key: "B", goods: 94000, landed: b, delivery: 10},
		{key: "A", goods: 100000, landed: a, delivery: 5},
	}
	c := scoreCriteria{wLanded: 100}
	ranked := weightedScore(bids, c)
	if ranked[0].landed != ranked[1].landed {
		t.Fatalf("both bids should tie on landed cost")
	}
	// Deterministic tie-break: lower landed first, then key ascending -> "A".
	if ranked[0].key != "A" {
		t.Errorf("tie should resolve deterministically to A, got %s", ranked[0].key)
	}
}

func TestNormalizeAmount(t *testing.T) {
	if got := normalizeAmount(100, 1.5); !nearly(got, 150) {
		t.Errorf("100 @1.5 = %v, want 150", got)
	}
	if got := normalizeAmount(100, 0); !nearly(got, 100) {
		t.Errorf("zero rate should pass through, got %v", got)
	}
	// A cheaper sticker price can still land higher once converted.
	cheap := normalizeAmount(94000, 1.3) // 122200
	if cheap < 110000 {
		t.Errorf("currency normalization should push the 94k bid above 110k, got %v", cheap)
	}
}

func TestWeightedScoreBestValue(t *testing.T) {
	bids := []normBid{
		{key: "cheap", goods: 90000, landed: 100000, delivery: 30, quality: 60},
		{key: "premium", goods: 110000, landed: 120000, delivery: 5, quality: 95},
	}
	c := scoreCriteria{wPrice: 10, wLanded: 10, wDelivery: 40, wQuality: 40}
	ranked := weightedScore(bids, c)
	if ranked[0].key != "premium" {
		t.Fatalf("best-value criteria should prefer premium, got %s", ranked[0].key)
	}
	if !nearly(bidScore(bids, c, "premium"), 94.52) {
		t.Errorf("premium score = %v, want 94.52", bidScore(bids, c, "premium"))
	}
	if len(weightedScore(nil, c)) != 0 {
		t.Error("no bids should yield no ranking")
	}
	// All-zero weights must not divide by zero.
	if got := weightedScore(bids, scoreCriteria{}); len(got) != 2 {
		t.Errorf("zero weights should still return all bids, got %d", len(got))
	}
}

func TestLowPriceStrategyRanksCheapest(t *testing.T) {
	bids := []normBid{
		{key: "x", goods: 100000, landed: 110000, delivery: 1},
		{key: "y", goods: 95000, landed: 105000, delivery: 20},
	}
	c := scoreCriteria{wLanded: 100}
	if got := weightedScore(bids, c)[0].key; got != "y" {
		t.Errorf("low-price strategy should award cheapest, got %s", got)
	}
}

func TestBidScoreStable(t *testing.T) {
	bids := []normBid{
		{key: "a", goods: 100, landed: 110, delivery: 5, quality: 80, terms: 70, risk: 20},
		{key: "b", goods: 120, landed: 130, delivery: 3, quality: 90, terms: 60, risk: 10},
	}
	c := scoreCriteria{wPrice: 20, wLanded: 20, wDelivery: 20, wQuality: 20, wTerms: 10, wRisk: 10}
	for _, b := range bids {
		if !nearly(bidScore(bids, c, b.key), bidScore(bids, c, b.key)) {
			t.Errorf("bidScore not stable for %s", b.key)
		}
	}
	if bidScore(bids, c, "missing") != 0 {
		t.Error("unknown bid should score 0")
	}
}

func TestDeadlinePassed(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	if !deadlinePassed(now, "2026-06-01") {
		t.Error("past date should lock bids")
	}
	if deadlinePassed(now, "2026-06-30") {
		t.Error("future date should keep bids open")
	}
	if deadlinePassed(now, "") {
		t.Error("no deadline means no lock")
	}
}

func TestLineTotalAndRound(t *testing.T) {
	if got := lineTotal(3, 100, 50); !nearly(got, 250) {
		t.Errorf("lineTotal = %v, want 250", got)
	}
	if got := round2(123.456); !nearly(got, 123.46) {
		t.Errorf("round2 = %v, want 123.46", got)
	}
}
