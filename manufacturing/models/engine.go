package models

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"nexa/core/orm"
)

// MRP engine: explode a work order's BOM into component demand lines.
// Explosion is server-side and idempotent — re-running replaces the demand
// lines, never duplicates them. Multi-level explosion (sub-assemblies) is a
// phase-2 enhancement; phase 1 explodes one BOM level.

const (
	bomModel   = "mrp.bom"
	bomLineMdl = "mrp.bom.line"
	woModel    = "mrp.workorder"
	woLineMdl  = "mrp.workorder.line"
)

func init() {
	orm.RegisterObjectAction(woModel, "action_mrp_explode", woExplode)
	orm.RegisterObjectAction(woModel, "action_mrp_consume", woConsume)
}

func loadWO(ctx context.Context, id int) map[string]interface{} {
	rows, err := orm.SearchPage(ctx, woModel, [][]interface{}{{"id", "=", id}}, 1, 0, "id asc")
	if err != nil || len(rows) == 0 {
		return nil
	}
	return rows[0]
}

func ormFloat(v interface{}) float64 {
	n, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(v)), 64)
	return n
}

func ormID(v interface{}) int {
	if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
		n, _ := strconv.Atoi(fmt.Sprint(arr[0]))
		return n
	}
	n, _ := strconv.Atoi(fmt.Sprint(v))
	return n
}

// woExplode replaces the work order's component demand with a fresh explosion
// of its BOM: required_qty = wo.product_qty / bom.product_qty * bom_line.qty.
func woExplode(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	wo := loadWO(bypass, id)
	if wo == nil {
		return "", fmt.Errorf("work order not found")
	}
	if st := orm.AsString(wo["state"]); st == "done" || st == "cancelled" {
		return "", fmt.Errorf("cannot explode a %s work order", st)
	}
	bomID := ormID(wo["bom_id"])
	if bomID <= 0 {
		// variant-aware auto-pick: args["variant"] selects a specific BOM grade
		domain := [][]interface{}{{"product_id", "=", ormID(wo["product_id"])}, {"state", "=", "active"}}
		if v := strings.TrimSpace(args["variant"]); v != "" {
			domain = append(domain, []interface{}{"variant", "=", v})
		}
		boms, err := orm.SearchPage(bypass, bomModel, domain, 1, 0, "id asc")
		if err != nil || len(boms) == 0 && strings.TrimSpace(args["variant"]) != "" {
			// variant not found — fall back to any active BOM
			boms, err = orm.SearchPage(bypass, bomModel,
				[][]interface{}{{"product_id", "=", ormID(wo["product_id"])}, {"state", "=", "active"}}, 1, 0, "id asc")
		}
		if err != nil || len(boms) == 0 {
			return "", fmt.Errorf("no active BOM for this product — create and activate one first")
		}
		bomID = ormID(boms[0]["id"])
		_ = orm.UpdateRecordByID(bypass, woModel, id, map[string]interface{}{"bom_id": bomID})
	}
	woQty := ormFloat(wo["product_qty"])

	// Idempotent: wipe existing demand lines, then write fresh ones.
	old, _ := orm.SearchPage(bypass, woLineMdl, [][]interface{}{{"workorder_id", "=", id}}, 2000, 0, "id asc")
	for _, l := range old {
		_ = orm.Unlink(bypass, woLineMdl, ormID(l["id"]))
	}
	inst, ok := orm.Registry[woLineMdl]
	if !ok {
		return "", fmt.Errorf("workorder line model not registered")
	}
	count := explodeBOM(bypass, inst, id, bomID, woQty, 1, map[int]bool{bomID: true})
	// confirmed work orders with demand are ready to start
	if orm.AsString(wo["state"]) == "draft" {
		_ = orm.UpdateRecordByID(bypass, woModel, id, map[string]interface{}{"state": "confirmed"})
	}
	return fmt.Sprintf("Exploded %d component lines (multi-level)", count), nil
}

