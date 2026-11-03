package models

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nexa/core/orm"
)

// Requisition lifecycle + budget control + approval engine (blueprint §4, §5,
// §6). Transitions are validated server-side and each writes an append-only
// status history. Separation of duties is enforced: the requester is never the
// approver, and material changes force re-approval by rebuilding the instance.
// Budget breaches only proceed under soft/conditional policy or an authorized,
// reason-carrying override.

const (
	historyModel    = "purchase.requisition.status.history"
	budgetTolerance = 5.0 // conditional-policy tolerance, % of approved budget
)

func init() {
	orm.RegisterObjectAction(reqModel, "action_submit", reqSubmit)
	orm.RegisterObjectAction(reqModel, "action_check_budget", reqCheckBudget)
	orm.RegisterObjectAction(reqModel, "action_request_approval", reqRequestApproval)
	orm.RegisterObjectAction(reqModel, "action_approve", reqApprove)
	orm.RegisterObjectAction(reqModel, "action_reject", reqReject)
	orm.RegisterObjectAction(reqModel, "action_resubmit", reqResubmit)
	orm.RegisterObjectAction(reqModel, "action_start_sourcing", reqToState([]RequisitionState{ReqStateApproved}, ReqStateSourcing, "Sourcing opened."))
	orm.RegisterObjectAction(reqModel, "action_mark_ordered", reqToState([]RequisitionState{ReqStateSourcing}, ReqStateOrdered, "Marked ordered."))
	orm.RegisterObjectAction(reqModel, "action_mark_partial", reqToState([]RequisitionState{ReqStateOrdered}, ReqStatePartial, "Partially received."))
	orm.RegisterObjectAction(reqModel, "action_mark_fulfilled", reqToState([]RequisitionState{ReqStateOrdered, ReqStatePartial}, ReqStateFulfilled, "Fulfilled."))
	orm.RegisterObjectAction(reqModel, "action_close", reqClose)
	orm.RegisterObjectAction(reqModel, "action_cancel", reqCancel)
	orm.RegisterObjectAction(reqModel, "action_override_budget", reqOverrideBudget)

	orm.RegisterObjectAction(instModel, "action_review_approve", instApprove)
	orm.RegisterObjectAction(instModel, "action_review_reject", instReject)
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

func reqSubmit(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, reqModel, id)
	if rec == nil {
		return "", fmt.Errorf("requisition not found")
	}
	if RequisitionState(str(rec["state"])) != ReqStateDraft {
		return "", fmt.Errorf("only draft requisitions can be submitted")
	}
	if len(rec) > 0 {
		if cnt, _ := orm.SearchCount(bypass, reqLineModel, [][]interface{}{{"requisition_id", "=", id}}); cnt == 0 {
			return "", fmt.Errorf("add at least one line before submitting")
		}
	}
	name := str(rec["name"])
	if name == "" || name == "New" {
		name = nextRef(bypass, "REQ")
	}
	if uid := orm.UIDFromContext(ctx); uid > 0 && intID(rec["requester_id"]) == 0 {
		_ = orm.UpdateRecordByID(bypass, reqModel, id, map[string]interface{}{"requester_id": uid})
		rec = loadRec(bypass, reqModel, id)
	}
	policy := BudgetPolicy(strOr(rec["budget_policy"], "conditional"))
	if err := transition(bypass, id, ReqStateDraft, ReqStateSubmitted, "Submitted", name); err != nil {
		return "", err
	}
	// Budget is checked as part of submission; the state then reflects the outcome.
	res := runBudgetCheck(bypass, rec, policy)
	return fmt.Sprintf("Submitted. Budget check: %s.", res), nil
}

// reqCheckBudget re-evaluates availability and stores a snapshot. A hard breach
// without an approved override keeps the requisition from advancing.
func reqCheckBudget(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, reqModel, id)
	if rec == nil {
		return "", fmt.Errorf("requisition not found")
	}
	policy := BudgetPolicy(strOr(rec["budget_policy"], "conditional"))
	res := runBudgetCheck(bypass, rec, policy)
	_ = orm.UpdateRecordByID(bypass, reqModel, id, map[string]interface{}{"state": string(ReqStateBudgetValidation)})
	return fmt.Sprintf("Budget validation: %s.", res), nil
}

