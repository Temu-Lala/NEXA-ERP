package models

import (
	"context"
	"fmt"
	"math"
	"time"

	"nexa/core/orm"
	"nexa/core/scheduler"
)

// Quality behaviour layer: pure sampling / verdict / separation-of-duties
// maths (unit tested, no DB) plus a scheduled proposal that opens a draft
// inspection for each received, plan-covered lot that has no check yet.

const (
	proposeCronCode = "stock.quality.check.propose"
	dateLayout      = "2006-01-02"

	planModel  = "stock.quality.plan"
	checkModel = "stock.quality.check"
	holdModel  = "stock.quality.hold"
)

func init() {
	scheduler.RegisterCronHandler(proposeCronCode, runProposeChecks)
}

// ---------------------------------------------------------------------------
// Pure maths
// ---------------------------------------------------------------------------

// sampleQty is the number of units to inspect for a received quantity at a
// given sampling percentage (ceil, capped at the received quantity). A zero
// or negative quantity yields zero.
func sampleQty(quantity, pct float64) float64 {
	if quantity <= 0 {
		return 0
	}
	if pct >= 100 {
		return quantity
	}
	if pct <= 0 {
		return 0
	}
	s := math.Ceil(quantity * pct / 100)
	if s > quantity {
		s = quantity
	}
	return s
}

// verdictFor decides pass / fail from defect and sampled quantities against a
// pass threshold expressed as a percentage of the sample that must be good.
// A zero sample can neither pass nor fail — it stays pending.
func verdictFor(sampleQty, defectQty, passThresholdPct float64) CheckResult {
	if sampleQty <= 0 {
		return ResultPending
	}
	goodPct := (sampleQty - defectQty) / sampleQty * 100
	if goodPct < 0 {
		goodPct = 0
	}
	if goodPct+1e-9 >= passThresholdPct {
		return ResultPass
	}
	// Anything below the threshold but not totally defective is conditional
	// (needs a disposition); a fully defective sample is a hard fail.
	if defectQty >= sampleQty {
		return ResultFail
	}
	return ResultCond
}

// canRelease enforces separation of duties: the person releasing a hold must
// be a real second person distinct from whoever opened it (§7.4).
func canRelease(openedByUID, releasedByUID int) bool {
	if releasedByUID <= 0 {
		return false
	}
	return openedByUID != releasedByUID
}

// ---------------------------------------------------------------------------
// Receipt proposal
// ---------------------------------------------------------------------------

// runProposeChecks opens a draft inspection for each posted incoming move
// line whose product is covered by an active plan and which does not already
// have a check for the same picking+product+lot.
func runProposeChecks(ctx context.Context, _ map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	plans := fetchAll(bypass, planModel, [][]interface{}{{"active", "=", true}})
	if len(plans) == 0 {
		return nil
	}
	inst, ok := orm.Registry[checkModel]
	if !ok {
		return fmt.Errorf("quality check model unavailable")
	}
	for _, pl := range plans {
		for _, line := range recentReceiptLines(bypass, pl) {
			existing, _ := orm.SearchCount(bypass, checkModel, [][]interface{}{
				{"picking_id", "=", line["picking_id"]},
				{"product_id", "=", line["product_id"]},
				{"lot_id", "=", line["lot_id"]},
			})
			if existing > 0 {
				continue
			}
			q := flt(line["quantity"])
			_, _ = orm.Create(bypass, inst, map[string]interface{}{
				"name":       fmt.Sprintf("QC-%s-%d", time.Now().Format(dateLayout), line["id"]),
				"plan_id":    intID(pl["id"]),
				"product_id": intID(line["product_id"]),
				"picking_id": intID(line["picking_id"]),
				"move_id":    intID(line["move_id"]),
				"lot_id":     intID(line["lot_id"]),
				"quantity":   q,
				"sample_qty": sampleQty(q, flt(pl["sample_percent"])),
				"result":     string(ResultPending),
				"state":      string(CheckDraft),
			})
			if orm.AsBool(pl["auto_quarantine"]) {
				if lot := intID(line["lot_id"]); lot > 0 {
					_ = orm.UpdateRecordByID(bypass, "stock.lot", lot,
						map[string]interface{}{"state": "quarantined"})
				}
			}
		}
	}
	return nil
}

// recentReceiptLines returns posted incoming move lines that the plan covers.
func recentReceiptLines(ctx context.Context, plan map[string]interface{}) []map[string]interface{} {
	scope := str(plan["scope"])
	sql := `
		SELECT ml.id, ml.move_id, ml.picking_id, ml.product_id, COALESCE(ml.lot_id,0), ml.quantity
		FROM stock_move_line ml
		JOIN stock_move m ON m.id = ml.move_id
		JOIN stock_picking p ON p.id = ml.picking_id
		JOIN product_product pr ON pr.id = ml.product_id
		WHERE p.operation = 'incoming' AND m.state = 'done'`
	args := []interface{}{}
	switch scope {
	case string(ScopeProduct):
		sql += ` AND ml.product_id = $1`
		args = append(args, intID(plan["product_id"]))
	case string(ScopeCategory):
		sql += ` AND pr.categ_id = $1`
		args = append(args, intID(plan["category_id"]))
	case string(ScopeSupplier):
		// The anchor picking carries no partner link yet; supplier-scoped
		// plans therefore cover all receipts until that field lands.
	}
	sql += ` ORDER BY ml.id DESC LIMIT 500`
	rows, err := orm.DB.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []map[string]interface{}
	for rows.Next() {
		var id, moveID, pickID, prodID, lotID int
		var qty float64
		if err := rows.Scan(&id, &moveID, &pickID, &prodID, &lotID, &qty); err != nil {
			continue
		}
		out = append(out, map[string]interface{}{
			"id": id, "move_id": moveID, "picking_id": pickID,
			"product_id": prodID, "lot_id": lotID, "quantity": qty,
		})
	}
	return out
}

// ---------------------------------------------------------------------------
// Local read/coercion helpers
// ---------------------------------------------------------------------------

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

func fetchAll(ctx context.Context, model string, domain [][]interface{}) []map[string]interface{} {
	var out []map[string]interface{}
	offset := 0
	const page = 500
	for {
		rows, err := orm.SearchPage(orm.ContextWithBypass(ctx, true), model, domain, page, offset, "id asc")
		if err != nil || len(rows) == 0 {
			break
		}
		out = append(out, rows...)
		if len(rows) < page {
			break
		}
		offset += page
	}
	return out
}