// explodeBOM writes demand lines for one BOM level, then recurses into any
// component that is itself manufactured (has an active BOM). qtyFinished is
// the number of finished units this BOM must produce; each component's demand
// = line.qty × qtyFinished / bom.product_qty. visited prevents BOM cycles;
// depth caps explosion at 5 levels.
func explodeBOM(ctx context.Context, inst orm.Model, woID, bomID int, qtyFinished float64, level int, visited map[int]bool) int {
	if level > 5 {
		return 0
	}
	bom := loadRec(ctx, bomModel, bomID)
	if bom == nil {
		return 0
	}
	lines, err := orm.SearchPage(ctx, bomLineMdl,
		[][]interface{}{{"bom_id", "=", bomID}}, 500, 0, "sequence asc, id asc")
	if err != nil {
		return 0
	}
	bomQty := ormFloat(bom["product_qty"])
	if bomQty <= 0 {
		bomQty = 1
	}
	ratio := qtyFinished / bomQty
	count := 0
	for i, bl := range lines {
		compQty := ormFloat(bl["product_qty"]) * ratio
		if compQty <= 0 {
			continue
		}
		compPID := ormID(bl["product_id"])
		if _, err := orm.Create(ctx, inst, map[string]interface{}{
			"workorder_id": woID,
			"product_id":   compPID,
			"product_qty":  compQty,
			"sequence":     level*1000 + (i+1)*10,
			"level":        level,
		}); err == nil {
			count++
		}
		// recurse: is this component itself manufactured?
		if level < 5 && !visited[bomID] {
			sub, _ := orm.SearchPage(ctx, bomModel,
				[][]interface{}{{"product_id", "=", compPID}, {"state", "=", "active"}}, 1, 0, "id asc")
			if len(sub) > 0 {
				subID := ormID(sub[0]["id"])
				if !visited[subID] {
					visited[subID] = true
					count += explodeBOM(ctx, inst, woID, subID, compQty, level+1, visited)
				}
			}
		}
	}
	return count
}

// woConsume marks component demand as consumed — the manual phase-1 path until
// stock.picking integration posts real stock moves in phase 2.
func woConsume(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	wo := loadWO(bypass, id)
	if wo == nil {
		return "", fmt.Errorf("work order not found")
	}
	if st := orm.AsString(wo["state"]); st == "done" || st == "cancelled" {
		return "", fmt.Errorf("cannot consume on a %s work order", st)
	}
	lines, err := orm.SearchPage(bypass, woLineMdl, [][]interface{}{{"workorder_id", "=", id}}, 500, 0, "id asc")
	if err != nil || len(lines) == 0 {
		return "", fmt.Errorf("no component demand — run MRP explode first")
	}
	// args["line_id"] + args["qty"] consume partially; no args = consume all.
	if lid := ormID(args["line_id"]); lid > 0 {
		qty := ormFloat(args["qty"])
		if qty <= 0 {
			return "", fmt.Errorf("qty required for partial consume")
		}
		_ = orm.UpdateRecordByID(bypass, woLineMdl, lid, map[string]interface{}{"qty_consumed": qty})
		return fmt.Sprintf("Consumed %.3f on line %d", qty, lid), nil
	}
	for _, l := range lines {
		_ = orm.UpdateRecordByID(bypass, woLineMdl, ormID(l["id"]),
			map[string]interface{}{"qty_consumed": l["product_qty"]})
	}
	// all demand consumed → work order done, record produced qty
	_ = orm.UpdateRecordByID(bypass, woModel, id, map[string]interface{}{
		"state":        "done",
		"qty_produced": wo["product_qty"],
	})
	return "All components consumed — work order done", nil
}

func loadRec(ctx context.Context, model string, id int) map[string]interface{} {
	rows, err := orm.SearchPage(ctx, model, [][]interface{}{{"id", "=", id}}, 1, 0, "id asc")
	if err != nil || len(rows) == 0 {
		return nil
	}
	return rows[0]
}

// ---------------------------------------------------------------------------
// Phase 2 — real stock moves. Component issue transfers parts stock→production;
// finished-goods receipt moves the product production→stock. Both create real
// stock.picking + stock.move rows so inventory stays authoritative in `stock`.
// ---------------------------------------------------------------------------

const (
	pickingModel = "stock.picking"
	moveModel    = "stock.move"
	locModel     = "stock.location"
)

func init() {
	orm.RegisterObjectAction(woModel, "action_mrp_stock_issue", woStockIssue)
	orm.RegisterObjectAction(woModel, "action_mrp_produce", woProduce)
}

// resolveLocations finds the production-usage location and the first active
// internal stock location. Args may override via loc_src / loc_dst.
func resolveLocations(ctx context.Context, args map[string]string, wantProd bool) (int, int) {
	prod, stock := 0, 0
	if wantProd {
		// source = stock, dest = production
		rows, _ := orm.SearchPage(ctx, locModel, [][]interface{}{{"usage", "=", "production"}, {"active", "=", true}}, 1, 0, "id asc")
		if len(rows) > 0 {
			prod = ormID(rows[0]["id"])
		}
		rows, _ = orm.SearchPage(ctx, locModel, [][]interface{}{{"usage", "=", "internal"}, {"active", "=", true}}, 1, 0, "id asc")
		if len(rows) > 0 {
			stock = ormID(rows[0]["id"])
		}
		return stock, prod
	}
	// receipt: source = production, dest = stock
	rows, _ := orm.SearchPage(ctx, locModel, [][]interface{}{{"usage", "=", "production"}, {"active", "=", true}}, 1, 0, "id asc")
	if len(rows) > 0 {
		prod = ormID(rows[0]["id"])
	}
	rows, _ = orm.SearchPage(ctx, locModel, [][]interface{}{{"usage", "=", "internal"}, {"active", "=", true}}, 1, 0, "id asc")
	if len(rows) > 0 {
		stock = ormID(rows[0]["id"])
	}
	return prod, stock
}

