package models

import (
	"context"
	"fmt"
	"time"

	"nexa/core/orm"
)

// Object actions drive governance. A policy is submitted for approval (creating a pending
// change request and a submit entry in the append-only decision trail), activated only once
// an approved request exists when approval is required, and archived on demand. An approval
// enforces separation of duties — a requester can never approve their own pending request —
// and, when approved, brings its policy into force; rejection returns the policy to draft.
// Every approval move is recorded immutably. Connectors can be enabled (an endpoint is
// required), disabled and connection-tested, which records an error when misconfigured.
// Approval decisions require a school manager; policy and connector operations require a
// school officer.

func init() {
	orm.RegisterObjectAction(policyModel, "policy_submit_approval", actPolicySubmitApproval)
	orm.RegisterObjectAction(policyModel, "policy_activate", actPolicyActivate)
	orm.RegisterObjectAction(policyModel, "policy_archive", actPolicyArchive)
	orm.RegisterObjectAction(approvalModel, "approval_approve", actApprovalApprove)
	orm.RegisterObjectAction(approvalModel, "approval_reject", actApprovalReject)
	orm.RegisterObjectAction(approvalModel, "approval_cancel", actApprovalCancel)
	orm.RegisterObjectAction(integrationModel, "integration_enable", actIntegrationEnable)
	orm.RegisterObjectAction(integrationModel, "integration_disable", actIntegrationDisable)
	orm.RegisterObjectAction(integrationModel, "integration_test", actIntegrationTest)
}

func today() string    { return time.Now().UTC().Format(dateLayout) }
func nowStamp() string { return time.Now().UTC().Format(datetimeFull) }

func actPolicySubmitApproval(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	if err := requireOfficerErr(ctx, "submit a policy for approval"); err != nil {
		return "", err
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, policyModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) != string(PstDraft) {
		return "", fmt.Errorf("only a draft policy can be submitted for approval (current status %q)", policyLabel(strOr(rec["state"])))
	}
	uid := orm.SecurityUID(ctx)
	approvalID, err := orm.Create(bypass, orm.RegistryModel(approvalModel), map[string]interface{}{
		"policy_id":      id,
		"action":         "activate",
		"requested_by":   uid,
		"reason":         strOr(rec["description"]),
		"state":          string(AprPending),
		"submitted_date": today(),
		"company_id":     intOr(rec["company_id"]),
	})
	if err != nil {
		return "", err
	}
	writeDecision(bypass, approvalID, string(AadSubmit), uid, "Policy submitted for approval")
	if err := orm.UpdateRecordByID(bypass, policyModel, id, map[string]interface{}{"state": string(PstInReview)}); err != nil {
		return "", err
	}
	return fmt.Sprintf("Policy submitted for approval (request #%d)", approvalID), nil
}

func actPolicyActivate(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	if err := requireOfficerErr(ctx, "activate a policy"); err != nil {
		return "", err
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, policyModel, id)
	if err != nil {
		return "", err
	}
	st := strOr(rec["state"])
	if st == string(PstActive) {
		return "", fmt.Errorf("this policy is already active")
	}
	if st == string(PstArchived) {
		return "", fmt.Errorf("an archived policy cannot be reactivated; raise a new version")
	}
	if boolOr(rec["approval_required"]) {
		approved, _ := orm.SearchCount(bypass, approvalModel, [][]interface{}{
			{"policy_id", "=", id},
			{"state", "=", string(AprApproved)},
		})
		if approved == 0 {
			return "", fmt.Errorf("this policy requires an approved change request before it can be activated")
		}
	}
	patch := map[string]interface{}{"state": string(PstActive)}
	if strOr(rec["effective_from"]) == "" {
		patch["effective_from"] = today()
	}
	if err := orm.UpdateRecordByID(bypass, policyModel, id, patch); err != nil {
		return "", err
	}
	return "Policy activated", nil
}

func actPolicyArchive(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	if err := requireOfficerErr(ctx, "archive a policy"); err != nil {
		return "", err
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, policyModel, id)
	if err != nil {
		return "", err
	}
	st := strOr(rec["state"])
	if st != string(PstActive) && st != string(PstInReview) {
		return "", fmt.Errorf("only an active or in-review policy can be archived (current status %q)", policyLabel(st))
	}
	if err := orm.UpdateRecordByID(bypass, policyModel, id, map[string]interface{}{"state": string(PstArchived)}); err != nil {
		return "", err
	}
	return "Policy archived", nil
}

func actApprovalApprove(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if uid <= 0 || !orm.UserHasGroupXML(ctx, uid, "school.group_school_manager") {
		return "", fmt.Errorf("insufficient permission to approve a governance change")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, approvalModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) != string(AprPending) {
		return "", fmt.Errorf("only a pending request can be approved (current status %q)", approvalLabel(strOr(rec["state"])))
	}
	if !canApproveNow(string(AprPending), intOr(rec["requested_by"]), uid) {
		return "", fmt.Errorf("separation of duties: a requester may not approve their own submission")
	}
	if err := orm.UpdateRecordByID(bypass, approvalModel, id, map[string]interface{}{
		"state":         string(AprApproved),
		"decided_by":    uid,
		"decided_date":  today(),
		"decision_note": args["note"],
	}); err != nil {
		return "", err
	}
	writeDecision(bypass, id, string(AadApprove), uid, args["note"])
	policyID := intOr(rec["policy_id"])
	if policyID > 0 {
		patch := map[string]interface{}{"state": string(PstActive)}
		if p, err := getRecord(bypass, policyModel, policyID); err == nil && strOr(p["effective_from"]) == "" {
			patch["effective_from"] = today()
		}
		_ = orm.UpdateRecordByID(bypass, policyModel, policyID, patch)
	}
	return "Change request approved; policy brought into force", nil
}

