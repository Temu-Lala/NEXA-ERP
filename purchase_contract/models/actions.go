package models

import (
	"context"
	"fmt"
	"strings"

	"nexa/core/orm"
)

// Contract workflow (blueprint §10). Purchase owns the agreement lifecycle and
// its append-only amendment history. Approval is required before a contract
// becomes active and, per the ownership rule, an active contract never bypasses
// requisition approval or budget control - it only supplies contracted rates and
// a ceiling that consumption respects.

func init() {
	orm.RegisterObjectAction(contractModel, "action_submit_approval", contractSubmit)
	orm.RegisterObjectAction(contractModel, "action_approve", contractApprove)
	orm.RegisterObjectAction(contractModel, "action_reject", contractReject)
	orm.RegisterObjectAction(contractModel, "action_suspend", contractSuspend)
	orm.RegisterObjectAction(contractModel, "action_resume", contractResume)
	orm.RegisterObjectAction(contractModel, "action_renew", contractRenew)
	orm.RegisterObjectAction(contractModel, "action_amend", contractAmend)
	orm.RegisterObjectAction(contractModel, "action_close", contractClose)
	orm.RegisterObjectAction(contractModel, "action_cancel", contractCancel)
	orm.RegisterObjectAction(contractModel, "action_record_consumption", contractRecordConsumption)
	orm.RegisterObjectAction(contractModel, "action_validate_price", contractValidatePrice)
}

func contractSubmit(ctx context.Context, method string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	c := loadRec(bypass, contractModel, id)
	if c == nil {
		return "", fmt.Errorf("contract not found")
	}
	if ContractState(str(c["state"])) != CStateDraft {
		return "", fmt.Errorf("only a draft contract can be submitted for approval")
	}
	if strings.TrimSpace(str(c["title"])) == "" || intID(c["partner_id"]) <= 0 {
		return "", fmt.Errorf("a contract needs a title and a supplier before submission")
	}
	if strings.TrimSpace(str(c["name"])) == "" || str(c["name"]) == "New" {
		_ = orm.UpdateRecordByID(bypass, contractModel, id, map[string]interface{}{"name": nextRef(bypass, "CON")})
	}
	_ = orm.UpdateRecordByID(bypass, contractModel, id, map[string]interface{}{"state": string(CStatePending)})
	appendHistory(bypass, id, "Submitted for approval.")
	return "Contract submitted for approval.", nil
}

func contractApprove(ctx context.Context, method string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	c := loadRec(bypass, contractModel, id)
	if c == nil {
		return "", fmt.Errorf("contract not found")
	}
	if ContractState(str(c["state"])) != CStatePending {
		return "", fmt.Errorf("only a pending contract can be approved")
	}
	uid := orm.UIDFromContext(ctx)
	if uid <= 0 {
		return "", fmt.Errorf("approver identity required")
	}
	if owner := intID(c["owner_id"]); owner > 0 && owner == uid {
		return "", fmt.Errorf("separation of duties: the contract owner cannot approve their own contract")
	}
	_ = orm.UpdateRecordByID(bypass, contractModel, id, map[string]interface{}{"state": string(CStateActive)})
	snapshotVersion(bypass, c, "Approved and activated.", uid)
	appendHistory(bypass, id, fmt.Sprintf("Approved by user %d; contract active.", uid))
	return "Contract approved and set to active.", nil
}

func contractReject(ctx context.Context, method string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	c := loadRec(bypass, contractModel, id)
	if c == nil {
		return "", fmt.Errorf("contract not found")
	}
	if ContractState(str(c["state"])) != CStatePending {
		return "", fmt.Errorf("only a pending contract can be rejected")
	}
	reason := strings.TrimSpace(vals["reason"])
	if reason == "" {
		reason = "Rejected at approval."
	}
	_ = orm.UpdateRecordByID(bypass, contractModel, id, map[string]interface{}{"state": string(CStateDraft)})
	appendHistory(bypass, id, "Rejected: "+reason+" (returned to draft).")
	return "Contract rejected and returned to draft.", nil
}

func contractSuspend(ctx context.Context, method string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	c := loadRec(bypass, contractModel, id)
	if c == nil {
		return "", fmt.Errorf("contract not found")
	}
	if ContractState(str(c["state"])) != CStateActive {
		return "", fmt.Errorf("only an active contract can be suspended")
	}
	_ = orm.UpdateRecordByID(bypass, contractModel, id, map[string]interface{}{"state": string(CStateSuspended)})
	note := strings.TrimSpace(vals["reason"])
	if note == "" {
		note = "Suspended."
	}
	appendHistory(bypass, id, note)
	return "Contract suspended.", nil
}

