package models

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nexa/core/orm"
)

// Receipt / delivery workflow (blueprint §11). Purchase records what physically
// arrived and coordinates acceptance; the authoritative stock movement is posted
// through Inventory (stock.picking) and is never faked here. Accepted quantities
// roll up to the PO line so the anchor can report received vs ordered. Services
// have no stock movement. Supplier returns reference (do not create) Accounting
// credit notes.

const (
	qtyAbsTol = 2.0 // absolute units allowed over/under before a flag
	qtyPctTol = 5.0 // percentage tolerance, whichever is larger
	equalTol  = 0.0005
)

func init() {
	orm.RegisterObjectAction(receiptModel, "action_inspect", receiptInspect)
	orm.RegisterObjectAction(receiptModel, "action_validate_receipt", receiptValidate)
	orm.RegisterObjectAction(receiptModel, "action_accept_service", receiptAcceptService)
	orm.RegisterObjectAction(receiptModel, "action_create_discrepancy", receiptCreateDiscrepancy)
	orm.RegisterObjectAction(receiptModel, "action_cancel", receiptCancel)

	orm.RegisterObjectAction(retModel, "action_authorize", retAuthorize)
	orm.RegisterObjectAction(retModel, "action_ship_return", retShipReturn)
	orm.RegisterObjectAction(retModel, "action_record_credit", retRecordCredit)
	orm.RegisterObjectAction(retModel, "action_close", retClose)

	orm.RegisterObjectAction(discModel, "action_resolve", discResolve)
	orm.RegisterObjectAction(discModel, "action_close", discClose)
	orm.RegisterObjectAction(claimModel, "action_submit", claimSubmit)
	orm.RegisterObjectAction(claimModel, "action_settle", claimSettle)
}

// ---------------------------------------------------------------------------
// Receipt lifecycle
// ---------------------------------------------------------------------------

func receiptInspect(ctx context.Context, _ string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, receiptModel, id)
	if rec == nil {
		return "", fmt.Errorf("receipt not found")
	}
	if ReceiptState(str(rec["state"])) != ReceiptDraft {
		return "", fmt.Errorf("only a draft receipt can move to inspection")
	}
	upd := map[string]interface{}{"state": string(ReceiptInspection)}
	if res := strings.TrimSpace(vals["inspection_result"]); res != "" {
		upd["inspection_result"] = res
	}
	if err := orm.UpdateRecordByID(bypass, receiptModel, id, upd); err != nil {
		return "", err
	}
	return "Receipt moved to inspection.", nil
}

// receiptValidate posts the accepted quantities to Inventory, rolls them up to
// the PO lines, flags discrepancies and sets the final receipt state.
func receiptValidate(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, receiptModel, id)
	if rec == nil {
		return "", fmt.Errorf("receipt not found")
	}
	st := ReceiptState(str(rec["state"]))
	if st != ReceiptDraft && st != ReceiptInspection {
		return "", fmt.Errorf("only a draft or inspected receipt can be validated")
	}
	lines, _ := orm.SearchPage(bypass, rcptLineMdl, [][]interface{}{{"receipt_id", "=", id}}, 500, 0, "id asc")
	if len(lines) == 0 {
		return "", fmt.Errorf("receipt has no lines")
	}
	if uid := orm.UIDFromContext(ctx); uid > 0 && intID(rec["receiver_id"]) == 0 {
		_ = orm.UpdateRecordByID(bypass, receiptModel, id, map[string]interface{}{"receiver_id": uid})
	}

	if strings.EqualFold(strOr(rec["receipt_type"], "goods"), "service") {
		_ = orm.UpdateRecordByID(bypass, receiptModel, id, map[string]interface{}{"state": string(ReceiptService)})
		return "Service receipt accepted (no stock movement).", nil
	}

	// Only genuinely stocked lines (product set + accepted qty > 0) are posted.
	var stockLines []map[string]interface{}
	totalAccepted := 0.0
	for _, ln := range lines {
		acc := flt(ln["accepted_qty"])
		if intID(ln["product_id"]) > 0 && acc > 0 {
			stockLines = append(stockLines, ln)
		}
		totalAccepted += acc
	}
	destLoc := resolveWarehouseLocation(bypass, intID(rec["warehouse_id"]))
	if len(stockLines) > 0 && destLoc == 0 {
		return "", fmt.Errorf("set a receiving warehouse so Inventory knows the destination")
	}

	flags := []string{}
	pickID := intID(rec["picking_ref"])
	if len(stockLines) > 0 && pickID == 0 {
		var err error
		pickID, err = postIncomingStock(bypass, rec, stockLines, destLoc)
		if err != nil {
			return "", err
		}
		_ = orm.UpdateRecordByID(bypass, receiptModel, id, map[string]interface{}{"picking_ref": pickID})
	}

	for _, ln := range lines {
		rollUpToPOLine(bypass, ln)
		if msg := discrepancyCheck(bypass, rec, ln); msg != "" {
			flags = append(flags, msg)
		}
	}

	final := ReceiptAccepted
	if totalAccepted <= 0 {
		final = ReceiptRejected
	} else if anyPartial(bypass, lines) {
		final = ReceiptPartially
	}
	_ = orm.UpdateRecordByID(bypass, receiptModel, id, map[string]interface{}{
		"state": string(final), "received_date": time.Now().Format(dateLayout),
	})
	msg := fmt.Sprintf("Receipt %s.", final)
	if pickID > 0 {
		msg += fmt.Sprintf(" Inventory posting #%d done.", pickID)
	}
	if len(flags) > 0 {
		msg += " Flags: " + strings.Join(flags, "; ")
	}
	return msg, nil
}

