package models

import "testing"

const today = "2026-06-15"

func TestWithinValidity(t *testing.T) {
	cases := []struct {
		name     string
		from, to string
		want     bool
	}{
		{"open-ended", "2026-01-01", "", true},
		{"future start", "2026-07-01", "", false},
		{"past end", "", "2026-06-01", false},
		{"fully open", "", "", true},
		{"inside range", "2026-01-01", "2026-12-31", true},
		{"boundary equal start", "2026-06-15", "", true},
		{"boundary equal end", "", "2026-06-15", true},
	}
	for _, c := range cases {
		if got := withinValidity(c.from, c.to, today); got != c.want {
			t.Errorf("withinValidity(%q,%q,%s)=%v want %v [%s]", c.from, c.to, today, got, c.want, c.name)
		}
	}
}

func TestCanViewFinancial(t *testing.T) {
	rows := []guardianLink{
		{GuardianID: 1, CanViewFinancial: true, Status: "active", ValidFrom: "2026-01-01"},
		{GuardianID: 2, CanViewFinancial: false, Status: "active", ValidFrom: "2026-01-01"},
		{GuardianID: 3, CanViewFinancial: true, Status: "revoked", ValidFrom: "2026-01-01"},
		{GuardianID: 4, CanViewFinancial: true, Status: "active", ValidFrom: "2026-01-01", ValidTo: "2026-05-31"},
	}
	if !canViewFinancial(rows, 1, today) {
		t.Error("guardian 1 should have financial access")
	}
	if canViewFinancial(rows, 2, today) {
		t.Error("guardian 2 opted out of financial view")
	}
	if canViewFinancial(rows, 3, today) {
		t.Error("revoked guardian 3 must not have financial access")
	}
	if canViewFinancial(rows, 4, today) {
		t.Error("expired-window guardian 4 must not have financial access")
	}
	if canViewFinancial(rows, 99, today) {
		t.Error("unknown guardian must not have financial access")
	}
}

func TestPrimaryGuardianID(t *testing.T) {
	// Latest valid_from primary wins.
	rows := []guardianLink{
		{GuardianID: 10, IsPrimary: true, Status: "active", ValidFrom: "2025-01-01"},
		{GuardianID: 20, IsPrimary: true, Status: "active", ValidFrom: "2026-03-01"},
		{GuardianID: 30, IsPrimary: false, Status: "active", ValidFrom: "2026-05-01"},
	}
	if got := primaryGuardianID(rows, today); got != 20 {
		t.Errorf("primaryGuardianID=%d want 20", got)
	}

	// Tie on valid_from breaks to higher guardian id deterministically.
	tie := []guardianLink{
		{GuardianID: 5, IsPrimary: true, Status: "active", ValidFrom: "2026-01-01"},
		{GuardianID: 7, IsPrimary: true, Status: "active", ValidFrom: "2026-01-01"},
	}
	if got := primaryGuardianID(tie, today); got != 7 {
		t.Errorf("tie primaryGuardianID=%d want 7", got)
	}

	// A revoked primary does not win; falls back to next live primary.
	revoked := []guardianLink{
		{GuardianID: 8, IsPrimary: true, Status: "revoked", ValidFrom: "2026-04-01"},
		{GuardianID: 9, IsPrimary: true, Status: "active", ValidFrom: "2026-02-01"},
	}
	if got := primaryGuardianID(revoked, today); got != 9 {
		t.Errorf("revoked-fallback primaryGuardianID=%d want 9", got)
	}

	// No live primary -> 0.
	none := []guardianLink{
		{GuardianID: 1, IsPrimary: false, Status: "active", ValidFrom: "2026-01-01"},
	}
	if got := primaryGuardianID(none, today); got != 0 {
		t.Errorf("no-primary primaryGuardianID=%d want 0", got)
	}
}
