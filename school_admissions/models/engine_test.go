package models

import "testing"

func TestAllowedTransitions(t *testing.T) {
	cases := []struct {
		action string
		from   string
		want   bool
	}{
		{"submit", "draft", true},
		{"submit", "submitted", false},
		{"review", "submitted", true},
		{"assess", "under_review", true},
		{"offer", "assessment", true},
		{"offer", "draft", false},
		{"waitlist", "under_review", true},
		{"reject", "waitlisted", true},
		{"accept", "offered", true},
		{"accept", "assessment", false},
		{"enroll", "accepted", true},
		{"enroll", "offered", false}, // offer alone must not enroll
		{"enroll", "draft", false},
		{"frobnicate", "draft", false}, // unknown action
	}
	for _, c := range cases {
		if got := allowed(c.action, c.from); got != c.want {
			t.Errorf("allowed(%q,%q)=%v want %v", c.action, c.from, got, c.want)
		}
	}
}

func TestTargetState(t *testing.T) {
	if got := targetState("offer"); got != "offered" {
		t.Errorf("targetState(offer)=%q want offered", got)
	}
	if got := targetState("enroll"); got != "enrolled" {
		t.Errorf("targetState(enroll)=%q want enrolled", got)
	}
	if got := targetState("nope"); got != "" {
		t.Errorf("targetState(nope)=%q want empty", got)
	}
}

func TestSeatsAvailable(t *testing.T) {
	if !seatsAvailable(0, 999) { // uncapped
		t.Error("capacity 0 must mean unlimited seats")
	}
	if !seatsAvailable(30, 29) {
		t.Error("29/30 should have a free seat")
	}
	if seatsAvailable(30, 30) {
		t.Error("30/30 should be full")
	}
	if seatsAvailable(1, 5) {
		t.Error("over capacity must report no seat")
	}
}

func TestOfferExpired(t *testing.T) {
	if offerExpired("2026-10-22", "") {
		t.Error("blank expiry never expires")
	}
	if !offerExpired("2026-10-22", "2026-10-01") {
		t.Error("past expiry should be expired")
	}
	if offerExpired("2026-10-22", "2026-11-01") {
		t.Error("future expiry should not be expired")
	}
	if offerExpired("2026-10-22", "2026-10-22") {
		t.Error("expiry equal to today is not yet past")
	}
}

func TestRankKey(t *testing.T) {
	if k := rankKey(" Tesfaye ", "Abebe", "2010-01-05"); k != "tesfaye|abebe|2010-01-05" {
		t.Errorf("rankKey trimming/lowercasing wrong: %q", k)
	}
	if k := rankKey("", "Abebe", "2010-01-05"); k != "" {
		t.Errorf("empty last name must yield empty fingerprint, got %q", k)
	}
	if k := rankKey("Tesfaye", "Abebe", ""); k != "" {
		t.Errorf("empty dob must yield empty fingerprint, got %q", k)
	}
}
