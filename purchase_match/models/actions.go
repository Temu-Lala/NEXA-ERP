package models

import (
	"context"
	"fmt"
	"strings"

	"nexa/core/orm"
)

// Supplier invoice matching workflow (blueprint §12). Purchase validates the
// bill against the PO and the accepted receipt, classifies variances and gates
// the hand-off to Accounts Payable. It never creates a bill, never posts a
// journal and never authorizes a payment: those belong to Accounting / AP, which
// are only referenced. Tolerances are configurable per case; a match is never
// auto-approved for being "close enough" - a residual variance must be explicitly
// resolved or waived by an authorized reviewer.

func init() {
	orm.RegisterObjectAction(caseModel, "action_compute_match", matchCompute)
	orm.RegisterObjectAction(caseModel, "action_escalate", matchEscalate)
	orm.RegisterObjectAction(caseModel, "action_approve_exceptions", matchApproveExceptions)
	orm.RegisterObjectAction(caseModel, "action_send_to_ap", matchSendToAP)
	orm.RegisterObjectAction(caseModel, "action_reset_draft", matchResetDraft)

	orm.RegisterObjectAction(excModel, "action_investigate", excInvestigate)
	orm.RegisterObjectAction(excModel, "action_resolve", excResolve)
	orm.RegisterObjectAction(excModel, "action_waive", excWaive)
}

// ---------------------------------------------------------------------------
// Match-case lifecycle
// ---------------------------------------------------------------------------

