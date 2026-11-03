package models

import (
	"context"
	"fmt"
	"time"

	"nexa/core/orm"
)

// Benefits & compensation workflow actions. Every value-bearing transition is
// maker-checker guarded (an HR Manager approves, and never their own request) and
// validated: an enrolment must satisfy the plan's tenure / department / dependent
// rules before it activates; a salary change must sit within its target grade band
// before it applies; a review line must respect the cycle budget before it approves.
// Applying an outcome only records a decision — it never posts into payroll.

func init() {
	orm.RegisterObjectAction(enrollmentModel, "action_enroll_submit", enrollSubmit)
	orm.RegisterObjectAction(enrollmentModel, "action_enroll_confirm", enrollConfirm)
	orm.RegisterObjectAction(enrollmentModel, "action_enroll_cancel", enrollCancel)

	orm.RegisterObjectAction(allowanceModel, "action_allowance_submit", allowanceSubmit)
	orm.RegisterObjectAction(allowanceModel, "action_allowance_approve", allowanceApprove)
	orm.RegisterObjectAction(allowanceModel, "action_allowance_refuse", allowanceRefuse)

	orm.RegisterObjectAction(compChangeModel, "action_change_submit", changeSubmit)
	orm.RegisterObjectAction(compChangeModel, "action_change_approve", changeApprove)
	orm.RegisterObjectAction(compChangeModel, "action_change_refuse", changeRefuse)

	orm.RegisterObjectAction(reviewCycleModel, "action_cycle_open", cycleOpen)
	orm.RegisterObjectAction(reviewCycleModel, "action_cycle_lock", cycleLock)
	orm.RegisterObjectAction(reviewCycleModel, "action_cycle_close", cycleClose)

	orm.RegisterObjectAction(reviewLineModel, "action_line_recommend", lineRecommend)
	orm.RegisterObjectAction(reviewLineModel, "action_line_approve", lineApprove)
	orm.RegisterObjectAction(reviewLineModel, "action_line_reject", lineReject)
}

// ---------------------------------------------------------------------------
// Enrolment
// ---------------------------------------------------------------------------

func enrollSubmit(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, enrollmentModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("enrolment not found")
	}
	if EnrollmentState(orm.AsString(rec["state"])) != EnrDraft {
		return "", fmt.Errorf("only a draft enrolment can be submitted")
	}
	return "", orm.UpdateRecordByID(bypass, enrollmentModel, id, map[string]interface{}{"state": string(EnrPending)})
}

func enrollConfirm(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can confirm an enrolment")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, enrollmentModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("enrolment not found")
	}
	if EnrollmentState(orm.AsString(rec["state"])) != EnrPending {
		return "", fmt.Errorf("only a pending enrolment can be confirmed")
	}
	if creator := intOr(rec["create_uid"]); creator != 0 && creator == uid {
		return "", fmt.Errorf("maker-checker: the enrolment requester cannot approve their own enrolment")
	}
	if err := checkEligibility(bypass, rec); err != nil {
		return "", err
	}
	return "", orm.UpdateRecordByID(bypass, enrollmentModel, id, map[string]interface{}{
		"state":         string(EnrActive),
		"approver_id":   uid,
		"approved_date": time.Now().UTC().Format(dateLayout),
	})
}

func enrollCancel(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can cancel an enrolment")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, enrollmentModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("enrolment not found")
	}
	if EnrollmentState(orm.AsString(rec["state"])) == EnrCancelled {
		return "", fmt.Errorf("this enrolment is already cancelled")
	}
	return "", orm.UpdateRecordByID(bypass, enrollmentModel, id, map[string]interface{}{"state": string(EnrCancelled)})
}

