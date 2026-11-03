package models

import (
	"context"
	"fmt"
	"time"

	"nexa/core/orm"
)

// HR-F12 actions. Governance is configuration, so the only writes here flip an
// HR-Manager authoring flag (activate a policy, enable an integration, stamp a
// manual sync) or describe a policy's chain for preview. Enabling an integration is
// bookkeeping only — it never fires an outbound call — and every action is gated to
// the Manager group, matching that these are tenant-wide controls.

const (
	settingModel     = "hr.setting"
	integrationModel = "hr.integration.setting"

	datetimeFull = "2006-01-02 15:04:05"
)

func init() {
	orm.RegisterObjectAction(policyModel, "action_policy_toggle", policyToggle)
	orm.RegisterObjectAction(policyModel, "action_policy_preview", policyPreview)
	orm.RegisterObjectAction(integrationModel, "action_integration_toggle", integrationToggle)
	orm.RegisterObjectAction(integrationModel, "action_integration_synced", integrationMarkSynced)
}

// policyToggle activates / deactivates an approval policy (Manager only).
func policyToggle(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireManager(ctx, uid) {
		return "", fmt.Errorf("only an HR Manager can change an approval policy")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, policyModel, id)
	if err != nil {
		return "", err
	}
	next := !orm.AsBool(rec["active"])
	if err := orm.UpdateRecordByID(bypass, policyModel, id, map[string]interface{}{"active": next}); err != nil {
		return "", err
	}
	if next {
		return "Policy activated", nil
	}
	return "Policy deactivated", nil
}

// policyPreview reports, for a policy, whether it enforces approval and how many
// mandatory steps its chain defines. Read-only; it changes nothing.
func policyPreview(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("only an HR Officer or Manager can preview a policy")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, policyModel, id)
	if err != nil {
		return "", err
	}
	threshold := floatOr(rec["amount_threshold"])
	mandatory := mandatoryLevelThresholds(bypass, id)
	levels := len(mandatory)
	if !approvalRequired(threshold, threshold, levels) {
		return "No approval enforced: the chain has no mandatory levels.", nil
	}
	return fmt.Sprintf("Approval required above %.2f (0 = every request), across %d mandatory level(s).", threshold, levels), nil
}

// integrationToggle enables / disables an integration (Manager only). It only
// records intent; no connection is attempted from here.
func integrationToggle(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireManager(ctx, uid) {
		return "", fmt.Errorf("only an HR Manager can change an integration")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, integrationModel, id)
	if err != nil {
		return "", err
	}
	next := !orm.AsBool(rec["enabled"])
	if err := orm.UpdateRecordByID(bypass, integrationModel, id, map[string]interface{}{"enabled": next}); err != nil {
		return "", err
	}
	if next {
		return "Integration enabled", nil
	}
	return "Integration disabled", nil
}

// integrationMarkSynced stamps the manual "last successful sync" time (bookkeeping).
func integrationMarkSynced(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireManager(ctx, uid) {
		return "", fmt.Errorf("only an HR Manager can update an integration's sync stamp")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	if _, err := getRecord(bypass, integrationModel, id); err != nil {
		return "", err
	}
	if err := orm.UpdateRecordByID(bypass, integrationModel, id, map[string]interface{}{
		"last_sync": time.Now().UTC().Format(datetimeFull),
	}); err != nil {
		return "", err
	}
	return "Sync stamp updated", nil
}

// ---------------------------------------------------------------------------
// Permission / record helpers (self-contained to this module)
// ---------------------------------------------------------------------------

func requireOfficer(ctx context.Context, uid int) bool {
	return orm.UserHasGroupXML(ctx, uid, "hr.group_hr_user") ||
		orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager")
}

func requireManager(ctx context.Context, uid int) bool {
	return orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager")
}

func getRecord(ctx context.Context, model string, id int) (map[string]interface{}, error) {
	rec, err := orm.SearchOne(ctx, model, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return nil, fmt.Errorf("record not found")
	}
	return rec, nil
}
