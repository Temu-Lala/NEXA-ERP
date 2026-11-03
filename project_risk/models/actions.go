package models

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"nexa/core/orm"
)

// Risk, issue and change-control actions (blueprint §11, §12, §20). Risks derive
// severity from probability×impact; issues move through resolve/close with a
// recorded outcome; change requests follow submit → approve/reject with separation
// of duties (the decider cannot be the requester) and, when approved, apply their
// effect to the project through a controlled write rather than a silent rewrite.

const (
	riskModel  = "project.risk"
	issueModel = "project.issue"
	changeMo   = "project.change.request"
	projModel  = "project.project"
	dateLayout = "2006-01-02"
)

func init() {
	orm.RegisterObjectAction(riskModel, "action_assess", riskAssess)
	orm.RegisterObjectAction(riskModel, "action_mitigate", riskMitigate)
	orm.RegisterObjectAction(riskModel, "action_close", riskClose)

	orm.RegisterObjectAction(issueModel, "action_start", issueStart)
	orm.RegisterObjectAction(issueModel, "action_resolve", issueResolve)
	orm.RegisterObjectAction(issueModel, "action_close", issueClose)

	orm.RegisterObjectAction(changeMo, "action_submit", chgSubmit)
	orm.RegisterObjectAction(changeMo, "action_approve", chgApprove)
	orm.RegisterObjectAction(changeMo, "action_reject", chgReject)
}

// ---------------------------------------------------------------------------
// Risks
// ---------------------------------------------------------------------------

// riskAssess recomputes severity from the matrix and moves the risk to mitigating,
// recording who assessed it.
func riskAssess(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, riskModel, id)
	if rec == nil {
		return "", fmt.Errorf("risk not found")
	}
	sev := computeSeverity(s(rec["probability"]), s(rec["impact"]))
	upd := map[string]interface{}{"severity": string(sev)}
	if s(rec["status"]) == "open" {
		upd["status"] = "mitigating"
	}
	if uid := orm.UIDFromContext(ctx); uid > 0 {
		upd["owner_id"] = orElseInt(rec["owner_id"], uid)
	}
	if err := orm.UpdateRecordByID(bypass, riskModel, id, upd); err != nil {
		return "", err
	}
	return fmt.Sprintf("Risk severity assessed as %s.", sev), nil
}

func riskMitigate(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	return riskSet(ctx, id, []string{"open"}, "mitigating", "Risk mitigation in progress.")
}

// riskClose requires a mitigation to have been recorded, so closures are not empty.
func riskClose(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, riskModel, id)
	if rec == nil {
		return "", fmt.Errorf("risk not found")
	}
	if strings.TrimSpace(s(rec["mitigation"])) == "" {
		return "", fmt.Errorf("record a mitigation before closing the risk")
	}
	if err := orm.UpdateRecordByID(bypass, riskModel, id, map[string]interface{}{"status": "closed"}); err != nil {
		return "", err
	}
	return "Risk closed.", nil
}

// ---------------------------------------------------------------------------
// Issues
// ---------------------------------------------------------------------------

func issueStart(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	return issueSet(ctx, id, []string{"new"}, "in_progress", "Issue under investigation.")
}

func issueResolve(ctx context.Context, _ string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, issueModel, id)
	if rec == nil {
		return "", fmt.Errorf("issue not found")
	}
	if st := s(rec["status"]); st != "new" && st != "in_progress" {
		return "", fmt.Errorf("only open issues can be resolved")
	}
	upd := map[string]interface{}{"status": "resolved", "resolution_date": time.Now().Format(dateLayout)}
	if r := strings.TrimSpace(vals["resolution"]); r != "" {
		upd["resolution"] = r
	} else if strings.TrimSpace(s(rec["resolution"])) == "" {
		return "", fmt.Errorf("a resolution note is required to resolve the issue")
	}
	if err := orm.UpdateRecordByID(bypass, issueModel, id, upd); err != nil {
		return "", err
	}
	return "Issue resolved.", nil
}

func issueClose(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	return issueSet(ctx, id, []string{"resolved"}, "closed", "Issue closed.")
}

// ---------------------------------------------------------------------------
// Change control (SoD enforced)
// ---------------------------------------------------------------------------

func chgSubmit(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, changeMo, id)
	if rec == nil {
		return "", fmt.Errorf("change request not found")
	}
	if s(rec["state"]) != "draft" {
		return "", fmt.Errorf("only draft change requests can be submitted")
	}
	upd := map[string]interface{}{"state": "submitted"}
	if uid := orm.UIDFromContext(ctx); uid > 0 && idOf(rec["requested_by"]) == 0 {
		upd["requested_by"] = uid
	}
	if err := orm.UpdateRecordByID(bypass, changeMo, id, upd); err != nil {
		return "", err
	}
	return "Change request submitted for decision.", nil
}