func createPicking(ctx context.Context, name, operation, origin string, src, dst int, moves []map[string]interface{}) (int, error) {
	pinst, ok := orm.Registry[pickingModel]
	if !ok {
		return 0, fmt.Errorf("stock.picking not registered")
	}
	pid, err := orm.Create(ctx, pinst, map[string]interface{}{
		"name": name, "operation": operation,
		"source_location_id": src, "destination_id": dst,
		"origin": origin, "state": "draft",
	})
	if err != nil {
		return 0, err
	}
	minst, ok := orm.Registry[moveModel]
	if !ok {
		return 0, fmt.Errorf("stock.move not registered")
	}
	for _, m := range moves {
		m["picking_id"] = pid
		m["source_location_id"] = src
		m["destination_id"] = dst
		m["origin"] = origin
		m["state"] = "draft"
		if _, err := orm.Create(ctx, minst, m); err != nil {
			return pid, err
		}
	}
	return pid, nil
}

// woStockIssue creates the component-issue picking (stock → production) for all
// un-consumed demand lines and links it on the work order.
func woStockIssue(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	wo := loadWO(bypass, id)
	if wo == nil {
		return "", fmt.Errorf("work order not found")
	}
	if existing := ormID(wo["issue_picking"]); existing > 0 {
		return fmt.Sprintf("Issue picking already exists (id %d)", existing), nil
	}
	lines, err := orm.SearchPage(bypass, woLineMdl, [][]interface{}{{"workorder_id", "=", id}}, 500, 0, "id asc")
	if err != nil || len(lines) == 0 {
		return "", fmt.Errorf("no component demand — run MRP explode first")
	}
	src, dst := resolveLocations(bypass, args, true)
	if src <= 0 || dst <= 0 {
		return "", fmt.Errorf("no production/internal stock locations found — check stock setup")
	}
	var moves []map[string]interface{}
	for _, l := range lines {
		need := ormFloat(l["product_qty"]) - ormFloat(l["qty_consumed"])
		if need <= 0 {
			continue
		}
		moves = append(moves, map[string]interface{}{
			"name":       fmt.Sprintf("%s component", orm.AsString(l["product_id"])),
			"product_id": ormID(l["product_id"]),
			"quantity":   need,
		})
	}
	if len(moves) == 0 {
		return "", fmt.Errorf("all components already consumed")
	}
	pid, err := createPicking(bypass, orm.AsString(wo["name"])+"-ISS", "internal",
		"MO/"+orm.AsString(wo["name"]), src, dst, moves)
	if err != nil {
		return "", err
	}
	_ = orm.UpdateRecordByID(bypass, woModel, id, map[string]interface{}{"issue_picking": pid})
	return fmt.Sprintf("Component issue picking %d created (%d moves) — confirm it in Inventory", pid, len(moves)), nil
}

// woProduce creates the finished-goods receipt picking (production → stock)
// and marks produced quantity.
func woProduce(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	wo := loadWO(bypass, id)
	if wo == nil {
		return "", fmt.Errorf("work order not found")
	}
	if st := orm.AsString(wo["state"]); st == "done" || st == "cancelled" {
		return "", fmt.Errorf("cannot produce on a %s work order", st)
	}
	if existing := ormID(wo["receipt_picking"]); existing > 0 {
		return fmt.Sprintf("Receipt picking already exists (id %d)", existing), nil
	}
	qty := ormFloat(args["qty"])
	if qty <= 0 {
		qty = ormFloat(wo["product_qty"])
	}
	src, dst := resolveLocations(bypass, args, false)
	if src <= 0 || dst <= 0 {
		return "", fmt.Errorf("no production/internal stock locations found — check stock setup")
	}
	pid, err := createPicking(bypass, orm.AsString(wo["name"])+"-FG", "incoming",
		"MO/"+orm.AsString(wo["name"]), src, dst, []map[string]interface{}{
			{"name": "Finished goods", "product_id": ormID(wo["product_id"]), "quantity": qty},
		})
	if err != nil {
		return "", err
	}
	// Backflush: components are consumed automatically proportional to output —
	// consumed = required × (produced / planned qty).
	if wo["backflush"] == true {
		lines, _ := orm.SearchPage(bypass, woLineMdl,
			[][]interface{}{{"workorder_id", "=", id}}, 500, 0, "id asc")
		planQty := ormFloat(wo["product_qty"])
		if planQty <= 0 {
			planQty = qty
		}
		ratio := qty / planQty
		for _, l := range lines {
			consumed := ormFloat(l["product_qty"]) * ratio
			_ = orm.UpdateRecordByID(bypass, woLineMdl, ormID(l["id"]),
				map[string]interface{}{"qty_consumed": consumed})
		}
	}
	// traceability: create/find the finished-goods lot for this batch
	lotID := ensureLot(bypass, ormID(wo["product_id"]), orm.AsString(wo["name"]))
	upd := map[string]interface{}{
		"receipt_picking": pid,
		"qty_produced":    qty,
		"state":           "done",
	}
	if lotID > 0 {
		upd["lot_id"] = lotID
	}
	_ = orm.UpdateRecordByID(bypass, woModel, id, upd)
	return fmt.Sprintf("Finished-goods receipt picking %d created — confirm it in Inventory to post stock", pid), nil
}