// matchCompute reconciles every compared line against its baseline, refreshes
// variances, raises exceptions for out-of-tolerance breaches (idempotent), runs
// duplicate detection and sets the case outcome + approvable amount.
func matchCompute(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, caseModel, id)
	if rec == nil {
		return "", fmt.Errorf("match case not found")
	}
	st := MatchState(str(rec["state"]))
	if st != MatchDraft && st != MatchInProgress {
		return "", fmt.Errorf("only a draft or in-progress match can be computed")
	}
	if strings.TrimSpace(str(rec["name"])) == "" || str(rec["name"]) == "New" {
		_ = orm.UpdateRecordByID(bypass, caseModel, id, map[string]interface{}{"name": nextRef(bypass, "MATCH")})
		rec = loadRec(bypass, caseModel, id)
	}

	poID := intID(rec["order_id"])
	poLineCache := loadPOLineMap(bypass, poID)
	acceptedByLine := buildAcceptedByPOLine(bypass, intID(rec["receipt_ref"]))
	method := MatchMethod(strOr(rec["method"], string(MethodThree)))
	useReceipt := method != MethodTwo

	tolPrice := flt(rec["tolerance_pct"])
	tolQty := flt(rec["tolerance_qty_pct"])
	rounding := flt(rec["rounding_tol"])

	lines, _ := orm.SearchPage(bypass, lineModel, [][]interface{}{{"match_id", "=", id}}, 500, 0, "id asc")
	if len(lines) == 0 {
		return "", fmt.Errorf("match case has no compared lines to reconcile")
	}

	var vars []lineVar
	poTotal, expTotal := 0.0, 0.0
	exceptions := 0
	for _, ln := range lines {
		lineID := intID(ln["id"])
		poLineID := intID(ln["order_line_id"])

		ordered := flt(ln["ordered_qty"])
		poPrice := flt(ln["po_unit_price"])
		if poLine, ok := poLineCache[poLineID]; ok {
			if ordered <= 0 {
				ordered = flt(poLine["product_qty"])
			}
			if poPrice <= 0 {
				poPrice = flt(poLine["price_unit"])
			}
			if strings.TrimSpace(str(ln["description"])) == "" {
				_ = orm.UpdateRecordByID(bypass, lineModel, lineID, map[string]interface{}{"description": str(poLine["name"])})
			}
		}

		accepted := flt(ln["accepted_qty"])
		if accepted <= 0 {
			accepted = acceptedByLine[poLineID]
		}
		invoiced := flt(ln["invoiced_qty"])
		invPrice := flt(ln["inv_unit_price"])

		expected := ordered
		if useReceipt {
			expected = accepted
		}

		v := reconcileMatchLine(expected, invoiced, poPrice, invPrice, tolQty, tolPrice, rounding)
		vars = append(vars, v)
		poTotal += round2(ordered * poPrice)
		expTotal += round2(expected * poPrice)

		_ = orm.UpdateRecordByID(bypass, lineModel, lineID, map[string]interface{}{
			"ordered_qty":      round3(ordered),
			"received_qty":     round3(acceptedByLine[poLineID]),
			"accepted_qty":     round3(accepted),
			"po_unit_price":    round2(poPrice),
			"qty_variance":     v.QtyVar,
			"price_variance":   v.PriceVar,
			"po_amount":        round2(ordered * poPrice),
			"inv_amount":       round2(invoiced * invPrice),
			"amount_variance":  v.AmountVar,
			"within_tolerance": !(v.QtyExc || v.PriceExc),
		})

		if v.QtyExc {
			if created := ensureException(bypass, id, lineID, ExcQuantity, SevMedium, v.AmountVar,
				fmt.Sprintf("Invoiced %.3f exceeds accepted %.3f beyond tolerance.", invoiced, expected)); created {
				exceptions++
			} else if isOpenExc(bypass, id, ExcQuantity, lineID) {
				exceptions++
			}
		}
		if v.PriceExc {
			if created := ensureException(bypass, id, lineID, ExcPrice, SevMedium, round2(v.PriceVar*invoiced),
				fmt.Sprintf("Invoice unit price %.2f deviates from PO price %.2f beyond %.2f%%.", invPrice, poPrice, tolPrice)); created {
				exceptions++
			} else if isOpenExc(bypass, id, ExcPrice, lineID) {
				exceptions++
			}
		}
	}

	// Header-level tax sanity check when an expected tax is not derivable: only
	// raise if an explicit tax tolerance was configured and the bill carries tax.
	tolTax := flt(rec["tolerance_tax_pct"])
	if tolTax > 0 && flt(rec["bill_amount"]) > 0 && !taxVarianceOk(0, flt(rec["bill_tax_amount"]), tolTax) && flt(rec["bill_tax_amount"]) > 0 {
		if created := ensureException(bypass, id, 0, ExcTax, SevLow, flt(rec["bill_tax_amount"]), "Invoice tax present but no PO tax baseline to compare; review required."); created {
			exceptions++
		} else if isOpenExc(bypass, id, ExcTax, 0) {
			exceptions++
		}
	}

	fp := duplicateFingerprint(intID(rec["partner_id"]), str(rec["bill_number"]), flt(rec["bill_amount"]), str(rec["currency"]), str(rec["bill_date"]))
	dup := isDuplicateFingerprint(bypass, fp, id)
	if dup {
		if created := ensureException(bypass, id, 0, ExcDuplicate, SevHigh, flt(rec["bill_amount"]), "A prior match shares this supplier/invoice/amount/currency/date fingerprint."); created {
			exceptions++
		} else if isOpenExc(bypass, id, ExcDuplicate, 0) {
			exceptions++
		}
	}

	// A missing-receipt signal for three/four-way cases with no receipt linked.
	if useReceipt && intID(rec["receipt_ref"]) <= 0 {
		if created := ensureException(bypass, id, 0, ExcNoReceipt, SevHigh, 0, "Three/four-way match selected but no accepted receipt is linked."); created {
			exceptions++
		} else if isOpenExc(bypass, id, ExcNoReceipt, 0) {
			exceptions++
		}
	}

	matched := approvableAmount(vars)
	result := matchOutcome(exceptions, dup)

	upd := map[string]interface{}{
		"state":           string(MatchInProgress),
		"result":          string(result),
		"method":          string(method),
		"fingerprint":     fp,
		"duplicate":       dup,
		"po_amount":       round2(poTotal),
		"expected_amount": round2(expTotal),
		"matched_amount":  matched,
	}
	if result == ResultException {
		upd["result"] = string(ResultException)
	}
	_ = orm.UpdateRecordByID(bypass, caseModel, id, upd)

	if result == ResultMatched && !dup {
		_ = publishMatchEvent(bypass, EvtMatched, id, matched, fp, fmt.Sprintf("Match %s cleared at %.2f, ready for AP review.", str(rec["name"]), matched))
	}

	summary := fmt.Sprintf("Match computed: %s. Approvable %.2f across %d line(s)", labelFor(result), matched, len(lines))
	if exceptions > 0 {
		summary += fmt.Sprintf(", %d open exception(s)", exceptions)
	}
	if dup {
		summary += ", DUPLICATE fingerprint detected (on hold for review)"
	}
	appendHistory(bypass, id, summary+".")
	return summary + ".", nil
}