// checkEligibility validates an enrolment against its plan's department, tenure
// and dependent limits.
func checkEligibility(ctx context.Context, rec map[string]interface{}) error {
	plan := loadPlan(ctx, intOr(rec["plan_id"]))
	if plan == nil {
		return fmt.Errorf("the enrolment's plan is missing")
	}
	if !activeInPeriod(dateStr(plan["valid_from"]), dateStr(plan["valid_to"]), dateStr(rec["start_date"])) {
		return fmt.Errorf("the plan is not valid on the coverage start date")
	}
	empID := intOr(rec["employee_id"])
	emp, err := orm.SearchOne(ctx, employeeModel, map[string]interface{}{"id": empID})
	if err != nil || emp == nil {
		return fmt.Errorf("enrolment employee not found")
	}
	if reqDept := intOr(plan["eligible_dept_id"]); reqDept > 0 && reqDept != intOr(emp["department_id"]) {
		return fmt.Errorf("the employee's department is not eligible for this plan")
	}
	if minDays := intOr(plan["min_tenure_days"]); !tenureDaysOK(dateStr(emp["hire_date"]), dateStr(rec["start_date"]), minDays) {
		return fmt.Errorf("the employee has not met the plan's minimum tenure")
	}
	if maxDep := intOr(plan["max_dependents"]); maxDep > 0 {
		n, _ := orm.SearchCount(ctx, dependentModel, [][]interface{}{{"enrollment_id", "=", intOr(rec["id"])}})
		if int(n) > maxDep {
			return fmt.Errorf("the enrolment has more dependents than the plan allows (%d)", maxDep)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Allowance / reimbursement
// ---------------------------------------------------------------------------

func allowanceSubmit(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, allowanceModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("allowance not found")
	}
	if AllowanceState(orm.AsString(rec["state"])) != AlwDraft {
		return "", fmt.Errorf("only a draft allowance can be submitted")
	}
	if floatOr(rec["amount"]) <= 0 {
		return "", fmt.Errorf("the allowance amount must be positive")
	}
	return "", orm.UpdateRecordByID(bypass, allowanceModel, id, map[string]interface{}{
		"state":        string(AlwSubmitted),
		"requested_by": orm.SecurityUID(ctx),
	})
}

func allowanceApprove(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can approve an allowance")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, allowanceModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("allowance not found")
	}
	if AllowanceState(orm.AsString(rec["state"])) != AlwSubmitted {
		return "", fmt.Errorf("only a submitted allowance can be approved")
	}
	if requester := intOr(rec["requested_by"]); requester != 0 && requester == uid {
		return "", fmt.Errorf("maker-checker: the requester cannot approve their own allowance")
	}
	return "", orm.UpdateRecordByID(bypass, allowanceModel, id, map[string]interface{}{
		"state":        string(AlwApproved),
		"approver_id":  uid,
		"approve_date": time.Now().UTC().Format(dateLayout),
	})
}

func allowanceRefuse(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can refuse an allowance")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, allowanceModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("allowance not found")
	}
	if AllowanceState(orm.AsString(rec["state"])) == AlwApproved {
		return "", fmt.Errorf("an approved allowance cannot be refused")
	}
	return "", orm.UpdateRecordByID(bypass, allowanceModel, id, map[string]interface{}{
		"state":       string(AlwRefused),
		"approver_id": uid,
	})
}

// ---------------------------------------------------------------------------
// Compensation change
// ---------------------------------------------------------------------------

func changeSubmit(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, compChangeModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("compensation change not found")
	}
	if CompChangeState(orm.AsString(rec["state"])) != ChgDraft {
		return "", fmt.Errorf("only a draft change can be submitted")
	}
	if dateStr(rec["effective_date"]) == "" {
		return "", fmt.Errorf("an effective date is required")
	}
	return "", orm.UpdateRecordByID(bypass, compChangeModel, id, map[string]interface{}{
		"state":        string(ChgSubmitted),
		"requested_by": orm.SecurityUID(ctx),
	})
}

func changeApprove(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can approve a compensation change")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, compChangeModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("compensation change not found")
	}
	if CompChangeState(orm.AsString(rec["state"])) != ChgSubmitted {
		return "", fmt.Errorf("only a submitted change can be approved")
	}
	if requester := intOr(rec["requested_by"]); requester != 0 && requester == uid {
		return "", fmt.Errorf("maker-checker: the requester cannot approve their own change")
	}
	if !withinBandValue(bypass, rec) {
		return "", fmt.Errorf("the new salary is outside the target grade's salary band")
	}
	return "", orm.UpdateRecordByID(bypass, compChangeModel, id, map[string]interface{}{
		"state":        string(ChgApplied),
		"approver_id":  uid,
		"approve_date": time.Now().UTC().Format(dateLayout),
	})
}

func changeRefuse(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can refuse a compensation change")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, compChangeModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("compensation change not found")
	}
	if CompChangeState(orm.AsString(rec["state"])) == ChgApplied {
		return "", fmt.Errorf("an applied change cannot be refused")
	}
	return "", orm.UpdateRecordByID(bypass, compChangeModel, id, map[string]interface{}{
		"state":       string(ChgRefused),
		"approver_id": uid,
	})
}

