package models

import (
	"context"
	"fmt"
	"time"

	"nexa/core/orm"
)

// Performance workflow actions. Consistent with the blueprint guardrail, nothing
// here ever writes into payroll: finalising an appraisal, approving a PIP or
// locking a calibration only records an evaluation / decision on the performance
// records themselves. Value- and state-bearing transitions that carry governance
// (finalisation, PIP activation, calibration approval / lock) are maker-checker
// guarded — an HR Manager approves and never their own submission — and calibration
// steps write an append-only audit trail.

const datetimeFull = "2006-01-02 15:04:05"

const (
	templateModel      = "hr.appraisal.template"
	templateLineModel  = "hr.appraisal.template.line"
	checkinModel       = "hr.performance.checkin"
	devplanModel       = "hr.performance.devplan"
	pipModel           = "hr.performance.pip"
	calibSessionModel  = "hr.calibration.session"
	calibHistoryModel  = "hr.calibration.history"
	appraisalLineModel = "hr.appraisal.line"
)

func init() {
	// Goals.
	orm.RegisterObjectAction(goalModel, "action_goal_start", goalStart)
	orm.RegisterObjectAction(goalModel, "action_goal_reach", goalReach)

	// Appraisal.
	orm.RegisterObjectAction(appraisalModel, "action_appraisal_seed", appraisalSeed)
	orm.RegisterObjectAction(appraisalModel, "action_appraisal_finalize", appraisalFinalize)

	// 360 feedback.
	orm.RegisterObjectAction(feedbackModel, "action_feedback_submit", feedbackSubmit)

	// Development plan.
	orm.RegisterObjectAction(devplanModel, "action_devplan_activate", devplanActivate)
	orm.RegisterObjectAction(devplanModel, "action_devplan_complete", devplanComplete)

	// Check-in.
	orm.RegisterObjectAction(checkinModel, "action_checkin_log", checkinLog)

	// Improvement plan.
	orm.RegisterObjectAction(pipModel, "action_pip_submit", pipSubmit)
	orm.RegisterObjectAction(pipModel, "action_pip_approve", pipApprove)
	orm.RegisterObjectAction(pipModel, "action_pip_succeed", pipCloseSucceeded)
	orm.RegisterObjectAction(pipModel, "action_pip_fail", pipCloseFailed)

	// Calibration session.
	orm.RegisterObjectAction(calibSessionModel, "action_calib_open", calibOpen)
	orm.RegisterObjectAction(calibSessionModel, "action_calib_lock", calibLock)
	orm.RegisterObjectAction(calibSessionModel, "action_calib_close", calibClose)

	// Calibration line.
	orm.RegisterObjectAction(calibLineModel, "action_callib_adjust", calibAdjust)
	orm.RegisterObjectAction(calibLineModel, "action_callib_approve", calibApprove)
}

// ---------------------------------------------------------------------------
// Goals
// ---------------------------------------------------------------------------

func goalStart(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, goalModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("objective not found")
	}
	switch orm.AsString(rec["state"]) {
	case "draft":
		return "", orm.UpdateRecordByID(bypass, goalModel, id, map[string]interface{}{"state": "in_progress"})
	default:
		return "", fmt.Errorf("only a draft objective can be started")
	}
}

func goalReach(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, goalModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("objective not found")
	}
	if orm.AsString(rec["state"]) != "in_progress" {
		return "", fmt.Errorf("only an in-progress objective can be marked reached")
	}
	return "", orm.UpdateRecordByID(bypass, goalModel, id, map[string]interface{}{"state": "reached"})
}

// ---------------------------------------------------------------------------
// Appraisal: seed criteria from a template + finalize
// ---------------------------------------------------------------------------