func contractResume(ctx context.Context, method string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	c := loadRec(bypass, contractModel, id)
	if c == nil {
		return "", fmt.Errorf("contract not found")
	}
	if ContractState(str(c["state"])) != CStateSuspended {
		return "", fmt.Errorf("only a suspended contract can be resumed")
	}
	_ = orm.UpdateRecordByID(bypass, contractModel, id, map[string]interface{}{"state": string(CStateActive)})
	appendHistory(bypass, id, "Resumed.")
	return "Contract resumed to active.", nil
}

// contractRenew extends the term to a new end date, records an amendment version
// and re-activates the agreement. Renewal is explicit and versioned; it never
// silently rolls a contract forward.
func contractRenew(ctx context.Context, method string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	c := loadRec(bypass, contractModel, id)
	if c == nil {
		return "", fmt.Errorf("contract not found")
	}
	st := ContractState(str(c["state"]))
	if st != CStateActive && st != CStateExpiring && st != CStateExpired && st != CStateRenewed {
		return "", fmt.Errorf("only an active, expiring, expired or renewed contract can be renewed")
	}
	newEnd := strings.TrimSpace(vals["date_end"])
	if newEnd == "" {
		return "", fmt.Errorf("provide a new expiry date to renew")
	}
	if _, ok := parseDay(newEnd); !ok {
		return "", fmt.Errorf("new expiry date is not a valid date")
	}
	uid := orm.UIDFromContext(ctx)
	newVersion := int(flt(c["version"])) + 1
	_ = orm.UpdateRecordByID(bypass, contractModel, id, map[string]interface{}{
		"date_end": newEnd, "state": string(CStateActive), "version": newVersion,
	})
	snapshotVersion(bypass, c, fmt.Sprintf("Renewed to %s (v%d).", newEnd, newVersion), uid)
	appendHistory(bypass, id, fmt.Sprintf("Renewed: new expiry %s, version %d.", newEnd, newVersion))
	return fmt.Sprintf("Contract renewed to %s (v%d).", newEnd, newVersion), nil
}

// contractAmend records an append-only snapshot of the current terms and bumps
// the version so the superseded wording is preserved.
func contractAmend(ctx context.Context, method string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	c := loadRec(bypass, contractModel, id)
	if c == nil {
		return "", fmt.Errorf("contract not found")
	}
	st := ContractState(str(c["state"]))
	if st == CStateDraft || st == CStateCancelled || st == CStateClosed {
		return "", fmt.Errorf("only a live contract can be amended")
	}
	summary := strings.TrimSpace(vals["change_summary"])
	if summary == "" {
		summary = "Amendment recorded."
	}
	uid := orm.UIDFromContext(ctx)
	newVersion := int(flt(c["version"])) + 1
	_ = orm.UpdateRecordByID(bypass, contractModel, id, map[string]interface{}{"version": newVersion})
	snapshotVersion(bypass, c, fmt.Sprintf("%s (v%d)", summary, newVersion), uid)
	appendHistory(bypass, id, fmt.Sprintf("Amendment v%d: %s", newVersion, summary))
	return fmt.Sprintf("Amendment recorded as version %d.", newVersion), nil
}

func contractClose(ctx context.Context, method string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	c := loadRec(bypass, contractModel, id)
	if c == nil {
		return "", fmt.Errorf("contract not found")
	}
	if ContractState(str(c["state"])) == CStateCancelled {
		return "", fmt.Errorf("a cancelled contract cannot be closed")
	}
	_ = orm.UpdateRecordByID(bypass, contractModel, id, map[string]interface{}{"state": string(CStateClosed), "active": false})
	appendHistory(bypass, id, "Contract closed.")
	return "Contract closed.", nil
}

func contractCancel(ctx context.Context, method string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	c := loadRec(bypass, contractModel, id)
	if c == nil {
		return "", fmt.Errorf("contract not found")
	}
	st := ContractState(str(c["state"]))
	if st == CStateClosed || st == CStateCancelled {
		return "", fmt.Errorf("contract already closed or cancelled")
	}
	reason := strings.TrimSpace(vals["reason"])
	if reason == "" {
		reason = "Cancelled."
	}
	_ = orm.UpdateRecordByID(bypass, contractModel, id, map[string]interface{}{"state": string(CStateCancelled), "active": false})
	appendHistory(bypass, id, "Cancelled: "+reason)
	return "Contract cancelled.", nil
}