// planned_minutes on a work order = sum(routing line duration) × qty ratio —
// the scheduled labor/machine time for the whole production run.
func init() {
	orm.RegisterCompute(woModel, "planned_minutes", []string{"product_qty", "bom_id"}, computeWOPPlannedMinutes)
}

func computeWOPPlannedMinutes(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	bomID := ormID(rec["bom_id"])
	if bomID <= 0 {
		return 0, nil
	}
	routings, err := orm.SearchPage(orm.ContextWithBypass(ctx, true), "mrp.routing",
		[][]interface{}{{"bom_id", "=", bomID}}, 10, 0, "id asc")
	if err != nil || len(routings) == 0 {
		return 0, nil
	}
	lines, err := orm.SearchPage(orm.ContextWithBypass(ctx, true), "mrp.routing.line",
		[][]interface{}{{"routing_id", "in", []interface{}{ormID(routings[0]["id"])}}}, 500, 0, "sequence asc, id asc")
	if err != nil {
		return 0, nil
	}
	perUnit := 0.0
	for _, l := range lines {
		perUnit += ormFloat(l["duration_min"])
	}
	return perUnit * ormFloat(rec["product_qty"]), nil
}

// ---------------------------------------------------------------------------
// Phase 4 — MRP run: demand (open sales orders) vs supply (stock on hand)
// produces make/buy plan lines. Make = product has an active BOM; buy =
// everything else (feeds purchase recommendations downstream).
// ---------------------------------------------------------------------------

const (
	planModel    = "mrp.plan"
	planLineMdl  = "mrp.plan.line"
	saleLineMdl  = "sale.order.line"
	saleOrderMdl = "sale.order"
	quantModel   = "stock.quant"
)

func init() {
	orm.RegisterObjectAction(planModel, "action_mrp_run", planRun)
}

// planRun executes the MRP: nets demand against on-hand stock and writes one
// plan line per product. Idempotent — re-running replaces the plan's lines.
func planRun(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	plan := loadRec(bypass, planModel, id)
	if plan == nil {
		return "", fmt.Errorf("plan not found")
	}
	horizon := ormFloat(plan["horizon_days"])
	if horizon <= 0 {
		horizon = 30
	}

	// Demand: sale order lines on non-draft/non-cancelled orders.
	orders, err := orm.SearchPage(bypass, saleOrderMdl,
		[][]interface{}{{"state", "not in", []interface{}{"draft", "cancel", "cancelled"}}}, 2000, 0, "id asc")
	if err != nil {
		return "", fmt.Errorf("cannot read sales orders: %v", err)
	}
	var orderIDs []interface{}
	for _, o := range orders {
		orderIDs = append(orderIDs, ormID(o["id"]))
	}
	demand := map[int]float64{}
	if len(orderIDs) > 0 {
		slines, err := orm.SearchPage(bypass, saleLineMdl,
			[][]interface{}{{"order_id", "in", orderIDs}}, 5000, 0, "id asc")
		if err == nil {
			for _, l := range slines {
				pid := ormID(l["product_id"])
				if pid > 0 {
					demand[pid] += ormFloat(l["product_uom_qty"])
				}
			}
		}
	}
	if len(demand) == 0 {
		return "No open sales-order demand — nothing to plan.", nil
	}

	// On-hand: sum stock.quant per product.
	onhand := map[int]float64{}
	quants, err := orm.SearchPage(bypass, quantModel, nil, 10000, 0, "id asc")
	if err == nil {
		for _, q := range quants {
			pid := ormID(q["product_id"])
			if pid > 0 {
				onhand[pid] += ormFloat(q["quantity"])
			}
		}
	}

	// Active BOM lookup: product → bom id.
	boms, _ := orm.SearchPage(bypass, bomModel,
		[][]interface{}{{"state", "=", "active"}}, 1000, 0, "id asc")
	bomByProduct := map[int]int{}
	for _, b := range boms {
		bomByProduct[ormID(b["product_id"])] = ormID(b["id"])
	}

	// Replace plan lines idempotently.
	old, _ := orm.SearchPage(bypass, planLineMdl, [][]interface{}{{"plan_id", "=", id}}, 2000, 0, "id asc")
	for _, l := range old {
		_ = orm.Unlink(bypass, planLineMdl, ormID(l["id"]))
	}
	inst, ok := orm.Registry[planLineMdl]
	if !ok {
		return "", fmt.Errorf("plan line model not registered")
	}
	makeN, buyN := 0, 0
	for pid, dq := range demand {
		oh := onhand[pid]
		net := dq - oh
		if net <= 0 {
			continue // covered by stock
		}
		action := ActBuy
		var bomID int
		if bid, ok := bomByProduct[pid]; ok {
			action = ActMake
			bomID = bid
			makeN++
		} else {
			buyN++
		}
		_, err := orm.Create(bypass, inst, map[string]interface{}{
			"plan_id":     id,
			"product_id":  pid,
			"demand_qty":  dq,
			"on_hand_qty": oh,
			"net_qty":     net,
			"action":      string(action),
			"bom_id":      bomID,
		})
		if err == nil {
			// make recommendations can spin up a draft work order on demand
		}
	}
	_ = orm.UpdateRecordByID(bypass, planModel, id, map[string]interface{}{"state": "done"})
	return fmt.Sprintf("MRP run complete: %d make, %d buy recommendations (%d products netted)", makeN, buyN, makeN+buyN), nil
}