// matchEscalate holds a case for investigation without changing its computed
// outcome. Escalation is a reviewable signal, never an auto-approval.
func matchEscalate(ctx context.Context, method string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, caseModel, id)
	if rec == nil {
		return "", fmt.Errorf("match case not found")
	}
	if MatchState(str(rec["state"])) == MatchDraft {
		return "", fmt.Errorf("compute the match before escalating")
	}
	_ = orm.UpdateRecordByID(bypass, caseModel, id, map[string]interface{}{
		"result": string(ResultHold), "escalated": true,
	})
	reason := strings.TrimSpace(vals["reason"])
	if reason == "" {
		reason = "Escalated for manual review."
	}
	appendHistory(bypass, id, "Escalated: "+reason)
	return "Match escalated and placed on hold.", nil
}

// matchApproveExceptions records an authorized reviewer once every exception is
// dispositioned (resolved or waived). The reviewer identity is stamped so the
// later AP hand-off can be attributed; approvals are never automatic.
func matchApproveExceptions(ctx context.Context, method string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, caseModel, id)
	if rec == nil {
		return "", fmt.Errorf("match case not found")
	}
	if strOr(rec["result"], string(ResultPending)) == string(ResultPending) {
		return "", fmt.Errorf("compute the match before approving")
	}
	if open := countUnresolvedExceptions(bypass, id); open > 0 {
		return "", fmt.Errorf("%d exception(s) still require resolution or waiver", open)
	}
	uid := orm.UIDFromContext(ctx)
	if uid <= 0 {
		return "", fmt.Errorf("approver identity required")
	}
	_ = orm.UpdateRecordByID(bypass, caseModel, id, map[string]interface{}{
		"approved": true, "reviewer_id": uid,
	})
	appendHistory(bypass, id, fmt.Sprintf("Exceptions approved by user %d.", uid))
	return "Match exceptions approved.", nil
}

// matchSendToAP releases the validated, approvable amount to Accounts Payable
// and consumes the originating budget commitment exactly once. It never posts a
// bill or authorizes a payment - AP owns those.
func matchSendToAP(ctx context.Context, method string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, caseModel, id)
	if rec == nil {
		return "", fmt.Errorf("match case not found")
	}
	if MatchState(str(rec["state"])) == MatchDraft {
		return "", fmt.Errorf("compute the match before sending to AP")
	}
	if boolOf(rec["sent_to_ap"]) {
		return "", fmt.Errorf("this match has already been sent to AP")
	}
	if boolOf(rec["duplicate"]) {
		return "", fmt.Errorf("a duplicate-suspect match cannot be sent to AP until reviewed")
	}
	if open := countUnresolvedExceptions(bypass, id); open > 0 {
		return "", fmt.Errorf("%d exception(s) unresolved; resolve or waive before AP hand-off", open)
	}
	if countWaivedExceptions(bypass, id) > 0 && !boolOf(rec["approved"]) {
		return "", fmt.Errorf("waived variances require an authorized approval first")
	}

	matched := flt(rec["matched_amount"])
	consumeCommitment(bypass, rec, matched)

	_ = orm.UpdateRecordByID(bypass, caseModel, id, map[string]interface{}{
		"sent_to_ap": true, "sent_to_ap_on": nowStr(), "result": string(ResultMatched), "state": string(MatchDone),
	})
	_ = publishMatchEvent(bypass, EvtSentToAP, id, matched, str(rec["fingerprint"]),
		fmt.Sprintf("Match %s released to AP for %.2f. Payment authorization stays with AP.", str(rec["name"]), matched))
	appendHistory(bypass, id, fmt.Sprintf("Sent to AP: %.2f released. No payment authorized by Purchase.", matched))
	return fmt.Sprintf("Match sent to AP: %.2f released for payment (Purchase does not pay).", matched), nil
}

