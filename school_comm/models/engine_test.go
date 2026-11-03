package models

import "testing"

func TestShouldAttempt(t *testing.T) {
	if !shouldAttempt("pending", 0, 3) {
		t.Fatal("pending should always be eligible")
	}
	if !shouldAttempt("failed", 2, 3) {
		t.Fatal("failed with attempts left should be eligible")
	}
	if shouldAttempt("failed", 3, 3) {
		t.Fatal("failed at max should not be eligible")
	}
	// zero/unset max falls back to the default ceiling
	if !shouldAttempt("failed", 2, 0) {
		t.Fatal("failed with default max should be eligible at 2 attempts")
	}
	if shouldAttempt("sent", 0, 3) {
		t.Fatal("sent is terminal")
	}
	if shouldAttempt("cancelled", 0, 3) {
		t.Fatal("cancelled is terminal")
	}
}

func TestBackoffMinutes(t *testing.T) {
	if got := backoffMinutes(0, 30); got != 30 {
		t.Fatalf("attempt 0 want 30 got %d", got)
	}
	if got := backoffMinutes(1, 30); got != 60 {
		t.Fatalf("attempt 1 want 60 got %d", got)
	}
	if got := backoffMinutes(2, 30); got != 120 {
		t.Fatalf("attempt 2 want 120 got %d", got)
	}
	// doubling is clamped to a day
	if got := backoffMinutes(20, 30); got != maxBackoffMinutes {
		t.Fatalf("want clamp %d got %d", maxBackoffMinutes, got)
	}
	// non-positive base falls back to the default
	if got := backoffMinutes(0, 0); got != defaultBaseBackoffMinutes {
		t.Fatalf("zero base want default got %d", got)
	}
}

func TestRecipientForAudience(t *testing.T) {
	cases := map[string]string{
		"all":           string(RecAll),
		"all_students":  string(RecStudent),
		"class":         string(RecStudent),
		"section":       string(RecStudent),
		"all_guardians": string(RecGuardian),
		"all_staff":     string(RecStaff),
		"":              string(RecAll),
	}
	for aud, want := range cases {
		if got := recipientForAudience(aud); got != want {
			t.Fatalf("audience %q want %q got %q", aud, want, got)
		}
	}
}
