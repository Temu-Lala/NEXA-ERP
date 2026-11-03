package models

import "testing"

// Unit tests for the pure condition-monitoring decisions. These guard the
// threshold comparison operators and the hysteresis clear band, which are the
// subtlest parts of the condition-evaluation cron.

func TestCompareCondition(t *testing.T) {
	cases := []struct {
		name string
		val  float64
		op   ConditionRuleOperator
		thr  float64
		want bool
	}{
		{"gt breach", 80, CondOpGt, 75, true},
		{"gt not breach", 70, CondOpGt, 75, false},
		{"gte equal", 75, CondOpGte, 75, true},
		{"lt breach", 10, CondOpLt, 20, true},
		{"lte equal", 20, CondOpLte, 20, true},
		{"eq within eps", 20.0000000001, CondOpEq, 20, true},
		{"neq", 21, CondOpNeq, 20, true},
		{"neq equal", 20, CondOpNeq, 20, false},
	}
	for _, c := range cases {
		if got := compareCondition(c.val, c.op, c.thr); got != c.want {
			t.Errorf("%s: compareCondition(%v,%s,%v)=%v want %v", c.name, c.val, c.op, c.thr, got, c.want)
		}
	}
}

func TestWithinClear(t *testing.T) {
	cases := []struct {
		name string
		val  float64
		op   ConditionRuleOperator
		thr  float64
		hyst float64
		want bool
	}{
		{"gt not below band yet", 74, CondOpGt, 75, 2, false}, // 74 <= 73 -> false
		{"gt clear past band", 72, CondOpGt, 75, 2, true},     // 72 <= 73 -> true
		{"lt clear above band", 22, CondOpLt, 20, 2, true},    // 22 >= 22 -> true
		{"lt still low", 19, CondOpLt, 20, 2, false},          // 19 >= 22 -> false
		{"eq clears immediately", 21, CondOpEq, 20, 0, true},  // not equal -> clear
	}
	for _, c := range cases {
		if got := withinClear(c.val, c.op, c.thr, c.hyst); got != c.want {
			t.Errorf("%s: withinClear(%v,%s,%v,%v)=%v want %v", c.name, c.val, c.op, c.thr, c.hyst, got, c.want)
		}
	}
}
