package models

import (
	"strings"
	"testing"
)

func TestOverlaps(t *testing.T) {
	cases := []struct {
		aStart, aDur, bStart, bDur int
		want                       bool
	}{
		{1, 1, 2, 1, false}, // adjacent, no overlap
		{1, 2, 2, 1, true},  // 1-2 vs 2-3 share period 2
		{1, 1, 1, 1, true},  // identical
		{3, 1, 1, 3, true},  // 3 within 1-3 range
		{5, 1, 1, 2, false}, // well apart
		{2, 0, 2, 0, true},  // zero durations normalise to 1
	}
	for _, c := range cases {
		if got := overlaps(c.aStart, c.aDur, c.bStart, c.bDur); got != c.want {
			t.Errorf("overlaps(%d,%d,%d,%d)=%v want %v", c.aStart, c.aDur, c.bStart, c.bDur, got, c.want)
		}
	}
}

func TestTeacherAvailable(t *testing.T) {
	if !teacherAvailable(nil, 3) {
		t.Error("empty allow-list means unrestricted")
	}
	if !teacherAvailable([]int{1, 2, 5}, 2) {
		t.Error("weekday 2 is allowed")
	}
	if teacherAvailable([]int{1, 2, 5}, 3) {
		t.Error("weekday 3 is not allowed")
	}
}

func TestWithinCapacity(t *testing.T) {
	if !withinCapacity(999, 0) {
		t.Error("capacity 0 means uncapped")
	}
	if !withinCapacity(20, 25) {
		t.Error("20 fits 25")
	}
	if withinCapacity(26, 25) {
		t.Error("26 exceeds 25")
	}
	if !withinCapacity(25, 25) {
		t.Error("exactly at capacity is allowed")
	}
}

func baseCand() entryView {
	return entryView{ID: 1, TeacherID: 10, SectionID: 100, RoomID: 5, Weekday: 1, Start: 2, Duration: 1, StudentCount: 20}
}

func TestFindConflictsAllRules(t *testing.T) {
	cand := baseCand()
	existing := []entryView{
		{ID: 2, TeacherID: 10, SectionID: 200, RoomID: 6, Weekday: 1, Start: 2, Duration: 1}, // teacher clash
		{ID: 3, TeacherID: 11, SectionID: 100, RoomID: 7, Weekday: 1, Start: 2, Duration: 1}, // section clash
		{ID: 4, TeacherID: 12, SectionID: 300, RoomID: 5, Weekday: 1, Start: 2, Duration: 1}, // room clash
	}
	got := findConflicts(cand, existing, []int{1, 2, 3}, 40)
	joined := strings.Join(got, "|")
	for _, want := range []string{"teacher double-booked", "section", "room"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected conflict mentioning %q, got %v", want, got)
		}
	}
	if len(got) != 3 {
		t.Errorf("expected exactly 3 conflicts, got %d: %v", len(got), got)
	}
}

func TestFindConflictsClean(t *testing.T) {
	cand := baseCand()
	existing := []entryView{
		{ID: 2, TeacherID: 10, SectionID: 100, RoomID: 5, Weekday: 1, Start: 5, Duration: 1}, // different period
		{ID: 1, TeacherID: 10, SectionID: 100, RoomID: 5, Weekday: 1, Start: 2, Duration: 1}, // same id (self) ignored
	}
	if got := findConflicts(cand, existing, nil, 0); len(got) != 0 {
		t.Errorf("expected no conflicts, got %v", got)
	}
}

func TestFindConflictsAvailabilityAndCapacity(t *testing.T) {
	cand := baseCand() // weekday 1, student count 20
	// No existing lessons, but teacher only allowed Tue-Thu and room caps at 15.
	got := findConflicts(cand, nil, []int{2, 3, 4}, 15)
	joined := strings.Join(got, "|")
	if !strings.Contains(joined, "not available") {
		t.Errorf("expected availability conflict, got %v", got)
	}
	if !strings.Contains(joined, "capacity") {
		t.Errorf("expected capacity conflict, got %v", got)
	}
	if len(got) != 2 {
		t.Errorf("expected exactly 2 conflicts, got %d: %v", len(got), got)
	}
}

func TestParseIntList(t *testing.T) {
	if got := parseIntList(""); got != nil {
		t.Errorf("empty should be nil, got %v", got)
	}
	got := parseIntList("1, 2,,x,4 ")
	want := []int{1, 2, 4}
	if len(got) != len(want) {
		t.Fatalf("parseIntList=%v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d got %d want %d", i, got[i], want[i])
		}
	}
}