// contractRecordConsumption appends a drawdown movement and enforces the value
// ceiling. The same source reference is never counted twice (idempotent), a
// credit can always be recorded, and a commit/invoice that would exceed the
// ceiling is blocked rather than silently accepted.
func contractRecordConsumption(ctx context.Context, method string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	c := loadRec(bypass, contractModel, id)
	if c == nil {
		return "", fmt.Errorf("contract not found")
	}
	st := ContractState(str(c["state"]))
	if st == CStateDraft || st == CStateCancelled || st == CStateClosed {
		return "", fmt.Errorf("a draft, cancelled or closed contract cannot record consumption")
	}
	kind := ConsumptionKind(strOr(vals["kind"], string(ConsCommit)))
	qty := flt(vals["quantity"])
	price := flt(vals["unit_price"])
	amount := flt(vals["amount"])
	if amount == 0 {
		amount = round2(qty * price)
	}
	reference := strings.TrimSpace(vals["reference"])
	if reference != "" {
		dup, _ := orm.SearchPage(bypass, consumptionModel, [][]interface{}{
			{"contract_id", "=", id}, {"kind", "=", string(kind)}, {"reference", "=", reference},
		}, 1, 0, "id asc")
		if len(dup) > 0 {
			return "Consumption already recorded for this reference (no double count).", nil
		}
	}

	ceiling := flt(c["value_ceiling"])
	used := flt(c["value_used"])
	if kind != ConsCredit && ceiling > 0 {
		prospective := contractUsage(used, amount, 0)
		if ceilingStatus(prospective, ceiling, warnPctDefault) == "exceeded" {
			return "", fmt.Errorf("this movement would exceed the contract value ceiling (used %.2f + %.2f > ceiling %.2f); escalate for an authorized increase", used, amount, ceiling)
		}
	}

	cid, err := createRec(bypass, consumptionModel, map[string]interface{}{
		"contract_id": id,
		"kind":        string(kind),
		"order_ref":   intID(vals["order_ref"]),
		"product_id":  intID(vals["product_id"]),
		"reference":   reference,
		"date":        nowStr(),
		"quantity":    round3(qty),
		"unit_price":  round2(price),
		"amount":      round2(amount),
		"note":        strings.TrimSpace(vals["note"]),
		"active":      true,
	})
	if err != nil {
		return "", err
	}
	_, _, _, newUsed := recomputeTotals(bypass, id)
	status := ceilingStatus(newUsed, ceiling, warnPctDefault)
	msg := fmt.Sprintf("Consumption #%d recorded (%s %.2f). Used %.2f of %s.",
		cid, string(kind), amount, newUsed, ceilingDesc(ceiling))
	appendHistory(bypass, id, msg+" Ceiling status: "+status+".")
	if status == "warning" {
		msg += " WARNING: approaching ceiling."
	}
	return msg, nil
}

// contractValidatePrice checks a proposed unit price against the contracted tier
// rate for a quantity, within the tolerance. It reports the contracted rate and
// the variance without altering anything.
func contractValidatePrice(ctx context.Context, method string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	c := loadRec(bypass, contractModel, id)
	if c == nil {
		return "", fmt.Errorf("contract not found")
	}
	qty := flt(vals["quantity"])
	proposed := flt(vals["unit_price"])
	tol := flt(vals["tolerance_pct"])
	tiers := loadContractTiers(bypass, id)
	rate, ok := resolveTierPrice(tiers, qty)
	if !ok {
		return "No contracted rate covers this quantity; PO price is not validated against the schedule.", nil
	}
	if priceWithinTolerance(rate, proposed, tol) {
		return fmt.Sprintf("Price %.2f is within tolerance of the contracted rate %.2f.", proposed, rate), nil
	}
	return fmt.Sprintf("Price %.2f EXCEEDS contracted rate %.2f beyond tolerance; review required.", proposed, rate), nil
}

// ---------------------------------------------------------------------------
// Local helpers
// ---------------------------------------------------------------------------

func snapshotVersion(ctx context.Context, c map[string]interface{}, summary string, uid int) {
	_, _ = createRec(ctx, versionModel, map[string]interface{}{
		"contract_id":    intID(c["id"]),
		"version":        int(flt(c["version"])),
		"title":          str(c["title"]),
		"snapshot":       snapshotText(c),
		"change_summary": summary,
		"changed_by":     uid,
		"changed_on":     nowStr(),
	})
}

func snapshotText(c map[string]interface{}) string {
	var b strings.Builder
	fmt.Fprintf(&b, "type=%s state=%s currency=%s",
		strOr(c["contract_type"], "-"), strOr(c["state"], "-"), strOr(c["currency"], "USD"))
	fmt.Fprintf(&b, " | term=%s..%s", strOr(c["date_start"], "-"), strOr(c["date_end"], "-"))
	fmt.Fprintf(&b, " | ceiling=%.2f min=%.2f max_qty=%.3f",
		flt(c["value_ceiling"]), flt(c["min_commitment"]), flt(c["max_quantity"]))
	return b.String()
}

func ceilingDesc(ceiling float64) string {
	if ceiling <= 0 {
		return "no ceiling"
	}
	return fmt.Sprintf("ceiling %.2f", ceiling)
}