func receiptAcceptService(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, receiptModel, id)
	if rec == nil {
		return "", fmt.Errorf("receipt not found")
	}
	if ReceiptState(str(rec["state"])) != ReceiptDraft {
		return "", fmt.Errorf("only a draft service receipt can be accepted")
	}
	_ = orm.UpdateRecordByID(bypass, receiptModel, id, map[string]interface{}{
		"state": string(ReceiptService), "received_date": time.Now().Format(dateLayout),
	})
	return "Service milestone accepted.", nil
}

func receiptCancel(ctx context.Context, _ string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, receiptModel, id)
	if rec == nil {
		return "", fmt.Errorf("receipt not found")
	}
	if ReceiptState(str(rec["state"])) != ReceiptDraft {
		return "", fmt.Errorf("only a draft receipt can be cancelled (post reversals via Inventory)")
	}
	note := appendNote(str(rec["notes"]), "Cancelled: "+strings.TrimSpace(vals["reason"]))
	_ = orm.UpdateRecordByID(bypass, receiptModel, id, map[string]interface{}{"active": false, "notes": note})
	return "Receipt cancelled.", nil
}

// receiptCreateDiscrepancy opens a discrepancy case for a line supplied via the
// action context (issue_type, quantity); it is the entry point to a claim.
func receiptCreateDiscrepancy(ctx context.Context, _ string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, receiptModel, id)
	if rec == nil {
		return "", fmt.Errorf("receipt not found")
	}
	lineID := intID(vals["receipt_line_id"])
	var line map[string]interface{}
	if lineID > 0 {
		line = loadRec(bypass, rcptLineMdl, lineID)
	} else {
		lines, _ := orm.SearchPage(bypass, rcptLineMdl, [][]interface{}{{"receipt_id", "=", id}}, 1, 0, "id asc")
		if len(lines) > 0 {
			line = lines[0]
		}
	}
	if line == nil {
		return "", fmt.Errorf("receipt line not found")
	}
	issue := DiscrepancyType(strings.TrimSpace(vals["issue_type"]))
	if issue == "" {
		issue = classifyDiscrepancy(flt(line["ordered_qty"]), flt(line["delivered_qty"]))
	}
	if issue == "" {
		issue = DiscNonconfom
	}
	qty := flt(vals["quantity"])
	if qty <= 0 {
		qty = flt(line["delivered_qty"]) - flt(line["accepted_qty"])
		if qty <= 0 {
			qty = flt(line["delivered_qty"])
		}
	}
	dm, ok := orm.Registry[discModel]
	if !ok {
		return "", fmt.Errorf("discrepancy model unavailable")
	}
	dID, err := orm.Create(bypass, dm, map[string]interface{}{
		"name":            nextRef(bypass, "DISC"),
		"receipt_id":      id,
		"receipt_line_id": intID(line["id"]),
		"order_id":        intID(rec["order_id"]),
		"product_id":      intID(line["product_id"]),
		"issue_type":      string(issue),
		"quantity":        round3(qty),
		"lot":             strOr(vals["lot"], str(line["lot"])),
		"serial_number":   strOr(vals["serial"], str(line["serial_number"])),
		"evidence":        strOr(vals["evidence"], ""),
		"on_hold":         boolOf(vals["on_hold"]),
		"state":           string(DiscOpen),
		"history":         fmt.Sprintf("Opened %s by uid %d.", time.Now().Format(dtLayout), orm.UIDFromContext(ctx)),
		"active":          true,
	})
	if err != nil {
		return "", err
	}
	_ = appendDiscrepancyNote(bypass, rec, fmt.Sprintf("Discrepancy %s opened (%s).", strOr(vals["name"], "new"), issue))
	return fmt.Sprintf("Discrepancy case #%d opened.", int(dID)), nil
}

