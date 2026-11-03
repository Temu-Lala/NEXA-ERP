package models

import (
	"testing"
)

func TestRound2(t *testing.T) {
	cases := []struct {
		in, want float64
	}{
		{3.14159, 3.14},
		{-3.14159, -3.14},
		{0.001, 0},
		{0, 0},
		{0.5, 0.5},
		{3.0, 3},
	}
	for _, c := range cases {
		if got := round2(c.in); got != c.want {
			t.Fatalf("round2(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestRequestDays(t *testing.T) {
	rec := map[string]interface{}{"date_from": "2026-03-10", "date_to": "2026-03-12"}
	if got := requestDays(rec); got != 3 {
		t.Fatalf("inclusive span should be 3 days, got %v", got)
	}
	same := map[string]interface{}{"date_from": "2026-03-10", "date_to": "2026-03-10"}
	if got := requestDays(same); got != 1 {
		t.Fatalf("single day should be 1, got %v", got)
	}
	empty := map[string]interface{}{"date_from": "", "date_to": "2026-03-10"}
	if got := requestDays(empty); got != 0 {
		t.Fatalf("missing date should be 0, got %v", got)
	}
}

func TestEffectiveLeaveDays(t *testing.T) {
	// Hours take precedence: 4h = 0.5 day.
	hours := map[string]interface{}{"hours": 4.0, "date_from": "2026-03-10", "date_to": "2026-03-10"}
	if got := effectiveLeaveDays(hours); got != 0.5 {
		t.Fatalf("4 hours should be 0.5 day, got %v", got)
	}
	// Half day of a 2-day span = 1.0.
	half := map[string]interface{}{"hours": 0.0, "half_day": true, "date_from": "2026-03-10", "date_to": "2026-03-11"}
	if got := effectiveLeaveDays(half); got != 1 {
		t.Fatalf("half of 2 days should be 1, got %v", got)
	}
	// Whole-day request.
	full := map[string]interface{}{"hours": 0.0, "half_day": false, "date_from": "2026-03-10", "date_to": "2026-03-12"}
	if got := effectiveLeaveDays(full); got != 3 {
		t.Fatalf("full 3-day request should be 3, got %v", got)
	}
}

func TestPeriodDate(t *testing.T) {
	got := periodDate()
	if len(got) != 10 || got[8:10] != "01" {
		t.Fatalf("period date should be first-of-month YYYY-MM-01, got %q", got)
	}
}
