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

// Planning engines (blueprint §9, §15, §17, §23). The replenishment evaluator
// is deliberately read-only against the stock ledger: it projects the
// inventory position from quants and open moves, then writes proposals and
// alerts — never quantities. Money-linked metrics come from the valuation
// layers so numerator and denominator share one source (§15).

const (
	evalCron       = "stock.replenishment.evaluate"
	staleCron      = "stock.reservation.release.stale"
	transitCron    = "stock.intransit.overdue.check"
	metricsCron    = "stock.report.metrics.compute"
	abcCron        = "stock.product.class.abc"
	staleAfterDays = 30
	demandWindow   = 90 // days of posted issues feeding daily demand

	ruleModel   = "stock.replenishment.rule"
	propModel   = "stock.replenishment.proposal"
	alertModel  = "stock.stockout.alert"
	metricModel = "stock.report.metrics"
	classModel  = "stock.product.class"
	dtLayout    = "2006-01-02 15:04:05"
	qtyEps      = 1e-6
)

func init() {
	scheduler.RegisterCronHandler(evalCron, runReplenishmentEvaluate)
	scheduler.RegisterCronHandler(staleCron, runStaleReservationRelease)
	scheduler.RegisterCronHandler(transitCron, runInTransitOverdue)
	scheduler.RegisterCronHandler(metricsCron, runMetricsCompute)
	scheduler.RegisterCronHandler(abcCron, runABCClassify)
}

// ---------------------------------------------------------------------------
// Pure planning maths (unit tested)
// ---------------------------------------------------------------------------

// reorderPoint: lead-time demand plus safety stock (blueprint §9 example
// 20/day × 7 days + 40 = 180).
func reorderPoint(dailyDemand float64, leadDays int, safety float64) float64 {
	if dailyDemand < 0 {
		dailyDemand = 0
	}
	return dailyDemand*float64(leadDays) + safety
}

// inventoryPosition: eligible (free) on-hand + confirmed incoming −
// committed demand (§9.3).
func inventoryPosition(onHand, incoming, committed float64) float64 {
	return onHand + incoming - committed
}

// replenishTrigger decides whether a rule is below its trigger level and
// which trigger label the proposal carries. A non-positive position always
// escalates to the stockout trigger.
func replenishTrigger(method ReplMethod, position, minQty, triggerLevel float64) (bool, ProposalTrigger) {
	if position <= -qtyEps {
		return true, TrigStockout
	}
	level := triggerLevel
	if method == MethodMinMax {
		level = minQty
	}
	if level <= qtyEps {
		return false, ""
	}
	if position < level-qtyEps {
		return true, ProposalTrigger(string(method))
	}
	return false, ""
}

// orderQuantity sizes the raw order: fixed batches when configured,
// otherwise top-up to the target (§9.4).
func orderQuantity(position, target, fixed float64) float64 {
	if fixed > qtyEps {
		return fixed
	}
	raw := target - position
	if raw < 0 {
		return 0
	}
	return raw
}

// applyPurchaseConstraints enforces MOQ, order multiple and per-order cap,
// recording every adjustment so the proposal can show its work.
func applyPurchaseConstraints(raw, moq, multiple, maxOrder float64) (float64, []string) {
	notes := []string{}
	if raw <= qtyEps {
		return 0, notes
	}
	q := raw
	if moq > qtyEps && q < moq {
		notes = append(notes, fmt.Sprintf("raised to MOQ %.3f", moq))
		q = moq
	}
	if multiple > 1+qtyEps {
		round := math.Ceil(q/multiple-qtyEps) * multiple
		if round > q+qtyEps {
			notes = append(notes, fmt.Sprintf("rounded up to order multiple %.3f", multiple))
			q = round
		}
	}
	if maxOrder > qtyEps && q > maxOrder {
		notes = append(notes, fmt.Sprintf("capped at max per order %.3f", maxOrder))
		q = maxOrder
	}
	return q, notes
}

// proposalKey is the stable dedup identity for one product/warehouse/origin
// lane; reruns while a proposal is open must not stack (§9.4).
func proposalKey(productID, warehouseID int, origin string) string {
	return fmt.Sprintf("P%d-W%d-%s", productID, warehouseID, origin)
}

