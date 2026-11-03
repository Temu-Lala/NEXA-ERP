package models

import (
	"context"
	"fmt"
	"time"

	"nexa/core/orm"
)

// Object actions for staff. A manager verifies a qualification (recording the verifier).
// A substitution moves requested -> assigned -> completed and may be cancelled from the
// first two states; assignment is refused if the substitute is the same person who is
// away. A performance review moves draft -> submitted -> approved, with approval (and the
// review date/approver) reserved for a manager.

func init() {
	orm.RegisterObjectAction(qualModel, "qualification_verify", actQualVerify)

	orm.RegisterObjectAction(substModel, "substitution_assign", actSubstAssign)
	orm.RegisterObjectAction(substModel, "substitution_complete", actSubstComplete)
	orm.RegisterObjectAction(substModel, "substitution_cancel", actSubstCancel)

	orm.RegisterObjectAction(perfModel, "performance_submit", actPerfSubmit)
	orm.RegisterObjectAction(perfModel, "performance_approve", actPerfApprove)
}

func actQualVerify(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireManager(ctx, uid) {
		return "", fmt.Errorf("only a School Manager/Director can verify a qualification")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, qualModel, id)
	if err != nil {
		return "", err
	}
	if boolOr(rec["verified"]) {
		return "Qualification is already verified", nil
	}
	if err := orm.UpdateRecordByID(bypass, qualModel, id, map[string]interface{}{
		"verified":       true,
		"verified_by_id": uid,
	}); err != nil {
		return "", err
	}
	return "Qualification verified", nil
}

func actSubstAssign(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to assign a substitution")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, substModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) != string(SubRequested) {
		return "", fmt.Errorf("only a requested substitution can be assigned (current status %q)", label(strOr(rec["state"])))
	}
	if intOr(rec["absent_staff_id"]) == intOr(rec["substitute_staff_id"]) {
		return "", fmt.Errorf("the substitute cannot be the same member who is away")
	}
	if err := orm.UpdateRecordByID(bypass, substModel, id, map[string]interface{}{"state": string(SubAssigned)}); err != nil {
		return "", err
	}
	return "Substitution assigned", nil
}

func actSubstComplete(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to complete a substitution")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, substModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) != string(SubAssigned) {
		return "", fmt.Errorf("only an assigned substitution can be completed (current status %q)", label(strOr(rec["state"])))
	}
	if err := orm.UpdateRecordByID(bypass, substModel, id, map[string]interface{}{"state": string(SubCompleted)}); err != nil {
		return "", err
	}
	return "Substitution marked completed", nil
}

func actSubstCancel(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to cancel a substitution")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, substModel, id)
	if err != nil {
		return "", err
	}
	switch strOr(rec["state"]) {
	case string(SubRequested), string(SubAssigned):
		// cancellable
	default:
		return "", fmt.Errorf("only a requested or assigned substitution can be cancelled (current status %q)", label(strOr(rec["state"])))
	}
	if err := orm.UpdateRecordByID(bypass, substModel, id, map[string]interface{}{"state": string(SubCancelled)}); err != nil {
		return "", err
	}
	return "Substitution cancelled", nil
}

func actPerfSubmit(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to submit a performance review")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, perfModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) != string(PerfDraft) {
		return "", fmt.Errorf("only a draft performance review can be submitted (current status %q)", label(strOr(rec["state"])))
	}
	if strOr(rec["period"]) == "" {
		return "", fmt.Errorf("a review period is required before submitting")
	}
	if err := orm.UpdateRecordByID(bypass, perfModel, id, map[string]interface{}{"state": string(PerfSubmitted)}); err != nil {
		return "", err
	}
	return "Performance review submitted for approval", nil
}

func actPerfApprove(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireManager(ctx, uid) {
		return "", fmt.Errorf("only a School Manager/Director can approve a performance review")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, perfModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) != string(PerfSubmitted) {
		return "", fmt.Errorf("only a submitted performance review can be approved (current status %q)", label(strOr(rec["state"])))
	}
	if err := orm.UpdateRecordByID(bypass, perfModel, id, map[string]interface{}{
		"state":          string(PerfApproved),
		"reviewed_by_id": uid,
		"review_date":    time.Now().UTC().Format(dateLayout),
	}); err != nil {
		return "", err
	}
	return "Performance review approved", nil
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

func getRecord(ctx context.Context, model string, id int) (map[string]interface{}, error) {
	rec, err := orm.SearchOne(ctx, model, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return nil, fmt.Errorf("record not found")
	}
	return rec, nil
}

func requireOfficer(ctx context.Context, uid int) bool {
	return orm.UserHasGroupXML(ctx, uid, "school.group_school_clerk") ||
		orm.UserHasGroupXML(ctx, uid, "school.group_school_manager")
}

func requireManager(ctx context.Context, uid int) bool {
	return orm.UserHasGroupXML(ctx, uid, "school.group_school_manager")
}

var stateLabels = map[string]string{
	"requested": "Requested", "assigned": "Assigned", "completed": "Completed", "cancelled": "Cancelled",
	"draft": "Draft", "submitted": "Submitted", "approved": "Approved",
}

func label(state string) string {
	if l, ok := stateLabels[state]; ok {
		return l
	}
	return state
}