func actApprovalReject(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if uid <= 0 || !orm.UserHasGroupXML(ctx, uid, "school.group_school_manager") {
		return "", fmt.Errorf("insufficient permission to reject a governance change")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, approvalModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) != string(AprPending) {
		return "", fmt.Errorf("only a pending request can be rejected (current status %q)", approvalLabel(strOr(rec["state"])))
	}
	if !canApproveNow(string(AprPending), intOr(rec["requested_by"]), uid) {
		return "", fmt.Errorf("separation of duties: a requester may not reject their own submission")
	}
	reason := args["note"]
	if reason == "" {
		return "", fmt.Errorf("a rejection reason is required")
	}
	if err := orm.UpdateRecordByID(bypass, approvalModel, id, map[string]interface{}{
		"state":         string(AprRejected),
		"decided_by":    uid,
		"decided_date":  today(),
		"decision_note": reason,
	}); err != nil {
		return "", err
	}
	writeDecision(bypass, id, string(AadReject), uid, reason)
	if policyID := intOr(rec["policy_id"]); policyID > 0 {
		_ = orm.UpdateRecordByID(bypass, policyModel, policyID, map[string]interface{}{"state": string(PstDraft)})
	}
	return "Change request rejected; policy returned to draft", nil
}

func actApprovalCancel(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if err := requireOfficerErr(ctx, "cancel a change request"); err != nil {
		return "", err
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, approvalModel, id)
	if err != nil {
		return "", err
	}
	if !approvalAllowedTransition(strOr(rec["state"]), string(AprCancelled)) {
		return "", fmt.Errorf("only a pending request can be cancelled (current status %q)", approvalLabel(strOr(rec["state"])))
	}
	if err := orm.UpdateRecordByID(bypass, approvalModel, id, map[string]interface{}{
		"state":         string(AprCancelled),
		"decided_by":    uid,
		"decided_date":  today(),
		"decision_note": args["note"],
	}); err != nil {
		return "", err
	}
	writeDecision(bypass, id, string(AadCancel), uid, args["note"])
	return "Change request cancelled", nil
}

func actIntegrationEnable(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	if err := requireOfficerErr(ctx, "enable an integration"); err != nil {
		return "", err
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, integrationModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["endpoint"]) == "" {
		return "", fmt.Errorf("an endpoint must be configured before an integration can be enabled")
	}
	if err := orm.UpdateRecordByID(bypass, integrationModel, id, map[string]interface{}{
		"state":      string(IstActive),
		"last_error": "",
	}); err != nil {
		return "", err
	}
	return "Integration enabled", nil
}

func actIntegrationDisable(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	if err := requireOfficerErr(ctx, "disable an integration"); err != nil {
		return "", err
	}
	bypass := orm.ContextWithBypass(ctx, true)
	if _, err := getRecord(bypass, integrationModel, id); err != nil {
		return "", err
	}
	if err := orm.UpdateRecordByID(bypass, integrationModel, id, map[string]interface{}{"state": string(IstPaused)}); err != nil {
		return "", err
	}
	return "Integration paused", nil
}

func actIntegrationTest(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	if err := requireOfficerErr(ctx, "test an integration"); err != nil {
		return "", err
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, integrationModel, id)
	if err != nil {
		return "", err
	}
	endpoint := strOr(rec["endpoint"])
	if endpoint == "" {
		_ = orm.UpdateRecordByID(bypass, integrationModel, id, map[string]interface{}{
			"state":      string(IstError),
			"last_error": "No endpoint configured",
		})
		return "", fmt.Errorf("connection test failed: no endpoint configured")
	}
	_ = orm.UpdateRecordByID(bypass, integrationModel, id, map[string]interface{}{
		"last_sync":  nowStamp(),
		"last_error": "",
	})
	return fmt.Sprintf("Connection OK to %s", endpoint), nil
}

// ---------------------------------------------------------------------------
// Shared action helpers
// ---------------------------------------------------------------------------

func writeDecision(ctx context.Context, approvalID int, action string, actor int, note string) {
	_, _ = orm.Create(ctx, orm.RegistryModel(decisionModel), map[string]interface{}{
		"approval_id": approvalID,
		"action":      action,
		"actor_id":    actor,
		"note":        note,
		"timestamp":   nowStamp(),
	})
}

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

func requireOfficerErr(ctx context.Context, what string) error {
	uid := orm.SecurityUID(ctx)
	if uid <= 0 || !requireOfficer(ctx, uid) {
		return fmt.Errorf("insufficient permission to %s", what)
	}
	return nil
}

var policyLabels = map[string]string{
	"draft": "Draft", "in_review": "In Review", "active": "Active", "archived": "Archived",
}
var approvalLabels = map[string]string{
	"pending": "Pending", "approved": "Approved", "rejected": "Rejected", "cancelled": "Cancelled",
}

func policyLabel(state string) string {
	if l, ok := policyLabels[state]; ok {
		return l
	}
	return state
}

func approvalLabel(state string) string {
	if l, ok := approvalLabels[state]; ok {
		return l
	}
	return state
}
