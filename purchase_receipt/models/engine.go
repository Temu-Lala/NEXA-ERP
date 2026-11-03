package models

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nexa/core/orm"
	"nexa/core/scheduler"
)

// Delivery / receipt engines (blueprint §11). Purchase coordinates receiving and
// reconciles quantities; it never edits stock directly. Physical postings go
// through the authoritative Inventory ledger (stock.picking) via object actions.
// All quantity maths is server-side on scale=3 decimals. Nothing here writes to
// Accounting; credit notes are only referenced.

const (
	schedModel   = "purchase.delivery.schedule"
	receiptModel = "purchase.receipt"
	rcptLineMdl  = "purchase.receipt.line"
	retModel     = "purchase.supplier.return"
	retLineMdl   = "purchase.supplier.return.line"
	discModel    = "purchase.discrepancy.case"
	claimModel   = "purchase.supplier.claim"
	poModel      = "purchase.order"
	poLineModel  = "purchase.order.line"
	pickingModel = "stock.picking"
	moveModel    = "stock.move"
	whModel      = "stock.warehouse"
	locModel     = "stock.location"

	receiptCron = "purchase.receipt.delivery.followup"

	dateLayout = "2006-01-02"
	dtLayout   = "2006-01-02 15:04:05"
)

func init() {
	scheduler.RegisterCronHandler(receiptCron, runDeliveryFollowUp)
}

// ---------------------------------------------------------------------------
// Pure quantity maths (unit tested)
// ---------------------------------------------------------------------------

// qtyReconcile classifies a receipt line by the arithmetic relationship between
// delivered, accepted, rejected and damaged quantities. Inventory only receives
// the accepted quantity, so any accepted+rejected+damaged that does not equal
// delivered is an accounting gap the receiver must resolve.
func qtyReconcile(ordered, delivered, accepted, rejected, damaged float64) string {
	if accepted < 0 || rejected < 0 || damaged < 0 {
		return "negative"
	}
	if delivered <= 0 {
		return "not_delivered"
	}
	if delivered < ordered {
		return "short"
	}
	if delivered > ordered {
		return "over"
	}
	if nearlyZero(accepted + rejected + damaged - delivered) {
		return "balanced"
	}
	return "accounting_gap"
}

// toleranceCheck compares a delivered quantity against an ordered quantity under
// both an absolute and a percentage tolerance. Over-delivery and under-delivery
// outside tolerance are reviewable signals, not hard blocks.
func toleranceCheck(ordered, delivered, absTol, pctTol float64) (breach bool, dir string) {
	diff := delivered - ordered
	if diff < 0 {
		dir = "under"
	} else {
		dir = "over"
	}
	if nearlyZero(diff) {
		return false, "equal"
	}
	abs := diff
	if abs < 0 {
		abs = -abs
	}
	limit := absTol
	if pctLimit := ordered * pctTol / 100.0; pctLimit > limit {
		limit = pctLimit
	}
	return abs > limit, dir
}

// classifyDiscrepancy maps an ordered/delivered pair to a discrepancy type.
func classifyDiscrepancy(ordered, delivered float64) DiscrepancyType {
	switch {
	case nearlyZero(ordered - delivered):
		return "" // no quantity discrepancy
	case delivered < ordered:
		return DiscShort
	default:
		return DiscExcess
	}
}

// poRemaining is the quantity still expected on a PO line after receipts.
func poRemaining(ordered, received float64) float64 {
	r := ordered - received
	if r < 0 {
		return 0
	}
	return r
}

func nearlyZero(v float64) bool {
	if v < 0 {
		v = -v
	}
	return v < 0.0005
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

func loadRec(ctx context.Context, model string, id int) map[string]interface{} {
	if id <= 0 {
		return nil
	}
	rec, err := orm.SearchOne(ctx, model, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
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

func str(v interface{}) string { return orm.AsString(v) }

func strOr(v interface{}, def string) string {
	if s := strings.TrimSpace(orm.AsString(v)); s != "" {
		return s
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
	default:
		n, _ := orm.CoerceInt64(v)
		return float64(n)
	}
}

func intID(v interface{}) int {
	n, _ := orm.CoerceInt64(v)
	return int(n)
}

func boolOf(v interface{}) bool {
	b, _ := v.(bool)
	return b
}

func round3(v float64) float64 {
	return float64(int64(v*1000+0.5)) / 1000
}

func nextRef(ctx context.Context, prefix string) string {
	if name, err := orm.NextSequence(ctx, prefix); err == nil && name != "" {
		return name
	}
	return fmt.Sprintf("%s/%05d", prefix, time.Now().Unix()%100000)
}

// resolveWarehouseLocation maps a receipt warehouse to its physical input
// location so the incoming stock.picking has an authoritative destination.
func resolveWarehouseLocation(ctx context.Context, whID int) int {
	if whID <= 0 {
		return 0
	}
	wh := loadRec(ctx, whModel, whID)
	if wh == nil {
		return 0
	}
	if loc := intID(wh["input_location_id"]); loc > 0 {
		return loc
	}
	rows, _ := orm.SearchPage(ctx, locModel, [][]interface{}{
		{"warehouse_id", "=", whID}, {"usage", "=", "internal"},
	}, 1, 0, "id asc")
	if len(rows) > 0 {
		return intID(rows[0]["id"])
	}
	return 0
}

// parseDay parses a date or datetime string into a day-truncated time.
func parseDay(v interface{}) (time.Time, bool) {
	s := strings.TrimSpace(str(v))
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{dateLayout, dtLayout, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Truncate(24 * time.Hour), true
		}
	}
	return time.Time{}, false
}

// ---------------------------------------------------------------------------
// Delivery follow-up cron (overdue + repeated-delay signals)
// ---------------------------------------------------------------------------

// runDeliveryFollowUp flags scheduled deliveries past their due date. Overdue
// schedules still in "planned" advance to "in_progress" with a follow-up note;
// this is idempotent (a schedule already in_progress is left untouched) so the
// periodic run never double-counts a delay.
func runDeliveryFollowUp(ctx context.Context, _ map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	today := time.Now().Truncate(24 * time.Hour)
	rows, err := orm.SearchPage(bypass, schedModel, [][]interface{}{
		{"state", "in", []interface{}{string(SchedPlanned), string(SchedInProgress)}},
		{"scheduled_date", "!=", ""},
	}, 2000, 0, "scheduled_date asc")
	if err != nil {
		return err
	}
	for _, r := range rows {
		due, ok := parseDay(r["scheduled_date"])
		if !ok || !due.Before(today) {
			continue
		}
		// Ensure the record carries a reference before writing state/notes.
		if str(r["name"]) == "" || str(r["name"]) == "New" {
			_ = orm.UpdateRecordByID(bypass, schedModel, intID(r["id"]), map[string]interface{}{
				"name": nextRef(bypass, "DEL"),
			})
		}
		if ScheduleState(str(r["state"])) == SchedPlanned {
			note := strings.TrimSpace(str(r["note"]))
			flag := fmt.Sprintf("Overdue since %s; follow-up required.", due.Format(dateLayout))
			if !strings.Contains(note, "Overdue since") {
				if note == "" {
					note = flag
				} else {
					note = note + "\n" + flag
				}
			}
			_ = orm.UpdateRecordByID(bypass, schedModel, intID(r["id"]), map[string]interface{}{
				"state": string(SchedInProgress), "note": note,
			})
		}
	}
	return nil
}
