package models

import (
	"math"
	"testing"
)

func almost(a, b float64) bool { return math.Abs(a-b) < 0.005 }

func TestRemainingBudget(t *testing.T) {
	if got := remainingBudget(1000, 300, 200); !almost(got, 500) {
		t.Errorf("remaining = %v, want 500", got)
	}
	if got := remainingBudget(1000, 900, 200); !almost(got, -100) {
		t.Errorf("over-committed remaining = %v, want -100", got)
	}
}

func TestBudgetDecision(t *testing.T) {
	// Over budget by 100 on a 1000 approved (tolerance 5% => limit 50).
	if r, blocked := budgetDecision(PolicyHard, 1000, 0, 0, 1100, 5); r != BudgetExceed || !blocked {
		t.Errorf("hard over: got %v/%v, want exceeded/true", r, blocked)
	}
	if r, blocked := budgetDecision(PolicySoft, 1000, 0, 0, 1100, 5); r != BudgetWarning || blocked {
		t.Errorf("soft over: got %v/%v, want warning/false", r, blocked)
	}
	// Conditional: 30 over is within the 50 tolerance -> warning; 100 over -> exceeded.
	if r, blocked := budgetDecision(PolicyConditional, 1000, 0, 0, 1030, 5); r != BudgetWarning || blocked {
		t.Errorf("conditional within tolerance: got %v/%v, want warning/false", r, blocked)
	}
	if r, blocked := budgetDecision(PolicyConditional, 1000, 0, 0, 1100, 5); r != BudgetExceed || !blocked {
		t.Errorf("conditional over tolerance: got %v/%v, want exceeded/true", r, blocked)
	}
	if r, blocked := budgetDecision(PolicyHard, 1000, 0, 0, 900, 5); r != BudgetOK || blocked {
		t.Errorf("within budget: got %v/%v, want ok/false", r, blocked)
	}
	if r, blocked := budgetDecision(PolicyNoBudget, 0, 0, 0, 5000, 5); r != BudgetNA || blocked {
		t.Errorf("no-budget: got %v/%v, want not_checked/false", r, blocked)
	}
}

func TestNetCommitment(t *testing.T) {
	if got := netCommitment(500, 100, 50); !almost(got, 350) {
		t.Errorf("net commitment = %v, want 350", got)
	}
	// Full consumption or release removes the open commitment.
	if got := netCommitment(500, 500, 0); got != 0 {
		t.Errorf("consumed net = %v, want 0", got)
	}
}

func TestSelectBands(t *testing.T) {
	bands := []amountBand{
		{from: 0, to: 1000, level: 1},
		{from: 1000, to: 5000, level: 2},
		{from: 5000, to: 0, level: 3}, // unbounded above
	}
	check := func(amount float64, want int) {
		got := selectBands(bands, amount)
		if len(got) != 1 || got[0] != want {
			t.Errorf("amount %.0f -> %v, want [%d]", amount, got, want)
		}
	}
	check(500, 1)
	check(1000, 2) // lower bound inclusive, [1000,5000)
	check(4999, 2)
	check(5000, 3) // unbounded band picks up the boundary
	check(99999, 3)
	if got := selectBands(nil, 500); len(got) != 0 {
		t.Errorf("no bands should yield no levels, got %v", got)
	}
}

func TestDetectSplitToEvade(t *testing.T) {
	if !detectSplitToEvade([]float64{400, 400}, 500) {
		t.Error("two sub-threshold items summing over threshold should flag")
	}
	if detectSplitToEvade([]float64{200, 200}, 500) {
		t.Error("sum under threshold should not flag")
	}
	if detectSplitToEvade([]float64{600, 100}, 500) {
		t.Error("an item already over threshold is not a split pattern")
	}
	if detectSplitToEvade([]float64{400}, 500) {
		t.Error("single item should not flag")
	}
	if detectSplitToEvade([]float64{400, 400}, 0) {
		t.Error("zero threshold should not flag")
	}
}

func TestRound2(t *testing.T) {
	if got := round2(123.456); !almost(got, 123.46) {
		t.Errorf("round2(123.456) = %v, want 123.46", got)
	}
	if got := round2(10.004); !almost(got, 10) {
		t.Errorf("round2(10.004) = %v, want 10.00", got)
	}
}
