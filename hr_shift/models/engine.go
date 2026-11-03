package models

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nexa/core/event"
	"nexa/core/orm"
)

// Time & Attendance behaviour layer.
//
// Two responsibilities live here:
//
//  1. Exception reconciliation. Whenever an hr.attendance punch pair is created
//     or updated we resolve the employee's scheduled shift for that day, measure
//     how far the actual punches fall outside the shift (net of grace, with
//     optional rounding) and stamp a stored verdict (status / late / early /
//     missing-punch / shift). This is written BACK onto hr.attendance so payroll
//     and reporting can filter on it. To avoid an update loop, we only persist a
//     change when a value actually differs from what is already stored.
//
//  2. Audited corrections + approvals. A punch correction snapshots the original
//     times, and only when an HR manager validates it are the requested times
//     written to the attendance (the correction keeps the pre-change values, so
//     nothing is silently overwritten). Overtime requests follow the same
//     maker-checker machine (draft -> confirm -> validate/refuse) where validate
//     and refuse are HR-manager only.
//
// The pure time maths is isolated in evaluatePunch / applyRounding / parseClock so
// it can be unit tested without a database.

const (
	attModel      = "hr.attendance"
	corrModel     = "hr.attendance.correction"
	overtimeModel = "hr.overtime.request"
	minsPerDay    = 1440
	checkInField  = "check_in"
	checkOutField = "check_out"
	datetimeFull  = "2006-01-02 15:04:05"
	dateLayout    = "2006-01-02"
)

func init() {
	event.Subscribe(orm.EventRecordCreated, onAttendanceChanged)
	event.Subscribe(orm.EventRecordUpdated, onAttendanceChanged)

	// Correction maker-checker workflow.
	orm.RegisterObjectAction(corrModel, "action_confirm", corrTransition("submit", false,
		[]CorrectionState{CorrDraft}, CorrConfirm))
	orm.RegisterObjectAction(corrModel, "action_validate", corrTransition("approve", true,
		[]CorrectionState{CorrConfirm}, CorrValidate))
	orm.RegisterObjectAction(corrModel, "action_refuse", corrTransition("refuse", true,
		[]CorrectionState{CorrDraft, CorrConfirm}, CorrRefuse))
	orm.RegisterObjectAction(corrModel, "action_reset", corrTransition("reset to draft", false,
		[]CorrectionState{CorrConfirm, CorrRefuse}, CorrDraft))

	// Overtime maker-checker workflow.
	orm.RegisterObjectAction(overtimeModel, "action_confirm", otTransition("submit", false,
		[]OvertimeState{OtDraft}, OtConfirm))
	orm.RegisterObjectAction(overtimeModel, "action_validate", otTransition("approve", true,
		[]OvertimeState{OtConfirm}, OtValidate))
	orm.RegisterObjectAction(overtimeModel, "action_refuse", otTransition("refuse", true,
		[]OvertimeState{OtDraft, OtConfirm}, OtRefuse))
	orm.RegisterObjectAction(overtimeModel, "action_reset", otTransition("reset to draft", false,
		[]OvertimeState{OtConfirm, OtRefuse}, OtDraft))
}

// ---------------------------------------------------------------------------
// Attendance exception reconciliation
// ---------------------------------------------------------------------------

func onAttendanceChanged(ctx context.Context, ev event.Event) error {
	model, _ := ev.Payload["model"].(string)
	if model != attModel {
		return nil
	}
	id, ok := orm.CoerceInt64(ev.Payload["id"])
	if !ok || id <= 0 {
		return nil
	}
	reconcileAttendance(ctx, int(id))
	return nil
}

