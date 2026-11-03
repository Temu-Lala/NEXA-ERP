package models

import "testing"

func TestAllowedTransition(t *testing.T) {
	ok := [][2]string{
		{"draft", "submitted"}, {"draft", "cancelled"},
		{"submitted", "under_review"}, {"submitted", "approved"}, {"submitted", "rejected"}, {"submitted", "cancelled"},
		{"under_review", "approved"}, {"under_review", "rejected"}, {"under_review", "cancelled"},
		{"approved", "fulfilled"},
	}
	for _, p := range ok {
		if !allowedTransition(p[0], p[1]) {
			t.Fatalf("%s -> %s should be allowed", p[0], p[1])
		}
	}
	bad := [][2]string{
		{"draft", "approved"}, {"draft", "fulfilled"},
		{"submitted", "draft"}, {"submitted", "fulfilled"},
		{"approved", "rejected"}, {"approved", "submitted"},
		{"rejected", "approved"}, {"fulfilled", "cancelled"}, {"cancelled", "submitted"},
	}
	for _, p := range bad {
		if allowedTransition(p[0], p[1]) {
			t.Fatalf("%s -> %s should be rejected", p[0], p[1])
		}
	}
}

func TestIsActionable(t *testing.T) {
	if !isActionable("submitted") || !isActionable("under_review") {
		t.Fatal("submitted and under_review are actionable")
	}
	for _, s := range []string{"draft", "approved", "rejected", "fulfilled", "cancelled"} {
		if isActionable(s) {
			t.Fatalf("%s should not be actionable", s)
		}
	}
}

func TestAgeDays(t *testing.T) {
	if got := ageDays("2026-01-01", "2026-01-11", "2026-06-01"); got != 10 {
		t.Fatalf("want 10 got %d", got)
	}
	// open request measures against today
	if got := ageDays("2026-01-01", "", "2026-01-06"); got != 5 {
		t.Fatalf("want 5 got %d", got)
	}
	if got := ageDays("", "", "2026-01-06"); got != 0 {
		t.Fatalf("blank start want 0 got %d", got)
	}
	// a future resolved date never yields negative age
	if got := ageDays("2026-01-10", "2026-01-01", "2026-01-01"); got != 0 {
		t.Fatalf("negative span want 0 got %d", got)
	}
	if got := ageDays("not-a-date", "", "2026-01-06"); got != 0 {
		t.Fatalf("unparseable want 0 got %d", got)
	}
}

func TestWidgetVisible(t *testing.T) {
	if !widgetVisible(true, false, false, "parent") {
		t.Fatal("parent tile should show to parent")
	}
	if widgetVisible(true, false, false, "student") {
		t.Fatal("parent-only tile should hide from student")
	}
	if !widgetVisible(false, true, true, "staff") {
		t.Fatal("staff tile should show to staff")
	}
	if widgetVisible(true, true, true, "unknown") {
		t.Fatal("unknown role sees nothing")
	}
}

func TestStudentInScope(t *testing.T) {
	linked := []int{11, 22, 33}
	if !studentInScope(11, 0, linked) {
		t.Fatal("linked learner should be in scope")
	}
	if !studentInScope(99, 99, linked) {
		t.Fatal("own student record should be in scope")
	}
	if studentInScope(55, 99, linked) {
		t.Fatal("unrelated learner should be out of scope")
	}
	if studentInScope(0, 99, linked) {
		t.Fatal("zero target is never in scope")
	}
	if studentInScope(11, 0, nil) {
		t.Fatal("no links means nothing but own is in scope")
	}
}