// chgApprove enforces that the decision-maker differs from the requester, records
// the decision, then applies the approved effect to the project (budget and/or
// target end date) through a controlled write.
func chgApprove(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, changeMo, id)
	if rec == nil {
		return "", fmt.Errorf("change request not found")
	}
	if s(rec["state"]) != "submitted" {
		return "", fmt.Errorf("only submitted change requests can be approved")
	}
	uid := orm.UIDFromContext(ctx)
	reqBy := idOf(rec["requested_by"])
	if uid > 0 && reqBy > 0 && uid == reqBy {
		return "", fmt.Errorf("a change request must be decided by someone other than its requester")
	}
	if err := orm.UpdateRecordByID(bypass, changeMo, id, map[string]interface{}{
		"state": "approved", "approved_by": orElseInt(0, uid), "decision_date": time.Now().Format(dateLayout),
	}); err != nil {
		return "", err
	}
	effected := applyChangeEffect(bypass, rec, uid)
	if effected != "" {
		return "Change request approved and applied: " + effected, nil
	}
	return "Change request approved.", nil
}

func chgReject(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, changeMo, id)
	if rec == nil {
		return "", fmt.Errorf("change request not found")
	}
	if s(rec["state"]) != "submitted" {
		return "", fmt.Errorf("only submitted change requests can be rejected")
	}
	uid := orm.UIDFromContext(ctx)
	if err := orm.UpdateRecordByID(bypass, changeMo, id, map[string]interface{}{
		"state": "rejected", "approved_by": orElseInt(0, uid), "decision_date": time.Now().Format(dateLayout),
	}); err != nil {
		return "", err
	}
	return "Change request rejected.", nil
}

// applyChangeEffect writes an approved change to the project: budget impact raises
// the approved budget; date impact extends the target end. It returns a short
// human-readable summary of what was applied.
func applyChangeEffect(ctx context.Context, rec map[string]interface{}, uid int) string {
	pid := idOf(rec["project_id"])
	if pid <= 0 {
		return ""
	}
	proj := load(ctx, projModel, pid)
	if proj == nil {
		return ""
	}
	upd := map[string]interface{}{}
	var parts []string

	if amt := flt(rec["budget_impact"]); amt != 0 {
		newBudget := flt(proj["budget"]) + amt
		upd["budget"] = newBudget
		parts = append(parts, fmt.Sprintf("budget adjusted to %.2f", newBudget))
	}
	if days := intFlt(rec["schedule_impact_days"]); days != 0 {
		if base := parseDate(firstNonEmpty(s(proj["target_end"]), s(proj["planned_end"]))); !base.IsZero() {
			upd["target_end"] = base.AddDate(0, 0, days).Format(dateLayout)
			parts = append(parts, fmt.Sprintf("target end shifted by %d day(s)", days))
		}
	}
	if len(upd) == 0 {
		return ""
	}
	if err := orm.UpdateRecordByID(ctx, projModel, pid, upd); err != nil {
		return "project update failed"
	}
	_ = uid
	return strings.Join(parts, "; ")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func riskSet(ctx context.Context, id int, from []string, to, msg string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, riskModel, id)
	if rec == nil {
		return "", fmt.Errorf("risk not found")
	}
	if !contains(from, s(rec["status"])) {
		return "", fmt.Errorf("transition to %s not allowed from %s", to, s(rec["status"]))
	}
	if err := orm.UpdateRecordByID(bypass, riskModel, id, map[string]interface{}{"status": to}); err != nil {
		return "", err
	}
	return msg, nil
}

func issueSet(ctx context.Context, id int, from []string, to, msg string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, issueModel, id)
	if rec == nil {
		return "", fmt.Errorf("issue not found")
	}
	if !contains(from, s(rec["status"])) {
		return "", fmt.Errorf("transition to %s not allowed from %s", to, s(rec["status"]))
	}
	if err := orm.UpdateRecordByID(bypass, issueModel, id, map[string]interface{}{"status": to}); err != nil {
		return "", err
	}
	return msg, nil
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func orElseInt(v interface{}, def int) int {
	if n := idOf(v); n > 0 {
		return n
	}
	return def
}

func flt(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int64:
		return float64(t)
	case int:
		return float64(t)
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f
	default:
		n, _ := orm.CoerceInt64(v)
		return float64(n)
	}
}

func intFlt(v interface{}) int { return int(flt(v)) }

// parseDate reads a date (or datetime) string into a time.Time, returning the
// zero time when the value is blank or unparseable so callers can skip it.
func parseDate(v interface{}) time.Time {
	str := strings.TrimSpace(s(v))
	if str == "" {
		return time.Time{}
	}
	for _, layout := range []string{dateLayout, "2006-01-02 15:04:05", time.RFC3339} {
		if t, err := time.Parse(layout, str); err == nil {
			return t
		}
	}
	return time.Time{}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