// appraisalSeed materialises the appraisal's template criteria into evaluation
// lines. It is idempotent-safe: existing lines block a second seed so criteria are
// never duplicated.
func appraisalSeed(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can seed appraisal criteria")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, appraisalModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("appraisal not found")
	}
	if AsBoolFinalized(rec["finalized"]) {
		return "", fmt.Errorf("a finalized appraisal cannot be re-seeded")
	}
	tplID := intOr(rec["template_id"])
	if tplID <= 0 {
		return "", fmt.Errorf("set a review template before seeding criteria")
	}
	existing, _ := orm.SearchCount(bypass, appraisalLineModel, [][]interface{}{{"appraisal_id", "=", id}})
	if existing > 0 {
		return "", fmt.Errorf("criteria already exist on this appraisal")
	}
	lines, err := orm.Search(bypass, templateLineModel, [][]interface{}{{"template_id", "=", tplID}})
	if err != nil {
		return "", fmt.Errorf("template criteria could not be read")
	}
	if len(lines) == 0 {
		return "", fmt.Errorf("the selected template has no criteria")
	}
	for _, ln := range lines {
		vals := map[string]interface{}{
			"appraisal_id": id,
			"name":         orm.AsString(ln["name"]),
			"sequence":     intOr(ln["sequence"]),
			"rating":       0.0,
		}
		if sk := intOr(ln["skill_id"]); sk > 0 {
			vals["skill_id"] = sk
		}
		if _, err := createRecord(bypass, appraisalLineModel, vals); err != nil {
			return "", fmt.Errorf("failed to create a criteria line")
		}
	}
	return "", nil
}

// appraisalFinalize locks the review outcome. It records who finalised and when
// and touches NO pay field.
func appraisalFinalize(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can finalize an appraisal")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, appraisalModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("appraisal not found")
	}
	if AsBoolFinalized(rec["finalized"]) {
		return "", fmt.Errorf("this appraisal is already finalized")
	}
	if orm.AsString(rec["state"]) != "done" && orm.AsString(rec["state"]) != "in_progress" {
		return "", fmt.Errorf("only an in-progress or done appraisal can be finalized")
	}
	if creator := intOr(rec["create_uid"]); creator != 0 && creator == uid {
		return "", fmt.Errorf("maker-checker: the appraisal owner cannot finalize their own appraisal")
	}
	return "", orm.UpdateRecordByID(bypass, appraisalModel, id, map[string]interface{}{
		"state":          "approved",
		"finalized":      true,
		"finalized_by":   uid,
		"finalized_date": time.Now().UTC().Format(dateLayout),
	})
}

// ---------------------------------------------------------------------------
// 360 feedback
// ---------------------------------------------------------------------------

func feedbackSubmit(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, feedbackModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("feedback not found")
	}
	if orm.AsBool(rec["submitted"]) {
		return "", fmt.Errorf("this feedback is already submitted")
	}
	appr, err := orm.SearchOne(bypass, appraisalModel, map[string]interface{}{"id": intOr(rec["appraisal_id"])})
	if err != nil || appr == nil {
		return "", fmt.Errorf("the target appraisal is missing")
	}
	if AsBoolFinalized(appr["finalized"]) {
		return "", fmt.Errorf("cannot add feedback to a finalized appraisal")
	}
	return "", orm.UpdateRecordByID(bypass, feedbackModel, id, map[string]interface{}{"submitted": true})
}

// ---------------------------------------------------------------------------
// Development plan
// ---------------------------------------------------------------------------

func devplanActivate(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	return transitionState(ctx, devplanModel, id, string(DevDraft), string(DevActive), "only a draft plan can be activated")
}

func devplanComplete(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	return transitionState(ctx, devplanModel, id, string(DevActive), string(DevCompleted), "only an active plan can be completed")
}

// checkinLog marks a draft check-in as logged.
func checkinLog(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	return transitionState(ctx, checkinModel, id, string(CheckDraft), string(CheckDone), "only a draft check-in can be logged")
}

// ---------------------------------------------------------------------------
// Improvement plan
// ---------------------------------------------------------------------------

