package models

import (
	"math"
	"testing"
)

func almost(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestRoundMoney(t *testing.T) {
	cases := []struct{ in, want float64 }{
		{3.333333, 3.33},
		{0.125, 0.13},
		{-0.125, -0.13},
		{2.5, 2.5},
		{0, 0},
		{1000, 1000},
	}
	for _, c := range cases {
		if got := roundMoney(c.in); !almost(got, c.want) {
			t.Errorf("roundMoney(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestRuleAmount(t *testing.T) {
	if got := ruleAmount("fixed", 0, 500, 1000, 2000); !almost(got, 500) {
		t.Errorf("fixed rule = %v, want 500", got)
	}
	if got := ruleAmount("percentage_basic", 20, 0, 1000, 2000); !almost(got, 200) {
		t.Errorf("pct_basic rule = %v, want 200", got)
	}
	if got := ruleAmount("percentage_gross", 10, 0, 1000, 2500); !almost(got, 250) {
		t.Errorf("pct_gross rule = %v, want 250", got)
	}
	// percentage_basic ignores gross.
	if got := ruleAmount("percentage_basic", 33.33, 0, 1000, 9999); !almost(got, 333.30) {
		t.Errorf("pct_basic 33.33%% of 1000 = %v, want 333.30", got)
	}
}

func TestStatutoryAmount(t *testing.T) {
	if got := statutoryAmount("percentage", "gross", 10, 0, 0, 1000, 3000); !almost(got, 300) {
		t.Errorf("10%% of gross = %v, want 300", got)
	}
	if got := statutoryAmount("percentage", "basic", 5, 0, 0, 1000, 3000); !almost(got, 50) {
		t.Errorf("5%% of basic = %v, want 50", got)
	}
	if got := statutoryAmount("fixed", "gross", 0, 50, 0, 1000, 3000); !almost(got, 50) {
		t.Errorf("fixed statutory = %v, want 50", got)
	}
	// Cap applies: 30% of 1000 = 300, capped at 100.
	if got := statutoryAmount("percentage", "gross", 30, 0, 100, 0, 1000); !almost(got, 100) {
		t.Errorf("capped statutory = %v, want 100", got)
	}
}

func TestNetOf(t *testing.T) {
	if got := netOf(3000, 500); !almost(got, 2500) {
		t.Errorf("netOf(3000,500) = %v, want 2500", got)
	}
	if got := netOf(100, 200); !almost(got, 0) {
		t.Errorf("netOf floor at zero = %v, want 0", got)
	}
}

func TestActiveInPeriod(t *testing.T) {
	cases := []struct {
		from, to, target string
		want             bool
	}{
		{"2024-01-01", "2024-12-31", "2024-06-01", true},
		{"2024-01-01", "2024-12-31", "2025-01-01", false},
		{"", "", "2024-06-01", true},
		{"2024-06-01", "", "2024-01-01", false},
		{"", "2024-05-31", "2024-06-01", false},
		{"2024-01-01", "2024-12-31", "", true}, // no target -> treat as covering
	}
	for _, c := range cases {
		if got := activeInPeriod(c.from, c.to, c.target); got != c.want {
			t.Errorf("activeInPeriod(%q,%q,%q) = %v, want %v", c.from, c.to, c.target, got, c.want)
		}
	}
}