// action_mrp_generate converts every "make" plan line into a draft work order
// (BOM-linked, already exploded) and every "buy" line stays a procurement
// recommendation. Work orders get a stable reference MO/PLAN-<lineID> and the
// plan line records the link — re-running skips lines that already have a WO.
func init() {
	orm.RegisterObjectAction(planModel, "action_mrp_generate", planGenerate)
}

func planGenerate(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	plan := loadRec(bypass, planModel, id)
	if plan == nil {
		return "", fmt.Errorf("plan not found")
	}
	lines, err := orm.SearchPage(bypass, planLineMdl,
		[][]interface{}{{"plan_id", "=", id}, {"action", "=", "make"}}, 500, 0, "id asc")
	if err != nil || len(lines) == 0 {
		return "No make recommendations on this plan.", nil
	}
	woInst, ok := orm.Registry[woModel]
	if !ok {
		return "", fmt.Errorf("work order model not registered")
	}
	created, skipped := 0, 0
	for _, l := range lines {
		if ormID(l["workorder_id"]) > 0 {
			skipped++
			continue
		}
		pid := ormID(l["product_id"])
		qty := ormFloat(l["net_qty"])
		if pid <= 0 || qty <= 0 {
			continue
		}
		woID, err := orm.Create(bypass, woInst, map[string]interface{}{
			"name":        fmt.Sprintf("MO/PLAN-%d", ormID(l["id"])),
			"product_id":  pid,
			"bom_id":      ormID(l["bom_id"]),
			"product_qty": qty,
			"state":       "draft",
			"origin":      "MRP/" + orm.AsString(plan["name"]),
			"description": "Auto-generated from MRP plan",
		})
		if err != nil {
			continue
		}
		_ = orm.UpdateRecordByID(bypass, planLineMdl, ormID(l["id"]),
			map[string]interface{}{"workorder_id": woID})
		// explode immediately so the WO arrives with component demand ready
		_, _ = woExplode(bypass, "object", woID, nil)
		created++
	}
	return fmt.Sprintf("Generated %d work orders (%d already existed)", created, skipped), nil
}

// ---------------------------------------------------------------------------
// Production costing: component cost (standard_price × required qty) +
// labor cost (routing minutes × workcenter rate) = work order cost.
// ---------------------------------------------------------------------------

func init() {
	deps := []string{"product_qty", "bom_id"}
	orm.RegisterCompute(woModel, "component_cost", deps, computeWOComponentCost)
	orm.RegisterCompute(woModel, "labor_cost", deps, computeWOLaborCost)
	orm.RegisterCompute(woModel, "production_cost", deps, computeWOProductionCost)
	orm.RegisterCompute(woModel, "unit_cost", deps, computeWOUnitCost)
}

func productStdPrice(ctx context.Context, pid int) float64 {
	if pid <= 0 {
		return 0
	}
	rows, err := orm.SearchPage(orm.ContextWithBypass(ctx, true), "product.product",
		[][]interface{}{{"id", "=", pid}}, 1, 0, "id asc")
	if err != nil || len(rows) == 0 {
		return 0
	}
	return ormFloat(rows[0]["standard_price"])
}

func woDemandLines(ctx context.Context, woID int) []map[string]interface{} {
	rows, err := orm.SearchPage(orm.ContextWithBypass(ctx, true), woLineMdl,
		[][]interface{}{{"workorder_id", "=", woID}}, 500, 0, "id asc")
	if err != nil {
		return nil
	}
	return rows
}

