package models

import (
	"context"
	"time"

	"nexa/core/orm"
	"nexa/core/scheduler"
)

// HR service-desk behaviour layer: the pure SLA maths (testable, no database)
// plus the computes that project an employee's related user id and a live SLA
// state onto a case, and a scheduled scan that durably flags cases whose
// response / resolution window has elapsed (idempotent, append-only history).

const (
	dateLayout   = "2006-01-02"
	datetimeFull = "2006-01-02 15:04:05"

	slaCronCode = "hr.service.sla.scan"
	slaWarnPct  = 75 // % of the window elapsed before it counts as "warning"

	employeeMod = "hr.employee"
)

const (
	teamModel     = "hr.service.team"
	categoryModel = "hr.service.category"
	caseModel     = "hr.service.case"
	caseMsgModel  = "hr.service.case.message"
	caseHistModel = "hr.service.case.history"
	requestModel  = "hr.selfservice.request"
)

func init() {
	orm.RegisterCompute(caseModel, "employee_user_id", []string{"employee_id"}, computeCaseEmployeeUser)
	orm.RegisterCompute(requestModel, "employee_user_id", []string{"employee_id"}, computeRequestEmployeeUser)
	orm.RegisterCompute(caseModel, "sla_state", []string{"state", "response_due", "resolution_due", "breached", "create_date"}, computeCaseSlaState)
	scheduler.RegisterCronHandler(slaCronCode, runSlaScan)
}

// ---------------------------------------------------------------------------
// Pure maths
// ---------------------------------------------------------------------------

// isTerminal reports whether a case state stops the SLA clock.
func isTerminal(state string) bool {
	switch CaseState(state) {
	case CaseResolved, CaseClosed, CaseCancelled:
		return true
	}
	return false
}

// windowState classifies the position of `now` inside the [start, due] window.
// An unset due is never a breach (on_track). Once now reaches due it is
// breached; while at least warnPct of the window has elapsed it is a warning.
func windowState(start, due, now time.Time, warnPct int) SlaState {
	if due.IsZero() {
		return SlaOnTrack
	}
	if !now.Before(due) {
		return SlaBreached
	}
	if start.IsZero() {
		return SlaOnTrack
	}
	total := due.Sub(start)
	if total > 0 {
		elapsed := now.Sub(start)
		if elapsed >= time.Duration(float64(total)*float64(warnPct)/100.0) {
			return SlaWarning
		}
	}
	return SlaOnTrack
}

// evaluateSla is the single source of truth for a case's live SLA banner.
//   - a closed / resolved / cancelled case is "met";
//   - a first-response miss on a not-yet-started case is "breached";
//   - otherwise the resolution window decides on_track / warning / breached.
func evaluateSla(state string, start, responseDue, resolutionDue, now time.Time) SlaState {
	if isTerminal(state) {
		return SlaMet
	}
	if (CaseState(state) == CaseNew || CaseState(state) == CaseAssigned) &&
		!responseDue.IsZero() && !now.Before(responseDue) {
		return SlaBreached
	}
	primary := resolutionDue
	if primary.IsZero() {
		primary = responseDue
	}
	return windowState(start, primary, now, slaWarnPct)
}

// isBreached is the durable flag the scan persists: an open case whose
// governing due date is in the past.
func isBreached(state string, responseDue, resolutionDue, now time.Time) bool {
	if isTerminal(state) {
		return false
	}
	if !responseDue.IsZero() && (CaseState(state) == CaseNew || CaseState(state) == CaseAssigned) && !now.Before(responseDue) {
		return true
	}
	due := resolutionDue
	if due.IsZero() {
		due = responseDue
	}
	return !due.IsZero() && !now.Before(due)
}

// ---------------------------------------------------------------------------
// Field helpers
// ---------------------------------------------------------------------------

func intOr(v interface{}) int {
	n, ok := orm.CoerceInt64(v)
	if !ok {
		return 0
	}
	return int(n)
}

func parseDT(v interface{}) time.Time {
	s := orm.AsString(v)
	if len(s) < 10 {
		return time.Time{}
	}
	for _, layout := range []string{datetimeFull, "2006-01-02T15:04:05", "2006-01-02 15:04:05Z07:00", dateLayout} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

// employeeUser resolves hr.employee.user_id for the given employee id.
func employeeUser(ctx context.Context, employeeID int) int {
	if employeeID <= 0 {
		return 0
	}
	emp, err := orm.SearchOne(orm.ContextWithBypass(ctx, true), employeeMod, map[string]interface{}{"id": employeeID})
	if err != nil || emp == nil {
		return 0
	}
	return intOr(emp["user_id"])
}

// ---------------------------------------------------------------------------
// Computes
// ---------------------------------------------------------------------------

func computeCaseEmployeeUser(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return employeeUser(ctx, intOr(rec["employee_id"])), nil
}

func computeRequestEmployeeUser(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return employeeUser(ctx, intOr(rec["employee_id"])), nil
}

func computeCaseSlaState(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	start := parseDT(rec["create_date"])
	return string(evaluateSla(
		orm.AsString(rec["state"]),
		start,
		parseDT(rec["response_due"]),
		parseDT(rec["resolution_due"]),
		time.Now().UTC(),
	)), nil
}

// ---------------------------------------------------------------------------
// SLA breach scan
// ---------------------------------------------------------------------------

// runSlaScan flags every open case whose SLA window has elapsed and records one
// append-only history entry per newly-breached case, so the stored flag (and any
// reporting filter) stays correct between reads.
func runSlaScan(ctx context.Context, _ map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	now := time.Now().UTC()
	cases, err := orm.Search(bypass, caseModel, [][]interface{}{
		{"state", "not in", []interface{}{string(CaseResolved), string(CaseClosed), string(CaseCancelled)}},
		{"breached", "=", false},
	})
	if err != nil {
		return nil
	}
	for _, c := range cases {
		if !isBreached(orm.AsString(c["state"]), parseDT(c["response_due"]), parseDT(c["resolution_due"]), now) {
			continue
		}
		id := intOr(c["id"])
		if id <= 0 {
			continue
		}
		_ = orm.UpdateRecordByID(bypass, caseModel, id, map[string]interface{}{"breached": true})
		writeHistory(bypass, id, HistSla, "breached", "false", "true", 0)
	}
	return nil
}
