package models

import (
	"context"
	"strconv"
	"strings"

	"nexa/core/orm"
)

// HR-F12 behaviour layer. The pure helpers carry the governance arithmetic —
// whether a request needs approval given a policy threshold and how many levels
// bite above a given amount, and how a scoped parameter resolves (department over
// company over global) — so it is unit-testable with no database. The only
// registered compute projects an approval policy's live level count from its lines.
// Nothing here enforces or mutates business records; these definitions are read by
// the operational modules and by the core workflow engine.

const (
	policyModel    = "hr.approval.policy"
	levelModel     = "hr.approval.level"
	parameterModel = "hr.parameter"
)

func init() {
	orm.RegisterCompute(policyModel, "level_count", []string{"id"}, computePolicyLevels)
}

// ---------------------------------------------------------------------------
// Pure logic (unit-tested)
// ---------------------------------------------------------------------------

// approvalRequired reports whether a request of `amount` needs approval under a
// policy with the given threshold and number of defined levels. A positive
// threshold that the amount does not exceed exempts the request; a zero (or
// negative) threshold means every request is approved. Either way a chain with no
// levels cannot require approval, so it returns false.
func approvalRequired(amount, threshold float64, levels int) bool {
	if levels <= 0 {
		return false
	}
	if threshold > 0 && amount <= threshold {
		return false
	}
	return true
}

// countRequiredLevels returns how many approval steps apply to a request of the
// given amount. A step applies when it has no minimum (threshold <= 0) or the
// amount reaches its minimum. The slice is the policy's per-level thresholds in
// order; mandatory filtering happens before the call.
func countRequiredLevels(amount float64, levelThresholds []float64) int {
	n := 0
	for _, t := range levelThresholds {
		if t <= 0 || amount >= t {
			n++
		}
	}
	return n
}

// pickScopedValue resolves a parameter by precedence: department wins over company,
// which wins over global. Empty strings are treated as "not set". It returns the
// chosen value and the scope it came from.
func pickScopedValue(departmentValue, companyValue, globalValue string) (string, ParamScope) {
	if strings.TrimSpace(departmentValue) != "" {
		return departmentValue, ScopeDepartment
	}
	if strings.TrimSpace(companyValue) != "" {
		return companyValue, ScopeCompany
	}
	return globalValue, ScopeGlobal
}

// paramAsBool interprets a stored parameter value as a boolean ("true"/"1"/"yes").
func paramAsBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "1", "yes", "y", "on":
		return true
	default:
		return false
	}
}

// paramAsInt parses an integer parameter value, returning def on failure.
func paramAsInt(v string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return n
}

// paramAsFloat parses a float parameter value, returning def on failure.
func paramAsFloat(v string, def float64) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil {
		return def
	}
	return f
}

// ---------------------------------------------------------------------------
// Field / value helpers
// ---------------------------------------------------------------------------

func intOr(v interface{}) int {
	n, ok := orm.CoerceInt64(v)
	if !ok {
		return 0
	}
	return int(n)
}

func floatOr(v interface{}) float64 {
	f, ok := orm.CoerceFloat64(v)
	if !ok {
		return 0
	}
	return f
}

func strOr(v interface{}) string {
	return strings.TrimSpace(orm.AsString(v))
}

// ---------------------------------------------------------------------------
// Computes
// ---------------------------------------------------------------------------

// computePolicyLevels counts the approval steps currently attached to a policy.
func computePolicyLevels(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	id := intOr(rec["id"])
	if id <= 0 {
		return 0, nil
	}
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), levelModel, [][]interface{}{
		{"policy_id", "=", id},
	})
	if err != nil {
		return 0, nil
	}
	return n, nil
}

// ---------------------------------------------------------------------------
// Read-only lookups (consumed by the preview action)
// ---------------------------------------------------------------------------

// mandatoryLevelThresholds returns the ordered per-level minimum amounts for a
// policy, restricted to mandatory levels, for the preview action to reason over.
func mandatoryLevelThresholds(ctx context.Context, policyID int) []float64 {
	rows, err := orm.Search(orm.ContextWithBypass(ctx, true), levelModel, [][]interface{}{
		{"policy_id", "=", policyID},
	})
	if err != nil {
		return nil
	}
	out := make([]float64, 0, len(rows))
	for _, r := range rows {
		if !orm.AsBool(r["mandatory"]) {
			continue
		}
		out = append(out, floatOr(r["threshold_amount"]))
	}
	return out
}
