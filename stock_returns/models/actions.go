package models

import (
	"context"
	"fmt"
	"time"

	"nexa/core/orm"
)

// Return workflow actions (§12). Confirm checks every line against the
// policy window measured from the delivery date; Receive executes the
// approved dispositions strictly through the anchor's validated transfers:
// restock and quarantine arrive as incoming moves, scrap then posts a
// stock.scrap from the quarantine bin, and repair/RTV/reject are recorded
// without stock movement (their outbound paths belong to other modules).

const dtLayout = "2006-01-02 15:04:05"

const (
	orderModel  = "stock.return.order"
	lineModel   = "stock.return.line"
	policyModel = "stock.return.policy"
)

func init() {
	orm.RegisterObjectAction(orderModel, "action_confirm", orderConfirm)
	orm.RegisterObjectAction(orderModel, "action_receive", orderReceive)
	orm.RegisterObjectAction(orderModel, "action_done", orderDone)
	orm.RegisterObjectAction(orderModel, "action_cancel", orderCancel)
	orm.RegisterObjectAction(lineModel, "action_inspect", lineInspect)
}

func orderConfirm(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	order := loadRec(bypass, orderModel, id)
	if order == nil {
		return "", fmt.Errorf("return order not found")
	}
	if str(order["state"]) != string(ReturnDraft) {
		return "", fmt.Errorf("only draft returns can be confirmed")
	}
	lines := orderLines(bypass, id)
	if len(lines) == 0 {
		return "", fmt.Errorf("return order has no lines")
	}

	deliveredAt := time.Time{}
	if pick := loadRec(bypass, "stock.picking", intID(order["origin_picking_id"])); pick != nil {
		deliveredAt = parseDT(str(pick["date_done"]))
		if deliveredAt.IsZero() {
			deliveredAt = parseDT(str(pick["create_date"]))
		}
	}
	eligibleCount := 0
	for _, ln := range lines {
		pol := policyFor(bypass, order, intID(ln["product_id"]))
		window := intFL(pol["window_days"])
		if window <= 0 {
			window = 30
		}
		note := "No delivery reference; manual review required."
		eligible := false
		if !deliveredAt.IsZero() {
			deadline := deliveredAt.AddDate(0, 0, window)
			eligible = time.Now().Before(deadline.AddDate(0, 0, 1))
			if eligible {
				note = fmt.Sprintf("Within %d-day policy window (delivered %s).",
					window, deliveredAt.Format("2006-01-02"))
			} else {
				note = fmt.Sprintf("Outside %d-day policy window (delivered %s).",
					window, deliveredAt.Format("2006-01-02"))
			}
		}
		upd := map[string]interface{}{"eligible": eligible, "policy_note": note}
		if !eligible && ReturnDisposition(str(ln["disposition"])) == DispRestock {
			upd["disposition"] = string(DispQuarantine)
			note += " Restock downgraded to quarantine."
			upd["policy_note"] = note
		}
		if err := orm.UpdateRecordByID(bypass, lineModel, intID(ln["id"]), upd); err != nil {
			return "", err
		}
		if eligible {
			eligibleCount++
		}
	}
	if err := orm.UpdateRecordByID(bypass, orderModel, id,
		map[string]interface{}{"state": string(ReturnConfirmed)}); err != nil {
		return "", err
	}
	return fmt.Sprintf("Return confirmed: %d/%d lines within policy.", eligibleCount, len(lines)), nil
}

