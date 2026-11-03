package models

import (
	"testing"
	"time"
)

func TestEventCategory(t *testing.T) {
	cases := []struct {
		src, dst string
		want     TraceEventType
	}{
		{"supplier", "internal", TraceEvtReceipt},
		{"internal", "customer", TraceEvtDelivery},
		{"customer", "returns", TraceEvtReturn},
		{"internal", "scrap", TraceEvtScrap},
		{"internal", "damaged", TraceEvtScrap},
		{"internal", "internal", TraceEvtInternal},
		{"", "transit", TraceEvtInternal},
	}
	for _, tc := range cases {
		if got := eventCategory(tc.src, tc.dst); got != tc.want {
			t.Fatalf("eventCategory(%q,%q)=%v want %v", tc.src, tc.dst, got, tc.want)
		}
	}
}

func TestExpirySeverity(t *testing.T) {
	if expirySeverity(-1) != ExpiryExpired {
		t.Fatal("negative days must be expired")
	}
	if expirySeverity(0) != ExpiryCritical || expirySeverity(7) != ExpiryCritical {
		t.Fatal("0..7 days must be critical")
	}
	if expirySeverity(8) != ExpiryWarning || expirySeverity(30) != ExpiryWarning {
		t.Fatal("8..30 days must be warning")
	}
}

func TestDaysUntil(t *testing.T) {
	today := time.Date(2026, 10, 20, 13, 0, 0, 0, time.UTC)
	if got := daysUntil(time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC), today); got != 5 {
		t.Fatalf("daysUntil +5 got %d", got)
	}
	if got := daysUntil(time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC), today); got != 0 {
		t.Fatalf("same day should be 0, got %d", got)
	}
	if got := daysUntil(time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC), today); got != -5 {
		t.Fatalf("past date should be negative, got %d", got)
	}
}