// reqRequestApproval builds the procurement approval instance from the matrix
// and moves the requisition to the approval stage.
func reqRequestApproval(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, reqModel, id)
	if rec == nil {
		return "", fmt.Errorf("requisition not found")
	}
	st := RequisitionState(str(rec["state"]))
	if st != ReqStateSubmitted && st != ReqStateBudgetValidation {
		return "", fmt.Errorf("requisition must be submitted / budget-validated first")
	}
	if RequisitionState(str(rec["state"])) == ReqStateBudgetValidation &&
		str(rec["budget_result"]) == string(BudgetExceed) && !hasApprovedOverride(bypass, id) {
		return "", fmt.Errorf("budget exceeded: an authorized override is required before approval")
	}
	snapshot := requisitionSnapshot(bypass, rec)
	instID, err := buildApprovalInstance(bypass, rec, snapshot)
	if err != nil {
		return "", err
	}
	_ = orm.UpdateRecordByID(bypass, reqModel, id, map[string]interface{}{
		"state": string(ReqStateApproval), "approval_instance": instID,
	})
	return fmt.Sprintf("Sent to procurement approval (instance #%d).", instID), nil
}

// reqApprove finalizes requisition approval. If a procurement instance exists it
// must be fully approved; the acting user may not be the requester (SoD). A new
// open budget commitment is reserved on approval.
func reqApprove(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, reqModel, id)
	if rec == nil {
		return "", fmt.Errorf("requisition not found")
	}
	if RequisitionState(str(rec["state"])) != ReqStateApproval {
		return "", fmt.Errorf("requisition is not in the approval stage")
	}
	uid := orm.UIDFromContext(ctx)
	if uid > 0 && intID(rec["requester_id"]) == uid {
		return "", fmt.Errorf("separation of duties: the requester cannot approve")
	}
	if iid := intID(rec["approval_instance"]); iid > 0 {
		inst := loadRec(bypass, instModel, iid)
		if inst == nil || InstanceState(str(inst["state"])) != InstApproved {
			return "", fmt.Errorf("all approval levels must be signed off first")
		}
	}
	if err := transition(bypass, id, ReqStateApproval, ReqStateApproved, "Approved", ""); err != nil {
		return "", err
	}
	// Reserve an open commitment for the approved amount (no double count: the
	// matching consume happens at invoice match in T5).
	if bid := intID(rec["budget_id"]); bid > 0 {
		_ = recordCommitment(bypass, id, bid, CommitReserve, flt(rec["est_total"]), "Approved requisition commitment")
	}
	return "Requisition approved; budget commitment reserved.", nil
}

func reqReject(ctx context.Context, _ string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, reqModel, id)
	if rec == nil {
		return "", fmt.Errorf("requisition not found")
	}
	st := RequisitionState(str(rec["state"]))
	if st != ReqStateSubmitted && st != ReqStateBudgetValidation && st != ReqStateApproval {
		return "", fmt.Errorf("only a pending requisition can be rejected")
	}
	reason := strings.TrimSpace(vals["reason"])
	if err := transition(bypass, id, st, ReqStateRejected, "Rejected", reason); err != nil {
		return "", err
	}
	if iid := intID(rec["approval_instance"]); iid > 0 {
		_ = orm.UpdateRecordByID(bypass, instModel, iid, map[string]interface{}{"state": string(InstRejected)})
	}
	return "Requisition rejected.", nil
}

// reqResubmit forces re-approval after a material change: it rebuilds the
// instance from the current content and returns to the approval stage.
func reqResubmit(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, reqModel, id)
	if rec == nil {
		return "", fmt.Errorf("requisition not found")
	}
	st := RequisitionState(str(rec["state"]))
	if st != ReqStateApproved && st != ReqStateRejected && st != ReqStateCancelled {
		return "", fmt.Errorf("only an approved, rejected or cancelled requisition can be resubmitted")
	}
	snapshot := requisitionSnapshot(bypass, rec)
	// Cancel the stale instance, then build a fresh one reflecting the change.
	if iid := intID(rec["approval_instance"]); iid > 0 {
		_ = orm.UpdateRecordByID(bypass, instModel, iid, map[string]interface{}{"state": string(InstCancelled)})
	}
	instID, err := buildApprovalInstance(bypass, rec, snapshot)
	if err != nil {
		return "", err
	}
	_ = orm.UpdateRecordByID(bypass, reqModel, id, map[string]interface{}{
		"state": string(ReqStateApproval), "approval_instance": instID,
	})
	writeHistory(bypass, id, st, ReqStateApproval, orm.UIDFromContext(ctx), "Re-approval after material change")
	return fmt.Sprintf("Resubmitted for re-approval (instance #%d).", instID), nil
}

func reqClose(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	return reqToState([]RequisitionState{ReqStateFulfilled, ReqStateOrdered, ReqStatePartial}, ReqStateClosed, "Requisition closed.")(ctx, "", id, nil)
}

