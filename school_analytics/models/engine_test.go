package models

import "testing"

func TestRatePct(t *testing.T) {
	if got := ratePct(3, 4); got != 75 {
		t.Fatalf("want 75 got %v", got)
	}
	if got := ratePct(0, 10); got != 0 {
		t.Fatalf("zero numerator want 0 got %v", got)
	}
	// a zero (or negative) denominator must never divide
	if got := ratePct(5, 0); got != 0 {
		t.Fatalf("zero denominator want 0 got %v", got)
	}
	if got := ratePct(1, 3); got != 33.33 {
		t.Fatalf("want 33.33 got %v", got)
	}
}

func TestAverageAndSum(t *testing.T) {
	if got := averageOf([]float64{1, 2, 3}); got != 2 {
		t.Fatalf("want 2 got %v", got)
	}
	if got := averageOf(nil); got != 0 {
		t.Fatalf("empty want 0 got %v", got)
	}
	if got := sumOf([]float64{1.5, 2.25, 0.25}); got != 4 {
		t.Fatalf("want 4 got %v", got)
	}
	if got := averageOf([]float64{10, 7}); got != 8.5 {
		t.Fatalf("want 8.5 got %v", got)
	}
}

func TestTrendDirection(t *testing.T) {
	if got := trendDirection(60, 50, 0.01); got != "up" {
		t.Fatalf("want up got %s", got)
	}
	if got := trendDirection(40, 50, 0.01); got != "down" {
		t.Fatalf("want down got %s", got)
	}
	if got := trendDirection(50.005, 50, 0.01); got != "flat" {
		t.Fatalf("within tolerance want flat got %s", got)
	}
	// non-positive tolerance falls back to the default
	if got := trendDirection(50, 50, 0); got != "flat" {
		t.Fatalf("equal want flat got %s", got)
	}
}

func TestBand(t *testing.T) {
	if got := band(10, 50, 80); got != "critical" {
		t.Fatalf("below warn want critical got %s", got)
	}
	if got := band(90, 50, 80); got != "good" {
		t.Fatalf("at/above good want good got %s", got)
	}
	if got := band(60, 50, 80); got != "warning" {
		t.Fatalf("between want warning got %s", got)
	}
}

func TestCompareTriggers(t *testing.T) {
	cases := []struct {
		value, threshold float64
		cmp              string
		want             bool
	}{
		{10, 20, "lt", true},
		{20, 20, "lt", false},
		{20, 20, "lte", true},
		{30, 20, "gt", true},
		{20, 20, "gte", true},
		{19, 20, "gte", false},
		{10, 20, "bogus", false},
	}
	for _, c := range cases {
		if got := compareTriggers(c.value, c.threshold, c.cmp); got != c.want {
			t.Fatalf("%v %s %v want %v got %v", c.value, c.cmp, c.threshold, c.want, got)
		}
	}
}

func TestParseInList(t *testing.T) {
	got := parseInList("present, late ,")
	if len(got) != 2 || got[0] != "present" || got[1] != "late" {
		t.Fatalf("want [present late] got %#v", got)
	}
	if got := parseInList(""); len(got) != 0 {
		t.Fatalf("empty want 0 got %d", len(got))
	}
}

func TestFormatMetric(t *testing.T) {
	if got := formatMetric(75, "percent"); got != "75.0%" {
		t.Fatalf("want 75.0%% got %s", got)
	}
	if got := formatMetric(8.5, "average"); got != "8.50" {
		t.Fatalf("want 8.50 got %s", got)
	}
	if got := formatMetric(42, "count"); got != "42" {
		t.Fatalf("want 42 got %s", got)
	}
}

func TestCoerceScalar(t *testing.T) {
	if v := coerceScalar("true"); v != true {
		t.Fatalf("want true got %#v", v)
	}
	if v := coerceScalar("FALSE"); v != false {
		t.Fatalf("want false got %#v", v)
	}
	if v := coerceScalar("12"); v != 12 {
		t.Fatalf("want int 12 got %#v", v)
	}
	if v := coerceScalar("active"); v != "active" {
		t.Fatalf("want string active got %#v", v)
	}
}
