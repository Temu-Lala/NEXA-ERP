package models

import (
	"context"
	"math"
	"time"

	"nexa/core/orm"
	"nexa/core/scheduler"
)

// Behaviour layer for attendance. The status tally, the rate formula, the finalize
// guard, the "did the mark change" predicate and the absence-threshold predicate are all
// pure and unit-tested (no DB). The computes project a session's live summary, and a
// nightly cron raises absence alerts for learners whose unexcused absences cross the
// threshold inside a rolling window.

const (
	sessionModel    = "school.attendance.session"
	lineModel       = "school.attendance.line"
	correctionModel = "school.attendance.correction"
	alertModel      = "school.absence.alert"
	studentModel    = "school.student"

	absenceCronCode = "school.attendance.absence"

	defaultAbsenceThreshold = 3
	defaultAbsenceWindow    = 30

	dateLayout   = "2006-01-02"
	datetimeFull = "2006-01-02 15:04:05"
)

// tally is a session's per-status counts.
type tally struct {
	present, absent, late, excused int
}

func (t tally) total() int { return t.present + t.absent + t.late + t.excused }

// tallyStatuses counts marks by status. Anything unrecognised is ignored so the summary
// stays consistent with the four known states.
func tallyStatuses(statuses []string) tally {
	var t tally
	for _, s := range statuses {
		switch s {
		case string(AttPresent):
			t.present++
		case string(AttAbsent):
			t.absent++
		case string(AttLate):
			t.late++
		case string(AttExcused):
			t.excused++
		}
	}
	return t
}

// attendanceRate is the percentage of marked learners counted as attending: present and
// late count toward attendance, while absent and excused do not. A session with no
// marks has a 0% rate. The result is rounded to two decimals.
func attendanceRate(present, late, total int) float64 {
	if total <= 0 {
		return 0
	}
	rate := float64(present+late) / float64(total) * 100
	return math.Round(rate*100) / 100
}

// finalizeAllowed blocks finalizing a session that has no marks recorded.
func finalizeAllowed(recordCount int) bool { return recordCount > 0 }

// statusChanged reports whether a correction should be logged (the mark actually moved).
func statusChanged(from, to string) bool { return from != to && to != "" }

// exceedsThreshold reports whether unexcused absences reach the configured threshold.
// A non-positive threshold never fires.
func exceedsThreshold(unexcused, threshold int) bool {
	return threshold > 0 && unexcused >= threshold
}

func init() {
	orm.RegisterCompute(sessionModel, "total_count", []string{"id"}, computeTotal)
	orm.RegisterCompute(sessionModel, "present_count", []string{"id"}, computePresent)
	orm.RegisterCompute(sessionModel, "absent_count", []string{"id"}, computeAbsent)
	orm.RegisterCompute(sessionModel, "late_count", []string{"id"}, computeLate)
	orm.RegisterCompute(sessionModel, "excused_count", []string{"id"}, computeExcused)
	orm.RegisterCompute(sessionModel, "attendance_rate", []string{"id"}, computeRate)
	scheduler.RegisterCronHandler(absenceCronCode, runAbsenceScan)
}

// ---------------------------------------------------------------------------
// Computes (non-stored session summaries)
// ---------------------------------------------------------------------------

func sessionTally(ctx context.Context, sessionID int) tally {
	if sessionID <= 0 {
		return tally{}
	}
	rows, err := orm.Search(orm.ContextWithBypass(ctx, true), lineModel, [][]interface{}{
		{"session_id", "=", sessionID},
	})
	if err != nil {
		return tally{}
	}
	statuses := make([]string, 0, len(rows))
	for _, r := range rows {
		statuses = append(statuses, strOr(r["status"]))
	}
	return tallyStatuses(statuses)
}