// abcClasses classifies consumption values (already sorted descending) at the
// classic 80 % / 95 % cumulative-value thresholds.
func abcClasses(valuesDesc []float64) []ABClass {
	total := 0.0
	for _, v := range valuesDesc {
		total += v
	}
	out := make([]ABClass, len(valuesDesc))
	if total <= 0 {
		for i := range out {
			out[i] = ABCNA
		}
		return out
	}
	cum := 0.0
	for i, v := range valuesDesc {
		cum += v
		switch {
		case cum <= total*0.80+1e-9:
			out[i] = ABCA
		case cum <= total*0.95+1e-9:
			out[i] = ABCB
		default:
			out[i] = ABCC
		}
	}
	return out
}

// xyzClass buckets demand variability: coefficient of variation over monthly
// consumption, needs at least three months of history.
func xyzClass(cv float64, months int) XYZClass {
	if months < 3 {
		return XYZNA
	}
	switch {
	case cv < 0.5:
		return XYXX
	case cv < 1.0:
		return XYXY
	default:
		return XYZZ
	}
}

// ---------------------------------------------------------------------------
// Replenishment evaluation
// ---------------------------------------------------------------------------

type ruleRow struct {
	id                             int
	productID, warehouseID         int
	method                         ReplMethod
	minQty, maxQty, safety         float64
	leadDays                       int
	fixed, moq, multiple, maxOrder float64
}

func runReplenishmentEvaluate(ctx context.Context, _ map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	rows, err := orm.DB.QueryContext(bypass, `
		SELECT id, product_id, warehouse_id, method, min_qty, max_qty,
		       safety_stock, lead_time_days, fixed_qty, min_order_qty,
		       order_multiple, max_order_qty
		FROM stock_replenishment_rule WHERE active ORDER BY id`)
	if err != nil {
		return nil // table absent before first install
	}
	defer rows.Close()
	var rules []ruleRow
	for rows.Next() {
		var r ruleRow
		var method string
		if err := rows.Scan(&r.id, &r.productID, &r.warehouseID, &method, &r.minQty,
			&r.maxQty, &r.safety, &r.leadDays, &r.fixed, &r.moq, &r.multiple,
			&r.maxOrder); err != nil {
			continue
		}
		r.method = ReplMethod(method)
		rules = append(rules, r)
	}
	rows.Close()

	for _, r := range rules {
		if err := evaluateRule(bypass, r); err != nil {
			return err
		}
	}
	return nil
}

