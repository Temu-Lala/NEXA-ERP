package models

import (
	"testing"
	"time"
)

// weekdayCalendar builds a Mon-Fri 09:00-17:00 UTC working calendar in memory so
// the deadline maths can be exercised without any database access.
func weekdayCalendar() *slaCalendar {
	c := &slaCalendar{loc: time.UTC, windows: map[time.Weekday][]hourWindow{}}
	for wd := time.Monday; wd <= time.Friday; wd++ {
		c.windows[wd] = []hourWindow{{from: 9, to: 17}}
	}
	c.hasTime = true
	return c
}

func TestAddBusinessMinutesSpansWeekend(t *testing.T) {
	cal := weekdayCalendar()
	start := time.Date(2024, 1, 5, 16, 0, 0, 0, time.UTC) // Friday 16:00
	want := time.Date(2024, 1, 8, 10, 0, 0, 0, time.UTC)  // Monday 10:00 (60 Fri + 60 Mon)
	got := addBusinessMinutes(cal, start, 120)
	if !got.Equal(want) {
		t.Fatalf("addBusinessMinutes = %s, want %s", got.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

func TestBusinessMinutesBetweenExcludesNonWorking(t *testing.T) {
	cal := weekdayCalendar()
	from := time.Date(2024, 1, 5, 16, 0, 0, 0, time.UTC) // Friday 16:00
	to := time.Date(2024, 1, 8, 10, 0, 0, 0, time.UTC)   // Monday 10:00
	if n := businessMinutesBetween(cal, from, to); n != 120 {
		t.Fatalf("businessMinutesBetween = %d, want 120", n)
	}
}

func TestBusinessCalendarSkipsHolidays(t *testing.T) {
	cal := weekdayCalendar()
	// Close Monday (the day the clock would otherwise roll into).
	cal.holiday = []holidayRange{{
		from: time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC),
		to:   time.Date(2024, 1, 9, 0, 0, 0, 0, time.UTC),
	}}
	start := time.Date(2024, 1, 5, 16, 0, 0, 0, time.UTC) // Friday 16:00
	want := time.Date(2024, 1, 9, 10, 0, 0, 0, time.UTC)  // Tuesday 10:00 (Mon closed)
	got := addBusinessMinutes(cal, start, 120)
	if !got.Equal(want) {
		t.Fatalf("holiday addBusinessMinutes = %s, want %s", got.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

func TestWallClockFallbackWhenNoCalendar(t *testing.T) {
	start := time.Date(2024, 1, 5, 16, 0, 0, 0, time.UTC)
	got := addBusinessMinutes(nil, start, 45)
	if !got.Equal(start.Add(45 * time.Minute)) {
		t.Fatalf("wall-clock due = %s, want %s", got.Format(time.RFC3339), start.Add(45*time.Minute).Format(time.RFC3339))
	}
	to := start.Add(2 * time.Hour)
	if n := businessMinutesBetween(nil, start, to); n != 120 {
		t.Fatalf("wall-clock minutes = %d, want 120", n)
	}
}
