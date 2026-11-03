package models

import (
	"context"
	"fmt"
	"time"

	"nexa/core/orm"
)

// Learning & development workflow actions. Budget-bearing and grading decisions
// are maker-checker guarded — an HR Manager approves a training request they did
// not raise. Approving a request only records a decision; it never posts a cost
// anywhere. Skill-gap detection compares each skill a job requires against the
// level the employee currently holds and upserts an open gap row (idempotent).

const (
	examQuestionModel = "hr.training.exam.question"
	pathCourseModel   = "hr.training.path.course"
	jobModel          = "hr.job"
)

func init() {
	// Training requests.
	orm.RegisterObjectAction(requestModel, "action_request_submit", requestSubmit)
	orm.RegisterObjectAction(requestModel, "action_request_approve", requestApprove)
	orm.RegisterObjectAction(requestModel, "action_request_refuse", requestRefuse)
	orm.RegisterObjectAction(requestModel, "action_request_cancel", requestCancel)

	// Exam attempts.
	orm.RegisterObjectAction(attemptModel, "action_attempt_submit", attemptSubmit)
	orm.RegisterObjectAction(attemptModel, "action_attempt_score", attemptScore)

	// Skill gaps.
	orm.RegisterObjectAction(skillGapModel, "action_gap_recompute", gapRecompute)
	orm.RegisterObjectAction(employeeModel, "action_detect_skill_gaps", detectSkillGaps)

	// Certificate alerts.
	orm.RegisterObjectAction(certAlertModel, "action_cert_alert_resolve", certAlertResolve)
}

// ---------------------------------------------------------------------------
// Training requests
// ---------------------------------------------------------------------------

func requestSubmit(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, requestModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("training request not found")
	}
	if RequestState(orm.AsString(rec["state"])) != ReqDraft {
		return "", fmt.Errorf("only a draft request can be submitted")
	}
	if intOr(rec["course_id"]) <= 0 {
		return "", fmt.Errorf("select a course before submitting")
	}
	return "", orm.UpdateRecordByID(bypass, requestModel, id, map[string]interface{}{
		"state":        string(ReqSubmitted),
		"requested_by": orm.SecurityUID(ctx),
	})
}

func requestApprove(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can approve a training request")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, requestModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("training request not found")
	}
	if RequestState(orm.AsString(rec["state"])) != ReqSubmitted {
		return "", fmt.Errorf("only a submitted request can be approved")
	}
	if requester := intOr(rec["requested_by"]); requester != 0 && requester == uid {
		return "", fmt.Errorf("maker-checker: the requester cannot approve their own training request")
	}
	return "", orm.UpdateRecordByID(bypass, requestModel, id, map[string]interface{}{
		"state":        string(ReqApproved),
		"approver_id":  uid,
		"approve_date": time.Now().UTC().Format(dateLayout),
	})
}

func requestRefuse(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can refuse a training request")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, requestModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("training request not found")
	}
	if RequestState(orm.AsString(rec["state"])) == ReqApproved {
		return "", fmt.Errorf("an approved request cannot be refused")
	}
	return "", orm.UpdateRecordByID(bypass, requestModel, id, map[string]interface{}{
		"state":       string(ReqRefused),
		"approver_id": uid,
	})
}

func requestCancel(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can cancel a training request")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, requestModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("training request not found")
	}
	if RequestState(orm.AsString(rec["state"])) == ReqCancelled {
		return "", fmt.Errorf("this request is already cancelled")
	}
	return "", orm.UpdateRecordByID(bypass, requestModel, id, map[string]interface{}{"state": string(ReqCancelled)})
}

// ---------------------------------------------------------------------------
// Exam attempts
// ---------------------------------------------------------------------------

func attemptSubmit(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, attemptModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("attempt not found")
	}
	if AttemptState(orm.AsString(rec["state"])) != AttInProgress {
		return "", fmt.Errorf("only an in-progress attempt can be submitted")
	}
	return "", orm.UpdateRecordByID(bypass, attemptModel, id, map[string]interface{}{
		"state":        string(AttSubmitted),
		"attempt_date": time.Now().UTC().Format(datetimeFull),
	})
}

// attemptScore finalises grading. It requires a manager (a self-scored attempt is
// not trusted) and recomputes the percentage / pass directly so the outcome is
// never gated on a stale derived column.
func attemptScore(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can score an attempt")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, attemptModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("attempt not found")
	}
	if AttemptState(orm.AsString(rec["state"])) != AttSubmitted {
		return "", fmt.Errorf("only a submitted attempt can be scored")
	}
	examID := intOr(rec["exam_id"])
	if examID <= 0 {
		return "", fmt.Errorf("the attempt has no exam")
	}
	exam, err := orm.SearchOne(bypass, examModel, map[string]interface{}{"id": examID})
	if err != nil || exam == nil {
		return "", fmt.Errorf("the attempt's exam is missing")
	}
	pct := attemptPct(floatOr(rec["total_score"]), floatOr(rec["max_score"]))
	passed := passedFor(pct, floatOr(exam["pass_mark"]))
	return "", orm.UpdateRecordByID(bypass, attemptModel, id, map[string]interface{}{
		"state":      string(AttScored),
		"percentage": pct,
		"passed":     passed,
	})
}

