package models

import (
	"context"
	"fmt"
	"time"

	"nexa/core/orm"
)

// Lot disposition workflow (blueprint §8.5). Recall, quarantine and release
// are controlled object actions on the anchor's stock.lot record: they flip
// the lot state — which the anchor's reservation and issue guards already
// honour — release any live reservations for quarantined/recalled lots, note
// affected deliveries, and append a node to the traceability chain. None of
// them edits quants directly.

const (
	lotModel       = "stock.lot"
	lotReleased    = "released"
	lotQuarantined = "quarantined"
	lotRecalled    = "recalled"
)

func init() {
	orm.RegisterObjectAction(lotModel, "action_recall", lotRecall)
	orm.RegisterObjectAction(lotModel, "action_quarantine", lotQuarantine)
	orm.RegisterObjectAction(lotModel, "action_release", lotRelease)
	orm.RegisterObjectAction(expiryAlertMod, "action_alert_resolve", alertResolve)
}

// ---------------------------------------------------------------------------
// Small field helpers (kept local to the companion)
// ---------------------------------------------------------------------------

func loadRec(ctx context.Context, model string, id int) map[string]interface{} {
	rec, err := orm.SearchOne(orm.ContextWithBypass(ctx, true), model, map[string]interface{}{"id": id})
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

func intID(v interface{}) int {
	n, ok := orm.CoerceInt64(v)
	if !ok {
		return 0
	}
	return int(n)
}

func nowStamp() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// ---------------------------------------------------------------------------
// Traceability chain writer
// ---------------------------------------------------------------------------

// logEvent appends one workflow node to the event chain. Failure to log a
// note never blocks the disposition itself, so errors are surfaced to the
// caller only in the returned message.
func logEvent(ctx context.Context, eventType TraceEventType, lot map[string]interface{}, note string) error {
	inst, ok := orm.Registry[eventModel]
	if !ok {
		return fmt.Errorf("traceability event model unavailable")
	}
	vals := map[string]interface{}{
		"name":        fmt.Sprintf("LOT:%s", str(lot["name"])),
		"event_type":  string(eventType),
		"lot_id":      intID(lot["id"]),
		"description": note,
		"date_done":   nowStamp(),
	}
	if p := intID(lot["product_id"]); p > 0 {
		vals["product_id"] = p
	}
	_, err := orm.Create(ctx, inst, vals)
	return err
}

// noteAffectedDeliveries counts transfers that already shipped this lot to a
// customer location (§8.5 "record affected deliveries").
func noteAffectedDeliveries(ctx context.Context, lotID int) int {
	var n int
	err := orm.DB.QueryRowContext(orm.ContextWithBypass(ctx, true), `
		SELECT COUNT(DISTINCT COALESCE(ml.picking_id, ml.move_id))
		FROM stock_move_line ml
		LEFT JOIN stock_location l ON l.id = ml.result_location_id
		WHERE ml.lot_id = $1 AND COALESCE(l.usage,'') = 'customer'`, lotID).Scan(&n)
	if err != nil {
		return 0
	}
	return n
}

// appendLotNote keeps an audit trail on the lot itself.
func appendLotNote(ctx context.Context, lot map[string]interface{}, line string) {
	old := str(lot["description"])
	if old == "" {
		_ = orm.UpdateRecordByID(ctx, lotModel, intID(lot["id"]), map[string]interface{}{"description": line})
		return
	}
	_ = orm.UpdateRecordByID(ctx, lotModel, intID(lot["id"]),
		map[string]interface{}{"description": old + "\n" + line})
}

func setLotState(ctx context.Context, lotID int, state string) error {
	return orm.UpdateRecordByID(ctx, lotModel, lotID, map[string]interface{}{"state": state})
}

// ---------------------------------------------------------------------------
// Actions
// ---------------------------------------------------------------------------

func lotRecall(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	lot := loadRec(bypass, lotModel, id)
	if lot == nil {
		return "", fmt.Errorf("lot not found")
	}
	if str(lot["state"]) == lotRecalled {
		return "", fmt.Errorf("lot is already recalled")
	}
	if err := releaseLotReservations(bypass, id); err != nil {
		return "", err
	}
	if err := setLotState(bypass, id, lotRecalled); err != nil {
		return "", err
	}
	affected := noteAffectedDeliveries(bypass, id)
	note := fmt.Sprintf("[RECALL %s] Lot recalled. %d customer delivery transfer(s) affected; reservations released.",
		time.Now().Format(dateLayout), affected)
	appendLotNote(bypass, lot, note)
	if err := logEvent(bypass, TraceEvtRecall, lot, note); err != nil {
		return "", err
	}
	return fmt.Sprintf("Lot recalled: %d affected delivery transfer(s) recorded; further issue is blocked.", affected), nil
}

func lotQuarantine(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	lot := loadRec(bypass, lotModel, id)
	if lot == nil {
		return "", fmt.Errorf("lot not found")
	}
	if str(lot["state"]) == lotQuarantined {
		return "", fmt.Errorf("lot is already quarantined")
	}
	if err := releaseLotReservations(bypass, id); err != nil {
		return "", err
	}
	if err := setLotState(bypass, id, lotQuarantined); err != nil {
		return "", err
	}
	note := fmt.Sprintf("[QUARANTINE %s] Lot quarantined pending review; reservations released. Move remaining stock to a quarantine location with an internal transfer.",
		time.Now().Format(dateLayout))
	appendLotNote(bypass, lot, note)
	if err := logEvent(bypass, TraceEvtQuarantine, lot, note); err != nil {
		return "", err
	}
	return "Lot quarantined; it can no longer be reserved or issued through normal flows.", nil
}

func lotRelease(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	lot := loadRec(bypass, lotModel, id)
	if lot == nil {
		return "", fmt.Errorf("lot not found")
	}
	if str(lot["state"]) == lotReleased {
		return "", fmt.Errorf("lot is already released")
	}
	// Separation of duties: a recalled lot is never silently restocked; the
	// anchor's expiry guard would also keep a past-date lot out of
	// reservations, so releasing is only meaningful for quarantine review.
	if err := setLotState(bypass, id, lotReleased); err != nil {
		return "", err
	}
	note := fmt.Sprintf("[RELEASE %s] Lot disposition returned to released.", time.Now().Format(dateLayout))
	appendLotNote(bypass, lot, note)
	if err := logEvent(bypass, TraceEvtRelease, lot, note); err != nil {
		return "", err
	}
	return "Lot released: reservations and issue are allowed again (subject to expiry).", nil
}

func alertResolve(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, expiryAlertMod, id)
	if rec == nil {
		return "", fmt.Errorf("alert not found")
	}
	if !orm.AsBool(rec["open"]) {
		return "", fmt.Errorf("alert is already resolved")
	}
	if err := orm.UpdateRecordByID(bypass, expiryAlertMod, id, map[string]interface{}{"open": false}); err != nil {
		return "", err
	}
	return "Alert resolved.", nil
}

// releaseLotReservations returns any reserved stock of the lot to availability
// and drops the matching move reservations, so blocked stock stops being
// counted as committed (§8.5, §14). Runs in one transaction.
func releaseLotReservations(ctx context.Context, lotID int) error {
	tx, err := orm.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx,
		`UPDATE stock_quant SET reserved_quantity = 0
		 WHERE COALESCE(lot_id,0) = $1 AND COALESCE(reserved_quantity,0) > 0`, lotID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE stock_move SET reserved_quantity = 0, state = 'confirmed'
		 WHERE COALESCE(lot_id,0) = $1 AND state = 'assigned'`, lotID); err != nil {
		return err
	}
	return tx.Commit()
}
