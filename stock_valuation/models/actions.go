package models

import (
	"context"
	"fmt"
	"time"

	"nexa/core/orm"
)

// Valuation workflow actions. Posted layer facts are never rewritten:
// reversals post compensating layers (blueprint §10.2). Landed costs run
// draft → confirmed → posted; posting distributes each charge across the
// transfer's receipt lines and re-bases the matching remaining pools, or —
// when the stock is already gone — publishes a COGS correction event so the
// cost trail stays complete and auditable.

func init() {
	orm.RegisterObjectAction(layerModel, "action_reverse", layerReverse)
	orm.RegisterObjectAction(landedModel, "action_confirm", landedConfirm)
	orm.RegisterObjectAction(landedModel, "action_post", landedPost)
	orm.RegisterObjectAction(landedModel, "action_cancel", landedCancel)
}

// ---------------------------------------------------------------------------
// Layer reversal (compensating entry)
// ---------------------------------------------------------------------------

func layerReverse(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, layerModel, id)
	if rec == nil {
		return "", fmt.Errorf("valuation layer not found")
	}
	if b, _ := rec["reversed"].(bool); b {
		return "", fmt.Errorf("layer already reversed")
	}
	qty := flt(rec["quantity"])
	value := int64(intID(rec["value_cents"]))
	unit := int64(intID(rec["unit_value_cents"]))

	// Compensating layer: exact mirror of the posted fact. A reversal of a
	// receipt leaves no pool; a reversal of an issue re-injects one.
	newQty, newVal := -qty, -value
	remQ, remV := 0.0, int64(0)
	if newQty > qtyEps {
		remQ, remV = newQty, newVal
	}
	reverseName := fmt.Sprintf("VAL-X-%d", id)
	reverseID, err := createRec(bypass, layerModel, map[string]interface{}{
		"name":                  reverseName,
		"product_id":            intID(rec["product_id"]),
		"picking_id":            intID(rec["picking_id"]),
		"move_id":               intID(rec["move_id"]),
		"method":                str(rec["method"]),
		"source":                string(SrcReversal),
		"quantity":              newQty,
		"unit_value_cents":      unit,
		"value_cents":           newVal,
		"remaining_qty":         remQ,
		"remaining_value_cents": remV,
		"currency_code":         str(rec["currency_code"]),
		"date":                  time.Now().Format(dtLayout),
		"reversal_of_id":        id,
		"description":           fmt.Sprintf("Compensating reversal of layer %s.", str(rec["name"])),
	})
	if err != nil {
		return "", err
	}
	// The original keeps its posted facts; only its consumable pool closes.
	if _, err := orm.DB.ExecContext(bypass, `
		UPDATE stock_valuation_layer
		SET remaining_qty = 0, remaining_value_cents = 0, reversed = TRUE
		WHERE id = $1 AND reversed = FALSE`, id); err != nil {
		return "", err
	}
	if err := publishEvent(bypass, reverseName+":EVT", EvtReversal,
		intID(rec["product_id"]), reverseID, newVal,
		fmt.Sprintf("Reversal of layer %s posted as %s.", str(rec["name"]), reverseName)); err != nil {
		return "", err
	}
	return "Reversal layer posted; original pool closed.", nil
}

// ---------------------------------------------------------------------------
// Landed cost workflow
// ---------------------------------------------------------------------------

func landedConfirm(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, landedModel, id)
	if rec == nil {
		return "", fmt.Errorf("landed cost not found")
	}
	if str(rec["state"]) != string(LandedDraft) {
		return "", fmt.Errorf("only draft landed costs can be confirmed")
	}
	if len(landedLines(bypass, id)) == 0 {
		return "", fmt.Errorf("add at least one cost line first")
	}
	if err := orm.UpdateRecordByID(bypass, landedModel, id,
		map[string]interface{}{"state": string(LandedConfirmed)}); err != nil {
		return "", err
	}
	return "Landed cost confirmed.", nil
}

