package models

import (
	"context"
	"fmt"
	"time"

	"nexa/core/orm"
)

// Object actions drive the attendance lifecycle. A session is opened for roll-call,
// populated with one line per learner in the section, then finalized (blocked when no
// marks exist). After finalization a mark is amended through the correction action, which
// refuses a change with no reason and writes an append-only school.attendance.correction
// row so the edit stays auditable. Alerts raised by the cron can be acknowledged and
// resolved. Populate/finalize/correct need clerk-or-manager; cancel and alert handling
// are consequential so they stay manager-gated where they alter an ended record.

func init() {
	orm.RegisterObjectAction(sessionModel, "attendance_open", actOpen)
	orm.RegisterObjectAction(sessionModel, "attendance_populate", actPopulate)
	orm.RegisterObjectAction(sessionModel, "attendance_finalize", actFinalize)
	orm.RegisterObjectAction(sessionModel, "attendance_cancel", actCancel)

	orm.RegisterObjectAction(lineModel, "attendance_correct", actCorrect)

	orm.RegisterObjectAction(alertModel, "absence_alert_acknowledge", actAckAlert)
	orm.RegisterObjectAction(alertModel, "absence_alert_resolve", actResolveAlert)
}

// ---------------------------------------------------------------------------
// Session actions
// ---------------------------------------------------------------------------

func actOpen(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to open an attendance session")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getSession(bypass, id)
	if err != nil {
		return "", err
	}
	switch strOr(rec["state"]) {
	case string(SessDraft):
		// ok
	case string(SessOpen):
		return "Session is already open", nil
	default:
		return "", fmt.Errorf("only a draft session can be opened (current status %q)", label(strOr(rec["state"])))
	}
	if err := orm.UpdateRecordByID(bypass, sessionModel, id, map[string]interface{}{
		"state": string(SessOpen),
	}); err != nil {
		return "", err
	}
	return "Attendance session opened", nil
}

// actPopulate creates one attendance line per active learner in the session's section,
// skipping learners already marked. It is refused once the session is finalized.
func actPopulate(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to populate an attendance session")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getSession(bypass, id)
	if err != nil {
		return "", err
	}
	state := strOr(rec["state"])
	if state == string(SessFinalized) || state == string(SessCancelled) {
		return "", fmt.Errorf("a %s session cannot be populated", label(state))
	}
	sectionID := intOr(rec["section_id"])
	if sectionID <= 0 {
		return "", fmt.Errorf("the session must name a section before it can be populated")
	}
	students, err := orm.Search(bypass, studentModel, [][]interface{}{
		{"section_id", "=", sectionID},
		{"active", "=", true},
	})
	if err != nil {
		return "", fmt.Errorf("learners could not be loaded: %w", err)
	}
	marked := map[int]bool{}
	existing, _ := orm.Search(bypass, lineModel, [][]interface{}{{"session_id", "=", id}})
	for _, l := range existing {
		marked[intOr(l["student_id"])] = true
	}
	created := 0
	for _, s := range students {
		studentID := intOr(s["id"])
		if studentID <= 0 || marked[studentID] {
			continue
		}
		if _, err := orm.Create(bypass, orm.RegistryModel(lineModel), map[string]interface{}{
			"session_id": id,
			"student_id": studentID,
			"status":     string(AttPresent),
		}); err != nil {
			return "", fmt.Errorf("a learner could not be added to the session: %w", err)
		}
		created++
	}
	if state == string(SessDraft) {
		_ = orm.UpdateRecordByID(bypass, sessionModel, id, map[string]interface{}{"state": string(SessOpen)})
	}
	return fmt.Sprintf("Added %d learner(s) to the session", created), nil
}

func actFinalize(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to finalize an attendance session")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getSession(bypass, id)
	if err != nil {
		return "", err
	}
	state := strOr(rec["state"])
	if state == string(SessFinalized) {
		return "Session is already finalized", nil
	}
	if state == string(SessCancelled) {
		return "", fmt.Errorf("a cancelled session cannot be finalized")
	}
	count, err := orm.SearchCount(bypass, lineModel, [][]interface{}{{"session_id", "=", id}})
	if err != nil {
		return "", err
	}
	if !finalizeAllowed(count) {
		return "", fmt.Errorf("finalize the session with at least one mark recorded")
	}
	if err := orm.UpdateRecordByID(bypass, sessionModel, id, map[string]interface{}{
		"state": string(SessFinalized),
	}); err != nil {
		return "", err
	}
	return fmt.Sprintf("Session finalized with %d mark(s)", count), nil
}

