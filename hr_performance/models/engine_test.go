package models

import (
	"math"
	"testing"
)

func approxEq(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestAchievementPct(t *testing.T) {
	cases := []struct {
		name        string
		target, act float64
		want        float64
	}{
		{"zero target yields zero", 0, 50, 0},
		{"negative target yields zero", -10, 5, 0},
		{"exact target", 100, 100, 100},
		{"half", 200, 50, 25},
		{"over target", 100, 150, 150},
		{"rounds to 2dp", 3, 1, 33.33},
	}
	for _, c := range cases {
		if got := achievementPct(c.target, c.act); !approxEq(got, c.want) {
			t.Errorf("%s: achievementPct(%v,%v)=%v want %v", c.name, c.target, c.act, got, c.want)
		}
	}
}

func TestWeightedAverage(t *testing.T) {
	cases := []struct {
		name            string
		scores, weights []float64
		want            float64
	}{
		{"empty", nil, nil, 0},
		{"zero weights", []float64{5, 4}, []float64{0, 0}, 0},
		{"equal weights", []float64{5, 4}, []float64{1, 1}, 4.5},
		{"unequal weights", []float64{5, 4}, []float64{3, 1}, 4.75},
		{"skips non-positive weight", []float64{5, 1, 4}, []float64{1, 0, 1}, 4.5},
		{"mismatched lengths use shared prefix", []float64{5, 3}, []float64{1}, 5},
	}
	for _, c := range cases {
		if got := weightedAverage(c.scores, c.weights); !approxEq(got, c.want) {
			t.Errorf("%s: weightedAverage(%v,%v)=%v want %v", c.name, c.scores, c.weights, got, c.want)
		}
	}
}

func TestCalibrationDelta(t *testing.T) {
	cases := []struct {
		proposed, calibrated, want float64
	}{
		{4.0, 4.0, 0},
		{3.5, 4.0, 0.5},
		{4.5, 4.0, -0.5},
		{3.33, 4.0, 0.67},
	}
	for _, c := range cases {
		if got := calibrationDelta(c.proposed, c.calibrated); !approxEq(got, c.want) {
			t.Errorf("calibrationDelta(%v,%v)=%v want %v", c.proposed, c.calibrated, got, c.want)
		}
	}
}

func TestRound2(t *testing.T) {
	cases := []struct{ in, want float64 }{
		{2.678, 2.68},
		{2.674, 2.67},
		{-2.678, -2.68},
		{3.333, 3.33},
		{1.0, 1.0},
	}
	for _, c := range cases {
		if got := round2(c.in); !approxEq(got, c.want) {
			t.Errorf("round2(%v)=%v want %v", c.in, got, c.want)
		}
	}
}