// reconcileAttendance recomputes the stored exception verdict for one attendance
// and writes it back only if something actually changed (idempotent, no loop).
func reconcileAttendance(ctx context.Context, attendanceID int) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, attModel, map[string]interface{}{"id": attendanceID})
	if err != nil || rec == nil {
		return
	}
	emp := intOr(rec["employee_id"])
	in := parseDateTime(rec[checkInField])
	if emp <= 0 || in.IsZero() {
		return
	}
	dateStr := in.UTC().Format(dateLayout)

	vals := map[string]interface{}{}
	shift := resolveShift(bypass, emp, dateStr)

	if shift == nil {
		// No schedule for the day: verdict is not_scheduled with zero deltas.
		setIfChangedInt(vals, "shift_id", rec, 0)
		setIfChangedStr(vals, "status", rec, string(AttNotScheduled))
		setIfChangedInt(vals, "minutes_late", rec, 0)
		setIfChangedInt(vals, "minutes_early", rec, 0)
		setIfChangedBool(vals, "missing_punch", rec, false)
	} else {
		startMin, okStart := parseClock(orm.AsString(shift["start_time"]))
		endMin, okEnd := parseClock(orm.AsString(shift["end_time"]))
		if !okStart || !okEnd {
			return
		}
		if orm.AsBool(shift["overnight"]) && endMin <= startMin {
			endMin += minsPerDay
		}
		graceIn := intOr(shift["grace_in_min"])
		graceOut := intOr(shift["grace_out_min"])
		rounding := intOr(shift["rounding_min"])

		base := time.Date(in.UTC().Year(), in.UTC().Month(), in.UTC().Day(), 0, 0, 0, 0, time.UTC)
		inMin := int(in.UTC().Sub(base).Minutes())
		hasOut := false
		outMin := 0
		if out := parseDateTime(rec[checkOutField]); !out.IsZero() {
			hasOut = true
			outMin = int(out.UTC().Sub(base).Minutes())
		}

		res := evaluatePunch(startMin, endMin, graceIn, graceOut, rounding, true, hasOut, inMin, outMin)

		setIfChangedInt(vals, "shift_id", rec, intOr(shift["id"]))
		setIfChangedStr(vals, "status", rec, string(res.status))
		setIfChangedInt(vals, "minutes_late", rec, res.late)
		setIfChangedInt(vals, "minutes_early", rec, res.early)
		setIfChangedBool(vals, "missing_punch", rec, res.missing)
	}

	if len(vals) == 0 {
		return
	}
	_ = orm.UpdateRecordByID(bypass, attModel, attendanceID, vals)
}

// resolveShift returns the hr.work.shift the employee is rostered onto for the
// given date, or nil when there is no active assignment.
func resolveShift(ctx context.Context, employeeID int, dateStr string) map[string]interface{} {
	assigns, err := orm.SearchPage(ctx, "hr.work.shift.assignment", [][]interface{}{
		{"employee_id", "=", employeeID},
		{"date", "=", dateStr},
		{"active", "=", true},
	}, 1, 0, "id ASC")
	if err != nil || len(assigns) == 0 {
		return nil
	}
	shiftID := intOr(assigns[0]["shift_id"])
	if shiftID <= 0 {
		return nil
	}
	shift, err := orm.SearchOne(ctx, "hr.work.shift", map[string]interface{}{"id": shiftID})
	if err != nil || shift == nil || orm.AsBool(shift["active"]) == false {
		return nil
	}
	return shift
}

// punchResult is the pure output of comparing punches against a shift window.
type punchResult struct {
	status  AttendanceStatus
	late    int
	early   int
	missing bool
}

// evaluatePunch is the pure exception rule. All times are minutes measured from
// midnight of the scheduled day (so an overnight end may exceed 1440).
func evaluatePunch(startMin, endMin, graceIn, graceOut, rounding int, hasIn, hasOut bool, inMin, outMin int) punchResult {
	res := punchResult{}
	if !hasIn || !hasOut {
		res.missing = true
		res.status = AttMissingPunch
	}
	if hasIn {
		raw := inMin - (startMin + graceIn)
		if raw < 0 {
			raw = 0
		}
		res.late = applyRounding(raw, rounding)
	}
	if hasOut {
		raw := (endMin - graceOut) - outMin
		if raw < 0 {
			raw = 0
		}
		res.early = applyRounding(raw, rounding)
	}
	if res.missing {
		return res
	}
	switch {
	case res.late > 0 && res.early > 0:
		res.status = AttLateEarly
	case res.late > 0:
		res.status = AttLate
	case res.early > 0:
		res.status = AttEarly
	default:
		res.status = AttPresent
	}
	return res
}