// orderReceive executes the dispositions through the ledger.
func orderReceive(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	order := loadRec(bypass, orderModel, id)
	if order == nil {
		return "", fmt.Errorf("return order not found")
	}
	if str(order["state"]) != string(ReturnConfirmed) {
		return "", fmt.Errorf("only confirmed returns can be received")
	}
	lines := orderLines(bypass, id)

	restockLoc, quarantineLoc, err := returnDestinations(bypass, intID(order["warehouse_id"]))
	if err != nil {
		return "", err
	}

	// Inspection gate first so a blocked receive never posts partially.
	for _, ln := range lines {
		if ReturnDisposition(str(ln["disposition"])) != DispRestock {
			continue
		}
		pol := policyFor(bypass, order, intID(ln["product_id"]))
		if gate, _ := pol["inspect_gate"].(bool); gate && !boolOf(ln["inspected"]) {
			return "", fmt.Errorf("line %d: restock requires inspection sign-off first", intID(ln["id"]))
		}
	}

	posted, scrapped, held := 0, 0, 0
	// One incoming transfer per destination keeps the ledger tidy.
	for _, dest := range []struct {
		locID   int
		restock bool
	}{{restockLoc, true}, {quarantineLoc, false}} {
		if dest.locID == 0 {
			continue
		}
		var moveIDs []map[string]interface{}
		for _, ln := range lines {
			d := ReturnDisposition(str(ln["disposition"]))
			wantLine := (dest.restock && d == DispRestock) ||
				(!dest.restock && (d == DispQuarantine || d == DispScrap))
			if !wantLine || flt(ln["quantity"]) <= 0 {
				continue
			}
			moveIDs = append(moveIDs, ln)
		}
		if len(moveIDs) == 0 {
			continue
		}
		pickName := fmt.Sprintf("RET-%d-%s", id, map[bool]string{true: "STK", false: "QTN"}[dest.restock])
		pickID, err := createRec(bypass, "stock.picking", map[string]interface{}{
			"name":           pickName,
			"operation":      "incoming",
			"destination_id": dest.locID,
			"origin":         fmt.Sprintf("Return %s", str(order["name"])),
			"state":          "draft",
		})
		if err != nil {
			return "", err
		}
		for _, ln := range moveIDs {
			if _, err := createRec(bypass, "stock.move", map[string]interface{}{
				"name":           fmt.Sprintf("%s / %s", str(order["name"]), str(ln["id"])),
				"product_id":     intID(ln["product_id"]),
				"picking_id":     pickID,
				"destination_id": dest.locID,
				"quantity":       flt(ln["quantity"]),
				"lot_id":         nilIfZero(ln["lot_id"]),
				"state":          "draft",
			}); err != nil {
				return "", err
			}
			if dest.restock {
				posted++
			} else {
				held++
			}
		}
		if _, err := orm.RunObjectAction(bypass, "stock.picking", pickID, "action_confirm", nil); err != nil {
			return "", fmt.Errorf("return transfer %s: confirm failed: %w", pickName, err)
		}
		if _, err := orm.RunObjectAction(bypass, "stock.picking", pickID, "action_transfer", nil); err != nil {
			return "", fmt.Errorf("return transfer %s: posting failed: %w", pickName, err)
		}
	}

	// Scrap: consume the just-arrived quarantine stock through the anchor.
	for _, ln := range lines {
		if ReturnDisposition(str(ln["disposition"])) != DispScrap {
			continue
		}
		src := quarantineLoc
		if src == 0 {
			src = restockLoc
		}
		scrapID, err := createRec(bypass, "stock.scrap", map[string]interface{}{
			"name":        fmt.Sprintf("SCR-RMA-%d-%d", id, intID(ln["id"])),
			"product_id":  intID(ln["product_id"]),
			"lot_id":      nilIfZero(ln["lot_id"]),
			"quantity":    flt(ln["quantity"]),
			"location_id": src,
			"picking_id":  intID(order["origin_picking_id"]),
			"reason":      "return",
			"state":       "draft",
		})
		if err != nil {
			return "", err
		}
		if _, err := orm.RunObjectAction(bypass, "stock.scrap", scrapID, "action_validate", nil); err != nil {
			return "", fmt.Errorf("return scrap %d: %w", scrapID, err)
		}
		scrapped++
	}

	// Serial lifecycle: returned evidence units leave the delivered state.
	for _, ln := range lines {
		if sid := intID(ln["serial_id"]); sid > 0 {
			_, _ = orm.DB.ExecContext(bypass,
				`UPDATE stock_serial SET state = 'returned' WHERE id = $1 AND state IN ('delivered', 'available')`, sid)
		}
	}

	if err := orm.UpdateRecordByID(bypass, orderModel, id,
		map[string]interface{}{"state": string(ReturnReceived)}); err != nil {
		return "", err
	}
	return fmt.Sprintf("Received: %d unit-line(s) restocked, %d held (quarantine/scrap), %d scrapped via ledger moves.",
		posted, held, scrapped), nil
}

func orderDone(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	return orderSetState(ctx, id, string(ReturnDone), string(ReturnReceived))
}

func orderCancel(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	order := loadRec(bypass, orderModel, id)
	if order == nil {
		return "", fmt.Errorf("return order not found")
	}
	if str(order["state"]) == string(ReturnReceived) || str(order["state"]) == string(ReturnDone) {
		return "", fmt.Errorf("received stock cannot be cancelled; correct via the ledger instead")
	}
	return orderSetState(ctx, id, string(ReturnCancelled), string(ReturnDraft), string(ReturnConfirmed))
}

