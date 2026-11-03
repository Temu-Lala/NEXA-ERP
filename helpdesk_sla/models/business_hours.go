package models

import (
	"context"
	"sort"
	"time"
	_ "time/tzdata"

	"nexa/core/orm"
)

// Business-time maths for SLA clocks. A policy's working calendar scopes deadlines
// to real working hours; when no calendar (or no scheduled hours) is present every
// helper degrades gracefully to wall-clock time so an SLA never fails to compute.

const slaDTLayout = "2006-01-02 15:04:05"

type hourWindow struct {
	from int // hour of day, inclusive
	to   int // hour of day, exclusive (may be 24)
}

type holidayRange struct {
	from time.Time // date at midnight in the calendar timezone
	to   time.Time // exclusive end date at midnight
}

// slaCalendar is the in-memory form of a helpdesk.business.calendar row.
type slaCalendar struct {
	loc     *time.Location
	windows map[time.Weekday][]hourWindow
	holiday []holidayRange
	hasTime bool // true when at least one working window is scheduled
}

func parseDT(s string) (time.Time, bool) {
	s = trim(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{slaDTLayout, time.RFC3339, "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	// A bare date is treated as midnight UTC.
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC(), true
	}
	return time.Time{}, false
}

func fmtDT(t time.Time) string {
	return t.UTC().Format(slaDTLayout)
}

func trim(s string) string {
	// local trim avoids pulling strings for one call
	out := []rune(s)
	start, end := 0, len(out)
	for start < end && (out[start] == ' ' || out[start] == '\t' || out[start] == '\n' || out[start] == '\r') {
		start++
	}
	for end > start && (out[end-1] == ' ' || out[end-1] == '\t' || out[end-1] == '\n' || out[end-1] == '\r') {
		end--
	}
	return string(out[start:end])
}

// loadCalendar resolves a business calendar (by id) into an slaCalendar. A zero /
// missing id, or a calendar with no scheduled hours, yields a nil result meaning
// "use wall-clock time".
func loadCalendar(ctx context.Context, calID int) *slaCalendar {
	if calID <= 0 {
		return nil
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, "helpdesk.business.calendar", map[string]interface{}{"id": calID})
	if err != nil || rec == nil {
		return nil
	}
	tzName := orm.AsString(rec["timezone"])
	if tzName == "" {
		tzName = "UTC"
	}
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		loc = time.UTC
	}
	cal := &slaCalendar{loc: loc, windows: map[time.Weekday][]hourWindow{}}

	lines, _ := orm.Search(bypass, "helpdesk.business.calendar.line",
		[][]interface{}{{"calendar_id", "=", calID}})
	for _, ln := range lines {
		wd := weekdayOf(orm.AsString(ln["day_of_week"]))
		f := intOrZero(ln["hour_from"])
		t := intOrZero(ln["hour_to"])
		if t <= f {
			continue
		}
		if f < 0 {
			f = 0
		}
		if t > 24 {
			t = 24
		}
		cal.windows[wd] = append(cal.windows[wd], hourWindow{from: f, to: t})
	}
	for wd := range cal.windows {
		ws := cal.windows[wd]
		sort.Slice(ws, func(i, j int) bool { return ws[i].from < ws[j].from })
		cal.windows[wd] = ws
		if len(ws) > 0 {
			cal.hasTime = true
		}
	}

	hols, _ := orm.Search(bypass, "helpdesk.business.calendar.holiday",
		[][]interface{}{{"calendar_id", "=", calID}})
	for _, h := range hols {
		from, ok := parseDate(loc, orm.AsString(h["date"]))
		if !ok {
			continue
		}
		to, ok2 := parseDate(loc, orm.AsString(h["date_to"]))
		if !ok2 || !to.After(from) {
			to = from.AddDate(0, 0, 1)
		}
		cal.holiday = append(cal.holiday, holidayRange{from: from, to: to})
	}
	if !cal.hasTime {
		return nil
	}
	return cal
}

func parseDate(loc *time.Location, s string) (time.Time, bool) {
	s = trim(s)
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("2006-01-02", s, loc)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func weekdayOf(name string) time.Weekday {
	switch name {
	case "monday":
		return time.Monday
	case "tuesday":
		return time.Tuesday
	case "wednesday":
		return time.Wednesday
	case "thursday":
		return time.Thursday
	case "friday":
		return time.Friday
	case "saturday":
		return time.Saturday
	case "sunday":
		return time.Sunday
	default:
		return time.Monday
	}
}

func intOrZero(v interface{}) int {
	n, ok := orm.CoerceInt64(v)
	if !ok {
		return 0
	}
	return int(n)
}

func (c *slaCalendar) isClosed(day time.Time) bool {
	for _, hr := range c.holiday {
		if !day.Before(hr.from) && day.Before(hr.to) {
			return true
		}
	}
	return false
}

// businessMinutesBetween returns the number of working minutes between two UTC
// instants. Without a schedule it equals the wall-clock minute difference.
func businessMinutesBetween(cal *slaCalendar, from, to time.Time) int {
	if cal == nil {
		return clampMinutes(to.Sub(from))
	}
	if !to.After(from) {
		return 0
	}
	loc := cal.loc
	fl := from.In(loc)
	tl := to.In(loc)
	total := 0
	day := startOfDay(fl)
	lastDay := startOfDay(tl)
	for !day.After(lastDay) {
		if !cal.isClosed(day) {
			for _, w := range cal.windows[day.Weekday()] {
				ws := day.Add(time.Duration(w.from) * time.Hour)
				we := day.Add(time.Duration(w.to) * time.Hour)
				lo, hi := ws, we
				if fl.After(lo) {
					lo = fl
				}
				if tl.Before(hi) {
					hi = tl
				}
				if hi.After(lo) {
					total += int(hi.Sub(lo) / time.Minute)
				}
			}
		}
		day = day.AddDate(0, 0, 1)
	}
	return total
}

// addBusinessMinutes returns the UTC instant reached after spending targetMins of
// working time starting at `start`. Without a schedule it simply adds wall-clock
// minutes. Bounded to 400 days to guard against degenerate calendars.
func addBusinessMinutes(cal *slaCalendar, start time.Time, targetMins int) time.Time {
	if targetMins <= 0 {
		return start
	}
	if cal == nil {
		return start.Add(time.Duration(targetMins) * time.Minute)
	}
	loc := cal.loc
	remaining := targetMins
	sl := start.In(loc)
	day := startOfDay(sl)
	for i := 0; i < 400; i++ {
		if !cal.isClosed(day) {
			for _, w := range cal.windows[day.Weekday()] {
				ws := day.Add(time.Duration(w.from) * time.Hour)
				we := day.Add(time.Duration(w.to) * time.Hour)
				segStart := ws
				if sl.After(segStart) && day.Equal(startOfDay(sl)) {
					segStart = sl
				}
				if !we.After(segStart) {
					continue
				}
				avail := int(we.Sub(segStart) / time.Minute)
				if avail <= 0 {
					continue
				}
				if remaining <= avail {
					return segStart.Add(time.Duration(remaining) * time.Minute).UTC()
				}
				remaining -= avail
			}
		}
		day = day.AddDate(0, 0, 1)
	}
	// Exhausted the horizon; fall back to wall-clock so a due date always exists.
	return start.Add(time.Duration(targetMins) * time.Minute)
}

func clampMinutes(d time.Duration) int {
	m := int(d / time.Minute)
	if m < 0 {
		return 0
	}
	return m
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