func evaluateRule(ctx context.Context, r ruleRow) error {
	onHand := scalarFloat(ctx, `
		SELECT COALESCE(SUM(q.quantity - q.reserved_quantity), 0)
		FROM stock_quant q JOIN stock_location l ON l.id = q.location_id
		WHERE q.product_id = $1 AND l.warehouse_id = $2
		  AND l.usage = 'internal' AND NOT l.blocked AND l.active`,
		r.productID, r.warehouseID)
	incoming := openOpQty(ctx, r.productID, r.warehouseID, "incoming")
	committed := openOpQty(ctx, r.productID, r.warehouseID, "outgoing")
	daily := scalarFloat(ctx, `
		SELECT COALESCE(SUM(ml.quantity), 0) / $2::float
		FROM stock_move_line ml
		JOIN stock_move m ON m.id = ml.move_id
		JOIN stock_picking p ON p.id = ml.picking_id
		WHERE ml.product_id = $1 AND p.operation = 'outgoing' AND m.state = 'done'
		  AND COALESCE(m.date_done, m.create_date) > now() - ($2 || ' days')::interval`,
		r.productID, demandWindow)

	pos := inventoryPosition(onHand, incoming, committed)
	rop := reorderPoint(daily, r.leadDays, r.safety)
	minQty, target := r.minQty, r.maxQty
	if minQty <= qtyEps || target <= qtyEps {
		// Product-level reorder rules as fallback (master data defaults).
		pmin, pmax := productReorder(ctx, r.productID)
		if minQty <= qtyEps {
			minQty = pmin
		}
		if target <= qtyEps {
			target = pmax
		}
	}

	fired, trigger := replenishTrigger(r.method, pos, minQty, rop)
	if !fired {
		return nil
	}
	if openPOExists(ctx, r.productID) {
		return nil // demand already covered by an open purchase order
	}
	key := proposalKey(r.productID, r.warehouseID, "cron")
	if openProposalExists(ctx, key) {
		return nil // dedup lane: the earlier proposal still stands
	}

	raw := orderQuantity(pos, target, r.fixed)
	if r.method == MethodRop && r.fixed <= qtyEps {
		raw = orderQuantity(pos, rop, 0)
	}
	qty, notes := applyPurchaseConstraints(raw, r.moq, r.multiple, r.maxOrder)
	if qty <= qtyEps {
		return nil
	}

	explain := fmt.Sprintf(
		"Position %.3f = free on-hand %.3f + incoming %.3f − committed %.3f. Daily demand %.3f (%d-day window).",
		pos, onHand, incoming, committed, daily, demandWindow)
	if r.method == MethodRop {
		explain += fmt.Sprintf(" Trigger at ROP %.3f (lead %dd × daily + safety %.3f); order up to ROP.",
			rop, r.leadDays, r.safety)
	} else {
		explain += fmt.Sprintf(" Trigger below min %.3f; order up to max %.3f.", minQty, target)
	}
	if len(notes) > 0 {
		explain += " Constraints applied: " + joinNotes(notes) + "."
	}

	_, err := createRec(ctx, propModel, map[string]interface{}{
		"name":          fmt.Sprintf("REPL-%d-%d-%s", r.productID, r.warehouseID, time.Now().Format("20060102")),
		"dedup_key":     key,
		"product_id":    r.productID,
		"warehouse_id":  r.warehouseID,
		"origin":        "cron",
		"rule_id":       r.id,
		"on_hand":       onHand,
		"incoming":      incoming,
		"committed":     committed,
		"position":      pos,
		"daily_demand":  daily,
		"trigger":       string(trigger),
		"target_qty":    math.Max(target, rop),
		"suggested_qty": qty,
		"explanation":   explain,
		"state":         string(PropDraft),
		"date":          time.Now().Format(dtLayout),
	})
	if err != nil {
		return err
	}

	if pos <= qtyEps {
		raiseAlert(ctx, r.productID, r.warehouseID, ReasonStockout, qty,
			fmt.Sprintf("rule:%d", r.id),
			fmt.Sprintf("Inventory position %.3f at or below zero; replenishment proposed.", pos))
	}
	return nil
}

func openOpQty(ctx context.Context, productID, warehouseID int, op string) float64 {
	locCol := "m.destination_id"
	if op != "incoming" {
		locCol = "m.source_location_id"
	}
	return scalarFloat(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(m.quantity), 0)
		FROM stock_move m
		JOIN stock_picking p ON p.id = m.picking_id
		JOIN stock_location l ON l.id = m.%s
		WHERE m.product_id = $1 AND p.operation = $2
		  AND m.state IN ('confirmed', 'assigned') AND l.warehouse_id = $3`, locCol),
		productID, op, warehouseID)
}

func productReorder(ctx context.Context, productID int) (float64, float64) {
	rows, err := orm.DB.QueryContext(ctx,
		`SELECT COALESCE(reorder_min,0), COALESCE(reorder_max,0) FROM product_product WHERE id = $1`, productID)
	if err != nil {
		return 0, 0
	}
	defer rows.Close()
	if rows.Next() {
		var min, max float64
		if err := rows.Scan(&min, &max); err == nil {
			return min, max
		}
	}
	return 0, 0
}

func openPOExists(ctx context.Context, productID int) bool {
	var ok bool
	err := orm.DB.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM purchase_order_line pol
			JOIN purchase_order po ON po.id = pol.order_id
			WHERE pol.product_id = $1 AND po.state IN ('draft', 'sent', 'purchase'))`,
		productID).Scan(&ok)
	if err != nil {
		return false // purchase tables unavailable
	}
	return ok
}