// ---------------------------------------------------------------------------
// Inventory integration + PO rollup
// ---------------------------------------------------------------------------

// postIncomingStock builds an incoming stock.picking for the accepted quantities
// and posts it through the authoritative Inventory ledger (confirm -> transfer).
func postIncomingStock(ctx context.Context, rec map[string]interface{}, lines []map[string]interface{}, destLoc int) (int, error) {
	pickName := fmt.Sprintf("RCV-%d-IN", intID(rec["id"]))
	pickID, err := createRec(ctx, pickingModel, map[string]interface{}{
		"name":           pickName,
		"operation":      "incoming",
		"destination_id": destLoc,
		"origin":         fmt.Sprintf("Receipt %s", str(rec["name"])),
		"state":          "draft",
	})
	if err != nil {
		return 0, err
	}
	for _, ln := range lines {
		if _, err := createRec(ctx, moveModel, map[string]interface{}{
			"name":           fmt.Sprintf("%s / %s", str(rec["name"]), strOr(ln["description"], "line")),
			"product_id":     intID(ln["product_id"]),
			"picking_id":     int(pickID),
			"destination_id": destLoc,
			"quantity":       round3(flt(ln["accepted_qty"])),
			"state":          "draft",
		}); err != nil {
			return 0, err
		}
	}
	if _, err := orm.RunObjectAction(ctx, pickingModel, int(pickID), "action_confirm", nil); err != nil {
		return 0, fmt.Errorf("inventory confirm failed: %w", err)
	}
	if _, err := orm.RunObjectAction(ctx, pickingModel, int(pickID), "action_transfer", nil); err != nil {
		return 0, fmt.Errorf("inventory posting failed: %w", err)
	}
	return int(pickID), nil
}

// rollUpToPOLine adds the accepted quantity to the anchor PO line's received
// total (idempotent per receipt line is guarded by the receipt lifecycle).
func rollUpToPOLine(ctx context.Context, line map[string]interface{}) {
	poLineID := intID(line["order_line_id"])
	if poLineID <= 0 {
		return
	}
	pl := loadRec(ctx, poLineModel, poLineID)
	if pl == nil {
		return
	}
	_ = orm.UpdateRecordByID(ctx, poLineModel, poLineID, map[string]interface{}{
		"qty_received": round3(flt(pl["qty_received"]) + flt(line["accepted_qty"])),
	})
}

func discrepancyCheck(ctx context.Context, rec map[string]interface{}, line map[string]interface{}) string {
	ordered := flt(line["ordered_qty"])
	delivered := flt(line["delivered_qty"])
	if ordered <= 0 {
		return ""
	}
	breach, dir := toleranceCheck(ordered, delivered, qtyAbsTol, qtyPctTol)
	if !breach {
		return ""
	}
	desc := fmt.Sprintf("line %s %s-delivered (%.3f vs %.3f ordered)", strOr(line["description"], "?"), dir, delivered, ordered)
	if dir == "over" {
		_ = openDiscrepancy(ctx, rec, line, DiscExcess, delivered-ordered)
	} else {
		_ = openDiscrepancy(ctx, rec, line, DiscShort, ordered-delivered)
	}
	return desc
}

func openDiscrepancy(ctx context.Context, rec map[string]interface{}, line map[string]interface{}, issue DiscrepancyType, qty float64) error {
	dm, ok := orm.Registry[discModel]
	if !ok {
		return fmt.Errorf("discrepancy model unavailable")
	}
	_, err := orm.Create(ctx, dm, map[string]interface{}{
		"name":            nextRef(ctx, "DISC"),
		"receipt_id":      intID(rec["id"]),
		"receipt_line_id": intID(line["id"]),
		"order_id":        intID(rec["order_id"]),
		"product_id":      intID(line["product_id"]),
		"issue_type":      string(issue),
		"quantity":        round3(qty),
		"state":           string(DiscOpen),
		"history":         fmt.Sprintf("Auto-opened on validation %s.", time.Now().Format(dtLayout)),
		"active":          true,
	})
	return err
}