func actCancel(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireManager(ctx, uid) {
		return "", fmt.Errorf("only a School Manager/Director can cancel an attendance session")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getSession(bypass, id)
	if err != nil {
		return "", err
	}
	state := strOr(rec["state"])
	if state == string(SessFinalized) {
		return "", fmt.Errorf("a finalized session cannot be cancelled; correct the records instead")
	}
	if state == string(SessCancelled) {
		return "Session is already cancelled", nil
	}
	if err := orm.UpdateRecordByID(bypass, sessionModel, id, map[string]interface{}{
		"state":  string(SessCancelled),
		"active": false,
		"notes":  withReason(strOr(rec["notes"]), args["reason"]),
	}); err != nil {
		return "", err
	}
	return "Attendance session cancelled", nil
}

// ---------------------------------------------------------------------------
// Record correction action
// ---------------------------------------------------------------------------

// actCorrect amends a learner's mark. Every change is journaled to the append-only
// correction log; once the parent session is finalized the line is also flagged as
// corrected so post-hoc edits are visible in the roll-call.
func actCorrect(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to correct an attendance record")
	}
	newStatus := args["status"]
	if !validStatus(newStatus) {
		return "", fmt.Errorf("a valid new status (present/absent/late/excused) is required")
	}
	reason := args["reason"]
	if reason == "" {
		return "", fmt.Errorf("a reason is required to correct an attendance record")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	line, err := getRecord(bypass, lineModel, id)
	if err != nil {
		return "", err
	}
	oldStatus := strOr(line["status"])
	if !statusChanged(oldStatus, newStatus) {
		return "No change to record", nil
	}
	sessionID := intOr(line["session_id"])
	session, err := getSession(bypass, sessionID)
	if err != nil {
		return "", err
	}
	if strOr(session["state"]) == string(SessCancelled) {
		return "", fmt.Errorf("records of a cancelled session cannot be corrected")
	}
	vals := map[string]interface{}{"status": newStatus}
	if strOr(session["state"]) == string(SessFinalized) {
		vals["corrected"] = true
	}
	if err := orm.UpdateRecordByID(bypass, lineModel, id, vals); err != nil {
		return "", err
	}
	appendCorrection(bypass, id, sessionID, intOr(line["student_id"]), oldStatus, newStatus, reason, uid)
	return fmt.Sprintf("Mark corrected: %s -> %s", label(oldStatus), label(newStatus)), nil
}

// ---------------------------------------------------------------------------
// Alert actions
// ---------------------------------------------------------------------------

func actAckAlert(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to acknowledge an absence alert")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, alertModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) != string(AlertOpen) {
		return "Alert is not open", nil
	}
	if err := orm.UpdateRecordByID(bypass, alertModel, id, map[string]interface{}{
		"state":     string(AlertAcknowledged),
		"ack_by_id": uid,
	}); err != nil {
		return "", err
	}
	return "Absence alert acknowledged", nil
}

func actResolveAlert(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireManager(ctx, uid) {
		return "", fmt.Errorf("only a School Manager/Director can resolve an absence alert")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	if err := orm.UpdateRecordByID(bypass, alertModel, id, map[string]interface{}{
		"state":     string(AlertResolved),
		"ack_by_id": uid,
		"notes":     withReason(strOr(mustGet(bypass, alertModel, id)["notes"]), args["reason"]),
	}); err != nil {
		return "", err
	}
	return "Absence alert resolved", nil
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

func appendCorrection(ctx context.Context, lineID, sessionID, studentID int, from, to, reason string, operator int) {
	_, _ = orm.Create(ctx, orm.RegistryModel(correctionModel), map[string]interface{}{
		"line_id":     lineID,
		"session_id":  sessionID,
		"student_id":  studentID,
		"from_status": from,
		"to_status":   to,
		"reason":      reason,
		"operator_id": operator,
		"timestamp":   time.Now().UTC().Format(datetimeFull),
	})
}

func validStatus(s string) bool {
	switch s {
	case string(AttPresent), string(AttAbsent), string(AttLate), string(AttExcused):
		return true
	}
	return false
}

func getSession(ctx context.Context, id int) (map[string]interface{}, error) {
	return getRecord(ctx, sessionModel, id)
}

func getRecord(ctx context.Context, model string, id int) (map[string]interface{}, error) {
	rec, err := orm.SearchOne(ctx, model, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return nil, fmt.Errorf("record not found")
	}
	return rec, nil
}

func mustGet(ctx context.Context, model string, id int) map[string]interface{} {
	rec, _ := orm.SearchOne(ctx, model, map[string]interface{}{"id": id})
	if rec == nil {
		return map[string]interface{}{}
	}
	return rec
}

func withReason(existing, reason string) string {
	if reason == "" {
		return existing
	}
	note := "Cancelled/Resolved: " + reason
	if existing == "" {
		return note
	}
	return existing + "\n" + note
}

func requireOfficer(ctx context.Context, uid int) bool {
	return orm.UserHasGroupXML(ctx, uid, "school.group_school_clerk") ||
		orm.UserHasGroupXML(ctx, uid, "school.group_school_manager")
}

func requireManager(ctx context.Context, uid int) bool {
	return orm.UserHasGroupXML(ctx, uid, "school.group_school_manager")
}

var stateLabels = map[string]string{
	"draft": "Draft", "open": "Open", "finalized": "Finalized", "cancelled": "Cancelled",
	"present": "Present", "absent": "Absent", "late": "Late", "excused": "Excused",
	"resolved": "Resolved", "acknowledged": "Acknowledged",
}

func label(state string) string {
	if l, ok := stateLabels[state]; ok {
		return l
	}
	return state
}
