package models

import "testing"

func TestMatchesOperator(t *testing.T) {
	cases := []struct {
		name     string
		actual   string
		op       ConditionOperator
		expected string
		want     bool
	}{
		{"equals", "urgent", OpEquals, "Urgent", true},
		{"equals-false", "high", OpEquals, "urgent", false},
		{"not-equals", "low", OpNotEquals, "urgent", true},
		{"contains", "payment outage reported", OpContains, "payment", true},
		{"contains-none", "login issue", OpContains, "payment", false},
		{"is-empty", "", OpIsEmpty, "", true},
		{"is-empty-blank", "   ", OpIsEmpty, "", true},
		{"is-not-empty", "x", OpIsNotEmpty, "", true},
		{"in", "high", OpIn, "high,urgent", true},
		{"in-miss", "low", OpIn, "high,urgent", false},
		{"not-in", "low", OpNotIn, "high,urgent", true},
		{"gt", "5", OpGreaterThan, "3", true},
		{"gt-eq", "3", OpGreaterThan, "3", false},
		{"lt", "2", OpLessThan, "3", true},
		{"gt-nonnumeric", "abc", OpGreaterThan, "3", false},
	}
	for _, c := range cases {
		if got := matchesOperator(c.actual, c.op, c.expected); got != c.want {
			t.Errorf("%s: matchesOperator(%q,%q,%q)=%v want %v", c.name, c.actual, c.op, c.expected, got, c.want)
		}
	}
}

func TestCombineResults(t *testing.T) {
	if !combineResults(nil, MatchAll) {
		t.Error("no conditions should match")
	}
	if !combineResults([]bool{true, false}, MatchAny) {
		t.Error("any should match with one true")
	}
	if combineResults([]bool{true, false}, MatchAll) {
		t.Error("all should not match with one false")
	}
	if !combineResults([]bool{true, true}, MatchAll) {
		t.Error("all should match when all true")
	}
	if combineResults([]bool{false, false}, MatchAny) {
		t.Error("any should not match when all false")
	}
}

func TestTriggerForHistoryField(t *testing.T) {
	if tr, ok := triggerForHistoryField("state"); !ok || tr != TriggerStateChanged {
		t.Errorf("state -> %q,%v", tr, ok)
	}
	if tr, ok := triggerForHistoryField("priority"); !ok || tr != TriggerPriorityChanged {
		t.Errorf("priority -> %q,%v", tr, ok)
	}
	if tr, ok := triggerForHistoryField("assigned_to"); !ok || tr != TriggerAssigneeChanged {
		t.Errorf("assigned_to -> %q,%v", tr, ok)
	}
	if _, ok := triggerForHistoryField("description"); ok {
		t.Error("description should not map to a *_changed trigger")
	}
}

func TestBumpPriority(t *testing.T) {
	steps := map[string]string{"low": "normal", "normal": "high", "high": "urgent", "urgent": "urgent", "": "normal"}
	for in, want := range steps {
		if got := bumpPriority(in); got != want {
			t.Errorf("bumpPriority(%q)=%q want %q", in, got, want)
		}
	}
}

func TestNextVersionNumber(t *testing.T) {
	if nextVersionNumber(0) != 1 {
		t.Error("first publish should be v1")
	}
	if nextVersionNumber(3) != 4 {
		t.Error("should increment")
	}
	if nextVersionNumber(-1) != 1 {
		t.Error("negative clamps to 1")
	}
}

func TestShouldExecute(t *testing.T) {
	if shouldExecute(true) {
		t.Error("dry-run must not execute")
	}
	if !shouldExecute(false) {
		t.Error("non-dry-run must execute")
	}
}

func TestInSetAndParseFloat(t *testing.T) {
	if !inSet("3", "1, 3 ,5") {
		t.Error("membership should trim")
	}
	if inSet("", "1,3") {
		t.Error("empty actual is never in set")
	}
	if _, ok := parseFloat("x"); ok {
		t.Error("non-numeric should fail parse")
	}
}