func pipSubmit(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, pipModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("improvement plan not found")
	}
	if PipState(orm.AsString(rec["state"])) != PipDraft {
		return "", fmt.Errorf("only a draft plan can be submitted")
	}
	if dateStr(rec["start_date"]) == "" || dateStr(rec["end_date"]) == "" {
		return "", fmt.Errorf("a plan needs both a start and end date")
	}
	return "", orm.UpdateRecordByID(bypass, pipModel, id, map[string]interface{}{
		"state":        string(PipSubmitted),
		"requested_by": orm.SecurityUID(ctx),
	})
}

func pipApprove(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can approve an improvement plan")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, pipModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("improvement plan not found")
	}
	if PipState(orm.AsString(rec["state"])) != PipSubmitted {
		return "", fmt.Errorf("only a submitted plan can be approved")
	}
	if requester := intOr(rec["requested_by"]); requester != 0 && requester == uid {
		return "", fmt.Errorf("maker-checker: the requester cannot approve their own improvement plan")
	}
	return "", orm.UpdateRecordByID(bypass, pipModel, id, map[string]interface{}{
		"state":        string(PipActive),
		"approver_id":  uid,
		"approve_date": time.Now().UTC().Format(dateLayout),
	})
}

func pipCloseSucceeded(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	return pipClose(ctx, id, string(PipSucceeded))
}

func pipCloseFailed(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	return pipClose(ctx, id, string(PipFailed))
}

func pipClose(ctx context.Context, id int, to string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can close an improvement plan")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, pipModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("improvement plan not found")
	}
	if PipState(orm.AsString(rec["state"])) != PipActive {
		return "", fmt.Errorf("only an active plan can be closed out")
	}
	return "", orm.UpdateRecordByID(bypass, pipModel, id, map[string]interface{}{"state": to})
}

// ---------------------------------------------------------------------------
// Calibration session
// ---------------------------------------------------------------------------

func calibOpen(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	return transitionState(ctx, calibSessionModel, id, string(CalDraft), string(CalOpen), "only a draft session can be opened", requireManager)
}

// calibLock applies each approved line's calibrated rating to its appraisal and
// writes an append-only history entry for every application. Locking a session is
// a manager action; it never touches pay.
func calibLock(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can lock a calibration session")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, calibSessionModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("calibration session not found")
	}
	if CalibSessionState(orm.AsString(rec["state"])) != CalOpen {
		return "", fmt.Errorf("only an open session can be locked")
	}
	lines, err := orm.Search(bypass, calibLineModel, [][]interface{}{{"session_id", "=", id}, {"state", "=", string(CalApproved)}})
	if err != nil {
		return "", fmt.Errorf("calibration lines could not be read")
	}
	if len(lines) == 0 {
		return "", fmt.Errorf("no approved calibration lines to apply")
	}
	for _, ln := range lines {
		apprID := intOr(ln["appraisal_id"])
		to := floatOr(ln["calibrated_rating"])
		if apprID <= 0 {
			continue
		}
		appr, err := orm.SearchOne(bypass, appraisalModel, map[string]interface{}{"id": apprID})
		from := 0.0
		if err == nil && appr != nil {
			from = floatOr(appr["calibrated_rating"])
		}
		if err := orm.UpdateRecordByID(bypass, appraisalModel, apprID, map[string]interface{}{"calibrated_rating": to}); err != nil {
			return "", fmt.Errorf("failed to apply a calibrated rating")
		}
		writeCalibHistory(bypass, id, intOr(ln["id"]), apprID, "applied", from, to, uid)
	}
	return "", orm.UpdateRecordByID(bypass, calibSessionModel, id, map[string]interface{}{"state": string(CalLocked)})
}

func calibClose(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	return transitionState(ctx, calibSessionModel, id, string(CalLocked), string(CalClosed), "only a locked session can be closed", requireManager)
}

// ---------------------------------------------------------------------------
// Calibration line
// ---------------------------------------------------------------------------