// applyRounding snaps a minute count to the nearest multiple of the rounding
// bucket (integer-safe: a value is rounded up once it passes the half-bucket).
func applyRounding(v, rounding int) int {
	if rounding <= 0 {
		return v
	}
	return ((2*v + rounding) / (2 * rounding)) * rounding
}

// parseClock converts an "HH:MM" string to minutes past midnight.
func parseClock(s string) (int, bool) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, false
	}
	h, okH := atoi(parts[0])
	m, okM := atoi(parts[1])
	if !okH || !okM || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

// ---------------------------------------------------------------------------
// Correction + overtime transitions
// ---------------------------------------------------------------------------

func corrTransition(verb string, managerOnly bool, from []CorrectionState, to CorrectionState) func(context.Context, string, int, map[string]string) (string, error) {
	return func(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
		uid := orm.SecurityUID(ctx)
		if err := orm.CheckModelAccess(ctx, uid, corrModel, "write"); err != nil {
			return "", err
		}
		if managerOnly && !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
			return "", fmt.Errorf("only an HR Manager can %s this correction", verb)
		}
		bypass := orm.ContextWithBypass(ctx, true)
		rec, err := orm.SearchOne(bypass, corrModel, map[string]interface{}{"id": id})
		if err != nil || rec == nil {
			return "", fmt.Errorf("record not found")
		}
		current := CorrectionState(orm.AsString(rec["state"]))
		if !corrAllowed(current, from) {
			return "", fmt.Errorf("cannot %s a correction in the %q stage", verb, corrStageName(current))
		}

		// On submit, snapshot the current punch (if not already captured).
		if to == CorrConfirm {
			if err := snapshotOriginals(bypass, rec); err != nil {
				return "", err
			}
		}
		// On approval, apply the requested times then re-reconcile the punch.
		if to == CorrValidate {
			if err := applyCorrection(bypass, rec); err != nil {
				return "", err
			}
		}
		if err := orm.UpdateRecordByID(bypass, corrModel, id, map[string]interface{}{"state": string(to)}); err != nil {
			return "", err
		}
		return "", nil
	}
}

func otTransition(verb string, managerOnly bool, from []OvertimeState, to OvertimeState) func(context.Context, string, int, map[string]string) (string, error) {
	return func(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
		uid := orm.SecurityUID(ctx)
		if err := orm.CheckModelAccess(ctx, uid, overtimeModel, "write"); err != nil {
			return "", err
		}
		if managerOnly && !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
			return "", fmt.Errorf("only an HR Manager can %s this overtime request", verb)
		}
		bypass := orm.ContextWithBypass(ctx, true)
		rec, err := orm.SearchOne(bypass, overtimeModel, map[string]interface{}{"id": id})
		if err != nil || rec == nil {
			return "", fmt.Errorf("record not found")
		}
		current := OvertimeState(orm.AsString(rec["state"]))
		if !otAllowed(current, from) {
			return "", fmt.Errorf("cannot %s an overtime request in the %q stage", verb, otStageName(current))
		}
		if err := orm.UpdateRecordByID(bypass, overtimeModel, id, map[string]interface{}{"state": string(to)}); err != nil {
			return "", err
		}
		return "", nil
	}
}

// snapshotOriginals records the attendance's current punches on the correction so
// the pre-change values remain even after the correction is applied.
func snapshotOriginals(ctx context.Context, rec map[string]interface{}) error {
	attID := intOr(rec["attendance_id"])
	if attID <= 0 {
		return nil
	}
	// Already captured: leave untouched (immutable originals).
	if !parseDateTime(rec["orig_check_in"]).IsZero() || !parseDateTime(rec["orig_check_out"]).IsZero() {
		return nil
	}
	att, err := orm.SearchOne(ctx, attModel, map[string]interface{}{"id": attID})
	if err != nil || att == nil {
		return nil
	}
	vals := map[string]interface{}{}
	if ci := orm.AsString(att[checkInField]); ci != "" {
		vals["orig_check_in"] = ci
	}
	if co := orm.AsString(att[checkOutField]); co != "" {
		vals["orig_check_out"] = co
	}
	if len(vals) == 0 {
		return nil
	}
	return orm.UpdateRecordByID(ctx, corrModel, intOr(rec["id"]), vals)
}

