package models

import (
	"math"
	"testing"
)

func almost(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestWeightedScore(t *testing.T) {
	// Equal weights: mean of scores and pct of maxima.
	lines := []map[string]interface{}{
		{"score": 4.0, "max_score": 5.0, "weight": 1.0},
		{"score": 5.0, "max_score": 5.0, "weight": 1.0},
	}
	avg, pct, rec := weightedScore(lines)
	if !almost(avg, 4.5) || !almost(pct, 90) || !rec {
		t.Errorf("equal-weight: avg=%v pct=%v rec=%v, want 4.5/90/true", avg, pct, rec)
	}

	// Weighted: heavy low score drags pct under the gate.
	lines2 := []map[string]interface{}{
		{"score": 2.0, "max_score": 5.0, "weight": 2.0},
		{"score": 5.0, "max_score": 5.0, "weight": 1.0},
	}
	avg2, pct2, rec2 := weightedScore(lines2)
	if !almost(pct2, 60) || rec2 {
		t.Errorf("weighted: pct=%v rec=%v, want 60/false", pct2, rec2)
	}
	if !almost(avg2, round2(7.0/3.0)) {
		t.Errorf("weighted avg=%v, want %v", avg2, round2(7.0/3.0))
	}

	// Zero weight is treated as 1.
	lines3 := []map[string]interface{}{{"score": 3.0, "max_score": 5.0, "weight": 0.0}}
	avg3, pct3, rec3 := weightedScore(lines3)
	if !almost(avg3, 3) || !almost(pct3, 60) || rec3 {
		t.Errorf("zero weight: avg=%v pct=%v rec=%v, want 3/60/false", avg3, pct3, rec3)
	}

	// Empty.
	if avg, pct, rec := weightedScore(nil); avg != 0 || pct != 0 || rec {
		t.Errorf("empty lines should yield 0/0/false")
	}
}

func TestRenderTemplate(t *testing.T) {
	body := "Hello {{applicant}}, your salary is {{salary}} starting {{start_date}}."
	vars := map[string]string{"applicant": "Al", "salary": "5000.00", "start_date": "2024-06-01"}
	got := renderTemplate(body, vars)
	want := "Hello Al, your salary is 5000.00 starting 2024-06-01."
	if got != want {
		t.Errorf("renderTemplate = %q, want %q", got, want)
	}
	// Unknown tokens are left untouched.
	if out := renderTemplate("{{a}} {{b}}", map[string]string{"a": "x"}); out != "x {{b}}" {
		t.Errorf("unknown token handling wrong: %q", out)
	}
}

func TestNormalizeEmail(t *testing.T) {
	if got := normalizeEmail("  A@B.COM "); got != "a@b.com" {
		t.Errorf("normalizeEmail = %q, want a@b.com", got)
	}
	if got := normalizeEmail(""); got != "" {
		t.Errorf("empty email should stay empty, got %q", got)
	}
}

func TestAddDays(t *testing.T) {
	if got := addDays("2024-01-01", 30); got != "2024-01-31" {
		t.Errorf("addDays(+30) = %q, want 2024-01-31", got)
	}
	if got := addDays("2024-01-01", 0); got != "2024-01-01" {
		t.Errorf("addDays(0) = %q, want 2024-01-01", got)
	}
	if got := addDays("", 5); got != "" {
		t.Errorf("addDays on empty date = %q, want empty", got)
	}
}

func TestRound2(t *testing.T) {
	cases := []struct{ in, want float64 }{
		{3.14159, 3.14},
		{-3.14159, -3.14},
		{0.001, 0},
		{2.5, 2.5},
		{0.125, 0.13},
	}
	for _, c := range cases {
		if got := round2(c.in); !almost(got, c.want) {
			t.Errorf("round2(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}