func computeTotal(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return sessionTally(ctx, intOr(rec["id"])).total(), nil
}
func computePresent(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return sessionTally(ctx, intOr(rec["id"])).present, nil
}
func computeAbsent(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return sessionTally(ctx, intOr(rec["id"])).absent, nil
}
func computeLate(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return sessionTally(ctx, intOr(rec["id"])).late, nil
}
func computeExcused(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return sessionTally(ctx, intOr(rec["id"])).excused, nil
}
func computeRate(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	t := sessionTally(ctx, intOr(rec["id"]))
	return attendanceRate(t.present, t.late, t.total()), nil
}

// ---------------------------------------------------------------------------
// Cron: raise absence alerts for chronic unexcused absences.
// ---------------------------------------------------------------------------

func runAbsenceScan(ctx context.Context, args map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	threshold := cronInt(args, "threshold", defaultAbsenceThreshold)
	window := cronInt(args, "window_days", defaultAbsenceWindow)
	if threshold <= 0 {
		threshold = defaultAbsenceThreshold
	}
	if window <= 0 {
		window = defaultAbsenceWindow
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -window).Format(dateLayout)

	sessions, err := orm.Search(bypass, sessionModel, [][]interface{}{
		{"state", "=", string(SessFinalized)},
		{"session_date", ">=", cutoff},
	})
	if err != nil {
		return err
	}

	// Accumulate unexcused absences (status = absent) per learner over the window.
	type agg struct {
		count         int
		first, last   string
		year, section int
	}
	byStudent := map[int]*agg{}
	for _, s := range sessions {
		sid := intOr(s["id"])
		lines, err := orm.Search(bypass, lineModel, [][]interface{}{
			{"session_id", "=", sid},
			{"status", "=", string(AttAbsent)},
		})
		if err != nil {
			continue
		}
		date := strOr(s["session_date"])
		year := intOr(s["academic_year_id"])
		section := intOr(s["section_id"])
		for _, l := range lines {
			studentID := intOr(l["student_id"])
			if studentID <= 0 {
				continue
			}
			a := byStudent[studentID]
			if a == nil {
				a = &agg{first: date, last: date, year: year, section: section}
				byStudent[studentID] = a
			}
			a.count++
			if date != "" && (a.first == "" || date < a.first) {
				a.first = date
			}
			if date > a.last {
				a.last = date
			}
		}
	}

	now := time.Now().UTC().Format(datetimeFull)
	for studentID, a := range byStudent {
		if !exceedsThreshold(a.count, threshold) {
			continue
		}
		if hasOpenAlert(bypass, studentID) {
			continue
		}
		_, _ = orm.Create(bypass, orm.RegistryModel(alertModel), map[string]interface{}{
			"student_id":       studentID,
			"academic_year_id": a.year,
			"section_id":       a.section,
			"absent_count":     a.count,
			"threshold":        threshold,
			"window_days":      window,
			"first_absent":     a.first,
			"last_absent":      a.last,
			"state":            string(AlertOpen),
			"raised_at":        now,
			"notes":            "Auto-raised by absence scan",
		})
	}
	return nil
}

// hasOpenAlert reports whether a learner already has an open absence alert (dedupe).
func hasOpenAlert(ctx context.Context, studentID int) bool {
	if studentID <= 0 {
		return false
	}
	n, err := orm.SearchCount(ctx, alertModel, [][]interface{}{
		{"student_id", "=", studentID},
		{"state", "=", string(AlertOpen)},
	})
	if err != nil {
		return true // fail safe: do not duplicate when the check errors
	}
	return n > 0
}

func cronInt(args map[string]interface{}, key string, def int) int {
	if args == nil {
		return def
	}
	if v, ok := args[key]; ok {
		if n, ok2 := orm.CoerceInt64(v); ok2 && n > 0 {
			return int(n)
		}
	}
	return def
}

// ---------------------------------------------------------------------------
// Shared helpers (self-contained to this module)
// ---------------------------------------------------------------------------

func intOr(v interface{}) int {
	n, ok := orm.CoerceInt64(v)
	if !ok {
		return 0
	}
	return int(n)
}

func strOr(v interface{}) string { return orm.AsString(v) }

func boolOr(v interface{}) bool { return orm.AsBool(v) }
