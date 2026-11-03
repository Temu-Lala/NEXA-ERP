package models

import "testing"

// HR-F12 pure-logic tests: the approval-decision and parameter-resolution rules the
// governance surfaces rely on. No database.

func TestApprovalRequired(t *testing.T) {
	cases := []struct {
		name              string
		amount, threshold float64
		levels            int
		want              bool
	}{
		{"no levels defined", 5000, 1000, 0, false},
		{"below positive threshold", 500, 1000, 2, false},
		{"at positive threshold", 1000, 1000, 2, false},
		{"above positive threshold", 1001, 1000, 2, true},
		{"zero threshold always approves", 0, 0, 1, true},
		{"zero threshold any amount", 999, 0, 3, true},
	}
	for _, c := range cases {
		got := approvalRequired(c.amount, c.threshold, c.levels)
		if got != c.want {
			t.Errorf("%s: approvalRequired(%.0f,%.0f,%d)=%v want %v", c.name, c.amount, c.threshold, c.levels, got, c.want)
		}
	}
}

func TestCountRequiredLevels(t *testing.T) {
	thresholds := []float64{0, 500, 1000, 2000}
	if got := countRequiredLevels(0, thresholds); got != 1 {
		t.Errorf("amount 0 should trigger only the no-minimum level, got %d", got)
	}
	if got := countRequiredLevels(1000, thresholds); got != 3 {
		t.Errorf("amount 1000 should trigger 0/500/1000 levels, got %d", got)
	}
	if got := countRequiredLevels(2500, thresholds); got != 4 {
		t.Errorf("amount above all thresholds should trigger all 4, got %d", got)
	}
	if got := countRequiredLevels(9999, nil); got != 0 {
		t.Errorf("no levels -> 0, got %d", got)
	}
}

func TestPickScopedValue(t *testing.T) {
	if v, s := pickScopedValue("dept", "comp", "glob"); v != "dept" || s != ScopeDepartment {
		t.Errorf("department should win, got %q/%s", v, s)
	}
	if v, s := pickScopedValue("  ", "comp", "glob"); v != "comp" || s != ScopeCompany {
		t.Errorf("blank dept should fall to company, got %q/%s", v, s)
	}
	if v, s := pickScopedValue("", "", "glob"); v != "glob" || s != ScopeGlobal {
		t.Errorf("only global should remain, got %q/%s", v, s)
	}
}

func TestParamCoercion(t *testing.T) {
	if !paramAsBool("YES") || !paramAsBool(" 1 ") || paramAsBool("no") {
		t.Errorf("paramAsBool mis-parsed")
	}
	if got := paramAsInt("42", -1); got != 42 {
		t.Errorf("paramAsInt want 42, got %d", got)
	}
	if got := paramAsInt("abc", 7); got != 7 {
		t.Errorf("paramAsInt should return default on error, got %d", got)
	}
	if got := paramAsFloat("3.14", 0); got != 3.14 {
		t.Errorf("paramAsFloat want 3.14, got %v", got)
	}
	if got := paramAsFloat("x", 1.5); got != 1.5 {
		t.Errorf("paramAsFloat should return default on error, got %v", got)
	}
}