func anyPartial(ctx context.Context, lines []map[string]interface{}) bool {
	for _, ln := range lines {
		if flt(ln["accepted_qty"]) > 0 && (flt(ln["rejected_qty"]) > 0 || flt(ln["damaged_qty"]) > 0) {
			return true
		}
		if flt(ln["delivered_qty"]) < flt(ln["ordered_qty"]) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Supplier return
// ---------------------------------------------------------------------------

func retAuthorize(ctx context.Context, _ string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	r := loadRec(bypass, retModel, id)
	if r == nil {
		return "", fmt.Errorf("return not found")
	}
	if ReturnState(str(r["state"])) != RetDraft {
		return "", fmt.Errorf("only a draft return can be authorized")
	}
	if cnt, _ := orm.SearchCount(bypass, retLineMdl, [][]interface{}{{"return_id", "=", id}}); cnt == 0 {
		return "", fmt.Errorf("return has no lines")
	}
	upd := map[string]interface{}{"state": string(RetAuthorized), "claim_status": "authorized"}
	if auth := strings.TrimSpace(vals["return_auth_ref"]); auth != "" {
		upd["return_auth_ref"] = auth
	}
	_ = orm.UpdateRecordByID(bypass, retModel, id, upd)
	return "Return authorized by supplier.", nil
}

// retShipReturn builds an outgoing stock.picking (Inventory-owned) to ship the
// goods back, then posts it. The credit note stays a reference for Accounting.
func retShipReturn(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	r := loadRec(bypass, retModel, id)
	if r == nil {
		return "", fmt.Errorf("return not found")
	}
	if ReturnState(str(r["state"])) != RetAuthorized {
		return "", fmt.Errorf("return must be authorized before shipping")
	}
	lines, _ := orm.SearchPage(bypass, retLineMdl, [][]interface{}{{"return_id", "=", id}}, 500, 0, "id asc")
	if len(lines) == 0 {
		return "", fmt.Errorf("return has no lines")
	}
	// Ship from the warehouse that received the goods, if known.
	srcLoc := 0
	if rc := loadRec(bypass, receiptModel, intID(r["receipt_id"])); rc != nil {
		srcLoc = resolveWarehouseLocation(bypass, intID(rc["warehouse_id"]))
	}
	pickName := fmt.Sprintf("RTS-%d-OUT", id)
	vals := map[string]interface{}{
		"name":      pickName,
		"operation": "outgoing",
		"origin":    fmt.Sprintf("Return %s", str(r["name"])),
		"state":     "draft",
	}
	if srcLoc > 0 {
		vals["source_location_id"] = srcLoc
	}
	pickID, err := createRec(bypass, pickingModel, vals)
	if err != nil {
		return "", err
	}
	for _, ln := range lines {
		if intID(ln["product_id"]) <= 0 || flt(ln["quantity"]) <= 0 {
			continue
		}
		mv := map[string]interface{}{
			"name":       fmt.Sprintf("%s / %s", str(r["name"]), strOr(ln["description"], "line")),
			"product_id": intID(ln["product_id"]),
			"picking_id": int(pickID),
			"quantity":   round3(flt(ln["quantity"])),
			"state":      "draft",
		}
		if srcLoc > 0 {
			mv["source_location_id"] = srcLoc
		}
		if _, err := createRec(bypass, moveModel, mv); err != nil {
			return "", err
		}
	}
	if _, err := orm.RunObjectAction(bypass, pickingModel, int(pickID), "action_confirm", nil); err != nil {
		return "", fmt.Errorf("return confirm failed: %w", err)
	}
	if _, err := orm.RunObjectAction(bypass, pickingModel, int(pickID), "action_transfer", nil); err != nil {
		return "", fmt.Errorf("return posting failed: %w", err)
	}
	_ = orm.UpdateRecordByID(bypass, retModel, id, map[string]interface{}{
		"state": string(RetShipped), "picking_ref": int(pickID), "claim_status": "shipped",
	})
	return fmt.Sprintf("Return shipped via inventory transfer #%d.", int(pickID)), nil
}

// retRecordCredit links an Accounting credit note (created outside Purchase); it
// never authorizes or posts payment.
func retRecordCredit(ctx context.Context, _ string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	r := loadRec(bypass, retModel, id)
	if r == nil {
		return "", fmt.Errorf("return not found")
	}
	if ReturnState(str(r["state"])) != RetShipped {
		return "", fmt.Errorf("return must be shipped before a credit is recorded")
	}
	cnID := intID(vals["credit_note_id"])
	if cnID <= 0 {
		return "", fmt.Errorf("a credit note reference (account.move id) is required")
	}
	_ = orm.UpdateRecordByID(bypass, retModel, id, map[string]interface{}{
		"state": string(RetCredited), "credit_note_ref": cnID, "claim_status": "credited",
	})
	return "Credit note referenced on the return (Accounting owns it).", nil
}

func retClose(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	r := loadRec(bypass, retModel, id)
	if r == nil {
		return "", fmt.Errorf("return not found")
	}
	if ReturnState(str(r["state"])) != RetCredited {
		return "", fmt.Errorf("only a credited return can be closed")
	}
	_ = orm.UpdateRecordByID(bypass, retModel, id, map[string]interface{}{"state": string(RetClosed)})
	return "Return closed.", nil
}

// ---------------------------------------------------------------------------
// Discrepancy + claim
// ---------------------------------------------------------------------------

func discResolve(ctx context.Context, _ string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	d := loadRec(bypass, discModel, id)
	if d == nil {
		return "", fmt.Errorf("discrepancy not found")
	}
	if DiscrepancyState(str(d["state"])) != DiscOpen && DiscrepancyState(str(d["state"])) != DiscHold {
		return "", fmt.Errorf("only an open or held discrepancy can be resolved")
	}
	disp := strings.TrimSpace(vals["disposition"])
	if disp == "" {
		return "", fmt.Errorf("a disposition (accept/replace/credit/return) is required")
	}
	note := strings.TrimSpace(vals["resolution"])
	hist := appendNote(str(d["history"]), fmt.Sprintf("Resolved %s (disposition %s) by uid %d.", time.Now().Format(dtLayout), disp, orm.UIDFromContext(ctx)))
	_ = orm.UpdateRecordByID(bypass, discModel, id, map[string]interface{}{
		"state": string(DiscResolved), "disposition": disp, "resolution": note, "history": hist,
	})
	return fmt.Sprintf("Discrepancy resolved: %s.", disp), nil
}

func discClose(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	d := loadRec(bypass, discModel, id)
	if d == nil {
		return "", fmt.Errorf("discrepancy not found")
	}
	if DiscrepancyState(str(d["state"])) != DiscResolved {
		return "", fmt.Errorf("only a resolved discrepancy can be closed")
	}
	_ = orm.UpdateRecordByID(bypass, discModel, id, map[string]interface{}{"state": string(DiscClosed)})
	return "Discrepancy closed.", nil
}

func claimSubmit(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	c := loadRec(bypass, claimModel, id)
	if c == nil {
		return "", fmt.Errorf("claim not found")
	}
	if ClaimState(str(c["state"])) != ClaimOpen {
		return "", fmt.Errorf("only an open claim can be submitted")
	}
	_ = orm.UpdateRecordByID(bypass, claimModel, id, map[string]interface{}{"state": string(ClaimSubmitted)})
	return "Claim submitted to supplier.", nil
}

func claimSettle(ctx context.Context, _ string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	c := loadRec(bypass, claimModel, id)
	if c == nil {
		return "", fmt.Errorf("claim not found")
	}
	if ClaimState(str(c["state"])) != ClaimSubmitted && ClaimState(str(c["state"])) != ClaimAccepted {
		return "", fmt.Errorf("claim must be submitted/accepted before settling")
	}
	_ = orm.UpdateRecordByID(bypass, claimModel, id, map[string]interface{}{
		"state": string(ClaimSettled), "resolved_on": time.Now().Format(dateLayout),
		"resolution_note": strings.TrimSpace(vals["resolution"]),
	})
	return "Claim settled.", nil
}

// ---------------------------------------------------------------------------
// Small shared helpers
// ---------------------------------------------------------------------------

func appendNote(existing, add string) string {
	add = strings.TrimSpace(add)
	if add == "" {
		return existing
	}
	if strings.TrimSpace(existing) == "" {
		return add
	}
	return existing + "\n" + add
}

func appendDiscrepancyNote(ctx context.Context, rec map[string]interface{}, add string) error {
	nd := appendNote(str(rec["discrepancy_note"]), add)
	return orm.UpdateRecordByID(ctx, receiptModel, intID(rec["id"]), map[string]interface{}{"discrepancy_note": nd})
}
