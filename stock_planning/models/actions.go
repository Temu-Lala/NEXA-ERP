package models

import (
	"context"
	"fmt"

	"nexa/core/orm"
)

// Planning workflow actions. Proposals move draft → approved → converted;
// conversion creates a DRAFT purchase requisition (Procurement decides from
// there — the planner never commits to a supplier). Alerts are worked through
// open → acknowledged → resolved.

func init() {
	orm.RegisterObjectAction(propModel, "action_approve", proposalApprove)
	orm.RegisterObjectAction(propModel, "action_convert", proposalConvert)
	orm.RegisterObjectAction(propModel, "action_cancel", proposalCancel)
	orm.RegisterObjectAction(alertModel, "action_acknowledge", alertAcknowledge)
	orm.RegisterObjectAction(alertModel, "action_resolve", alertResolve)
}

func proposalApprove(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, propModel, id)
	if rec == nil {
		return "", fmt.Errorf("proposal not found")
	}
	if str(rec["state"]) != string(PropDraft) {
		return "", fmt.Errorf("only draft proposals can be approved")
	}
	if flt(rec["suggested_qty"]) <= 0 {
		return "", fmt.Errorf("proposal suggests nothing to order")
	}
	if err := orm.UpdateRecordByID(bypass, propModel, id,
		map[string]interface{}{"state": string(PropApproved)}); err != nil {
		return "", err
	}
	return "Proposal approved for conversion.", nil
}

// proposalConvert writes one draft requisition with a single line for the
// proposed product and quantity, priced from the product master.
func proposalConvert(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, propModel, id)
	if rec == nil {
		return "", fmt.Errorf("proposal not found")
	}
	if str(rec["state"]) != string(PropApproved) {
		return "", fmt.Errorf("only approved proposals can be converted")
	}
	if _, ok := orm.Registry["purchase.requisition"]; !ok {
		return "", fmt.Errorf("purchase_requisition is not installed; cannot convert")
	}
	prodID := intID(rec["product_id"])
	qty := flt(rec["suggested_qty"])
	var price float64
	var prodName string
	if prod := loadRec(bypass, "product.product", prodID); prod != nil {
		price = flt(prod["standard_price"])
		prodName = str(prod["name"])
	}
	if prodName == "" {
		prodName = fmt.Sprintf("Product %d", prodID)
	}
	whName := ""
	if wh := loadRec(bypass, "stock.warehouse", intID(rec["warehouse_id"])); wh != nil {
		whName = str(wh["name"])
	}

	reqName := fmt.Sprintf("REQ-PL-%d", id)
	reqID, err := createRec(bypass, "purchase.requisition", map[string]interface{}{
		"name":          reqName,
		"req_type":      "stock_replenishment",
		"priority":      priorityFromTrigger(str(rec["trigger"])),
		"justification": str(rec["explanation"]),
		"branch":        whName,
		"est_total":     qty * price,
		"state":         "draft",
	})
	if err != nil {
		return "", err
	}
	if _, err := createRec(bypass, "purchase.requisition.line", map[string]interface{}{
		"requisition_id": reqID,
		"name":           fmt.Sprintf("Replenishment: %s", prodName),
		"product_id":     prodID,
		"quantity":       qty,
		"est_price":      price,
		"est_subtotal":   qty * price,
	}); err != nil {
		return "", err
	}
	if err := orm.UpdateRecordByID(bypass, propModel, id, map[string]interface{}{
		"state":          string(PropConverted),
		"requisition_id": reqID,
	}); err != nil {
		return "", err
	}
	return fmt.Sprintf("Converted to draft requisition %s (%.3f × %s).", reqName, qty, prodName), nil
}

func proposalCancel(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, propModel, id)
	if rec == nil {
		return "", fmt.Errorf("proposal not found")
	}
	switch str(rec["state"]) {
	case string(PropDraft), string(PropApproved):
	default:
		return "", fmt.Errorf("only open proposals can be cancelled")
	}
	if err := orm.UpdateRecordByID(bypass, propModel, id,
		map[string]interface{}{"state": string(PropCancelled)}); err != nil {
		return "", err
	}
	return "Proposal cancelled; the dedup lane is free again.", nil
}

func alertAcknowledge(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	return alertSetState(ctx, id, string(AlertAcked), string(AlertOpen))
}

func alertResolve(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	return alertSetState(ctx, id, string(AlertResolved), string(AlertOpen), string(AlertAcked))
}

func alertSetState(ctx context.Context, id int, to string, from ...string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, alertModel, id)
	if rec == nil {
		return "", fmt.Errorf("alert not found")
	}
	ok := false
	for _, f := range from {
		if str(rec["state"]) == f {
			ok = true
		}
	}
	if !ok {
		return "", fmt.Errorf("alert is not in a state to move to %s", to)
	}
	if err := orm.UpdateRecordByID(bypass, alertModel, id,
		map[string]interface{}{"state": to}); err != nil {
		return "", err
	}
	return "Alert updated.", nil
}

// priorityFromTrigger escalates stockouts to urgent requisitions so the
// procurement queue sees the same urgency the planner computed.
func priorityFromTrigger(trigger string) string {
	if ProposalTrigger(trigger) == TrigStockout {
		return "urgent"
	}
	return "high"
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func loadRec(ctx context.Context, model string, id int) map[string]interface{} {
	if id <= 0 {
		return nil
	}
	rec, err := orm.SearchOne(ctx, model, map[string]interface{}{"id": id})
	if err != nil {
		return nil
	}
	return rec
}

func str(v interface{}) string {
	if v == nil {
		return ""
	}
	return orm.AsString(v)
}

func flt(v interface{}) float64 {
	f, ok := orm.CoerceFloat64(v)
	if !ok {
		return 0
	}
	return f
}

func intID(v interface{}) int {
	n, ok := orm.CoerceInt64(v)
	if !ok {
		return 0
	}
	return int(n)
}
