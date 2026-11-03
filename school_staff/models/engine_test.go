package models

import "testing"

func TestPerformanceBand(t *testing.T) {
	cases := []struct {
		score float64
		want  string
	}{
		{95, "Outstanding"},
		{90, "Outstanding"},
		{89, "Exceeds Expectations"},
		{75, "Exceeds Expectations"},
		{70, "Meets Expectations"},
		{60, "Meets Expectations"},
		{55, "Developing"},
		{40, "Developing"},
		{39, "Unsatisfactory"},
		{-5, "Unsatisfactory"},
	}
	for _, c := range cases {
		if got := performanceBand(c.score); got != c.want {
			t.Fatalf("performanceBand(%v) = %q, want %q", c.score, got, c.want)
		}
	}
}

func TestCoversDate(t *testing.T) {
	if !coversDate("2026-03-01", "2026-03-10", "2026-03-05") {
		t.Fatal("date inside range should match")
	}
	if !coversDate("2026-03-01", "2026-03-10", "2026-03-01") {
		t.Fatal("range start is inclusive")
	}
	if !coversDate("2026-03-01", "2026-03-10", "2026-03-10") {
		t.Fatal("range end is inclusive")
	}
	if coversDate("2026-03-01", "2026-03-10", "2026-03-11") {
		t.Fatal("date after range should not match")
	}
	if coversDate("", "2026-03-10", "2026-03-05") {
		t.Fatal("blank from never matches")
	}
	if !coversDate("2026-03-05", "", "2026-03-05") {
		t.Fatal("blank to is a single-day range that matches its start")
	}
	if coversDate("2026-03-05", "", "2026-03-06") {
		t.Fatal("single-day range should not cover the next day")
	}
}