// woCostRatio scales per-unit values by product_qty / bom.product_qty.
func woCostRatio(ctx context.Context, rec map[string]interface{}) float64 {
	bomID := ormID(rec["bom_id"])
	if bomID <= 0 {
		return ormFloat(rec["product_qty"])
	}
	bom := loadRec(orm.ContextWithBypass(ctx, true), bomModel, bomID)
	if bom == nil {
		return ormFloat(rec["product_qty"])
	}
	bq := ormFloat(bom["product_qty"])
	if bq <= 0 {
		bq = 1
	}
	return ormFloat(rec["product_qty"]) / bq
}

func computeWOComponentCost(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	total := 0.0
	for _, l := range woDemandLines(ctx, ormID(rec["id"])) {
		total += ormFloat(l["product_qty"]) * productStdPrice(ctx, ormID(l["product_id"]))
	}
	return total, nil
}

func computeWOLaborCost(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	bomID := ormID(rec["bom_id"])
	if bomID <= 0 {
		return 0, nil
	}
	routings, err := orm.SearchPage(orm.ContextWithBypass(ctx, true), "mrp.routing",
		[][]interface{}{{"bom_id", "=", bomID}}, 10, 0, "id asc")
	if err != nil || len(routings) == 0 {
		return 0, nil
	}
	lines, err := orm.SearchPage(orm.ContextWithBypass(ctx, true), "mrp.routing.line",
		[][]interface{}{{"routing_id", "=", ormID(routings[0]["id"])}}, 500, 0, "sequence asc, id asc")
	if err != nil {
		return 0, nil
	}
	ratio := woCostRatio(ctx, rec)
	total := 0.0
	for _, l := range lines {
		wcID := ormID(l["workcenter_id"])
		wcRows, _ := orm.SearchPage(orm.ContextWithBypass(ctx, true), "mrp.workcenter",
			[][]interface{}{{"id", "=", wcID}}, 1, 0, "id asc")
		rate := 0.0
		if len(wcRows) > 0 {
			rate = ormFloat(wcRows[0]["cost_per_hour"])
		}
		total += ormFloat(l["duration_min"]) * ratio / 60 * rate
	}
	return total, nil
}

func computeWOProductionCost(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	cc, _ := computeWOComponentCost(ctx, rec)
	lc, _ := computeWOLaborCost(ctx, rec)
	return ormFloat(cc) + ormFloat(lc), nil
}

func computeWOUnitCost(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	qty := ormFloat(rec["qty_produced"])
	if qty <= 0 {
		qty = ormFloat(rec["product_qty"])
	}
	if qty <= 0 {
		return 0, nil
	}
	tc, _ := computeWOProductionCost(ctx, rec)
	return ormFloat(tc) / qty, nil
}

// ---------------------------------------------------------------------------
// Finite-capacity scheduling: workcenter load from open work orders.
// scheduled_minutes = Σ over open WOs of (routing minutes for this WC × qty
// ratio). utilization_pct compares that against weekly capacity
// (capacity_h × 5 working days × 60).
// ---------------------------------------------------------------------------

func init() {
	orm.RegisterCompute("mrp.workcenter", "scheduled_minutes", []string{"id"}, computeWCScheduledMinutes)
	orm.RegisterCompute("mrp.workcenter", "utilization_pct", []string{"id"}, computeWCUtilization)
}

func wcOpenWorkload(ctx context.Context, wcID int) float64 {
	bypass := orm.ContextWithBypass(ctx, true)
	// open work orders
	wos, err := orm.SearchPage(bypass, woModel,
		[][]interface{}{{"state", "in", []interface{}{"confirmed", "in_progress"}}}, 1000, 0, "id asc")
	if err != nil {
		return 0
	}
	total := 0.0
	for _, wo := range wos {
		bomID := ormID(wo["bom_id"])
		if bomID <= 0 {
			continue
		}
		bom := loadRec(bypass, bomModel, bomID)
		if bom == nil {
			continue
		}
		routings, _ := orm.SearchPage(bypass, "mrp.routing",
			[][]interface{}{{"bom_id", "=", bomID}}, 10, 0, "id asc")
		if len(routings) == 0 {
			continue
		}
		lines, _ := orm.SearchPage(bypass, "mrp.routing.line",
			[][]interface{}{{"routing_id", "=", ormID(routings[0]["id"])}, {"workcenter_id", "=", wcID}}, 100, 0, "id asc")
		bq := ormFloat(bom["product_qty"])
		if bq <= 0 {
			bq = 1
		}
		ratio := ormFloat(wo["product_qty"]) / bq
		for _, l := range lines {
			total += ormFloat(l["duration_min"]) * ratio
		}
	}
	return total
}

