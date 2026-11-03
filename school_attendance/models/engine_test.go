package models

import "testing"

func TestTallyStatuses(t *testing.T) {
	statuses := []string{"present", "absent", "present", "late", "excused", "absent", "unknown"}
	got := tallyStatuses(statuses)
	if got.present != 2 || got.absent != 2 || got.late != 1 || got.excused != 1 {
		t.Fatalf("tally = %+v, want present2 absent2 late1 excused1", got)
	}
	if got.total() != 6 {
		t.Fatalf("total = %d, want 6 (unknown ignored)", got.total())
	}
}

func TestAttendanceRate(t *testing.T) {
	if r := attendanceRate(0, 0, 0); r != 0 {
		t.Fatalf("empty rate = %v, want 0", r)
	}
	// present+late count; absent+excused do not. 8 of 10 present/late -> 80%.
	if r := attendanceRate(7, 1, 10); r != 80 {
		t.Fatalf("rate = %v, want 80", r)
	}
	// Rounds to two decimals: 1 of 3 -> 33.33.
	if r := attendanceRate(1, 0, 3); r != 33.33 {
		t.Fatalf("rate = %v, want 33.33", r)
	}
}

func TestFinalizeAllowed(t *testing.T) {
	if finalizeAllowed(0) {
		t.Fatal("an empty session must not be finalizable")
	}
	if !finalizeAllowed(1) {
		t.Fatal("a session with marks must be finalizable")
	}
}

func TestStatusChanged(t *testing.T) {
	if statusChanged("present", "present") {
		t.Fatal("same status is not a change")
	}
	if statusChanged("present", "") {
		t.Fatal("empty target is not a change")
	}
	if !statusChanged("present", "absent") {
		t.Fatal("present->absent is a change")
	}
}

func TestExceedsThreshold(t *testing.T) {
	if exceedsThreshold(5, 0) {
		t.Fatal("non-positive threshold must never fire")
	}
	if exceedsThreshold(2, 3) {
		t.Fatal("2 absences below threshold 3 must not fire")
	}
	if !exceedsThreshold(3, 3) {
		t.Fatal("3 absences at threshold 3 must fire")
	}
	if !exceedsThreshold(4, 3) {
		t.Fatal("4 absences above threshold 3 must fire")
	}
}
