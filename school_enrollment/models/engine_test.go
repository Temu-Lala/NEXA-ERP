package models

import "testing"

func TestEnrollmentAllowed(t *testing.T) {
	cases := []struct {
		action, from string
		want         bool
	}{
		{"confirm", "draft", true},
		{"confirm", "active", false},
		{"transfer", "active", true},
		{"transfer", "draft", false},
		{"withdraw", "active", true},
		{"withdraw", "graduated", false},
		{"graduate", "active", true},
		{"graduate", "withdrawn", false},
		{"cancel", "draft", true},
		{"cancel", "active", false},
		{"frobnicate", "draft", false},
	}
	for _, c := range cases {
		if got := allowed(c.action, c.from); got != c.want {
			t.Errorf("allowed(%q,%q)=%v want %v", c.action, c.from, got, c.want)
		}
	}
}

func TestEnrollmentTargetState(t *testing.T) {
	if got := targetState("confirm"); got != "active" {
		t.Errorf("targetState(confirm)=%q want active", got)
	}
	if got := targetState("graduate"); got != "graduated" {
		t.Errorf("targetState(graduate)=%q want graduated", got)
	}
	if got := targetState("nope"); got != "" {
		t.Errorf("targetState(nope)=%q want empty", got)
	}
}

func TestEnrollmentIsCurrent(t *testing.T) {
	today := "2026-06-15"
	if !isCurrent("active", "", today) {
		t.Error("active with no exit date is current")
	}
	if !isCurrent("active", "2026-12-31", today) {
		t.Error("active before exit date is current")
	}
	if isCurrent("active", "2026-06-01", today) {
		t.Error("active past its exit date is not current")
	}
	if isCurrent("graduated", "", today) {
		t.Error("non-active is never current")
	}
	if isCurrent("draft", "", today) {
		t.Error("draft is not current")
	}
}

func TestBlocksDuplicate(t *testing.T) {
	if blocksDuplicate(0) {
		t.Error("zero active should not block")
	}
	if !blocksDuplicate(1) {
		t.Error("an existing active enrollment should block")
	}
}
