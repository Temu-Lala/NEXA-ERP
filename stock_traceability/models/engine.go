package models

import (
	"context"
	"fmt"
	"time"

	"nexa/core/orm"
	"nexa/core/scheduler"
)

// Traceability behaviour layer: a watermark-style sync that materialises the
// event chain from posted ledger operations (idempotent, append-only), and an
// expiry monitor that raises one open alert per lot+severity bucket while a
// lot sits inside the warning window. Pure classification helpers are unit
// tested without a database.

const (
	traceSyncCode  = "stock.traceability.event.sync"
	expiryScanCode = "stock.lot.expiry.scan"

	expiryWarnDays = 30 // window in which the monitor starts raising alerts
	expiryCritDays = 7  // remaining days at which an alert becomes critical
	eventModel     = "stock.traceability.event"
	expiryAlertMod = "stock.lot.expiry.alert"
	dateLayout     = "2006-01-02"
)

func init() {
	scheduler.RegisterCronHandler(traceSyncCode, runTraceSync)
	scheduler.RegisterCronHandler(expiryScanCode, runExpiryScan)
}

// ---------------------------------------------------------------------------
// Pure helpers (unit tested)
// ---------------------------------------------------------------------------

// eventCategory classifies a posted operation by the usage of its endpoints
// (blueprint §8.1): supplier→receipt, →customer delivery, customer→return,
// →scrap/damaged scrap, anything else internal.
func eventCategory(srcUsage, dstUsage string) TraceEventType {
	switch {
	case srcUsage == "supplier":
		return TraceEvtReceipt
	case dstUsage == "customer":
		return TraceEvtDelivery
	case srcUsage == "customer":
		return TraceEvtReturn
	case dstUsage == "scrap" || dstUsage == "damaged":
		return TraceEvtScrap
	default:
		return TraceEvtInternal
	}
}

// expirySeverity buckets remaining days into an alert level. Negative days
// mean the lot is already past its expiration date.
func expirySeverity(days int) ExpirySeverity {
	switch {
	case days < 0:
		return ExpiryExpired
	case days <= expiryCritDays:
		return ExpiryCritical
	default:
		return ExpiryWarning
	}
}

// daysUntil measures whole days from today to the given date; negative when
// the date has passed.
func daysUntil(target, today time.Time) int {
	a := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	b := time.Date(target.Year(), target.Month(), target.Day(), 0, 0, 0, 0, time.UTC)
	return int(b.Sub(a).Hours() / 24)
}

// ---------------------------------------------------------------------------
// Event chain sync
// ---------------------------------------------------------------------------

// traceSyncSQL inserts one event per posted move line that has no event yet.
// NOT EXISTS on the origin operation id keeps repeated runs idempotent, and
// the chain stays append-only — events are never rewritten.
const traceSyncSQL = `
INSERT INTO stock_traceability_event
	(name, event_type, product_id, lot_id, serial_id, picking_id,
	 source_location_id, result_location_id, quantity, date_done, source_line, description)
SELECT 'ML-' || ml.id,
	CASE
		WHEN COALESCE(sl.usage,'') = 'supplier' THEN 'receipt'
		WHEN COALESCE(dl.usage,'') = 'customer' THEN 'delivery'
		WHEN COALESCE(sl.usage,'') = 'customer' THEN 'return'
		WHEN COALESCE(dl.usage,'') IN ('scrap','damaged') THEN 'scrap'
		ELSE 'internal'
	END,
	ml.product_id, ml.lot_id, ml.serial_id, ml.picking_id,
	ml.source_location_id, ml.result_location_id, ml.quantity,
	COALESCE(m.date_done, now()), ml.id,
	CASE WHEN COALESCE(p.name,'') = '' THEN 'Derived from posted operation'
	     ELSE 'Derived from posted operation of transfer ' || p.name END
FROM stock_move_line ml
JOIN stock_move m ON m.id = ml.move_id
LEFT JOIN stock_picking p ON p.id = ml.picking_id
LEFT JOIN stock_location sl ON sl.id = ml.source_location_id
LEFT JOIN stock_location dl ON dl.id = ml.result_location_id
WHERE NOT EXISTS (SELECT 1 FROM stock_traceability_event e WHERE e.source_line = ml.id)`

func runTraceSync(ctx context.Context, _ map[string]interface{}) error {
	_, err := orm.DB.ExecContext(orm.ContextWithBypass(ctx, true), traceSyncSQL)
	return err
}

// ---------------------------------------------------------------------------
// Expiry monitor
// ---------------------------------------------------------------------------

type expiryLotRow struct {
	lotID     int
	productID int
	lotName   string
	expiry    string // YYYY-MM-DD
}

// expiryCandidateSQL lists released lots expiring inside the warning window.
// Quarantined/recalled lots are handled by their workflows, not re-alerted.
const expiryCandidateSQL = `
SELECT id, COALESCE(product_id,0), COALESCE(name,''), expiration_on::text
FROM stock_lot
WHERE COALESCE(state,'released') = 'released'
  AND expiration_on IS NOT NULL
  AND expiration_on <= CURRENT_DATE + $1::int
ORDER BY expiration_on, id`

func runExpiryScan(ctx context.Context, _ map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	rows, err := orm.DB.QueryContext(bypass, expiryCandidateSQL, expiryWarnDays)
	if err != nil {
		return nil
	}
	var candidates []expiryLotRow
	for rows.Next() {
		var r expiryLotRow
		if err := rows.Scan(&r.lotID, &r.productID, &r.lotName, &r.expiry); err == nil {
			candidates = append(candidates, r)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil
	}

	today := time.Now().UTC()
	inst, ok := orm.Registry[expiryAlertMod]
	if !ok {
		return fmt.Errorf("expiry alert model unavailable")
	}
	for _, c := range candidates {
		exp, err := time.Parse(dateLayout, first10(c.expiry))
		if err != nil {
			continue
		}
		days := daysUntil(exp, today)
		sev := expirySeverity(days)
		// One open alert per lot+bucket keeps runs idempotent while still
		// letting a warning escalate to critical/expired.
		open, _ := orm.SearchCount(bypass, expiryAlertMod, [][]interface{}{
			{"lot_id", "=", c.lotID}, {"severity", "=", string(sev)}, {"open", "=", true},
		})
		if open > 0 {
			continue
		}
		word := "expires"
		hint := "Monitor the lot; FEFO will consume it first."
		switch sev {
		case ExpiryExpired:
			word = "expired on"
			hint = "Proposed action: quarantine the remaining stock of this lot and block future reservations."
		case ExpiryCritical:
			hint = "Consume within the week or plan a quarantine."
		}
		msg := fmt.Sprintf("Lot %s %s %s. %d day(s) remaining. %s",
			orDash(c.lotName), word, first10(c.expiry), days, hint)
		vals := map[string]interface{}{
			"lot_id":         c.lotID,
			"expiration_on":  first10(c.expiry),
			"days_remaining": days,
			"severity":       string(sev),
			"open":           true,
			"message":        msg,
		}
		if c.productID > 0 {
			vals["product_id"] = c.productID
		}
		_, _ = orm.Create(bypass, inst, vals)
	}
	return nil
}

func first10(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func orDash(s string) string {
	if s == "" {
		return "(unnamed)"
	}
	return s
}
