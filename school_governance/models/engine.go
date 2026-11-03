package models

import (
	"context"
	"time"

	"nexa/core/orm"
	"nexa/core/scheduler"
)

// Behaviour layer for school governance. The control-plane rules are pure and unit-tested:
// a policy is effective now only when it is active and today falls inside its (optional)
// effective window; the approval workflow follows a strict state machine; separation of
// duties forbids a requester approving their own pending submission; a connector is due for
// sync only when active and its last sync is older than the configured interval; and a
// policy whose end date has passed is a candidate for auto-archival. The computes project
// a policy's effective flag, parameter count and pending-approval signal, an approval's
// can-approve flag for the current user, and a connector's days-since-sync. A daily cron
// archives expired active policies.

const (
	policyModel      = "school.governance.policy"
	parameterModel   = "school.governance.policy.parameter"
	approvalModel    = "school.governance.approval"
	decisionModel    = "school.governance.approval.decision"
	accessRoleModel  = "school.governance.access.role"
	integrationModel = "school.governance.integration"

	expiryCronCode = "school.governance.expiry"

	dateLayout   = "2006-01-02"
	datetimeFull = "2006-01-02 15:04:05"

	defaultSyncIntervalMinutes = 60
)

// policyEffective reports whether an active policy is currently within its window. Dates
// are ISO strings and compare lexicographically.
func policyEffective(state, from, to, today string) bool {
	if state != string(PstActive) {
		return false
	}
	if from != "" && from > today {
		return false
	}
	if to != "" && to < today {
		return false
	}
	return true
}

// shouldAutoArchive reports whether a policy's end date has strictly passed.
func shouldAutoArchive(to, today string) bool {
	return to != "" && to < today
}

// approvalAllowedTransition reports whether an approval may move between states.
func approvalAllowedTransition(from, to string) bool {
	switch from {
	case string(AprPending):
		return to == string(AprApproved) || to == string(AprRejected) || to == string(AprCancelled)
	default: // approved/rejected/cancelled are terminal
		return false
	}
}

// canApproveNow enforces separation of duties: only a signed-in user other than the
// requester may act on a pending approval.
func canApproveNow(state string, requestedBy, actor int) bool {
	if state != string(AprPending) {
		return false
	}
	if actor <= 0 {
		return false
	}
	return actor != requestedBy
}

// integrationEligibleForSync reports whether a connector is due for another sync.
func integrationEligibleForSync(state string, active bool, lastSync string, intervalMinutes int, now time.Time) bool {
	if !active || state != string(IstActive) {
		return false
	}
	if intervalMinutes <= 0 {
		intervalMinutes = defaultSyncIntervalMinutes
	}
	if lastSync == "" {
		return true
	}
	t, err := time.Parse(datetimeFull, lastSync)
	if err != nil {
		return true
	}
	return now.Sub(t) >= time.Duration(intervalMinutes)*time.Minute
}

// bumpVersion increments a policy version, tolerating a non-positive stored value.
func bumpVersion(v int) int {
	if v < 1 {
		return 1
	}
	return v + 1
}

// ---------------------------------------------------------------------------
// Computes
// ---------------------------------------------------------------------------

func init() {
	orm.RegisterCompute(policyModel, "is_effective", []string{"state", "effective_from", "effective_to"}, computePolicyEffective)
	orm.RegisterCompute(policyModel, "parameter_count", []string{"id"}, computePolicyParameterCount)
	orm.RegisterCompute(policyModel, "pending_approval", []string{"id"}, computePolicyPendingApproval)
	orm.RegisterCompute(approvalModel, "can_approve", []string{"state", "requested_by"}, computeApprovalCanApprove)
	orm.RegisterCompute(integrationModel, "days_since_sync", []string{"last_sync"}, computeIntegrationDaysSinceSync)
	scheduler.RegisterCronHandler(expiryCronCode, runGovernanceExpiry)
}

func computePolicyEffective(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return policyEffective(strOr(rec["state"]), strOr(rec["effective_from"]), strOr(rec["effective_to"]), time.Now().UTC().Format(dateLayout)), nil
}

func computePolicyParameterCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), parameterModel, [][]interface{}{{"policy_id", "=", intOr(rec["id"])}})
	if err != nil {
		return 0, nil
	}
	return n, nil
}

func computePolicyPendingApproval(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), approvalModel, [][]interface{}{
		{"policy_id", "=", intOr(rec["id"])},
		{"state", "=", string(AprPending)},
	})
	if err != nil {
		return false, nil
	}
	return n > 0, nil
}

func computeApprovalCanApprove(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return canApproveNow(strOr(rec["state"]), intOr(rec["requested_by"]), orm.SecurityUID(ctx)), nil
}

func computeIntegrationDaysSinceSync(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	ls := strOr(rec["last_sync"])
	if ls == "" {
		return 0, nil
	}
	t, err := time.Parse(datetimeFull, ls)
	if err != nil {
		return 0, nil
	}
	d := int(time.Now().UTC().Sub(t).Hours() / 24)
	if d < 0 {
		d = 0
	}
	return d, nil
}

// ---------------------------------------------------------------------------
// Cron: auto-archive policies whose effective window has closed
// ---------------------------------------------------------------------------

func runGovernanceExpiry(ctx context.Context, _ map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	today := time.Now().UTC().Format(dateLayout)
	rows, err := orm.Search(bypass, policyModel, [][]interface{}{{"state", "=", string(PstActive)}})
	if err != nil {
		return err
	}
	for _, r := range rows {
		if shouldAutoArchive(strOr(r["effective_to"]), today) {
			_ = orm.UpdateRecordByID(bypass, policyModel, intOr(r["id"]), map[string]interface{}{"state": string(PstArchived)})
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Shared coercion helpers (ORM values arrive as interface{})
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

func floatOr(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	}
	return 0
}
