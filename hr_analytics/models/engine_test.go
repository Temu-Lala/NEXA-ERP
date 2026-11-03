package models

import (
	"math"
	"testing"
	"time"
)

// HR-F11 pure-logic tests: the workforce rates and projections that the snapshot
// refresh and scenario computes rely on. These carry no database.

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestTurnoverRate(t *testing.T) {
	cases := []struct {
		name                 string
		term, opening, close int
		want                 float64
	}{
		{"empty population is zero", 0, 0, 0, 0},
		{"no terminations", 0, 10, 10, 0},
		{"half of avg", 5, 10, 10, 50},
		{"avg denominator", 3, 10, 8, 33.33},
		{"all left", 10, 10, 0, 200},
	}
	for _, c := range cases {
		got := turnoverRate(c.term, c.opening, c.close)
		if !almostEqual(got, c.want) {
			t.Errorf("%s: turnoverRate(%d,%d,%d)=%.2f want %.2f", c.name, c.term, c.opening, c.close, got, c.want)
		}
	}
}

func TestRetentionRate(t *testing.T) {
	if got := retentionRate(0, 10); !almostEqual(got, 100) {
		t.Errorf("no terminations should retain 100, got %.2f", got)
	}
	if got := retentionRate(2, 10); !almostEqual(got, 80) {
		t.Errorf("2 of 10 lost -> 80, got %.2f", got)
	}
	if got := retentionRate(0, 0); !almostEqual(got, 0) {
		t.Errorf("no opening population -> 0, got %.2f", got)
	}
	// Over-loss (terminations exceed opening) must clamp to 0, never go negative.
	if got := retentionRate(15, 10); !almostEqual(got, 0) {
		t.Errorf("should clamp to 0, got %.2f", got)
	}
}

func TestProjectHeadcount(t *testing.T) {
	cases := []struct {
		name              string
		base              int
		growth, attrition float64
		want              int
	}{
		{"no base", 0, 10, 0, 0},
		{"growth only", 100, 10, 0, 110},
		{"growth minus attrition", 100, 10, 10, 99},
		{"attrition only", 100, 0, 20, 80},
		{"full attrition clamps", 100, 0, 150, 0},
	}
	for _, c := range cases {
		got := projectHeadcount(c.base, c.growth, c.attrition)
		if got != c.want {
			t.Errorf("%s: projectHeadcount(%d,%.1f,%.1f)=%d want %d", c.name, c.base, c.growth, c.attrition, got, c.want)
		}
	}
}

func TestAverage(t *testing.T) {
	if got := average(nil); !almostEqual(got, 0) {
		t.Errorf("empty average should be 0, got %.2f", got)
	}
	if got := average([]float64{2, 4, 6}); !almostEqual(got, 4) {
		t.Errorf("mean of 2,4,6 want 4, got %.2f", got)
	}
}

func TestSafeDiv(t *testing.T) {
	if got := safeDiv(100, 0); !almostEqual(got, 0) {
		t.Errorf("divide by zero must return 0, got %.2f", got)
	}
	if got := safeDiv(100, 3); !almostEqual(got, 33.33) {
		t.Errorf("100/3 rounded want 33.33, got %.2f", got)
	}
}

func TestInPeriod(t *testing.T) {
	start := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.June, 30, 0, 0, 0, 0, time.UTC)
	if inPeriod(time.Time{}, start, end) {
		t.Errorf("zero date is never in period")
	}
	if !inPeriod(time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), start, end) {
		t.Errorf("mid-range date should be in period")
	}
	if inPeriod(time.Date(2025, time.December, 31, 0, 0, 0, 0, time.UTC), start, end) {
		t.Errorf("before-start date should not be in period")
	}
}