func orderSetState(ctx context.Context, id int, to string, from ...string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	order := loadRec(bypass, orderModel, id)
	if order == nil {
		return "", fmt.Errorf("return order not found")
	}
	ok := false
	for _, f := range from {
		if str(order["state"]) == f {
			ok = true
		}
	}
	if !ok {
		return "", fmt.Errorf("return is not in a state to move to %s", to)
	}
	if err := orm.UpdateRecordByID(bypass, orderModel, id,
		map[string]interface{}{"state": to}); err != nil {
		return "", err
	}
	return "Return order updated.", nil
}

// lineInspect stamps the separation-of-duties evidence: whoever signs off is
// recorded, and restock lines become receivable once inspected.
func lineInspect(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	ln := loadRec(bypass, lineModel, id)
	if ln == nil {
		return "", fmt.Errorf("return line not found")
	}
	order := loadRec(bypass, orderModel, intID(ln["order_id"]))
	if order == nil {
		return "", fmt.Errorf("return order not found")
	}
	st := ReturnState(str(order["state"]))
	if st != ReturnConfirmed && st != ReturnReceived {
		return "", fmt.Errorf("inspection is recorded against confirmed or received returns")
	}
	uid := orm.UIDFromContext(ctx)
	if err := orm.UpdateRecordByID(bypass, lineModel, id, map[string]interface{}{
		"inspected": true, "inspected_by": uid,
	}); err != nil {
		return "", err
	}
	return fmt.Sprintf("Line %d signed off by user %d.", id, uid), nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func returnDestinations(ctx context.Context, warehouseID int) (int, int, error) {
	if warehouseID <= 0 {
		return 0, 0, fmt.Errorf("return order has no warehouse")
	}
	wh := loadRec(ctx, "stock.warehouse", warehouseID)
	if wh == nil {
		return 0, 0, fmt.Errorf("warehouse not found")
	}
	restock := intID(wh["input_location_id"])
	if restock == 0 {
		restock = intID(wh["return_location_id"])
	}
	if restock == 0 {
		return 0, 0, fmt.Errorf("warehouse has no receipt location; confirm its locations first")
	}
	// Held stock lands in the warehouse quarantine bin when one exists;
	// otherwise a matching internal bin keeps the disposition separated.
	var quarantine int
	if err := orm.DB.QueryRowContext(ctx, `
		SELECT COALESCE(
			(SELECT id FROM stock_location
			 WHERE warehouse_id = $1 AND usage = 'quarantine' AND active LIMIT 1),
			(SELECT id FROM stock_location
			 WHERE warehouse_id = $1 AND usage = 'internal' AND active
			   AND name ILIKE '%quarantine%' LIMIT 1), 0)`,
		warehouseID).Scan(&quarantine); err != nil {
		quarantine = 0
	}
	return restock, quarantine, nil
}

// policyFor resolves the product policy, then the order-level or global
// fallback, then a synthesized default (30-day window with the gate on).
func policyFor(ctx context.Context, order map[string]interface{}, productID int) map[string]interface{} {
	if pid := intID(order["policy_id"]); pid > 0 {
		if pol := loadRec(ctx, policyModel, pid); pol != nil {
			return pol
		}
	}
	rows, err := orm.SearchPage(ctx, policyModel, [][]interface{}{
		{"active", "=", true},
	}, 10, 0, "id asc")
	if err == nil {
		var fallback map[string]interface{}
		for _, r := range rows {
			if intID(r["product_id"]) == productID {
				return r
			}
			if intID(r["product_id"]) == 0 && fallback == nil {
				fallback = r
			}
		}
		if fallback != nil {
			return fallback
		}
	}
	return map[string]interface{}{"window_days": 30, "inspect_gate": true}
}

func orderLines(ctx context.Context, orderID int) []map[string]interface{} {
	rows, err := orm.SearchPage(ctx, lineModel, [][]interface{}{
		{"order_id", "=", orderID},
	}, 500, 0, "id asc")
	if err != nil {
		return nil
	}
	return rows
}

func nilIfZero(v interface{}) interface{} {
	if intID(v) == 0 {
		return nil
	}
	return intID(v)
}

func boolOf(v interface{}) bool {
	b, _ := v.(bool)
	return b
}

func intFL(v interface{}) int {
	return intID(v)
}

func parseDT(s string) time.Time {
	for _, layout := range []string{dtLayout, time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	if len(s) >= 10 {
		if t, err := time.Parse("2006-01-02", s[:10]); err == nil {
			return t
		}
	}
	return time.Time{}
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

func createRec(ctx context.Context, model string, vals map[string]interface{}) (int, error) {
	inst, ok := orm.Registry[model]
	if !ok {
		return 0, fmt.Errorf("model %s unavailable", model)
	}
	return orm.Create(ctx, inst, vals)
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