// matchResetDraft returns a computed case to draft so it can be re-run after a
// bill or receipt correction. Resolved/waived decisions are preserved.
func matchResetDraft(ctx context.Context, method string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, caseModel, id)
	if rec == nil {
		return "", fmt.Errorf("match case not found")
	}
	if boolOf(rec["sent_to_ap"]) {
		return "", fmt.Errorf("a match already sent to AP cannot be reset")
	}
	_ = orm.UpdateRecordByID(bypass, caseModel, id, map[string]interface{}{
		"state": string(MatchDraft), "result": string(ResultPending), "duplicate": false, "approved": false,
	})
	appendHistory(bypass, id, "Reset to draft for re-computation.")
	return "Match reset to draft.", nil
}

// ---------------------------------------------------------------------------
// Exception lifecycle
// ---------------------------------------------------------------------------

func excInvestigate(ctx context.Context, method string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	exc := loadRec(bypass, excModel, id)
	if exc == nil {
		return "", fmt.Errorf("exception not found")
	}
	if ExcState(str(exc["state"])) != ExcStateOpen {
		return "", fmt.Errorf("only an open exception can move to investigating")
	}
	_ = orm.UpdateRecordByID(bypass, excModel, id, map[string]interface{}{"state": string(ExcStateInvestigating)})
	return "Exception moved to investigating.", nil
}

func excResolve(ctx context.Context, method string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	exc := loadRec(bypass, excModel, id)
	if exc == nil {
		return "", fmt.Errorf("exception not found")
	}
	res := strings.TrimSpace(vals["resolution"])
	if res == "" {
		return "", fmt.Errorf("a resolution note is required to resolve an exception")
	}
	uid := orm.UIDFromContext(ctx)
	_ = orm.UpdateRecordByID(bypass, excModel, id, map[string]interface{}{
		"state": string(ExcStateResolved), "resolution": res, "resolved_by": uid,
	})
	return "Exception resolved.", nil
}

func excWaive(ctx context.Context, method string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	exc := loadRec(bypass, excModel, id)
	if exc == nil {
		return "", fmt.Errorf("exception not found")
	}
	// A duplicate is never waivable: it must be resolved (cancelled/corrected),
	// never accepted as "close enough".
	if ExceptionType(str(exc["exc_type"])) == ExcDuplicate {
		return "", fmt.Errorf("a duplicate exception cannot be waived; it must be resolved")
	}
	reason := strings.TrimSpace(vals["resolution"])
	if reason == "" {
		return "", fmt.Errorf("a waiver reason is required")
	}
	uid := orm.UIDFromContext(ctx)
	_ = orm.UpdateRecordByID(bypass, excModel, id, map[string]interface{}{
		"state": string(ExcStateWaived), "resolution": reason, "resolved_by": uid,
	})
	return "Exception waived.", nil
}

// ---------------------------------------------------------------------------
// Local helpers
// ---------------------------------------------------------------------------

func ensureException(ctx context.Context, caseID, lineID int, t ExceptionType, sev ExcSeverity, diff float64, desc string) bool {
	rows, _ := orm.SearchPage(ctx, excModel, [][]interface{}{
		{"match_id", "=", caseID},
		{"exc_type", "=", string(t)},
		{"match_line_id", "=", lineID},
	}, 1, 0, "id asc")
	if len(rows) > 0 {
		return false
	}
	_, _ = createRec(ctx, excModel, map[string]interface{}{
		"name":          nextRef(ctx, "EXC"),
		"match_id":      caseID,
		"match_line_id": lineID,
		"exc_type":      string(t),
		"severity":      string(sev),
		"description":   desc,
		"diff_amount":   round2(diff),
		"state":         string(ExcStateOpen),
		"active":        true,
	})
	_ = publishMatchEvent(ctx, EvtExcCreated, caseID, diff, "",
		fmt.Sprintf("procurement.invoice_match.exception_created: %s (line %d)", string(t), lineID))
	return true
}