func computeWCScheduledMinutes(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return wcOpenWorkload(ctx, ormID(rec["id"])), nil
}

func computeWCUtilization(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	capH := ormFloat(rec["capacity_h"])
	if capH <= 0 {
		return 0, nil
	}
	weeklyCap := capH * 5 * 60 // minutes per week
	if weeklyCap <= 0 {
		return 0, nil
	}
	return wcOpenWorkload(ctx, ormID(rec["id"])) / weeklyCap * 100, nil
}

// ---------------------------------------------------------------------------
// Scrap / yield / backflush.
// ---------------------------------------------------------------------------

func init() {
	orm.RegisterCompute(woModel, "yield_pct", []string{"qty_produced", "qty_scrapped"}, computeWOYield)
	orm.RegisterObjectAction(woModel, "action_mrp_scrap", woScrap)
}

// computeWOYield = produced / (produced + scrapped) × 100.
func computeWOYield(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	prod := ormFloat(rec["qty_produced"])
	scrap := ormFloat(rec["qty_scrapped"])
	total := prod + scrap
	if total <= 0 {
		return 0, nil
	}
	return prod / total * 100, nil
}

// woScrap records scrapped quantity with a reason — feeds yield analytics.
func woScrap(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	wo := loadWO(bypass, id)
	if wo == nil {
		return "", fmt.Errorf("work order not found")
	}
	if st := orm.AsString(wo["state"]); st == "done" || st == "cancelled" {
		return "", fmt.Errorf("cannot scrap a %s work order", st)
	}
	qty := ormFloat(args["qty"])
	if qty <= 0 {
		return "", fmt.Errorf("qty required")
	}
	newScrap := ormFloat(wo["qty_scrapped"]) + qty
	_ = orm.UpdateRecordByID(bypass, woModel, id, map[string]interface{}{"qty_scrapped": newScrap})
	return fmt.Sprintf("Scrapped %.3f — total scrap %.3f (yield %.1f%%)", qty, newScrap, mustYield(wo, newScrap)), nil
}

func mustYield(wo map[string]interface{}, scrap float64) float64 {
	prod := ormFloat(wo["qty_produced"])
	total := prod + scrap
	if total <= 0 {
		return 0
	}
	return prod / total * 100
}

// ---------------------------------------------------------------------------
// Batch/lot traceability: finished lot on produce + genealogy report.
// ---------------------------------------------------------------------------

const lotModel = "stock.lot"

func init() {
	orm.RegisterObjectAction(woModel, "action_mrp_trace", woTrace)
}

// ensureLot finds or creates a stock.lot for a product/batch name.
func ensureLot(ctx context.Context, productID int, lotName string) int {
	rows, err := orm.SearchPage(ctx, lotModel,
		[][]interface{}{{"name", "=", lotName}, {"product_id", "=", productID}}, 1, 0, "id asc")
	if err == nil && len(rows) > 0 {
		return ormID(rows[0]["id"])
	}
	inst, ok := orm.Registry[lotModel]
	if !ok {
		return 0
	}
	id, err := orm.Create(ctx, inst, map[string]interface{}{
		"name":       lotName,
		"product_id": productID,
		"state":      "released",
		"active":     true,
	})
	if err != nil {
		return 0
	}
	return id
}

// woTrace prints the genealogy: finished lot → component lots consumed.
func woTrace(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	wo := loadWO(bypass, id)
	if wo == nil {
		return "", fmt.Errorf("work order not found")
	}
	lotID := ormID(wo["lot_id"])
	lotName := "(none)"
	if lotID > 0 {
		if lr := loadRec(bypass, lotModel, lotID); lr != nil {
			lotName = orm.AsString(lr["name"])
		}
	}
	lines, _ := orm.SearchPage(bypass, woLineMdl,
		[][]interface{}{{"workorder_id", "=", id}}, 500, 0, "sequence asc, id asc")
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Trace WO %s → lot %s\n", orm.AsString(wo["name"]), lotName))
	for _, l := range lines {
		prod := loadRec(bypass, "product.product", ormID(l["product_id"]))
		pname := "?"
		if prod != nil {
			pname = orm.AsString(prod["name"])
		}
		cl := "-"
		if lid := ormID(l["lot_id"]); lid > 0 {
			if lr := loadRec(bypass, lotModel, lid); lr != nil {
				cl = orm.AsString(lr["name"])
			}
		}
		sb.WriteString(fmt.Sprintf("  L%d %s ×%.3f ← lot %s (consumed %.3f)\n",
			int(ormFloat(l["level"])), pname, ormFloat(l["product_qty"]), cl, ormFloat(l["qty_consumed"])))
	}
	return sb.String(), nil
}