// landedPost allocates every charge across the transfer's receipt lines by
// the chosen basis (§10.4). Remainders are deterministic: floor-proportional
// with the residual on the last line.
func landedPost(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, landedModel, id)
	if rec == nil {
		return "", fmt.Errorf("landed cost not found")
	}
	if str(rec["state"]) != string(LandedConfirmed) {
		return "", fmt.Errorf("only confirmed landed costs can be posted")
	}
	lines := landedLines(bypass, id)
	if len(lines) == 0 {
		return "", fmt.Errorf("no cost lines to post")
	}
	pickID := intID(rec["picking_id"])
	receipts := receiptLinesFor(bypass, pickID)
	if len(receipts) == 0 {
		return "", fmt.Errorf("transfer has no posted receipt lines to cost")
	}
	weights := costWeights(bypass, str(rec["basis"]), receipts)

	total := int64(0)
	adjusted, corrected := 0, 0
	for _, cl := range lines {
		amount := int64(intID(cl["amount_cents"]))
		if amount == 0 {
			continue
		}
		total += amount
		allocation := allocateLandedCents(amount, weights)
		if allocation == nil {
			// Basis degenerate (e.g. zero weights): keep the money visible
			// as a COGS correction instead of dropping it silently.
			publishEvent(bypass, fmt.Sprintf("LC-%d-COGS-%d", id, intID(cl["id"])),
				EvtCOGSCorrection, 0, 0, amount,
				fmt.Sprintf("Landed cost %s line %s: basis weights degenerate, cost expensed.",
					str(rec["name"]), str(cl["name"])))
			corrected++
			continue
		}
		for i, cents := range allocation {
			if cents == 0 {
				continue
			}
			if applyToPool(bypass, receipts[i].lineID, cents) {
				adjusted++
				continue
			}
			publishEvent(bypass, fmt.Sprintf("LC-%d-COGS-%d-%d", id, intID(cl["id"]), i),
				EvtCOGSCorrection, receipts[i].prodID, 0, cents,
				fmt.Sprintf("Landed cost %s: receipt line %d pool exhausted, %d cents expensed to COGS.",
					str(rec["name"]), receipts[i].lineID, cents))
			corrected++
		}
	}

	// Audit trail: one value-only layer (no quantity) tied to the transfer.
	if _, err := createRec(bypass, layerModel, map[string]interface{}{
		"name":                  fmt.Sprintf("VAL-LC-%d", id),
		"product_id":            receipts[0].prodID,
		"picking_id":            pickID,
		"method":                string(MethodFIFO),
		"source":                string(SrcLandedCost),
		"quantity":              0,
		"unit_value_cents":      0,
		"value_cents":           total,
		"remaining_qty":         0,
		"remaining_value_cents": 0,
		"currency_code":         "USD",
		"date":                  time.Now().Format(dtLayout),
		"description": fmt.Sprintf("Landed cost %s: %d cents across %d receipt lines (%d re-based, %d expensed).",
			str(rec["name"]), total, len(receipts), adjusted, corrected),
	}); err != nil {
		return "", err
	}
	if err := orm.UpdateRecordByID(bypass, landedModel, id,
		map[string]interface{}{"state": string(LandedPosted)}); err != nil {
		return "", err
	}
	if err := publishEvent(bypass, fmt.Sprintf("LC-%d-EVT", id), EvtLandedCost,
		receipts[0].prodID, 0, total,
		fmt.Sprintf("Landed cost %s posted: %d cents.", str(rec["name"]), total)); err != nil {
		return "", err
	}
	return fmt.Sprintf("Posted %d cents; %d line allocations re-based pools.", total, adjusted), nil
}

func landedCancel(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, landedModel, id)
	if rec == nil {
		return "", fmt.Errorf("landed cost not found")
	}
	switch str(rec["state"]) {
	case string(LandedDraft), string(LandedConfirmed):
	default:
		return "", fmt.Errorf("posted or cancelled landed costs cannot be cancelled; reverse via a COGS correction")
	}
	if err := orm.UpdateRecordByID(bypass, landedModel, id,
		map[string]interface{}{"state": string(LandedCancelled)}); err != nil {
		return "", err
	}
	return "Landed cost cancelled.", nil
}

// ---------------------------------------------------------------------------
// Landed cost helpers
// ---------------------------------------------------------------------------

type receiptLine struct {
	lineID, pickID, prodID int
	qty                    float64
	valueCents             int64
}

// receiptLinesFor loads the transfer's posted incoming move lines together
// with the value of the receipt layer synced for each line (0 while the
// valuation cron has not run yet).
func receiptLinesFor(ctx context.Context, pickingID int) []receiptLine {
	if pickingID <= 0 {
		return nil
	}
	rows, err := orm.DB.QueryContext(ctx, `
		SELECT ml.id, ml.product_id, ml.quantity,
		       COALESCE((SELECT SUM(l.value_cents) FROM stock_valuation_layer l
		                 WHERE l.source_line = ml.id AND l.source = 'receipt'), 0)
		FROM stock_move_line ml
		JOIN stock_move m ON m.id = ml.move_id
		JOIN stock_picking p ON p.id = ml.picking_id
		WHERE ml.picking_id = $1 AND m.state = 'done' AND p.operation = 'incoming'
		ORDER BY ml.id`, pickingID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []receiptLine
	for rows.Next() {
		var r receiptLine
		if err := rows.Scan(&r.lineID, &r.prodID, &r.qty, &r.valueCents); err != nil {
			continue
		}
		r.pickID = pickingID
		out = append(out, r)
	}
	return out
}

// costWeights projects receipt lines onto the selected allocation basis.
func costWeights(ctx context.Context, basis string, receipts []receiptLine) []float64 {
	w := make([]float64, len(receipts))
	for i, r := range receipts {
		switch basis {
		case string(BasisWeight), string(BasisVolume):
			if prod := loadRec(ctx, "product.product", r.prodID); prod != nil {
				factor := flt(prod["weight"])
				if basis == string(BasisVolume) {
					factor = flt(prod["volume"])
				}
				w[i] = r.qty * factor
			}
		case string(BasisValue):
			w[i] = float64(r.valueCents)
		default: // quantity
			w[i] = r.qty
		}
	}
	return w
}

// applyToPool re-bases the remaining pool of the receipt layer synced for a
// ledger line. Returns false when the pool is gone (consumed or not synced
// yet) — the caller then raises a COGS correction.
func applyToPool(ctx context.Context, sourceLineID int, cents int64) bool {
	var layerID int
	err := orm.DB.QueryRowContext(ctx, `
		SELECT id FROM stock_valuation_layer
		WHERE source_line = $1 AND source = 'receipt' AND remaining_qty > 0
		ORDER BY id LIMIT 1`, sourceLineID).Scan(&layerID)
	if err != nil {
		return false
	}
	return addPoolValue(ctx, layerID, cents) == nil
}

func landedLines(ctx context.Context, costID int) []map[string]interface{} {
	rows, err := orm.SearchPage(ctx, landedLine, [][]interface{}{
		{"cost_id", "=", costID},
	}, 500, 0, "id asc")
	if err != nil {
		return nil
	}
	return rows
}

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
