package models

import (
	"testing"
	"time"
)

func mk(y int, m time.Month, d, hh, mm int) time.Time {
	return time.Date(y, m, d, hh, mm, 0, 0, time.UTC)
}

func TestIsTerminal(t *testing.T) {
	open := []string{"new", "assigned", "in_progress", "pending"}
	done := []string{"resolved", "closed", "cancelled"}
	for _, s := range open {
		if isTerminal(s) {
			t.Errorf("isTerminal(%q)=true want false", s)
		}
	}
	for _, s := range done {
		if !isTerminal(s) {
			t.Errorf("isTerminal(%q)=false want true", s)
		}
	}
}

func TestWindowState(t *testing.T) {
	start := mk(2026, 1, 1, 0, 0)
	due := mk(2026, 1, 11, 0, 0) // 10-day window
	cases := []struct {
		name string
		now  time.Time
		want SlaState
	}{
		{"unset due is on_track", start, SlaOnTrack},
		{"early is on_track", mk(2026, 1, 5, 0, 0), SlaOnTrack},
		{"past 75% is warning", mk(2026, 1, 8, 12, 0), SlaWarning},
		{"at due is breached", due, SlaBreached},
		{"past due is breached", mk(2026, 1, 12, 0, 0), SlaBreached},
	}
	for _, c := range cases {
		if got := windowState(start, due, c.now, slaWarnPct); got != c.want {
			t.Errorf("windowState %s: got %v want %v", c.name, got, c.want)
		}
	}
	// Zero start never warns.
	if got := windowState(time.Time{}, due, mk(2026, 1, 9, 0, 0), slaWarnPct); got != SlaOnTrack {
		t.Errorf("zero start should be on_track, got %v", got)
	}
}

func TestEvaluateSla(t *testing.T) {
	start := mk(2026, 1, 1, 0, 0)
	resp := mk(2026, 1, 2, 0, 0)
	reso := mk(2026, 1, 11, 0, 0)

	if got := evaluateSla(string(CaseClosed), start, resp, reso, mk(2026, 1, 1, 6, 0)); got != SlaMet {
		t.Errorf("closed case should be met, got %v", got)
	}
	// First-response miss on a new case.
	if got := evaluateSla(string(CaseNew), start, resp, reso, mk(2026, 1, 3, 0, 0)); got != SlaBreached {
		t.Errorf("missed first response should be breached, got %v", got)
	}
	// In progress before any due is on_track.
	if got := evaluateSla(string(CaseInProgress), start, resp, reso, mk(2026, 1, 2, 12, 0)); got != SlaOnTrack {
		t.Errorf("mid-window in-progress should be on_track, got %v", got)
	}
	// Past the resolution due is breached.
	if got := evaluateSla(string(CaseInProgress), start, resp, reso, mk(2026, 1, 12, 0, 0)); got != SlaBreached {
		t.Errorf("past resolution due should be breached, got %v", got)
	}
}

func TestIsBreached(t *testing.T) {
	resp := mk(2026, 1, 2, 0, 0)
	reso := mk(2026, 1, 11, 0, 0)
	if isBreached(string(CaseResolved), resp, reso, mk(2026, 1, 12, 0, 0)) {
		t.Error("a resolved case is never breached")
	}
	if !isBreached(string(CaseInProgress), resp, reso, mk(2026, 1, 12, 0, 0)) {
		t.Error("past resolution due while open should be breached")
	}
	if isBreached(string(CaseInProgress), resp, reso, mk(2026, 1, 5, 0, 0)) {
		t.Error("before the window closes should not be breached")
	}
	if !isBreached(string(CaseNew), resp, time.Time{}, mk(2026, 1, 3, 0, 0)) {
		t.Error("missed first response with no resolution due should breach")
	}
}
