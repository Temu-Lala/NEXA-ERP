package models

import (
	"testing"
	"time"
)

func TestSameSnapshot(t *testing.T) {
	open := map[string]interface{}{
		"department_id":   int64(5),
		"job_id":          int64(9),
		"manager_id":      int64(3),
		"employment_type": "full_time",
		"work_location":   " HQ ",
	}
	base := placement{department: 5, job: 9, manager: 3, employmentType: "full_time", workLoc: "HQ"}
	if !sameSnapshot(open, base) {
		t.Fatalf("expected equal snapshot (whitespace in stored location tolerated)")
	}
	if sameSnapshot(open, placement{department: 6, job: 9, manager: 3, employmentType: "full_time", workLoc: "HQ"}) {
		t.Fatalf("department change must not compare equal")
	}
	if sameSnapshot(open, placement{department: 5, job: 9, manager: 4, employmentType: "full_time", workLoc: "HQ"}) {
		t.Fatalf("manager change must not compare equal")
	}
	if sameSnapshot(open, placement{department: 5, job: 9, manager: 3, employmentType: "part_time", workLoc: "HQ"}) {
		t.Fatalf("employment-type change must not compare equal")
	}
}

func TestReasonFor(t *testing.T) {
	p := placement{department: 5, job: 9, manager: 3}
	if got := reasonFor(p, nil, true); got != HistoryHire {
		t.Fatalf("first snapshot reason = %v, want hire", got)
	}
	prev := map[string]interface{}{"department_id": int64(1), "job_id": int64(9), "manager_id": int64(3)}
	if got := reasonFor(p, prev, false); got != HistoryTransfer {
		t.Fatalf("department move reason = %v, want transfer", got)
	}
	same := map[string]interface{}{"department_id": int64(5), "job_id": int64(9), "manager_id": int64(3)}
	if got := reasonFor(p, same, false); got != HistoryAdjustment {
		t.Fatalf("non-org change reason = %v, want adjustment", got)
	}
}

func TestExpiryWord(t *testing.T) {
	today := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	if got := expiryWord(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), today); got != "expired" {
		t.Fatalf("past date word = %q, want expired", got)
	}
	if got := expiryWord(time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC), today); got != "expires" {
		t.Fatalf("future date word = %q, want expires", got)
	}
	if got := expiryWord(today, today); got != "expires" {
		t.Fatalf("same-day word = %q, want expires", got)
	}
}

func TestParseDate(t *testing.T) {
	if got := parseDate("2026-03-05"); !got.Equal(time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("plain date parse = %v", got)
	}
	if got := parseDate("2026-03-05 12:30:00"); got.Year() != 2026 || got.Month() != 3 || got.Day() != 5 {
		t.Fatalf("datetime parse should keep the day component, got %v", got)
	}
	if got := parseDate(""); !got.IsZero() {
		t.Fatalf("empty date should be zero time, got %v", got)
	}
	if got := parseDate(nil); !got.IsZero() {
		t.Fatalf("nil date should be zero time, got %v", got)
	}
}

func TestFirstEffective(t *testing.T) {
	if got := firstEffective(map[string]interface{}{"hire_date": "2024-01-10"}); got != "2024-01-10" {
		t.Fatalf("hire date should be honoured, got %q", got)
	}
	if got := firstEffective(map[string]interface{}{"hire_date": ""}); got != today() {
		t.Fatalf("empty hire date should fall back to today, got %q", got)
	}
}

func TestIntOr(t *testing.T) {
	if got := intOr(int64(42)); got != 42 {
		t.Fatalf("int64 coerce = %d", got)
	}
	if got := intOr("nope"); got != 0 {
		t.Fatalf("non-numeric should be 0, got %d", got)
	}
	if got := intOr(nil); got != 0 {
		t.Fatalf("nil should be 0, got %d", got)
	}
}