// applyCorrection writes the approved requested punches onto the attendance,
// flags it corrected and re-runs the exception reconciliation.
func applyCorrection(ctx context.Context, rec map[string]interface{}) error {
	attID := intOr(rec["attendance_id"])
	if attID <= 0 {
		return fmt.Errorf("correction has no linked attendance")
	}
	vals := map[string]interface{}{"corrected": true}
	if ci := strings.TrimSpace(orm.AsString(rec["requested_check_in"])); ci != "" {
		vals[checkInField] = ci
	}
	if co := strings.TrimSpace(orm.AsString(rec["requested_check_out"])); co != "" {
		vals[checkOutField] = co
	}
	if len(vals) == 1 {
		// Only the corrected flag: ensure originals are still snapshotted.
		if err := snapshotOriginals(ctx, rec); err != nil {
			return err
		}
	}
	if err := orm.UpdateRecordByID(ctx, attModel, attID, vals); err != nil {
		return err
	}
	if err := orm.UpdateRecordByID(ctx, corrModel, intOr(rec["id"]), map[string]interface{}{"applied": true}); err != nil {
		return err
	}
	reconcileAttendance(ctx, attID)
	return nil
}

// ---------------------------------------------------------------------------
// State helpers
// ---------------------------------------------------------------------------

func corrAllowed(current CorrectionState, allowed []CorrectionState) bool {
	for _, s := range allowed {
		if current == s {
			return true
		}
	}
	return false
}

func corrStageName(s CorrectionState) string {
	switch s {
	case CorrDraft:
		return "Draft"
	case CorrConfirm:
		return "Waiting Approval"
	case CorrValidate:
		return "Approved"
	case CorrRefuse:
		return "Refused"
	default:
		return "Unset"
	}
}

func otAllowed(current OvertimeState, allowed []OvertimeState) bool {
	for _, s := range allowed {
		if current == s {
			return true
		}
	}
	return false
}

func otStageName(s OvertimeState) string {
	switch s {
	case OtDraft:
		return "Draft"
	case OtConfirm:
		return "Waiting Approval"
	case OtValidate:
		return "Approved"
	case OtRefuse:
		return "Refused"
	default:
		return "Unset"
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func intOr(v interface{}) int {
	n, ok := orm.CoerceInt64(v)
	if !ok {
		return 0
	}
	return int(n)
}

func atoi(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	neg := false
	if s[0] == '-' {
		neg = true
		s = s[1:]
	}
	if s == "" {
		return 0, false
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	if neg {
		n = -n
	}
	return n, true
}

// setIfChanged* add a field to vals only when it differs from the live record, so
// a re-fired event produces no write (and therefore no update loop).
func setIfChangedStr(vals map[string]interface{}, field string, rec map[string]interface{}, want string) {
	if orm.AsString(rec[field]) != want {
		vals[field] = want
	}
}

func setIfChangedInt(vals map[string]interface{}, field string, rec map[string]interface{}, want int) {
	if intOr(rec[field]) != want {
		vals[field] = want
	}
}

func setIfChangedBool(vals map[string]interface{}, field string, rec map[string]interface{}, want bool) {
	if orm.AsBool(rec[field]) != want {
		vals[field] = want
	}
}

// parseDateTime tolerates the shapes the ORM returns for a DateTime column.
func parseDateTime(v interface{}) time.Time {
	switch t := v.(type) {
	case time.Time:
		return t.UTC()
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return time.Time{}
		}
		for _, layout := range []string{datetimeFull, time.RFC3339, "2006-01-02T15:04:05", dateLayout} {
			if parsed, err := time.Parse(layout, s); err == nil {
				return parsed.UTC()
			}
		}
	}
	return time.Time{}
}