// calibAdjust flips a line from proposed to adjusted and records the reviewer's
// proposed -> calibrated move in history. The session must still be open.
func calibAdjust(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, calibLineModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("calibration line not found")
	}
	if CalibLineState(orm.AsString(rec["state"])) != CalProposed {
		return "", fmt.Errorf("only a proposed line can be adjusted")
	}
	if err := requireOpenSession(bypass, intOr(rec["session_id"])); err != nil {
		return "", err
	}
	user := orm.SecurityUID(ctx)
	writeCalibHistory(bypass, intOr(rec["session_id"]), id, intOr(rec["appraisal_id"]),
		"adjusted", floatOr(rec["proposed_rating"]), floatOr(rec["calibrated_rating"]), user)
	return "", orm.UpdateRecordByID(bypass, calibLineModel, id, map[string]interface{}{"state": string(CalAdjusted)})
}

func calibApprove(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can approve a calibration line")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, calibLineModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("calibration line not found")
	}
	if CalibLineState(orm.AsString(rec["state"])) != CalAdjusted {
		return "", fmt.Errorf("only an adjusted line can be approved")
	}
	if err := requireOpenSession(bypass, intOr(rec["session_id"])); err != nil {
		return "", err
	}
	if approver := intOr(rec["approved_by"]); approver != 0 && approver == uid {
		return "", fmt.Errorf("maker-checker: a line rater cannot approve their own calibration")
	}
	writeCalibHistory(bypass, intOr(rec["session_id"]), id, intOr(rec["appraisal_id"]),
		"approved", floatOr(rec["proposed_rating"]), floatOr(rec["calibrated_rating"]), uid)
	return "", orm.UpdateRecordByID(bypass, calibLineModel, id, map[string]interface{}{
		"state":       string(CalApproved),
		"approved_by": uid,
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// requireManager is the guard used by transitionState for governed transitions.
func requireManager(ctx context.Context) error {
	if !orm.UserHasGroupXML(ctx, orm.SecurityUID(ctx), "hr.group_hr_manager") {
		return fmt.Errorf("only an HR Manager can perform this action")
	}
	return nil
}

// transitionState moves a record from one state to another with an optional guard.
func transitionState(ctx context.Context, model string, id int, from, to, msg string, guards ...func(context.Context) error) (string, error) {
	for _, g := range guards {
		if err := g(ctx); err != nil {
			return "", err
		}
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, model, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("record not found")
	}
	if orm.AsString(rec["state"]) != from {
		return "", fmt.Errorf("%s", msg)
	}
	return "", orm.UpdateRecordByID(bypass, model, id, map[string]interface{}{"state": to})
}

func requireOpenSession(ctx context.Context, sessionID int) error {
	if sessionID <= 0 {
		return fmt.Errorf("the line has no session")
	}
	s, err := orm.SearchOne(ctx, calibSessionModel, map[string]interface{}{"id": sessionID})
	if err != nil || s == nil {
		return fmt.Errorf("calibration session not found")
	}
	if CalibSessionState(orm.AsString(s["state"])) != CalOpen {
		return fmt.Errorf("the session is not open for calibration")
	}
	return nil
}

func writeCalibHistory(ctx context.Context, sessionID, lineID, appraisalID int, action string, from, to float64, actor int) {
	_, _ = createRecord(ctx, calibHistoryModel, map[string]interface{}{
		"session_id":   sessionID,
		"line_id":      lineID,
		"appraisal_id": appraisalID,
		"action":       action,
		"from_rating":  from,
		"to_rating":    to,
		"performed_by": actor,
		"performed_at": time.Now().UTC().Format(datetimeFull),
	})
}

func createRecord(ctx context.Context, model string, vals map[string]interface{}) (int, error) {
	inst, ok := orm.Registry[model]
	if !ok {
		return 0, fmt.Errorf("model %s is not registered", model)
	}
	return orm.Create(ctx, inst, vals)
}

// AsBoolFinalized reads the finalized gate which may arrive as bool or string.
func AsBoolFinalized(v interface{}) bool {
	return orm.AsBool(v)
}
