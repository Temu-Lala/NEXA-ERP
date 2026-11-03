package models

import (
	"context"
	"testing"
)

// The roll-up is held at the two-decimal scale hours are entered on, so a sum of
// thirds never leaks floating-point noise into a number someone reads as effort.
func TestWorklogRound2(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
	}{
		{0, 0},
		{1.006, 1.01},
		{1.234, 1.23},
		{2.5, 2.5},
		{12.999, 13.00},
		{-0.001, 0},
	}
	for _, c := range cases {
		if got := round2(c.in); got != c.want {
			t.Errorf("round2(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

// Logged time is a fact about work done: a negative entry would subtract effort
// that was really spent, and a non-numeric one cannot be summed. Both are refused
// at the write rather than corrected after the fact.
func TestConstraintWorklogHours(t *testing.T) {
	ctx := context.Background()
	if err := constraintWorklogHours(ctx, map[string]interface{}{"hours": 3.5}); err != nil {
		t.Errorf("positive hours should pass, got %v", err)
	}
	if err := constraintWorklogHours(ctx, map[string]interface{}{"hours": 0}); err != nil {
		t.Errorf("zero hours should pass, got %v", err)
	}
	if err := constraintWorklogHours(ctx, map[string]interface{}{"hours": -1}); err == nil {
		t.Error("negative hours should be refused")
	}
	if err := constraintWorklogHours(ctx, map[string]interface{}{"hours": "abc"}); err == nil {
		t.Error("non-numeric hours should be refused")
	}
}