func isOpenExc(ctx context.Context, caseID int, t ExceptionType, lineID int) bool {
	rows, _ := orm.SearchPage(ctx, excModel, [][]interface{}{
		{"match_id", "=", caseID},
		{"exc_type", "=", string(t)},
		{"match_line_id", "=", lineID},
		{"state", "in", []interface{}{string(ExcStateOpen), string(ExcStateInvestigating)}},
	}, 1, 0, "id asc")
	return len(rows) > 0
}

func countUnresolvedExceptions(ctx context.Context, caseID int) int {
	n, _ := orm.SearchCount(ctx, excModel, [][]interface{}{
		{"match_id", "=", caseID},
		{"state", "in", []interface{}{string(ExcStateOpen), string(ExcStateInvestigating)}},
	})
	return int(n)
}

func countWaivedExceptions(ctx context.Context, caseID int) int {
	n, _ := orm.SearchCount(ctx, excModel, [][]interface{}{
		{"match_id", "=", caseID}, {"state", "=", string(ExcStateWaived)},
	})
	return int(n)
}

// consumeCommitment turns the reserved budget commitment into consumed spend on
// AP hand-off, exactly once per match. References to the requisition/budget and
// the commitment ledger stay loose (no Accounting writes); a missing ledger
// model is skipped rather than fatal.
func consumeCommitment(ctx context.Context, rec map[string]interface{}, matched float64) {
	if intID(rec["commitment_ref"]) > 0 || matched <= 0 {
		return
	}
	reqID := intID(rec["requisition_ref"])
	budID := intID(rec["budget_ref"])
	if reqID <= 0 || budID <= 0 {
		return
	}
	name := nextRef(ctx, "CONS")
	cid, err := createRec(ctx, commitmentMdl, map[string]interface{}{
		"name":           name,
		"requisition_id": reqID,
		"budget_id":      budID,
		"kind":           "consume",
		"amount":         round2(matched),
		"occured_on":     timeNowDay(),
		"reason":         fmt.Sprintf("Invoice match %s consumed on AP hand-off", str(rec["name"])),
		"note":           "Auto-consumed by purchase_match; single posting guarded by commitment_ref.",
		"active":         true,
	})
	if err != nil || cid <= 0 {
		return
	}
	_ = orm.UpdateRecordByID(ctx, caseModel, intID(rec["id"]), map[string]interface{}{"commitment_ref": cid})
}

// loadPOLineMap indexes a PO's lines by id for baseline lookup.
func loadPOLineMap(ctx context.Context, poID int) map[int]map[string]interface{} {
	out := map[int]map[string]interface{}{}
	if poID <= 0 {
		return out
	}
	rows, _ := orm.SearchPage(ctx, poLineModel, [][]interface{}{{"order_id", "=", poID}}, 500, 0, "id asc")
	for _, r := range rows {
		out[intID(r["id"])] = r
	}
	return out
}

// buildAcceptedByPOLine maps each PO line to the accepted quantity recorded on
// the linked T4 receipt, which is the authoritative three-way baseline.
func buildAcceptedByPOLine(ctx context.Context, receiptID int) map[int]float64 {
	out := map[int]float64{}
	if receiptID <= 0 {
		return out
	}
	rows, _ := orm.SearchPage(ctx, rcptLineModel, [][]interface{}{{"receipt_id", "=", receiptID}}, 500, 0, "id asc")
	for _, r := range rows {
		poLineID := intID(r["order_line_id"])
		if poLineID <= 0 {
			continue
		}
		out[poLineID] += flt(r["accepted_qty"])
	}
	return out
}

func labelFor(r MatchResult) string {
	switch r {
	case ResultMatched:
		return "MATCHED"
	case ResultException:
		return "EXCEPTION"
	case ResultHold:
		return "ON HOLD"
	default:
		return "PENDING"
	}
}
