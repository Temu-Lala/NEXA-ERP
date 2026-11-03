package models

import (
	"math"
	"testing"
)

func almost(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestContribution(t *testing.T) {
	if got := contribution(string(ContributionFixed), 500, 40000, 0); !almost(got, 500) {
		t.Errorf("fixed = %v, want 500", got)
	}
	if got := contribution(string(ContributionPct), 10, 40000, 0); !almost(got, 4000) {
		t.Errorf("10%% of 40000 = %v, want 4000", got)
	}
	// cap clamps the percentage amount.
	if got := contribution(string(ContributionPct), 10, 40000, 2000); !almost(got, 2000) {
		t.Errorf("capped = %v, want 2000", got)
	}
	// fixed above cap is also clamped.
	if got := contribution(string(ContributionFixed), 5000, 40000, 2000); !almost(got, 2000) {
		t.Errorf("fixed over cap = %v, want 2000", got)
	}
	// cap 0 means no cap.
	if got := contribution(string(ContributionPct), 50, 100000, 0); !almost(got, 50000) {
		t.Errorf("no-cap = %v, want 50000", got)
	}
}

func TestEmployeeContribution(t *testing.T) {
	if got := employeeContribution(7.5, 40000); !almost(got, 3000) {
		t.Errorf("7.5%% of 40000 = %v, want 3000", got)
	}
	if got := employeeContribution(0, 40000); !almost(got, 0) {
		t.Errorf("0%% = %v, want 0", got)
	}
}

func TestIncreasePct(t *testing.T) {
	if got := increasePct(50000, 55000); !almost(got, 10) {
		t.Errorf("50000->55000 = %v, want 10", got)
	}
	if got := increasePct(0, 55000); !almost(got, 0) {
		t.Errorf("zero base must be 0, got %v", got)
	}
	if got := increasePct(50000, 45000); !almost(got, -10) {
		t.Errorf("decrease = %v, want -10", got)
	}
}

func TestWithinBand(t *testing.T) {
	cases := []struct {
		salary, min, max float64
		want             bool
	}{
		{50000, 40000, 60000, true},
		{39999, 40000, 60000, false},
		{60001, 40000, 60000, false},
		{100, 0, 0, true},    // unbounded band
		{100, 200, 0, false}, // below min
		{100, 0, 50, false},  // above max
		{0, 0, 0, true},      // all zeros pass
	}
	for _, c := range cases {
		if got := withinBand(c.salary, c.min, c.max); got != c.want {
			t.Errorf("withinBand(%v,%v,%v) = %v, want %v", c.salary, c.min, c.max, got, c.want)
		}
	}
}

func TestTenureDaysOK(t *testing.T) {
	if !tenureDaysOK("2020-01-01", "2020-01-10", 0) {
		t.Error("min 0 must always pass")
	}
	if !tenureDaysOK("2020-01-01", "2020-06-01", 90) {
		t.Error("151 days >= 90 should pass")
	}
	if tenureDaysOK("2020-01-01", "2020-01-10", 90) {
		t.Error("9 days < 90 should fail")
	}
	if tenureDaysOK("", "2020-06-01", 90) {
		t.Error("missing hire date with a min requirement should fail")
	}
	if !tenureDaysOK("", "2020-06-01", 0) {
		t.Error("missing hire date with no requirement should pass")
	}
}

func TestActiveInPeriod(t *testing.T) {
	if !activeInPeriod("", "", "2024-05-01") {
		t.Error("un-dated row must always qualify")
	}
	if activeInPeriod("2024-06-01", "", "2024-05-01") {
		t.Error("start after target should not qualify")
	}
	if activeInPeriod("", "2024-04-01", "2024-05-01") {
		t.Error("end before target should not qualify")
	}
	if !activeInPeriod("2024-01-01", "2024-12-31", "2024-05-01") {
		t.Error("in-window should qualify")
	}
}

func TestRoundMoney(t *testing.T) {
	cases := []struct{ in, want float64 }{
		{1.0, 1.0},
		{2.678, 2.68},
		{2.674, 2.67},
		{-2.678, -2.68},
		{3.333, 3.33},
	}
	for _, c := range cases {
		if got := roundMoney(c.in); !almost(got, c.want) {
			t.Errorf("roundMoney(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}