func openProposalExists(ctx context.Context, key string) bool {
	var ok bool
	err := orm.DB.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM stock_replenishment_proposal
		               WHERE dedup_key = $1 AND state IN ('draft', 'approved'))`,
		key).Scan(&ok)
	return err == nil && ok
}

// ---------------------------------------------------------------------------
// Exception engines (§17)
// ---------------------------------------------------------------------------

// runStaleReservationRelease frees reservations held by confirmed moves past
// their due date for 30+ days, mirroring the anchor's release order (newest
// quant slices first) with GREATEST-guarded updates.
func runStaleReservationRelease(ctx context.Context, _ map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	rows, err := orm.DB.QueryContext(bypass, fmt.Sprintf(`
		SELECT id, product_id, COALESCE(source_location_id, 0), COALESCE(lot_id, 0),
		       reserved_quantity
		FROM stock_move
		WHERE state = 'confirmed' AND reserved_quantity > 0
		  AND COALESCE(date_due, create_date) < now() - interval '%d days'
		ORDER BY id LIMIT 200`, staleAfterDays))
	if err != nil {
		return nil
	}
	type staler struct {
		id, prod, src, lot int
		rsvd               float64
	}
	var stale []staler
	for rows.Next() {
		var s staler
		if err := rows.Scan(&s.id, &s.prod, &s.src, &s.lot, &s.rsvd); err != nil {
			continue
		}
		stale = append(stale, s)
	}
	rows.Close()

	for _, s := range stale {
		qrows, err := orm.DB.QueryContext(bypass, `
			WITH RECURSIVE sub AS (
				SELECT id FROM stock_location WHERE id = $1
				UNION ALL SELECT l.id FROM stock_location l JOIN sub ON l.parent_id = sub.id)
			SELECT q.id, q.reserved_quantity FROM stock_quant q
			WHERE q.product_id = $2 AND q.location_id IN (SELECT id FROM sub)
			  AND ((q.lot_id IS NULL AND $3 = 0) OR q.lot_id = NULLIF($3, 0))
			  AND q.reserved_quantity > 0
			ORDER BY q.in_date DESC NULLS LAST, q.id DESC`, s.src, s.prod, s.lot)
		if err != nil {
			continue
		}
		remaining := s.rsvd
		var release []struct {
			id   int
			rsvd float64
		}
		for qrows.Next() {
			var id int
			var rsvd float64
			if err := qrows.Scan(&id, &rsvd); err != nil {
				continue
			}
			take := rsvd
			if take > remaining {
				take = remaining
			}
			release = append(release, struct {
				id   int
				rsvd float64
			}{id, take})
			remaining -= take
			if remaining <= qtyEps {
				break
			}
		}
		qrows.Close()
		for _, r := range release {
			if r.rsvd <= qtyEps {
				continue
			}
			if _, err := orm.DB.ExecContext(bypass,
				`UPDATE stock_quant SET reserved_quantity = GREATEST(0, reserved_quantity - $1) WHERE id = $2`,
				r.rsvd, r.id); err != nil {
				return err
			}
		}
		if _, err := orm.DB.ExecContext(bypass,
			`UPDATE stock_move SET reserved_quantity = 0 WHERE id = $1 AND state = 'confirmed'`,
			s.id); err != nil {
			return err
		}
		raiseAlert(bypass, s.prod, 0, ReasonReservation, s.rsvd,
			fmt.Sprintf("move:%d", s.id),
			fmt.Sprintf("Reservation on move %d older than %d days released back to availability.",
				s.id, staleAfterDays))
	}
	return nil
}

// runInTransitOverdue raises alerts for confirmed inbound moves past their
// due date so the exception console shows late supply before the stockout.
func runInTransitOverdue(ctx context.Context, _ map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	rows, err := orm.DB.QueryContext(bypass, `
		SELECT m.id, m.product_id, COALESCE(l.warehouse_id, 0), m.quantity,
		       TO_CHAR(m.date_due, 'YYYY-MM-DD')
		FROM stock_move m
		JOIN stock_picking p ON p.id = m.picking_id
		JOIN stock_location l ON l.id = m.destination_id
		WHERE p.operation = 'incoming' AND m.state IN ('confirmed', 'assigned')
		  AND m.date_due IS NOT NULL AND m.date_due < now()
		ORDER BY m.date_due LIMIT 500`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var moveID, prodID, whID int
		var qty float64
		var due string
		if err := rows.Scan(&moveID, &prodID, &whID, &qty, &due); err != nil {
			continue
		}
		raiseAlert(bypass, prodID, whID, ReasonInTransit, qty,
			fmt.Sprintf("move:%d", moveID),
			fmt.Sprintf("Inbound move %d due %s still open; downstream replenishment assumptions may be stale.",
				moveID, due))
	}
	return nil
}

func raiseAlert(ctx context.Context, productID, warehouseID int, reason AlertReason, qty float64, sourceRef, note string) {
	var exists bool
	if err := orm.DB.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM stock_stockout_alert
		               WHERE source_ref = $1 AND reason = $2 AND state = 'open')`,
		sourceRef, string(reason)).Scan(&exists); err == nil && exists {
		return
	}
	name := fmt.Sprintf("ALRT-%s-%s", reason, sourceRef)
	_, _ = createRec(ctx, alertModel, map[string]interface{}{
		"name":         name,
		"product_id":   productID,
		"warehouse_id": warehouseID,
		"reason":       string(reason),
		"quantity":     qty,
		"source_ref":   sourceRef,
		"state":        string(AlertOpen),
		"date":         time.Now().Format(dtLayout),
		"note":         note,
	})
}

