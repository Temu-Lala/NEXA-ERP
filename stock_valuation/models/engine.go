package models

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"nexa/core/orm"
	"nexa/core/scheduler"
)

// Valuation behaviour layer. Pure cost maths (unit tested, no DB) plus the
// ledger-sync cron that turns posted move lines into valuation layers and
// consumes the remaining pools. Money is integer cents end to end; posted
// layer facts are immutable — only remaining_* pools shrink.

const (
	syncCronCode = "stock.valuation.sync"
	dtLayout     = "2006-01-02 15:04:05"

	layerModel  = "stock.valuation.layer"
	eventModel  = "stock.valuation.event"
	landedModel = "stock.landed.cost"
	landedLine  = "stock.landed.cost.line"
	qtyEps      = 1e-6
)

func init() {
	scheduler.RegisterCronHandler(syncCronCode, runValuationSync)
}

// ---------------------------------------------------------------------------
// Pure cost maths
// ---------------------------------------------------------------------------

// LayerSnap is one candidate pool slice for consumption.
type LayerSnap struct {
	ID          int
	RemQty      float64
	RemValCents int64
	Seq         int // creation order (id works as proxy)
}

// LayerSlice is a consumption allocation against one layer.
type LayerSlice struct {
	LayerID int
	Qty     float64
	Cents   int64
}

// fifoConsume allocates demand oldest-first across remaining pools. The cost
// of a full-layer slice is that layer's exact remaining value (no rounding
// drift); partial slices cost remVal * take / remQ rounded. Returns the
// allocation and any unfulfilled quantity.
func fifoConsume(snaps []LayerSnap, qty float64) (slices []LayerSlice, short float64) {
	if qty <= qtyEps {
		return nil, 0
	}
	ordered := append([]LayerSnap(nil), snaps...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Seq < ordered[j].Seq })
	remaining := qty
	for _, s := range ordered {
		if remaining <= qtyEps {
			break
		}
		if s.RemQty <= qtyEps {
			continue
		}
		take := s.RemQty
		if take > remaining {
			take = remaining
		}
		cents := s.RemValCents
		if take < s.RemQty-qtyEps {
			cents = int64(math.Round(float64(s.RemValCents) * take / s.RemQty))
		}
		slices = append(slices, LayerSlice{LayerID: s.ID, Qty: take, Cents: cents})
		remaining -= take
	}
	return slices, math.Max(remaining, 0)
}

// avgConsume prices demand at the blended pool average (Σremaining value /
// Σremaining quantity) and spreads the quantity FIFO-ordered so pools stay
// internally consistent. Shortfall mirrors the available pool.
func avgConsume(snaps []LayerSnap, qty float64) (slices []LayerSlice, short float64) {
	var tq float64
	var tv int64
	for _, s := range snaps {
		if s.RemQty > qtyEps {
			tq += s.RemQty
			tv += s.RemValCents
		}
	}
	slices, short = fifoConsume(snaps, qty)
	if tq <= qtyEps || len(slices) == 0 {
		return slices, short
	}
	total := int64(math.Round(float64(tv) * (qty - short) / tq))
	// Re-price each slice at the blended average, deterministic residual to
	// the last slice so Σ slice == pool-average cost exactly.
	var acc int64
	for i := range slices {
		if i == len(slices)-1 {
			slices[i].Cents = total - acc
			break
		}
		slices[i].Cents = int64(math.Round(float64(tv) * slices[i].Qty / tq))
		acc += slices[i].Cents
	}
	return slices, short
}

// avgUnitCents recomputes the perpetual moving average unit cost after a
// receipt blends into the existing pool.
func avgUnitCents(prevQty float64, prevValCents int64, inQty float64, inValCents int64) int64 {
	q := prevQty + inQty
	if q <= qtyEps {
		return 0
	}
	return int64(math.Round(float64(prevValCents+inValCents) / q))
}

