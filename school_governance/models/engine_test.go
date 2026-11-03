package models

import (
	"testing"
	"time"
)

func TestPolicyEffective(t *testing.T) {
	today := "2026-06-15"
	if !policyEffective("active", "2026-01-01", "2026-12-31", today) {
		t.Fatal("active policy within window should be effective")
	}
	if policyEffective("draft", "2026-01-01", "2026-12-31", today) {
		t.Fatal("non-active policy is never effective")
	}
	if policyEffective("active", "2026-07-01", "", today) {
		t.Fatal("future start is not yet effective")
	}
	if policyEffective("active", "", "2026-06-01", today) {
		t.Fatal("past end is no longer effective")
	}
	if !policyEffective("active", "", "", today) {
		t.Fatal("active with no window is always effective")
	}
	if !policyEffective("active", "2026-06-15", "2026-06-15", today) {
		t.Fatal("boundary dates are inclusive")
	}
}

func TestShouldAutoArchive(t *testing.T) {
	if !shouldAutoArchive("2026-06-01", "2026-06-15") {
		t.Fatal("past end date should trigger archival")
	}
	if shouldAutoArchive("2026-06-30", "2026-06-15") {
		t.Fatal("future end date should not trigger archival")
	}
	if shouldAutoArchive("", "2026-06-15") {
		t.Fatal("open-ended policy should never auto-archive")
	}
	if shouldAutoArchive("2026-06-15", "2026-06-15") {
		t.Fatal("end date equal to today is still effective")
	}
}

func TestApprovalAllowedTransition(t *testing.T) {
	if !approvalAllowedTransition("pending", "approved") ||
		!approvalAllowedTransition("pending", "rejected") ||
		!approvalAllowedTransition("pending", "cancelled") {
		t.Fatal("pending may move to approved/rejected/cancelled")
	}
	for _, from := range []string{"approved", "rejected", "cancelled"} {
		if approvalAllowedTransition(from, "pending") || approvalAllowedTransition(from, "approved") {
			t.Fatalf("%s should be terminal", from)
		}
	}
}

func TestCanApproveNow(t *testing.T) {
	if !canApproveNow("pending", 5, 7) {
		t.Fatal("a different user may approve a pending request")
	}
	if canApproveNow("pending", 7, 7) {
		t.Fatal("separation of duties: requester may not approve their own")
	}
	if canApproveNow("pending", 5, 0) {
		t.Fatal("anonymous user may not approve")
	}
	if canApproveNow("approved", 5, 7) {
		t.Fatal("only pending requests may be acted on")
	}
}

func TestIntegrationEligibleForSync(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	if integrationEligibleForSync("active", false, "", 60, now) {
		t.Fatal("inactive connector is never eligible")
	}
	if integrationEligibleForSync("paused", true, "", 60, now) {
		t.Fatal("non-active state is not eligible")
	}
	if !integrationEligibleForSync("active", true, "", 60, now) {
		t.Fatal("never-synced active connector is eligible")
	}
	if integrationEligibleForSync("active", true, now.Add(-10*time.Minute).Format(datetimeFull), 60, now) {
		t.Fatal("recently synced connector is not yet due")
	}
	if !integrationEligibleForSync("active", true, now.Add(-90*time.Minute).Format(datetimeFull), 60, now) {
		t.Fatal("connector past its interval is due")
	}
	// non-positive interval falls back to the default
	if integrationEligibleForSync("active", true, now.Add(-10*time.Minute).Format(datetimeFull), 0, now) {
		t.Fatal("default interval should hold recent syncs back")
	}
}

func TestBumpVersion(t *testing.T) {
	if got := bumpVersion(3); got != 4 {
		t.Fatalf("want 4 got %d", got)
	}
	if got := bumpVersion(0); got != 1 {
		t.Fatalf("non-positive want 1 got %d", got)
	}
	if got := bumpVersion(-5); got != 1 {
		t.Fatalf("negative want 1 got %d", got)
	}
}