// ---------------------------------------------------------------------------
// Metrics & classification (§15, §23)
// ---------------------------------------------------------------------------

type prodValue struct {
	prodID int
	cents  int64
}

// runMetricsCompute writes turnover and days-in-cover per product from the
// valuation layers: turnover = trailing-year COGS / average inventory value,
// with the average approximated as ending remaining value + half the year's
// consumption (steady-flow assumption recorded in Detail).
func runMetricsCompute(ctx context.Context, _ map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	cogs := consumptionByProduct(bypass, 365)
	remaining := remainingValueByProduct(bypass)
	stamp := time.Now().Format(dtLayout)
	for _, p := range cogs {
		avgCents := remaining[p.prodID] + p.cents/2
		if avgCents <= 0 {
			continue
		}
		turnover := float64(p.cents) / float64(avgCents)
		dio := 365.0 / turnover
		detail := fmt.Sprintf("COGS 365d = %d cents; avg inventory = %d cents (ending remaining %d + %d/2).",
			p.cents, avgCents, remaining[p.prodID], p.cents)
		for _, m := range []struct {
			kind  MetricKind
			value float64
		}{{MetricTurnover, turnover}, {MetricDIO, dio},
			{MetricCOGS, float64(p.cents) / 100}, {MetricAvgValue, float64(avgCents) / 100}} {
			if _, err := createRec(bypass, metricModel, map[string]interface{}{
				"name":        fmt.Sprintf("MET-%s-%d-%s", m.kind, p.prodID, time.Now().Format("20060102")),
				"metric":      string(m.kind),
				"product_id":  p.prodID,
				"value":       m.value,
				"period_days": 365,
				"date_end":    stamp,
				"detail":      detail,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

// runABCClassify re-ranks annual consumption value into ABC and monthly
// demand variability into XYZ, upserting one class row per product.
func runABCClassify(ctx context.Context, _ map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	values := consumptionByProduct(bypass, 365)
	if len(values) == 0 {
		return nil
	}
	sort.SliceStable(values, func(i, j int) bool { return values[i].cents > values[j].cents })
	desc := make([]float64, len(values))
	for i, v := range values {
		desc[i] = float64(v.cents)
	}
	classes := abcClasses(desc)
	variability := demandVariability(bypass)
	stamp := time.Now().Format(dtLayout)

	for i, v := range values {
		cv, months := 0.0, 0
		if s, ok := variability[v.prodID]; ok {
			cv, months = s.cv, s.months
		}
		val := float64(v.cents) / 100
		xyz := string(xyzClass(cv, months))
		existing, _ := orm.SearchOne(bypass, classModel, map[string]interface{}{"product_id": v.prodID})
		if existing != nil {
			if err := orm.UpdateRecordByID(bypass, classModel, intID(existing["id"]), map[string]interface{}{
				"abc_class": string(classes[i]), "xyz_class": xyz,
				"annual_value": val, "rank": i + 1, "demand_cv": cv, "last_run": stamp,
			}); err != nil {
				return err
			}
			continue
		}
		_, _ = createRec(bypass, classModel, map[string]interface{}{
			"name":         fmt.Sprintf("ABC-%d", v.prodID),
			"product_id":   v.prodID,
			"abc_class":    string(classes[i]),
			"xyz_class":    string(xyzClass(cv, months)),
			"annual_value": val,
			"rank":         i + 1,
			"demand_cv":    cv,
			"last_run":     stamp,
			"note":         "ABC at 80/95 cumulative value thresholds; XYZ from CV of monthly issues (need >= 3 months).",
		})
	}
	return nil
}

func consumptionByProduct(ctx context.Context, days int) []prodValue {
	rows, err := orm.DB.QueryContext(ctx, fmt.Sprintf(`
		SELECT product_id, -SUM(value_cents)::bigint
		FROM stock_valuation_layer
		WHERE source = 'issue' AND value_cents < 0
		  AND COALESCE(date, create_date) > now() - interval '%d days'
		GROUP BY product_id HAVING -SUM(value_cents) > 0
		ORDER BY product_id`, days))
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []prodValue
	for rows.Next() {
		var p prodValue
		if err := rows.Scan(&p.prodID, &p.cents); err != nil {
			continue
		}
		out = append(out, p)
	}
	return out
}

func remainingValueByProduct(ctx context.Context) map[int]int64 {
	out := map[int]int64{}
	rows, err := orm.DB.QueryContext(ctx, `
		SELECT product_id, SUM(remaining_value_cents)::bigint
		FROM stock_valuation_layer WHERE quantity > 0
		GROUP BY product_id`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var cents int64
		if err := rows.Scan(&id, &cents); err == nil {
			out[id] = cents
		}
	}
	return out
}

type variStats struct {
	cv     float64
	months int
}

// demandVariability computes the coefficient of variation of monthly issued
// quantities per product over the trailing twelve months.
func demandVariability(ctx context.Context) map[int]variStats {
	out := map[int]variStats{}
	rows, err := orm.DB.QueryContext(ctx, `
		SELECT product_id,
		       DATE_TRUNC('month', COALESCE(date, create_date))::date::text,
		       -SUM(quantity)
		FROM stock_valuation_layer
		WHERE source = 'issue'
		  AND COALESCE(date, create_date) > now() - interval '365 days'
		GROUP BY 1, 2 ORDER BY 1, 2`)
	if err != nil {
		return out
	}
	defer rows.Close()
	series := map[int][]float64{}
	for rows.Next() {
		var pid int
		var month string
		var qty float64
		if err := rows.Scan(&pid, &month, &qty); err != nil {
			continue
		}
		series[pid] = append(series[pid], qty)
	}
	for pid, vals := range series {
		mean := 0.0
		for _, v := range vals {
			mean += v
		}
		mean /= float64(len(vals))
		if mean <= qtyEps {
			out[pid] = variStats{cv: 1.5, months: len(vals)}
			continue
		}
		var varSum float64
		for _, v := range vals {
			varSum += (v - mean) * (v - mean)
		}
		out[pid] = variStats{cv: math.Sqrt(varSum/float64(len(vals))) / mean, months: len(vals)}
	}
	return out
}

func joinNotes(notes []string) string {
	out := ""
	for i, n := range notes {
		if i > 0 {
			out += "; "
		}
		out += n
	}
	return out
}

// ---------------------------------------------------------------------------
// Local helpers
// ---------------------------------------------------------------------------

func scalarFloat(ctx context.Context, query string, args ...interface{}) float64 {
	var v float64
	if err := orm.DB.QueryRowContext(ctx, query, args...).Scan(&v); err != nil {
		return 0
	}
	return v
}

func createRec(ctx context.Context, model string, vals map[string]interface{}) (int, error) {
	inst, ok := orm.Registry[model]
	if !ok {
		return 0, fmt.Errorf("model %s unavailable", model)
	}
	return orm.Create(ctx, inst, vals)
}