// allocateLandedCents distributes a total across weight-proportional lines.
// Floor semantics with the rounding remainder on the LAST line, so the sum
// always equals the total and the result is reproducible (§10.4).
func allocateLandedCents(total int64, weights []float64) []int64 {
	var sum float64
	for _, w := range weights {
		if w > 0 {
			sum += w
		}
	}
	if len(weights) == 0 || sum <= 0 {
		return nil
	}
	out := make([]int64, len(weights))
	var acc int64
	for i, w := range weights {
		if i == len(weights)-1 {
			out[i] = total - acc
			continue
		}
		if w > 0 {
			out[i] = int64(float64(total) * w / sum)
			acc += out[i]
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Ledger sync
// ---------------------------------------------------------------------------

type valLine struct {
	lineID, moveID, pickID, prodID int
	qty                            float64
	date                           string
}

func runValuationSync(ctx context.Context, _ map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	for _, l := range postedLines(bypass, "incoming") {
		if err := postReceiptLayer(bypass, l); err != nil {
			return err
		}
	}
	for _, l := range postedLines(bypass, "outgoing") {
		if err := postIssueLayer(bypass, l); err != nil {
			return err
		}
	}
	return nil
}

// postedLines reads done move lines for an operation without a layer yet
// (watermark = stock_move_line.id kept on the layer).
func postedLines(ctx context.Context, operation string) []valLine {
	rows, err := orm.DB.QueryContext(ctx, `
		SELECT ml.id, ml.move_id, ml.picking_id, ml.product_id, ml.quantity,
		       COALESCE(m.date_done::text, TO_CHAR(now(),'YYYY-MM-DD HH24:MI:SS'))
		FROM stock_move_line ml
		JOIN stock_move m ON m.id = ml.move_id
		JOIN stock_picking p ON p.id = ml.picking_id
		WHERE m.state = 'done' AND p.operation = $1
		  AND NOT EXISTS (SELECT 1 FROM stock_valuation_layer l
		                  WHERE l.source_line = ml.id AND l.source = $2)
		ORDER BY COALESCE(m.date_done, now()), ml.id LIMIT 500`,
		operation, map[string]string{"incoming": string(SrcReceipt), "outgoing": string(SrcIssue)}[operation])
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []valLine
	for rows.Next() {
		var v valLine
		if err := rows.Scan(&v.lineID, &v.moveID, &v.pickID, &v.prodID, &v.qty, &v.date); err != nil {
			continue
		}
		out = append(out, v)
	}
	return out
}

// productCostCents resolves the receipt unit cost in cents from the product
// master (PO-line pricing arrives with the purchase integration; until then
// the standard cost is the documented receipt basis).
func productCostCents(ctx context.Context, productID int) (int64, ValMethod) {
	prod, err := orm.SearchOne(ctx, "product.product", map[string]interface{}{"id": productID})
	if err != nil || prod == nil {
		return 0, MethodStandard
	}
	cents := int64(math.Round(flt(prod["standard_price"]) * 100))
	switch str(prod["cost_method"]) {
	case string(MethodFIFO):
		return cents, MethodFIFO
	case string(MethodAverage):
		return cents, MethodAverage
	default:
		return cents, MethodStandard
	}
}

func postReceiptLayer(ctx context.Context, l valLine) error {
	unit, method := productCostCents(ctx, l.prodID)
	total := int64(math.Round(float64(unit) * l.qty))
	name := fmt.Sprintf("VAL-R-%d", l.lineID)
	if _, err := createLayer(ctx, name, l, method, SrcReceipt, l.qty, unit, total, l.date); err != nil {
		return err
	}
	return publishEvent(ctx, name+":EVT", EvtLayerPosted, l.prodID, 0, total,
		fmt.Sprintf("Receipt layer for ledger line %d (%.3f @ %d cents).", l.lineID, l.qty, unit))
}

func postIssueLayer(ctx context.Context, l valLine) error {
	_, method := productCostCents(ctx, l.prodID)
	snaps := remainingPool(ctx, l.prodID)
	var slices []LayerSlice
	var short float64
	switch method {
	case MethodAverage:
		slices, short = avgConsume(snaps, l.qty)
	default: // fifo and standard both consume concrete pools oldest-first
		slices, short = fifoConsume(snaps, l.qty)
	}
	cost := int64(0)
	for _, s := range slices {
		cost += s.Cents
	}
	if short > qtyEps {
		// Negative-stock relaxation: price the shortfall at the last known
		// unit cost so the ledger still balances (§10.2 edge).
		unit := int64(0)
		if len(snaps) > 0 {
			last := snaps[len(snaps)-1]
			if last.RemQty > qtyEps {
				unit = int64(math.Round(float64(last.RemValCents) / last.RemQty))
			} else if last.RemValCents != 0 {
				unit = last.RemValCents
			}
		}
		cost += int64(math.Round(float64(unit) * short))
	}
	name := fmt.Sprintf("VAL-I-%d", l.lineID)
	unit := int64(0)
	if l.qty > qtyEps {
		unit = int64(math.Round(float64(cost) / l.qty))
	}
	// Issues carry no remaining pool of their own.
	if _, err := createLayer(ctx, name, l, method, SrcIssue, -l.qty, unit, -cost, l.date, true); err != nil {
		return err
	}
	for _, s := range slices {
		if err := shrinkPool(ctx, s.LayerID, s.Qty, s.Cents); err != nil {
			return err
		}
	}
	return publishEvent(ctx, name+":EVT", EvtConsumption, l.prodID, 0, -cost,
		fmt.Sprintf("Consumption for ledger line %d: %.3f units costing %d cents.", l.lineID, l.qty, cost))
}

func createLayer(ctx context.Context, name string, l valLine, method ValMethod, src LayerSource,
	qty float64, unit, total int64, date string, zeroPool ...bool) (int, error) {
	remQ, remV := qty, total
	if len(zeroPool) > 0 && zeroPool[0] {
		remQ, remV = 0, 0
	}
	return createRec(ctx, layerModel, map[string]interface{}{
		"name":                  name,
		"product_id":            l.prodID,
		"picking_id":            l.pickID,
		"move_id":               l.moveID,
		"source_line":           l.lineID,
		"method":                string(method),
		"source":                string(src),
		"quantity":              qty,
		"unit_value_cents":      unit,
		"value_cents":           total,
		"remaining_qty":         remQ,
		"remaining_value_cents": remV,
		"currency_code":         "USD",
		"date":                  date,
	})
}

// remainingPool lists the product's consumable layers oldest-first.
func remainingPool(ctx context.Context, productID int) []LayerSnap {
	rows, err := orm.DB.QueryContext(ctx, `
		SELECT id, remaining_qty, remaining_value_cents
		FROM stock_valuation_layer
		WHERE product_id = $1 AND remaining_qty > $2 AND quantity > 0
		ORDER BY COALESCE(date, create_date), id`, productID, qtyEps)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []LayerSnap
	seq := 0
	for rows.Next() {
		var s LayerSnap
		if err := rows.Scan(&s.ID, &s.RemQty, &s.RemValCents); err != nil {
			continue
		}
		seq++
		s.Seq = seq
		out = append(out, s)
	}
	return out
}

// shrinkPool decrements a layer's remaining pool inside a locked UPDATE; the
// WHERE guard keeps remaining non-negative under concurrent syncs.
func shrinkPool(ctx context.Context, layerID int, qty float64, cents int64) error {
	res, err := orm.DB.ExecContext(ctx, `
		UPDATE stock_valuation_layer
		SET remaining_qty = GREATEST(remaining_qty - $2, 0),
		    remaining_value_cents = GREATEST(remaining_value_cents - $3, 0)
		WHERE id = $1 AND quantity > 0`, layerID, qty, cents)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("valuation layer %d not found to shrink", layerID)
	}
	return nil
}

// addPoolValue raises a layer's remaining value only (landed costs re-basis
// the per-unit cost of existing stock without adding quantity, §10.4).
func addPoolValue(ctx context.Context, layerID int, cents int64) error {
	res, err := orm.DB.ExecContext(ctx, `
		UPDATE stock_valuation_layer
		SET remaining_value_cents = remaining_value_cents + $2
		WHERE id = $1 AND remaining_qty > 0`, layerID, cents)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("valuation layer %d not available for landed cost", layerID)
	}
	return nil
}

func publishEvent(ctx context.Context, name string, evType EventType, productID, layerID int, cents int64, message string) error {
	_, err := createRec(ctx, eventModel, map[string]interface{}{
		"name":         name,
		"event_type":   string(evType),
		"product_id":   productID,
		"layer_id":     layerID,
		"amount_cents": cents,
		"date":         time.Now().Format(dtLayout),
		"message":      message,
	})
	return err
}

// ---------------------------------------------------------------------------
// Local helpers
// ---------------------------------------------------------------------------

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
