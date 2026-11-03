package models

import (
	"testing"
	"time"
)

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

func TestAddDays(t *testing.T) {
	if got := addDays(time.Time{}, 10); !got.IsZero() {
		t.Fatalf("zero input must yield zero, got %v", got)
	}
	if got := addDays(d(2026, time.January, 31), 30); !got.Equal(d(2026, time.March, 2)) {
		t.Fatalf("expected Mar 2, got %v", got)
	}
}

func TestDeriveLastWorkingDay(t *testing.T) {
	init := d(2026, time.February, 1)
	if got := deriveLastWorkingDay(time.Time{}, 30); !got.IsZero() {
		t.Fatalf("no initiation date => no last working day")
	}
	if got := deriveLastWorkingDay(init, 0); !got.Equal(init) {
		t.Fatalf("zero notice should equal init date, got %v", got)
	}
	if got := deriveLastWorkingDay(init, 14); !got.Equal(d(2026, time.February, 15)) {
		t.Fatalf("expected Feb 15, got %v", got)
	}
}

func TestComplianceStatus(t *testing.T) {
	today := d(2026, time.June, 1)

	// Never completed stays pending, whatever the cadence.
	if state, due := complianceStatus(90, time.Time{}, today); state != CompPending || !due.IsZero() {
		t.Fatalf("never-completed must be pending with no due, got %v/%v", state, due)
	}

	// One-time requirement, once completed, stays compliant.
	if state, due := complianceStatus(0, d(2020, time.January, 1), today); state != CompCompliant || !due.IsZero() {
		t.Fatalf("one-time completed should be compliant/no due, got %v/%v", state, due)
	}

	// Renewal not yet due -> compliant, with a future due date.
	last := d(2026, time.May, 1)
	state, due := complianceStatus(60, last, today)
	if state != CompCompliant {
		t.Fatalf("expected compliant before due, got %v", state)
	}
	if want := d(2026, time.June, 30); !due.Equal(want) {
		t.Fatalf("expected due Jun 30, got %v", due)
	}

	// Renewal past due -> expired.
	state, due = complianceStatus(30, last, today)
	if state != CompExpired {
		t.Fatalf("expected expired after due, got %v", state)
	}
	if want := d(2026, time.May, 31); !due.Equal(want) {
		t.Fatalf("expected due May 31, got %v", due)
	}
}

func TestTermAllowed(t *testing.T) {
	valid := [][2]string{
		{"draft", "submitted"},
		{"draft", "cancelled"},
		{"submitted", "approved"},
		{"submitted", "draft"}, // rejection returns to draft
		{"approved", "completed"},
		{"approved", "cancelled"},
	}
	for _, p := range valid {
		if !termAllowed(p[0], p[1]) {
			t.Fatalf("expected %s->%s to be allowed", p[0], p[1])
		}
	}
	invalid := [][2]string{
		{"draft", "approved"},
		{"draft", "completed"},
		{"completed", "draft"},
		{"cancelled", "submitted"},
		{"submitted", "completed"},
	}
	for _, p := range invalid {
		if termAllowed(p[0], p[1]) {
			t.Fatalf("expected %s->%s to be rejected", p[0], p[1])
		}
	}
}

func TestCaseAllowed(t *testing.T) {
	if !caseAllowed("draft", "submitted") {
		t.Fatal("draft->submitted should be allowed")
	}
	if !caseAllowed("submitted", "investigating") {
		t.Fatal("submitted->investigating should be allowed")
	}
	if !caseAllowed("resolved", "closed") {
		t.Fatal("resolved->closed should be allowed")
	}
	if !caseAllowed("resolved", "investigating") {
		t.Fatal("resolved should be reopenable to investigating")
	}
	if caseAllowed("closed", "draft") {
		t.Fatal("closed is terminal")
	}
	if caseAllowed("cancelled", "submitted") {
		t.Fatal("cancelled is terminal")
	}
}
