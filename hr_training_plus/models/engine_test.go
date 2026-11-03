package models

import (
	"math"
	"testing"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestAttemptPct(t *testing.T) {
	cases := []struct {
		total, max, want float64
	}{
		{0, 0, 0},
		{50, 0, 0},
		{10, 10, 100},
		{5, 10, 50},
		{7, 3, 233.33},
		{1, 3, 33.33},
	}
	for _, c := range cases {
		if got := attemptPct(c.total, c.max); !approx(got, c.want) {
			t.Errorf("attemptPct(%v,%v)=%v want %v", c.total, c.max, got, c.want)
		}
	}
}

func TestPassedFor(t *testing.T) {
	cases := []struct {
		pct, passMark float64
		want          bool
	}{
		{60, 60, true},
		{59.99, 60, false},
		{100, 50, true},
		{0, 0, false},   // unset pass mark is not a gate
		{50, -1, false}, // negative pass mark treated as unset
	}
	for _, c := range cases {
		if got := passedFor(c.pct, c.passMark); got != c.want {
			t.Errorf("passedFor(%v,%v)=%v want %v", c.pct, c.passMark, got, c.want)
		}
	}
}

func TestLevelIndex(t *testing.T) {
	cases := map[string]int{
		"beginner": 0, "intermediate": 1, "advanced": 2, "expert": 3, "unknown": 0, "": 0,
	}
	for lvl, want := range cases {
		if got := levelIndex(lvl); got != want {
			t.Errorf("levelIndex(%q)=%d want %d", lvl, got, want)
		}
	}
}

func TestGapBetween(t *testing.T) {
	cases := []struct {
		cur, req string
		want     int
	}{
		{"beginner", "expert", 3},
		{"advanced", "intermediate", 0}, // exceeding requirement is not a gap
		{"expert", "expert", 0},
		{"intermediate", "advanced", 1},
		{"", "advanced", 2},
	}
	for _, c := range cases {
		if got := gapBetween(c.cur, c.req); got != c.want {
			t.Errorf("gapBetween(%q,%q)=%d want %d", c.cur, c.req, got, c.want)
		}
	}
}

func TestRound2(t *testing.T) {
	cases := []struct{ in, want float64 }{
		{2.678, 2.68},
		{2.674, 2.67},
		{-2.678, -2.68},
		{3.333, 3.33},
	}
	for _, c := range cases {
		if got := round2(c.in); !approx(got, c.want) {
			t.Errorf("round2(%v)=%v want %v", c.in, got, c.want)
		}
	}
}