// ---------------------------------------------------------------------------
// Production scenarios: in-memory BOM explosion vs stock — no WOs created.
// ---------------------------------------------------------------------------

const (
	scModel = "mrp.scenario"
	scLineM = "mrp.scenario.line"
	quantM  = "stock.quant"
)

func init() {
	orm.RegisterObjectAction(scModel, "action_scenario_run", scenarioRun)
}

// simExplode accumulates component demand for qtyFinished of bomID.
func simExplode(ctx context.Context, bomID int, qtyFinished float64, depth int, visited map[int]bool, demand map[int]float64) {
	if depth > 5 || visited[bomID] {
		return
	}
	visited[bomID] = true
	bom := loadRec(ctx, bomModel, bomID)
	if bom == nil {
		return
	}
	bq := ormFloat(bom["product_qty"])
	if bq <= 0 {
		bq = 1
	}
	ratio := qtyFinished / bq
	lines, err := orm.SearchPage(ctx, bomLineMdl,
		[][]interface{}{{"bom_id", "=", bomID}}, 500, 0, "sequence asc, id asc")
	if err != nil {
		return
	}
	for _, bl := range lines {
		pid := ormID(bl["product_id"])
		need := ormFloat(bl["product_qty"]) * ratio
		if need <= 0 {
			continue
		}
		// if component is itself manufactured, explode it instead
		sub, _ := orm.SearchPage(ctx, bomModel,
			[][]interface{}{{"product_id", "=", pid}, {"state", "=", "active"}}, 1, 0, "id asc")
		if len(sub) > 0 {
			simExplode(ctx, ormID(sub[0]["id"]), need, depth+1, visited, demand)
			continue
		}
		demand[pid] += need
	}
}

// onHand sums stock.quant for a product.
func onHand(ctx context.Context, pid int) float64 {
	rows, err := orm.SearchPage(ctx, quantM,
		[][]interface{}{{"product_id", "=", pid}}, 500, 0, "id asc")
	if err != nil {
		return 0
	}
	total := 0.0
	for _, r := range rows {
		total += ormFloat(r["quantity"])
	}
	return total
}

// scenarioRun simulates every line: explode BOM → net vs stock → shortages.
func scenarioRun(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	sc := loadRec(bypass, scModel, id)
	if sc == nil {
		return "", fmt.Errorf("scenario not found")
	}
	lines, err := orm.SearchPage(bypass, scLineM,
		[][]interface{}{{"scenario_id", "=", id}}, 500, 0, "id asc")
	if err != nil || len(lines) == 0 {
		return "", fmt.Errorf("scenario has no lines")
	}
	_ = orm.UpdateRecordByID(bypass, scModel, id, map[string]interface{}{"state": "running"})
	summary := strings.Builder{}
	totalShort := 0
	for _, l := range lines {
		pid := ormID(l["product_id"])
		qty := ormFloat(l["qty"])
		bomID := ormID(l["bom_id"])
		if bomID <= 0 {
			bs, _ := orm.SearchPage(bypass, bomModel,
				[][]interface{}{{"product_id", "=", pid}, {"state", "=", "active"}}, 1, 0, "id asc")
			if len(bs) > 0 {
				bomID = ormID(bs[0]["id"])
			}
		}
		if bomID <= 0 {
			_ = orm.UpdateRecordByID(bypass, scLineM, ormID(l["id"]),
				map[string]interface{}{"result": "NO ACTIVE BOM — cannot simulate"})
			summary.WriteString(fmt.Sprintf("%s ×%.2f: no BOM\n", pid, qty))
			continue
		}
		demand := map[int]float64{}
		simExplode(bypass, bomID, qty, 0, map[int]bool{}, demand)
		shortages := []string{}
		for cpid, need := range demand {
			have := onHand(bypass, cpid)
			if have < need {
				short := need - have
				totalShort++
				shortages = append(shortages, fmt.Sprintf("comp %d: need %.2f have %.2f (short %.2f)", cpid, need, have, short))
			}
		}
		res := fmt.Sprintf("%d components checked", len(demand))
		if len(shortages) > 0 {
			res = "SHORTAGES: " + strings.Join(shortages, "; ")
		} else {
			res = fmt.Sprintf("OK — %d components, stock sufficient", len(demand))
		}
		_ = orm.UpdateRecordByID(bypass, scLineM, ormID(l["id"]), map[string]interface{}{"result": res})
		summary.WriteString(fmt.Sprintf("product %d ×%.2f → %s\n", pid, qty, res))
	}
	_ = orm.UpdateRecordByID(bypass, scModel, id, map[string]interface{}{
		"state":   "done",
		"run_at":  time.Now().UTC().Format(time.RFC3339),
		"summary": summary.String(),
	})
	return fmt.Sprintf("Scenario complete — %d shortage group(s)", totalShort), nil
}