// ---------------------------------------------------------------------------
// Skill gaps
// ---------------------------------------------------------------------------

// gapRecompute refreshes the current / required levels from the skill tables for
// this gap's employee + job + skill, then the stored gap follows.
func gapRecompute(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, skillGapModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("skill gap not found")
	}
	cur := currentLevel(bypass, intOr(rec["employee_id"]), intOr(rec["skill_id"]))
	req := requiredLevel(bypass, intOr(rec["job_id"]), intOr(rec["skill_id"]))
	return "", orm.UpdateRecordByID(bypass, skillGapModel, id, map[string]interface{}{
		"current_level":  cur,
		"required_level": req,
		"gap":            gapBetween(cur, req),
		"detected_date":  time.Now().UTC().Format(dateLayout),
	})
}

// detectSkillGaps compares each skill the employee's job requires against the
// level the employee currently holds and upserts an open gap row per skill.
func detectSkillGaps(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	emp, err := orm.SearchOne(bypass, employeeModel, map[string]interface{}{"id": id})
	if err != nil || emp == nil {
		return "", fmt.Errorf("employee not found")
	}
	jobID := intOr(emp["job_id"])
	if jobID <= 0 {
		return "", fmt.Errorf("the employee has no job position to compare against")
	}
	reqs, err := orm.Search(bypass, jobSkillModel, [][]interface{}{{"job_id", "=", jobID}})
	if err != nil {
		return "", fmt.Errorf("job requirements could not be read")
	}
	today := time.Now().UTC().Format(dateLayout)
	created, updated := 0, 0
	for _, r := range reqs {
		skillID := intOr(r["skill_id"])
		if skillID <= 0 {
			continue
		}
		reqLevel := orm.AsString(r["level"])
		curLevel := currentLevel(bypass, id, skillID)
		gap := gapBetween(curLevel, reqLevel)
		vals := map[string]interface{}{
			"required_level": reqLevel,
			"current_level":  curLevel,
			"gap":            gap,
			"detected_date":  today,
		}
		existing, _ := orm.SearchOne(bypass, skillGapModel, map[string]interface{}{"employee_id": id, "skill_id": skillID})
		if existing != nil && intOr(existing["id"]) > 0 {
			vals["job_id"] = jobID
			if gap <= 0 {
				vals["state"] = string(GapAddressed)
			}
			_ = orm.UpdateRecordByID(bypass, skillGapModel, intOr(existing["id"]), vals)
			updated++
			continue
		}
		if gap <= 0 {
			continue // no shortfall: nothing to open
		}
		vals["employee_id"] = id
		vals["job_id"] = jobID
		vals["skill_id"] = skillID
		vals["state"] = string(GapOpen)
		if _, err := createRecord(bypass, skillGapModel, vals); err == nil {
			created++
		}
	}
	return fmt.Sprintf("Detected skill gaps: %d new, %d refreshed.", created, updated), nil
}

func currentLevel(ctx context.Context, employeeID, skillID int) string {
	if employeeID <= 0 || skillID <= 0 {
		return "beginner"
	}
	row, err := orm.SearchOne(ctx, employeeSkillModel, map[string]interface{}{"employee_id": employeeID, "skill_id": skillID})
	if err != nil || row == nil {
		return "beginner"
	}
	if lvl := orm.AsString(row["level"]); lvl != "" {
		return lvl
	}
	return "beginner"
}

func requiredLevel(ctx context.Context, jobID, skillID int) string {
	if jobID <= 0 || skillID <= 0 {
		return "beginner"
	}
	row, err := orm.SearchOne(ctx, jobSkillModel, map[string]interface{}{"job_id": jobID, "skill_id": skillID})
	if err != nil || row == nil {
		return "beginner"
	}
	if lvl := orm.AsString(row["level"]); lvl != "" {
		return lvl
	}
	return "beginner"
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func certAlertResolve(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can resolve a certificate alert")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, certAlertModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("alert not found")
	}
	if !orm.AsBool(rec["open"]) {
		return "", fmt.Errorf("this alert is already resolved")
	}
	return "", orm.UpdateRecordByID(bypass, certAlertModel, id, map[string]interface{}{"open": false})
}

func createRecord(ctx context.Context, model string, vals map[string]interface{}) (int, error) {
	inst, ok := orm.Registry[model]
	if !ok {
		return 0, fmt.Errorf("model %s is not registered", model)
	}
	return orm.Create(ctx, inst, vals)
}