func reqCancel(ctx context.Context, _ string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, reqModel, id)
	if rec == nil {
		return "", fmt.Errorf("requisition not found")
	}
	st := RequisitionState(str(rec["state"]))
	if st == ReqStateFulfilled || st == ReqStateClosed || st == ReqStateCancelled {
		return "", fmt.Errorf("cannot cancel a fulfilled, closed or cancelled requisition")
	}
	reason := strings.TrimSpace(vals["reason"])
	if err := transition(bypass, id, st, ReqStateCancelled, "Cancelled", reason); err != nil {
		return "", err
	}
	// Cancel releases any reserved commitment so budget headroom returns.
	if bid := intID(rec["budget_id"]); bid > 0 {
		_ = recordCommitment(bypass, id, bid, CommitRelease, flt(rec["est_total"]), "Cancellation released commitment")
	}
	if iid := intID(rec["approval_instance"]); iid > 0 {
		_ = orm.UpdateRecordByID(bypass, instModel, iid, map[string]interface{}{"state": string(InstCancelled)})
	}
	return "Requisition cancelled; reserved commitment released.", nil
}

// reqOverrideBudget records an authorized budget override so the requisition may
// advance despite a breach. The acting user becomes the authority on the record.
func reqOverrideBudget(ctx context.Context, _ string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, reqModel, id)
	if rec == nil {
		return "", fmt.Errorf("requisition not found")
	}
	if RequisitionState(str(rec["state"])) != ReqStateBudgetValidation {
		return "", fmt.Errorf("an override can only be recorded at the budget-validation stage")
	}
	reason := strings.TrimSpace(vals["reason"])
	if reason == "" {
		return "", fmt.Errorf("a business reason is required to override a budget breach")
	}
	om, ok := orm.Registry[overrideModel]
	if !ok {
		return "", fmt.Errorf("override model unavailable")
	}
	bid := intID(rec["budget_id"])
	approved, actual, open, _ := availableBudget(bypass, bid)
	over := actual + open + flt(rec["est_total"]) - approved
	if over < 0 {
		over = 0
	}
	if _, err := orm.Create(bypass, om, map[string]interface{}{
		"name":           nextRef(bypass, "OVR"),
		"requisition_id": id,
		"budget_id":      bid,
		"policy":         strOr(rec["budget_policy"], "conditional"),
		"over_amount":    round2(over),
		"reason":         reason,
		"authority_id":   orm.UIDFromContext(ctx),
		"state":          string(OverrideApproved),
		"granted_on":     time.Now().Format(dtLayout),
		"active":         true,
	}); err != nil {
		return "", err
	}
	_ = orm.UpdateRecordByID(bypass, reqModel, id, map[string]interface{}{"budget_result": string(BudgetWarning)})
	return "Budget override authorized; requisition may proceed to approval.", nil
}

// ---------------------------------------------------------------------------
// Approval instance review
// ---------------------------------------------------------------------------

// instApprove signs off the lowest pending level. SoD: the approver may not be
// the requisition requester. When every level is approved the instance flips to
// approved so the requisition can be finalized.
func instApprove(ctx context.Context, _ string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	inst := loadRec(bypass, instModel, id)
	if inst == nil {
		return "", fmt.Errorf("approval instance not found")
	}
	if InstanceState(str(inst["state"])) != InstPending {
		return "", fmt.Errorf("instance is not pending")
	}
	req := loadRec(bypass, reqModel, intID(inst["requisition_id"]))
	uid := orm.UIDFromContext(ctx)
	if req != nil && uid > 0 && intID(req["requester_id"]) == uid {
		return "", fmt.Errorf("separation of duties: the requester cannot approve")
	}
	steps, _ := orm.SearchPage(bypass, stepModel, [][]interface{}{
		{"instance_id", "=", id}, {"state", "=", string(StepPending)},
	}, 200, 0, "level asc, sequence asc, id asc")
	if len(steps) == 0 {
		return "", fmt.Errorf("no pending approval level")
	}
	step := steps[0]
	stepID := intID(step["id"])
	decided := string(DecApproved)
	now := time.Now().Format(dtLayout)
	upd := map[string]interface{}{
		"decision": decided, "state": string(StepApproved),
		"decided_by": uid, "decided_on": now,
	}
	if c := strings.TrimSpace(vals["comment"]); c != "" {
		upd["comment"] = c
	}
	if err := orm.UpdateRecordByID(bypass, stepModel, stepID, upd); err != nil {
		return "", err
	}
	remaining, _ := orm.SearchCount(bypass, stepModel, [][]interface{}{
		{"instance_id", "=", id}, {"state", "=", string(StepPending)},
	})
	snapshot := str(inst["snapshot"])
	logEvent(bypass, id, stepID, uid, "approved", snapshot, fmt.Sprintf("Level %d approved.", int(flt(step["level"]))))
	if remaining == 0 {
		_ = orm.UpdateRecordByID(bypass, instModel, id, map[string]interface{}{
			"state": string(InstApproved), "completed_on": now,
		})
		return "All approval levels signed off.", nil
	}
	return fmt.Sprintf("Level approved; %d level(s) still pending.", remaining), nil
}

