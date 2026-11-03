package models

import "testing"

func TestQtyReconcile(t *testing.T) {
	cases := []struct {
		name                                            string
		ordered, delivered, accepted, rejected, damaged float64
		want                                            string
	}{
		{"balanced", 10, 10, 8, 1, 1, "balanced"},
		{"short", 10, 8, 8, 0, 0, "short"},
		{"over", 10, 12, 12, 0, 0, "over"},
		{"not_delivered", 10, 0, 0, 0, 0, "not_delivered"},
		{"gap", 10, 10, 6, 1, 1, "accounting_gap"}, // 6+1+1=8 != 10
		{"negative", 10, 10, -1, 0, 0, "negative"},
	}
	for _, c := range cases {
		if got := qtyReconcile(c.ordered, c.delivered, c.accepted, c.rejected, c.damaged); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestToleranceCheck(t *testing.T) {
	// 100 ordered, 103 delivered: abs diff 3 <= absTol 2? no; pct 5% => limit 5. 3<=5 ok.
	if breach, dir := toleranceCheck(100, 103, 2, 5); breach || dir != "over" {
		t.Errorf("103/100 within 5%%: got %v/%s want false/over", breach, dir)
	}
	// 100 ordered, 108 delivered: diff 8 > limit 5 => breach over.
	if breach, dir := toleranceCheck(100, 108, 2, 5); !breach || dir != "over" {
		t.Errorf("108/100: got %v/%s want true/over", breach, dir)
	}
	// 100 ordered, 97 delivered: diff 3 <= 5 => ok under.
	if breach, dir := toleranceCheck(100, 97, 2, 5); breach || dir != "under" {
		t.Errorf("97/100: got %v/%s want false/under", breach, dir)
	}
	// Small order where absolute tolerance dominates: 10 ordered, 12 delivered.
	// diff 2; pct limit 0.5; absTol 2 => limit max(2,0.5)=2; 2>2 false => not breach.
	if breach, _ := toleranceCheck(10, 12, 2, 5); breach {
		t.Errorf("12/10 within absTol 2 should not breach")
	}
	// Exact equality is never a breach.
	if breach, dir := toleranceCheck(10, 10, 2, 5); breach || dir != "equal" {
		t.Errorf("equal: got %v/%s want false/equal", breach, dir)
	}
}

func TestClassifyDiscrepancy(t *testing.T) {
	if got := classifyDiscrepancy(10, 8); got != DiscShort {
		t.Errorf("8/10 want short, got %s", got)
	}
	if got := classifyDiscrepancy(10, 12); got != DiscExcess {
		t.Errorf("12/10 want excess, got %s", got)
	}
	if got := classifyDiscrepancy(10, 10); got != "" {
		t.Errorf("10/10 want none, got %q", got)
	}
}

func TestPORemaining(t *testing.T) {
	if got := poRemaining(100, 40); got != 60 {
		t.Errorf("remaining = %v want 60", got)
	}
	if got := poRemaining(100, 120); got != 0 {
		t.Errorf("over-received remaining = %v want 0", got)
	}
}

func TestRound3(t *testing.T) {
	if got := round3(1.23456); got != 1.235 {
		t.Errorf("round3 = %v want 1.235", got)
	}
	if got := round3(2.0001); got != 2 {
		t.Errorf("round3 = %v want 2", got)
	}
}