// withinBandValue recomputes the band check directly (independent of the stored
// compute timing) so an approval is never gated on a stale derived column.
func withinBandValue(ctx context.Context, rec map[string]interface{}) bool {
	gradeID := intOr(rec["new_grade_id"])
	if gradeID <= 0 {
		return true
	}
	g, err := orm.SearchOne(ctx, gradeModel, map[string]interface{}{"id": gradeID})
	if err != nil || g == nil {
		return true
	}
	return withinBand(floatOr(rec["new_salary"]), floatOr(g["salary_min"]), floatOr(g["salary_max"]))
}

// ---------------------------------------------------------------------------
// Review cycle
// ---------------------------------------------------------------------------

func cycleOpen(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can open a review cycle")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, reviewCycleModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("cycle not found")
	}
	if ReviewCycleState(orm.AsString(rec["state"])) != CycPlanning {
		return "", fmt.Errorf("only a planning cycle can be opened")
	}
	return "", orm.UpdateRecordByID(bypass, reviewCycleModel, id, map[string]interface{}{"state": string(CycActive)})
}

func cycleLock(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can lock a review cycle")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, reviewCycleModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("cycle not found")
	}
	st := ReviewCycleState(orm.AsString(rec["state"]))
	if st != CycActive && st != CycReview {
		return "", fmt.Errorf("only an active or in-review cycle can be locked")
	}
	return "", orm.UpdateRecordByID(bypass, reviewCycleModel, id, map[string]interface{}{"state": string(CycLocked)})
}

func cycleClose(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can close a review cycle")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, reviewCycleModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("cycle not found")
	}
	if ReviewCycleState(orm.AsString(rec["state"])) != CycLocked {
		return "", fmt.Errorf("only a locked cycle can be closed")
	}
	return "", orm.UpdateRecordByID(bypass, reviewCycleModel, id, map[string]interface{}{"state": string(CycClosed)})
}

// ---------------------------------------------------------------------------
// Review line
// ---------------------------------------------------------------------------

func lineRecommend(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, reviewLineModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("review line not found")
	}
	if ReviewLineState(orm.AsString(rec["state"])) != RevDraft {
		return "", fmt.Errorf("only a draft review line can be recommended")
	}
	if floatOr(rec["proposed_salary"]) <= 0 {
		return "", fmt.Errorf("a proposed salary is required before recommending")
	}
	if !cycleEditable(bypass, intOr(rec["cycle_id"])) {
		return "", fmt.Errorf("the cycle is not open for recommendations")
	}
	return "", orm.UpdateRecordByID(bypass, reviewLineModel, id, map[string]interface{}{"state": string(RevRecommended)})
}

func lineApprove(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can approve a review line")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, reviewLineModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("review line not found")
	}
	if ReviewLineState(orm.AsString(rec["state"])) != RevRecommended {
		return "", fmt.Errorf("only a recommended review line can be approved")
	}
	if creator := intOr(rec["create_uid"]); creator != 0 && creator == uid {
		return "", fmt.Errorf("maker-checker: the reviewer cannot approve their own recommendation")
	}
	if !budgetOK(bypass, rec) {
		return "", fmt.Errorf("the proposed increase exceeds the cycle budget cap")
	}
	return "", orm.UpdateRecordByID(bypass, reviewLineModel, id, map[string]interface{}{"state": string(RevApproved)})
}

func lineReject(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can reject a review line")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, reviewLineModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("review line not found")
	}
	if ReviewLineState(orm.AsString(rec["state"])) == RevApproved {
		return "", fmt.Errorf("an approved review line cannot be rejected")
	}
	return "", orm.UpdateRecordByID(bypass, reviewLineModel, id, map[string]interface{}{"state": string(RevRejected)})
}

func cycleEditable(ctx context.Context, cycleID int) bool {
	if cycleID <= 0 {
		return false
	}
	c, err := orm.SearchOne(ctx, reviewCycleModel, map[string]interface{}{"id": cycleID})
	if err != nil || c == nil {
		return false
	}
	st := ReviewCycleState(orm.AsString(c["state"]))
	return st == CycActive || st == CycReview
}

// budgetOK recomputes the increase percentage against the cycle cap directly, so
// an approval is never gated on a stale derived column.
func budgetOK(ctx context.Context, rec map[string]interface{}) bool {
	cycleID := intOr(rec["cycle_id"])
	if cycleID <= 0 {
		return true
	}
	c, err := orm.SearchOne(ctx, reviewCycleModel, map[string]interface{}{"id": cycleID})
	if err != nil || c == nil {
		return true
	}
	cap := floatOr(c["budget_increase_pct"])
	if cap <= 0 {
		return true
	}
	pct := increasePct(floatOr(rec["current_salary"]), floatOr(rec["proposed_salary"]))
	return pct <= cap
}
