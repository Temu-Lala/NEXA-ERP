package models

import (
	"testing"
)

func TestParseClock(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"09:00", 540, true},
		{"17:30", 1050, true},
		{"00:00", 0, true},
		{"23:59", 1439, true},
		{" 08:15 ", 495, true},
		{"9:05", 545, true},
		{"24:00", 0, false},
		{"12:60", 0, false},
		{"09", 0, false},
		{"", 0, false},
		{"ab:cd", 0, false},
	}
	for _, c := range cases {
		got, ok := parseClock(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("parseClock(%q) = (%d,%v), want (%d,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestApplyRounding(t *testing.T) {
	cases := []struct {
		v, bucket, want int
	}{
		{7, 0, 7},   // no rounding
		{0, 15, 0},  //
		{7, 15, 0},  // below half bucket -> 0
		{8, 15, 15}, // at/above half -> 15
		{22, 15, 15},
		{23, 15, 30},
		{45, 15, 45}, // exact multiple preserved
		{-5, 15, 0},  // negative guarded upstream but keep deterministic
	}
	for _, c := range cases {
		if got := applyRounding(c.v, c.bucket); got != c.want {
			t.Errorf("applyRounding(%d,%d) = %d, want %d", c.v, c.bucket, got, c.want)
		}
	}
}

func TestEvaluatePunch(t *testing.T) {
	// 09:00 (540) -> 17:00 (1020), no grace, no rounding.
	base := struct{ start, end, gi, go_, rnd int }{540, 1020, 0, 0, 0}

	cases := []struct {
		name                string
		hasIn, hasOut       bool
		in, out             int
		wantStatus          AttendanceStatus
		wantLate, wantEarly int
		wantMissing         bool
	}{
		{"on time", true, true, 540, 1020, AttPresent, 0, 0, false},
		{"10 late", true, true, 550, 1020, AttLate, 10, 0, false},
		{"15 early", true, true, 540, 1005, AttEarly, 0, 15, false},
		{"late and early", true, true, 555, 1000, AttLateEarly, 15, 20, false},
		{"no checkout", true, false, 540, 0, AttMissingPunch, 0, 0, true},
		{"missing punch still late", true, false, 600, 0, AttMissingPunch, 60, 0, true},
	}
	for _, c := range cases {
		res := evaluatePunch(base.start, base.end, base.gi, base.go_, base.rnd, c.hasIn, c.hasOut, c.in, c.out)
		if res.status != c.wantStatus || res.late != c.wantLate || res.early != c.wantEarly || res.missing != c.wantMissing {
			t.Errorf("%s: got (status=%v late=%d early=%d missing=%v), want (status=%v late=%d early=%d missing=%v)",
				c.name, res.status, res.late, res.early, res.missing,
				c.wantStatus, c.wantLate, c.wantEarly, c.wantMissing)
		}
	}
}

func TestEvaluatePunchGrace(t *testing.T) {
	// 09:00 start with a 10-minute in-grace and 5-minute out-grace.
	res := evaluatePunch(540, 1020, 10, 5, 0, true, true, 548, 1018)
	if res.late != 0 {
		t.Errorf("expected arrival within grace to be 0 late, got %d", res.late)
	}
	if res.early != 0 {
		t.Errorf("expected departure within grace to be 0 early, got %d", res.early)
	}
	if res.status != AttPresent {
		t.Errorf("expected present within grace, got %v", res.status)
	}
	// Arrive 20 late, grace 10 -> counted late 10.
	res2 := evaluatePunch(540, 1020, 10, 5, 0, true, true, 560, 1020)
	if res2.late != 10 || res2.status != AttLate {
		t.Errorf("grace netting failed: late=%d status=%v", res2.late, res2.status)
	}
}

func TestEvaluatePunchOvernightRounding(t *testing.T) {
	// 22:00 (1320) -> 06:00 next day (1800), 15-minute rounding.
	// Arrive 22:20 (1340) => raw late 20 -> rounds to 15; leave 05:40 (1780) => raw early 20 -> 15.
	res := evaluatePunch(1320, 1800, 0, 0, 15, true, true, 1340, 1780)
	if res.late != 15 || res.early != 15 {
		t.Errorf("overnight rounding: late=%d early=%d, want 15/15", res.late, res.early)
	}
	if res.status != AttLateEarly {
		t.Errorf("overnight status = %v, want late_and_early", res.status)
	}
}