func instReject(ctx context.Context, _ string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	inst := loadRec(bypass, instModel, id)
	if inst == nil {
		return "", fmt.Errorf("approval instance not found")
	}
	if InstanceState(str(inst["state"])) != InstPending {
		return "", fmt.Errorf("instance is not pending")
	}
	uid := orm.UIDFromContext(ctx)
	steps, _ := orm.SearchPage(bypass, stepModel, [][]interface{}{
		{"instance_id", "=", id}, {"state", "=", string(StepPending)},
	}, 1, 0, "level asc, id asc")
	if len(steps) > 0 {
		_ = orm.UpdateRecordByID(bypass, stepModel, intID(steps[0]["id"]), map[string]interface{}{
			"decision": string(DecRejected), "state": string(StepRejected),
			"decided_by": uid, "decided_on": time.Now().Format(dtLayout),
			"comment": strings.TrimSpace(vals["reason"]),
		})
	}
	_ = orm.UpdateRecordByID(bypass, instModel, id, map[string]interface{}{
		"state": string(InstRejected), "completed_on": time.Now().Format(dtLayout),
	})
	logEvent(bypass, id, 0, uid, "rejected", str(inst["snapshot"]), "Requisition rejected at approval.")
	return "Requisition rejected at approval.", nil
}

// ---------------------------------------------------------------------------
// Budget check + history helpers
// ---------------------------------------------------------------------------

// runBudgetCheck computes availability, stores a purchase.budget.check snapshot
// and writes the resulting classification back to the requisition.
func runBudgetCheck(ctx context.Context, rec map[string]interface{}, policy BudgetPolicy) BudgetResult {
	id := intID(rec["id"])
	bid := intID(rec["budget_id"])
	proposed := flt(rec["est_total"])
	approved, actual, open, available := availableBudget(ctx, bid)
	result, _ := budgetDecision(policy, approved, actual, open, proposed, budgetTolerance)
	if bid <= 0 {
		result = BudgetNA
		available = 0
	}
	remaining := available - proposed
	if cm, ok := orm.Registry[budgetCheckMdl]; ok {
		_, _ = orm.Create(ctx, cm, map[string]interface{}{
			"name":            nextRef(ctx, "BCH"),
			"requisition_id":  id,
			"budget_id":       bid,
			"check_date":      time.Now().Format(dateLayout),
			"approved_budget": round2(approved),
			"actual_spend":    round2(actual),
			"commitments":     round2(open),
			"proposed_amt":    round2(proposed),
			"remaining":       round2(remaining),
			"result":          string(result),
			"note":            fmt.Sprintf("Policy %s; available %.2f.", policy, available),
		})
	}
	_ = orm.UpdateRecordByID(ctx, reqModel, id, map[string]interface{}{"budget_result": string(result)})
	return result
}

func hasApprovedOverride(ctx context.Context, reqID int) bool {
	n, _ := orm.SearchCount(ctx, overrideModel, [][]interface{}{
		{"requisition_id", "=", reqID}, {"state", "=", string(OverrideApproved)},
	})
	return n > 0
}

// reqToState is a generic guarded transition used for the simple forward moves.
func reqToState(from []RequisitionState, to RequisitionState, msg string) orm.ObjectActionFunc {
	return func(ctx context.Context, _ string, id int, vals map[string]string) (string, error) {
		bypass := orm.ContextWithBypass(ctx, true)
		rec := loadRec(bypass, reqModel, id)
		if rec == nil {
			return "", fmt.Errorf("requisition not found")
		}
		cur := RequisitionState(str(rec["state"]))
		allowed := false
		for _, s := range from {
			if cur == s {
				allowed = true
				break
			}
		}
		if !allowed {
			return "", fmt.Errorf("transition to %s not allowed from %s", to, cur)
		}
		if err := transition(bypass, id, cur, to, msg, ""); err != nil {
			return "", err
		}
		return msg, nil
	}
}

// transition updates state and appends a history row atomically-ish.
func transition(ctx context.Context, id int, from, to RequisitionState, label, name string) error {
	upd := map[string]interface{}{"state": string(to)}
	if name != "" {
		upd["name"] = name
	}
	if err := orm.UpdateRecordByID(ctx, reqModel, id, upd); err != nil {
		return err
	}
	writeHistory(ctx, id, from, to, orm.UIDFromContext(ctx), label)
	return nil
}

func writeHistory(ctx context.Context, reqID int, from, to RequisitionState, uid int, reason string) {
	hm, ok := orm.Registry[historyModel]
	if !ok {
		return
	}
	vals := map[string]interface{}{
		"requisition_id": reqID,
		"from_state":     string(from),
		"to_state":       string(to),
		"reason":         reason,
		"timestamp":      time.Now().Format(dtLayout),
	}
	if uid > 0 {
		vals["user_id"] = uid
	}
	_, _ = orm.Create(ctx, hm, vals)
}
