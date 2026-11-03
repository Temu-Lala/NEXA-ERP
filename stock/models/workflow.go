package models

import (
	"context"
	"fmt"
	"time"

	"nexa/core/orm"
)

// Workflow actions (blueprint §4.3, §6.2, §11.2, §13). Every stock-changing
// path runs through these registered object actions; none of them edits a
// quant directly — quants move only via the ledger engine in ledger.go.

func init() {
	orm.RegisterObjectAction("stock.picking", "action_confirm", pickingConfirm)
	orm.RegisterObjectAction("stock.picking", "action_transfer", pickingTransfer)
	orm.RegisterObjectAction("stock.picking", "action_cancel", pickingCancel)
	orm.RegisterObjectAction("stock.warehouse", "action_ensure_locations", warehouseEnsureLocations)
	orm.RegisterObjectAction("stock.scrap", "action_validate", scrapValidate)
	orm.RegisterObjectAction("stock.scrap", "action_cancel", scrapCancel)
	orm.RegisterObjectAction("stock.inventory", "action_confirm", inventoryConfirm)
	orm.RegisterObjectAction("stock.inventory", "action_validate", inventoryValidate)
	orm.RegisterObjectAction("stock.inventory", "action_cancel", inventoryCancel)

	orm.RegisterCompute("stock.quant", "available_quantity", []string{"quantity", "reserved_quantity"}, computeQuantAvailable)
	orm.RegisterCompute("stock.inventory.line", "difference_qty", []string{"theoretical_qty", "counted_qty"}, computeInventoryLineDiff)
	orm.RegisterCompute("stock.lot", "qty_remaining", []string{"id"}, computeLotQtyRemaining)
}

// ---------------------------------------------------------------------------
// Compute functions
// ---------------------------------------------------------------------------

func computeQuantAvailable(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	return flt(rec["quantity"]) - flt(rec["reserved_quantity"]), nil
}

func computeInventoryLineDiff(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	return flt(rec["counted_qty"]) - flt(rec["theoretical_qty"]), nil
}

func computeLotQtyRemaining(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	lotID := idOf(rec["id"])
	if lotID <= 0 {
		return 0.0, nil
	}
	var sum float64
	row := orm.DB.QueryRowContext(bypassCtx(ctx),
		`SELECT COALESCE(SUM(quantity),0) FROM stock_quant WHERE lot_id = $1`, lotID)
	if err := row.Scan(&sum); err != nil {
		return 0.0, nil
	}
	return sum, nil
}

// ---------------------------------------------------------------------------
// Shared read helpers
// ---------------------------------------------------------------------------

func loadRec(ctx context.Context, model string, id int) map[string]interface{} {
	rec, err := orm.SearchOne(ctx, model, map[string]interface{}{"id": id})
	if err != nil {
		return nil
	}
	return rec
}

func fetchStock(ctx context.Context, model string, domain [][]interface{}) []map[string]interface{} {
	var out []map[string]interface{}
	offset := 0
	const page = 500
	for {
		rows, err := orm.SearchPage(ctx, model, domain, page, offset, "id asc")
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

func parseDT(s string) time.Time {
	for _, layout := range []string{
		"2006-01-02 15:04:05.999999-07:00",
		"2006-01-02 15:04:05.999999",
		moveDateLayout,
		time.RFC3339Nano,
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// productTracking returns the product's tracking policy ("none", "lot", "serial").
func productTracking(ctx context.Context, productID int) string {
	if productID <= 0 {
		return "none"
	}
	p := loadRec(ctx, "product.product", productID)
	if p == nil {
		return "none"
	}
	tr := str(p["tracking"])
	if tr == "" {
		return "none"
	}
	return tr
}

func allowNegativeFor(ctx context.Context, tx orm.TxWrapper, locID int) bool {
	if locID <= 0 {
		return false
	}
	var ok bool
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(w.allow_negative, false)
		FROM stock_location l LEFT JOIN stock_warehouse w ON w.id = l.warehouse_id
		WHERE l.id = $1`, locID).Scan(&ok)
	if err != nil {
		return false
	}
	return ok
}

// checkLotDisposition blocks quarantined, recalled or expired lots from
// issuing through normal flows (blueprint §7.3, §8.4, §8.5). The block is
// lifted when the destination is a blocked-handling location (quarantine,
// scrap, damaged): recall and expiry workflows must still be able to pull
// the stock out of sellable positions.
func checkLotDisposition(ctx context.Context, lotID int, issuing bool, destLocID int) error {
	if lotID <= 0 || !issuing {
		return nil
	}
	switch locationUsage(ctx, destLocID) {
	case string(LocationUsageQuarantine), string(LocationUsageScrap), string(LocationUsageDamaged):
		return nil
	}
	lot := loadRec(ctx, "stock.lot", lotID)
	if lot == nil {
		return nil
	}
	name := str(lot["name"])
	state := str(lot["state"])
	if state != "" && state != string(LotStateReleased) {
		return fmt.Errorf("lot %s is %s and cannot be issued through normal flows", name, state)
	}
	day := time.Now().Format("2006-01-02")
	if exp := str(lot["expiration_on"]); exp != "" && exp < day {
		return fmt.Errorf("lot %s expired on %s and cannot be issued", name, exp[:10])
	}
	if rem := str(lot["removal_on"]); rem != "" && rem < day {
		return fmt.Errorf("lot %s reached its removal date %s and cannot be issued", name, rem[:10])
	}
	return nil
}

// locationUsage returns the usage code of a location ("" when unknown).
func locationUsage(ctx context.Context, locID int) string {
	if locID <= 0 {
		return ""
	}
	rec := loadRec(ctx, "stock.location", locID)
	if rec == nil {
		return ""
	}
	return str(rec["usage"])
}

// ---------------------------------------------------------------------------
// Picking: confirm (reserve) → transfer (post) → cancel (release)
// ---------------------------------------------------------------------------

func defaultLocations(ctx context.Context, pick map[string]interface{}) (src, dst map[string]interface{}) {
	src, dst = map[string]interface{}{}, map[string]interface{}{}
	op := PickingOperation(str(pick["operation"]))
	whID := 0
	if tid := idOf(pick["picking_type_id"]); tid > 0 {
		if pt := loadRec(ctx, "stock.picking.type", tid); pt != nil {
			whID = idOf(pt["warehouse_id"])
		}
	}
	var wh map[string]interface{}
	if whID > 0 {
		wh = loadRec(ctx, "stock.warehouse", whID)
	}
	pickSrc := idOf(pick["source_location_id"])
	pickDst := idOf(pick["destination_id"])
	anyLoc := func(usage string) int {
		rows := fetchStock(ctx, "stock.location", [][]interface{}{{"usage", "=", usage}})
		if len(rows) > 0 {
			return idOf(rows[0]["id"])
		}
		return 0
	}
	switch op {
	case PickingOperationIncoming:
		if wh != nil {
			src["destination_id"] = idOf(wh["input_location_id"])
		}
		src["source_location_id"] = anyLoc(string(LocationUsageSupplier))
	case PickingOperationOutgoing:
		if wh != nil {
			src["source_location_id"] = idOf(wh["output_location_id"])
		}
		src["destination_id"] = anyLoc(string(LocationUsageCustomer))
	case PickingOperationInternal:
		if wh != nil {
			src["source_location_id"] = idOf(wh["output_location_id"])
			src["destination_id"] = idOf(wh["input_location_id"])
		}
	}
	if pickSrc > 0 {
		delete(src, "source_location_id")
	}
	if pickDst > 0 {
		delete(src, "destination_id")
	}
	_ = dst
	return src, dst
}

func pickingConfirm(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := bypassCtx(ctx)
	pick := loadRec(bypass, "stock.picking", id)
	if pick == nil {
		return "", fmt.Errorf("transfer not found")
	}
	if PickingState(str(pick["state"])) != PickingStateDraft {
		return "", fmt.Errorf("only draft transfers can be confirmed")
	}
	moves := fetchStock(bypass, "stock.move", [][]interface{}{{"picking_id", "=", id}})
	if len(moves) == 0 {
		return "", fmt.Errorf("transfer has no product moves to confirm")
	}

	// Stamp operation defaults on the header and moves from the first save.
	headUpd := map[string]interface{}{"state": string(PickingStateConfirmed)}
	if defs, _ := defaultLocations(bypass, pick); len(defs) > 0 {
		for k, v := range defs {
			if iv, ok := v.(int); ok && iv > 0 && idOf(pick[k]) == 0 {
				headUpd[k] = iv
			}
		}
	}
	op := PickingOperation(str(pick["operation"]))
	reservedAll := op != PickingOperationIncoming
	now := nowStamp()
	for _, mv := range moves {
		mid := idOf(mv["id"])
		state := PickingState(str(mv["state"]))
		if state != PickingStateDraft && state != PickingStateConfirmed {
			continue
		}
		upd := map[string]interface{}{"state": string(PickingStateConfirmed)}
		if str(mv["date_due"]) == "" {
			if sd := str(pick["scheduled_date"]); sd != "" {
				upd["date_due"] = sd
			}
		}
		if idOf(mv["source_location_id"]) == 0 {
			upd["source_location_id"] = idOf(pick["source_location_id"])
		}
		if idOf(mv["destination_id"]) == 0 {
			upd["destination_id"] = idOf(pick["destination_id"])
		}
		if op != PickingOperationIncoming {
			res, fully := reserveMove(bypass, mid, now)
			upd["reserved_quantity"] = res
			if fully {
				upd["state"] = string(PickingStateAssigned)
			} else {
				reservedAll = false
			}
		}
		_ = orm.UpdateRecordByID(bypass, "stock.move", mid, upd)
	}
	if reservedAll && op != PickingOperationIncoming {
		headUpd["state"] = string(PickingStateAssigned)
	}
	if err := orm.UpdateRecordByID(bypass, "stock.picking", id, headUpd); err != nil {
		return "", err
	}
	if headUpd["state"] == string(PickingStateAssigned) {
		return "Transfer confirmed: all products reserved.", nil
	}
	return "Transfer confirmed; some products are not fully available yet.", nil
}

// reserveMove atomically reserves up to the move demand on the source (or
// descendant) quants, FIFO order. Returns reserved total and whether the move
// is fully covered.
func reserveMove(ctx context.Context, moveID int, _ string) (float64, bool) {
	var reserved float64
	var fully bool
	err := withTx(ctx, func(tx orm.TxWrapper) error {
		var productID, srcID, lotID int
		var demand float64
		row := tx.QueryRowContext(ctx,
			`SELECT product_id, COALESCE(source_location_id,0), COALESCE(lot_id,0), quantity
			 FROM stock_move WHERE id = $1`, moveID)
		if err := row.Scan(&productID, &srcID, &lotID, &demand); err != nil {
			return fmt.Errorf("move %d not found", moveID)
		}
		if demand <= qtyEps {
			reserved, fully = 0, true
			return nil
		}
		locIDs, err := locationAndDescendants(ctx, tx, srcID)
		if err != nil {
			return err
		}
		rows, err := lockQuants(ctx, tx, productID, locIDs, lotID)
		if err != nil {
			return err
		}
		// Reservation candidates honour lot disposition and expiry (§8.4–§8.5):
		// quarantined, recalled, expired or removed lots are never proposed.
		lotIDsSeen := make([]int, 0, len(rows))
		seen := map[int]bool{}
		for _, q := range rows {
			if q.lotID.Valid && !seen[int(q.lotID.Int64)] {
				seen[int(q.lotID.Int64)] = true
				lotIDsSeen = append(lotIDsSeen, int(q.lotID.Int64))
			}
		}
		guards := lotGuardInfo(ctx, tx, lotIDsSeen)
		today := time.Now()
		snaps := make([]QuantSnap, 0, len(rows))
		for _, q := range rows {
			var t time.Time
			if q.inDate.Valid {
				t = parseDT(q.inDate.String)
			}
			var g lotGuard
			lot := 0
			if q.lotID.Valid {
				lot = int(q.lotID.Int64)
				gg, ok := guards[lot]
				if !ok || !eligibleForReservation(gg, today) {
					continue
				}
				g = gg
			}
			snaps = append(snaps, QuantSnap{ID: q.id, LotID: lot, Qty: q.qty, Rsvd: q.rsvd, InDate: t, Expiry: g.Expiry})
		}
		allocs, short := planAllocation(demand, snaps)
		for _, a := range allocs {
			if _, err := tx.ExecContext(ctx,
				`UPDATE stock_quant SET reserved_quantity = reserved_quantity + $1 WHERE id = $2`,
				a.Qty, a.QuantID); err != nil {
				return err
			}
			reserved += a.Qty
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE stock_move SET reserved_quantity = $1 WHERE id = $2`, reserved, moveID); err != nil {
			return err
		}
		fully = short <= qtyEps
		return nil
	})
	if err != nil {
		return 0, false
	}
	return reserved, fully
}

func pickingTransfer(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := bypassCtx(ctx)
	pick := loadRec(bypass, "stock.picking", id)
	if pick == nil {
		return "", fmt.Errorf("transfer not found")
	}
	st := PickingState(str(pick["state"]))
	if st != PickingStateConfirmed && st != PickingStateAssigned {
		return "", fmt.Errorf("only confirmed or assigned transfers can be validated")
	}
	op := PickingOperation(str(pick["operation"]))
	allMoves := fetchStock(bypass, "stock.move", [][]interface{}{{"picking_id", "=", id}})
	var moves []map[string]interface{}
	for _, mv := range allMoves {
		switch PickingState(str(mv["state"])) {
		case PickingStateConfirmed, PickingStateAssigned:
			moves = append(moves, mv)
		}
	}
	if len(moves) == 0 {
		return "", fmt.Errorf("no open moves to validate")
	}

	type linePlan struct {
		qty     float64
		lotID   int
		serial  int
		srcLoc  int
		destLoc int
	}

	backorderCount := 0
	postedCount := 0
	err := withTx(bypass, func(tx orm.TxWrapper) error {
		for _, mv := range moves {
			mid := idOf(mv["id"])
			productID := idOf(mv["product_id"])
			srcID := idOf(mv["source_location_id"])
			dstID := idOf(mv["destination_id"])
			demand := flt(mv["quantity"])
			moveLot := idOf(mv["lot_id"])
			moveRsvd := flt(mv["reserved_quantity"])

			// Collect planned operation lines (recorded fulfillments).
			var plans []linePlan
			rows, err := tx.QueryContext(ctx,
				`SELECT COALESCE(quantity,0), COALESCE(lot_id,0), COALESCE(serial_id,0),
				        COALESCE(source_location_id,0), COALESCE(result_location_id,0)
				 FROM stock_move_line WHERE move_id = $1 ORDER BY id`, mid)
			if err != nil {
				return err
			}
			for rows.Next() {
				var p linePlan
				if err := rows.Scan(&p.qty, &p.lotID, &p.serial, &p.srcLoc, &p.destLoc); err != nil {
					rows.Close()
					return err
				}
				if p.qty > qtyEps {
					plans = append(plans, p)
				}
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return err
			}

			doneTotal := 0.0
			for _, p := range plans {
				doneTotal += p.qty
			}

			tracking := productTracking(ctx, productID)
			if len(plans) > 0 {
				// Explicit operations must honour the tracking policy (§4.4 invariants 6–7).
				seenSerials := map[int]bool{}
				for i := range plans {
					p := &plans[i]
					if p.srcLoc == 0 {
						p.srcLoc = srcID
					}
					if p.destLoc == 0 {
						p.destLoc = dstID
					}
					if p.lotID == 0 {
						p.lotID = moveLot
					}
					if tracking == "lot" && p.lotID == 0 {
						return fmt.Errorf("product %d is lot-tracked: record the lot on every operation line", productID)
					}
					if tracking == "serial" {
						if p.serial == 0 {
							return fmt.Errorf("product %d is serial-numbered: record the serial on every operation line", productID)
						}
						if p.qty != 1 {
							return fmt.Errorf("serial-numbered operations must be one unit per line")
						}
						if seenSerials[p.serial] {
							return fmt.Errorf("serial recorded twice on the same move")
						}
						seenSerials[p.serial] = true
					}
					if p.lotID > 0 {
						if err := checkLotDisposition(ctx, p.lotID, op != PickingOperationIncoming, p.destLoc); err != nil {
							return err
						}
					}
				}
			} else {
				// Simple flow: no operations recorded — post demand (incoming)
				// or the reserved amount / demand for other channels and write
				// the ledger lines ourselves so history stays complete.
				q := demand
				if op != PickingOperationIncoming && moveRsvd > qtyEps && moveRsvd < demand {
					q = moveRsvd
				}
				if q <= qtyEps {
					return fmt.Errorf("move %d has nothing reserved or available to post", mid)
				}
				plans = []linePlan{{qty: q, lotID: moveLot, srcLoc: srcID, destLoc: dstID}}
			}

			_, backorder := splitCompletion(demand, doneTotal)
			if backorder > qtyEps {
				if !pickingTypeAllowsBackorder(ctx, idOf(pick["picking_type_id"])) {
					backorder = 0 // short ship closes the move
				} else {
					name := str(mv["name"])
					if name == "" {
						name = str(pick["name"])
					}
					if _, err := insertMove(ctx, tx, map[string]interface{}{
						"name": name, "product_id": productID, "picking_id": id,
						"source_location_id": srcID, "destination_id": dstID,
						"quantity": backorder, "reserved_quantity": 0,
						"state":    string(PickingStateConfirmed),
						"date_due": str(mv["date_due"]), "origin": str(mv["origin"]),
						"lot_id": moveLot, "procurement_group_id": idOf(mv["procurement_group_id"]),
					}); err != nil {
						return err
					}
					backorderCount++
				}
			}

			consumedRsvd := 0.0
			for _, p := range plans {
				if p.lotID > 0 && op != PickingOperationIncoming {
					if err := checkLotDisposition(ctx, p.lotID, true, p.destLoc); err != nil {
						return err
					}
				}
				if op != PickingOperationIncoming {
					release := 0.0
					if rem := moveRsvd - consumedRsvd; rem > qtyEps {
						release = rem
						if release > p.qty {
							release = p.qty
						}
					}
					neg, err := allowNegativeForTx(ctx, tx, p.srcLoc)
					if err != nil {
						return err
					}
					if err := decreaseQuant(ctx, tx, productID, p.srcLoc, p.lotID, p.qty, release, neg); err != nil {
						return err
					}
					consumedRsvd += release
				}
				if err := increaseQuant(ctx, tx, productID, p.destLoc, p.lotID, p.qty); err != nil {
					return err
				}
				if _, err := insertMoveLine(ctx, tx, map[string]interface{}{
					"move_id": mid, "picking_id": id, "product_id": productID,
					"lot_id": p.lotID, "serial_id": p.serial, "quantity": p.qty,
					"source_location_id": p.srcLoc, "result_location_id": p.destLoc,
				}); err != nil {
					return err
				}
			}
			if _, err := tx.ExecContext(ctx,
				`UPDATE stock_move SET state = 'done', date_done = now(), reserved_quantity = GREATEST(0, reserved_quantity - $1) WHERE id = $2`,
				consumedRsvd, mid); err != nil {
				return err
			}
			postedCount++
		}

		// Header closes only when nothing stays open (backorders keep it going).
		var open int
		if err := tx.QueryRowContext(ctx,
			`SELECT count(*) FROM stock_move WHERE picking_id = $1 AND state IN ('draft','confirmed','assigned')`,
			id).Scan(&open); err != nil {
			return err
		}
		if open == 0 {
			if _, err := tx.ExecContext(ctx,
				`UPDATE stock_picking SET state = 'done', date_done = now() WHERE id = $1`, id); err != nil {
				return err
			}
		} else {
			if _, err := tx.ExecContext(ctx,
				`UPDATE stock_picking SET state = 'confirmed' WHERE id = $1`, id); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	// Serial lifecycle after successful posting (§8.3).
	updateSerialStates(bypass, id, op)

	if backorderCount > 0 {
		return fmt.Sprintf("Validated %d move(s); %d backorder move(s) created for the missing quantities.", postedCount, backorderCount), nil
	}
	return fmt.Sprintf("Validated %d move(s); stock ledger posted.", postedCount), nil
}

func allowNegativeForTx(ctx context.Context, tx orm.TxWrapper, locID int) (bool, error) {
	if locID <= 0 {
		return false, nil
	}
	var ok bool
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(w.allow_negative, false)
		FROM stock_location l LEFT JOIN stock_warehouse w ON w.id = l.warehouse_id
		WHERE l.id = $1`, locID).Scan(&ok)
	if err == nil {
		return ok, nil
	}
	return false, nil
}

func pickingTypeAllowsBackorder(ctx context.Context, typeID int) bool {
	if typeID <= 0 {
		return true
	}
	pt := loadRec(ctx, "stock.picking.type", typeID)
	if pt == nil {
		return true
	}
	return orm.AsBool(pt["backorder_ok"])
}

func updateSerialStates(ctx context.Context, pickingID int, op PickingOperation) {
	toState := string(SerialStateDelivered)
	if op == PickingOperationIncoming {
		toState = string(SerialStateAvailable)
	}
	rows, err := orm.DB.QueryContext(ctx,
		`SELECT DISTINCT serial_id, COALESCE(result_location_id,0) FROM stock_move_line WHERE picking_id = $1 AND COALESCE(serial_id,0) > 0`,
		pickingID)
	if err != nil {
		return
	}
	type upd struct {
		id  int
		loc int
	}
	var list []upd
	for rows.Next() {
		var u upd
		if err := rows.Scan(&u.id, &u.loc); err == nil {
			list = append(list, u)
		}
	}
	rows.Close()
	for _, u := range list {
		vals := map[string]interface{}{"state": toState}
		if u.loc > 0 {
			vals["location_id"] = u.loc
		}
		_ = orm.UpdateRecordByID(ctx, "stock.serial", u.id, vals)
	}
}

func pickingCancel(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := bypassCtx(ctx)
	pick := loadRec(bypass, "stock.picking", id)
	if pick == nil {
		return "", fmt.Errorf("transfer not found")
	}
	st := PickingState(str(pick["state"]))
	if st == PickingStateDone {
		return "", fmt.Errorf("posted transfers cannot be cancelled; use a reverse transfer instead")
	}
	allMoves := fetchStock(bypass, "stock.move", [][]interface{}{{"picking_id", "=", id}})
	var moves []map[string]interface{}
	for _, mv := range allMoves {
		switch PickingState(str(mv["state"])) {
		case PickingStateDraft, PickingStateConfirmed, PickingStateAssigned:
			moves = append(moves, mv)
		}
	}
	err := withTx(bypass, func(tx orm.TxWrapper) error {
		for _, mv := range moves {
			mid := idOf(mv["id"])
			reserved := flt(mv["reserved_quantity"])
			if reserved > qtyEps {
				if err := releaseReservation(ctx, tx, mid, reserved); err != nil {
					return err
				}
			}
			if _, err := tx.ExecContext(ctx,
				`UPDATE stock_move SET state = 'cancelled', reserved_quantity = 0 WHERE id = $1`, mid); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, `UPDATE stock_picking SET state = 'cancelled' WHERE id = $1`, id)
		return err
	})
	if err != nil {
		return "", err
	}
	return "Transfer cancelled; reservations released.", nil
}

// releaseReservation gives reserved quantities back to availability, newest
// slice first (mirror of the FIFO allocation), within the caller's tx.
func releaseReservation(ctx context.Context, tx orm.TxWrapper, moveID int, reserved float64) error {
	var productID, srcID, lotID int
	if err := tx.QueryRowContext(ctx,
		`SELECT product_id, COALESCE(source_location_id,0), COALESCE(lot_id,0) FROM stock_move WHERE id = $1`,
		moveID).Scan(&productID, &srcID, &lotID); err != nil {
		return err
	}
	locIDs, err := locationAndDescendants(ctx, tx, srcID)
	if err != nil {
		return err
	}
	rows, err := lockQuants(ctx, tx, productID, locIDs, lotID)
	if err != nil {
		return err
	}
	remaining := reserved
	for i := len(rows) - 1; i >= 0 && remaining > qtyEps; i-- {
		q := rows[i]
		take := q.rsvd
		if take > remaining {
			take = remaining
		}
		if take <= qtyEps {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE stock_quant SET reserved_quantity = GREATEST(0, reserved_quantity - $1) WHERE id = $2`,
			take, q.id); err != nil {
			return err
		}
		remaining -= take
	}
	return nil
}

// ---------------------------------------------------------------------------
// Warehouse bootstrap
// ---------------------------------------------------------------------------

func warehouseEnsureLocations(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := bypassCtx(ctx)
	wh := loadRec(bypass, "stock.warehouse", id)
	if wh == nil {
		return "", fmt.Errorf("warehouse not found")
	}
	code := str(wh["code"])
	if code == "" {
		code = str(wh["name"])
	}
	rootID := 0
	roots := fetchStock(bypass, "stock.location", [][]interface{}{
		{"warehouse_id", "=", id}, {"usage", "=", string(LocationUsageView)},
	})
	if len(roots) > 0 {
		rootID = idOf(roots[0]["id"])
	} else {
		inst, ok := orm.Registry["stock.location"]
		if !ok {
			return "", fmt.Errorf("location model unavailable")
		}
		newID, err := orm.Create(bypass, inst, map[string]interface{}{
			"name": code, "usage": string(LocationUsageView), "warehouse_id": id,
		})
		if err != nil {
			return "", err
		}
		rootID = newID
	}
	specs := []struct {
		field string
		name  string
		usage string
	}{
		{"input_location_id", code + "/INPUT", string(LocationUsageInternal)},
		{"output_location_id", code + "/OUTPUT", string(LocationUsageInternal)},
		{"return_location_id", code + "/RETURNS", string(LocationUsageReturns)},
		{"scrap_location_id", code + "/SCRAP", string(LocationUsageScrap)},
		{"transit_location_id", code + "/TRANSIT", string(LocationUsageTransit)},
	}
	createdCount := 0
	inst, ok := orm.Registry["stock.location"]
	if !ok {
		return "", fmt.Errorf("location model unavailable")
	}
	for _, sp := range specs {
		cur := idOf(wh[sp.field])
		if cur > 0 {
			if rec := loadRec(bypass, "stock.location", cur); rec != nil {
				continue
			}
		}
		locID, err := orm.Create(bypass, inst, map[string]interface{}{
			"name": sp.name, "usage": sp.usage, "warehouse_id": id, "parent_id": rootID,
		})
		if err != nil {
			return "", err
		}
		_ = orm.UpdateRecordByID(bypass, "stock.warehouse", id, map[string]interface{}{sp.field: locID})
		createdCount++
	}
	if createdCount == 0 {
		return "Standard locations already configured.", nil
	}
	return fmt.Sprintf("Created %d standard location(s) for %s.", createdCount, code), nil
}

// ---------------------------------------------------------------------------
// Scrap
// ---------------------------------------------------------------------------

func scrapValidate(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := bypassCtx(ctx)
	rec := loadRec(bypass, "stock.scrap", id)
	if rec == nil {
		return "", fmt.Errorf("scrap order not found")
	}
	if ScrapState(str(rec["state"])) != ScrapStateDraft {
		return "", fmt.Errorf("only draft scrap orders can be validated")
	}
	qty := flt(rec["quantity"])
	if qty <= qtyEps {
		return "", fmt.Errorf("scrap quantity must be positive")
	}
	productID := idOf(rec["product_id"])
	srcID := idOf(rec["location_id"])
	lotID := idOf(rec["lot_id"])
	scrapID := idOf(rec["scrap_location_id"])
	if scrapID <= 0 {
		scrapID = resolveScrapLocation(bypass, srcID)
	}
	if scrapID <= 0 {
		return "", fmt.Errorf("no scrap location available; run Initialize Locations on the warehouse first")
	}
	if tracking := productTracking(ctx, productID); tracking == "lot" && lotID == 0 {
		return "", fmt.Errorf("product is lot-tracked: specify the lot to scrap")
	}
	if err := checkLotDisposition(ctx, lotID, true, scrapID); err != nil {
		return "", err
	}
	err := withTx(bypass, func(tx orm.TxWrapper) error {
		if err := decreaseQuant(ctx, tx, productID, srcID, lotID, qty, 0, false); err != nil {
			return err
		}
		if err := increaseQuant(ctx, tx, productID, scrapID, lotID, qty); err != nil {
			return err
		}
		moveID, err := insertMove(ctx, tx, map[string]interface{}{
			"name": "Scrap " + str(rec["name"]), "product_id": productID,
			"source_location_id": srcID, "destination_id": scrapID,
			"quantity": qty, "state": string(PickingStateDone),
			"lot_id": lotID, "origin": "SCR:" + str(rec["name"]),
		})
		if err != nil {
			return err
		}
		_, err = insertMoveLine(ctx, tx, map[string]interface{}{
			"move_id": moveID, "product_id": productID, "lot_id": lotID,
			"quantity": qty, "source_location_id": srcID, "result_location_id": scrapID,
		})
		return err
	})
	if err != nil {
		return "", err
	}
	if lotID > 0 && trackingIsSerial(bypass, productID) {
		_ = orm.UpdateRecordByID(bypass, "stock.serial", lotID, map[string]interface{}{"state": string(SerialStateScrapped)})
	}
	if err := orm.UpdateRecordByID(bypass, "stock.scrap", id, map[string]interface{}{
		"state": string(ScrapStateDone), "scrap_location_id": scrapID,
	}); err != nil {
		return "", err
	}
	return "Stock scrapped and posted to the scrap location.", nil
}

func trackingIsSerial(ctx context.Context, productID int) bool {
	return productTracking(ctx, productID) == "serial"
}

func resolveScrapLocation(ctx context.Context, fromLocID int) int {
	rows := fetchStock(ctx, "stock.location", [][]interface{}{{"usage", "=", string(LocationUsageScrap)}})
	if len(rows) > 0 {
		return idOf(rows[0]["id"])
	}
	return 0
}

func scrapCancel(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := bypassCtx(ctx)
	rec := loadRec(bypass, "stock.scrap", id)
	if rec == nil {
		return "", fmt.Errorf("scrap order not found")
	}
	if ScrapState(str(rec["state"])) != ScrapStateDraft {
		return "", fmt.Errorf("only draft scrap orders can be cancelled")
	}
	if err := orm.UpdateRecordByID(bypass, "stock.scrap", id,
		map[string]interface{}{"state": string(ScrapStateCancelled)}); err != nil {
		return "", err
	}
	return "Scrap order cancelled.", nil
}

// ---------------------------------------------------------------------------
// Cycle count / inventory adjustment (blueprint §11)
// ---------------------------------------------------------------------------

// inventoryConfirm freezes the expected quantities: existing blank lines are
// filled from the quant projection, and a location-scoped count seeds lines
// for every quant found in the count location. Blind counts keep the
// theoretical value server-side only (view hides it from counters).
func inventoryConfirm(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := bypassCtx(ctx)
	rec := loadRec(bypass, "stock.inventory", id)
	if rec == nil {
		return "", fmt.Errorf("inventory not found")
	}
	if InventoryState(str(rec["state"])) != InventoryStateDraft {
		return "", fmt.Errorf("only draft inventories can be confirmed")
	}
	locID := idOf(rec["location_id"])
	lines := fetchStock(bypass, "stock.inventory.line", [][]interface{}{{"inventory_id", "=", id}})
	inst, ok := orm.Registry["stock.inventory.line"]
	if !ok {
		return "", fmt.Errorf("inventory line model unavailable")
	}
	seen := map[string]bool{}
	seeded := 0
	for _, ln := range lines {
		key := fmt.Sprintf("%d-%d-%d", idOf(ln["product_id"]), idOf(ln["location_id"]), idOf(ln["lot_id"]))
		seen[key] = true
	}
	if locID > 0 {
		quants := fetchStock(bypass, "stock.quant", [][]interface{}{{"location_id", "=", locID}})
		for _, q := range quants {
			key := fmt.Sprintf("%d-%d-%d", idOf(q["product_id"]), locID, idOf(q["lot_id"]))
			if seen[key] {
				continue
			}
			_, _ = orm.Create(bypass, inst, map[string]interface{}{
				"inventory_id": id, "product_id": idOf(q["product_id"]),
				"location_id": locID, "lot_id": idOf(q["lot_id"]),
				"theoretical_qty": flt(q["quantity"]), "counted_qty": 0,
			})
			seen[key] = true
			seeded++
		}
	}
	for _, ln := range lines {
		exp := expectedQty(bypass, idOf(ln["product_id"]), key2(idOf(ln["location_id"]), locID), idOf(ln["lot_id"]))
		_ = orm.UpdateRecordByID(bypass, "stock.inventory.line", idOf(ln["id"]),
			map[string]interface{}{"theoretical_qty": exp})
	}
	if err := orm.UpdateRecordByID(bypass, "stock.inventory", id,
		map[string]interface{}{"state": string(InventoryStateProgress)}); err != nil {
		return "", err
	}
	return fmt.Sprintf("Count opened with %d line(s); %d new line(s) seeded from quants.", len(lines)+seeded, seeded), nil
}

func key2(locID, fallback int) int {
	if locID > 0 {
		return locID
	}
	return fallback
}

func expectedQty(ctx context.Context, productID, locID, lotID int) float64 {
	if productID <= 0 || locID <= 0 {
		return 0
	}
	var sum float64
	err := orm.DB.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(quantity),0) FROM stock_quant
		 WHERE product_id = $1 AND location_id = $2 AND COALESCE(lot_id,0) = COALESCE($3,0)`,
		productID, locID, nullableInt(lotID)).Scan(&sum)
	if err != nil {
		return 0
	}
	return sum
}

func nullableInt(v int) interface{} {
	if v == 0 {
		return nil
	}
	return v
}

// inventoryValidate posts the counted variances through adjustment moves —
// a count never overwrites balances directly (blueprint §11.2 invariant).
func inventoryValidate(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := bypassCtx(ctx)
	rec := loadRec(bypass, "stock.inventory", id)
	if rec == nil {
		return "", fmt.Errorf("inventory not found")
	}
	if InventoryState(str(rec["state"])) != InventoryStateProgress {
		return "", fmt.Errorf("only counted inventories in progress can be validated")
	}
	lines := fetchStock(bypass, "stock.inventory.line", [][]interface{}{{"inventory_id", "=", id}})
	ref := str(rec["name"])
	posted := 0
	err := withTx(bypass, func(tx orm.TxWrapper) error {
		for _, ln := range lines {
			diff := flt(ln["counted_qty"]) - flt(ln["theoretical_qty"])
			if diff > -qtyEps && diff < qtyEps {
				continue
			}
			productID := idOf(ln["product_id"])
			locID := key2(idOf(ln["location_id"]), idOf(rec["location_id"]))
			lotID := idOf(ln["lot_id"])
			if locID <= 0 || productID <= 0 {
				continue
			}
			if diff > 0 {
				if err := increaseQuant(ctx, tx, productID, locID, lotID, diff); err != nil {
					return err
				}
				mvID, err := insertMove(ctx, tx, map[string]interface{}{
					"name": "Count gain " + ref, "product_id": productID,
					"destination_id": locID, "quantity": diff,
					"state": string(PickingStateDone), "lot_id": lotID, "origin": "INV:" + ref,
				})
				if err != nil {
					return err
				}
				_, err = insertMoveLine(ctx, tx, map[string]interface{}{
					"move_id": mvID, "product_id": productID, "lot_id": lotID,
					"quantity": diff, "result_location_id": locID,
				})
				if err != nil {
					return err
				}
			} else {
				// Count-driven shortage: post the write-off even if the
				// projection says there is less (projection is what we fix).
				if err := decreaseQuant(ctx, tx, productID, locID, lotID, -diff, 0, true); err != nil {
					return err
				}
				mvID, err := insertMove(ctx, tx, map[string]interface{}{
					"name": "Count loss " + ref, "product_id": productID,
					"source_location_id": locID, "quantity": -diff,
					"state": string(PickingStateDone), "lot_id": lotID, "origin": "INV:" + ref,
				})
				if err != nil {
					return err
				}
				_, err = insertMoveLine(ctx, tx, map[string]interface{}{
					"move_id": mvID, "product_id": productID, "lot_id": lotID,
					"quantity": -diff, "source_location_id": locID,
				})
				if err != nil {
					return err
				}
			}
			if err := orm.UpdateRecordByID(bypass, "stock.inventory.line", idOf(ln["id"]),
				map[string]interface{}{"theoretical_qty": flt(ln["counted_qty"]), "difference_qty": 0}); err != nil {
				return err
			}
			posted++
		}
		_, err := tx.ExecContext(ctx, `UPDATE stock_inventory SET state = 'done' WHERE id = $1`, id)
		return err
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Count validated: %d variance(s) posted as adjustment moves.", posted), nil
}

func inventoryCancel(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := bypassCtx(ctx)
	rec := loadRec(bypass, "stock.inventory", id)
	if rec == nil {
		return "", fmt.Errorf("inventory not found")
	}
	st := InventoryState(str(rec["state"]))
	if st != InventoryStateDraft && st != InventoryStateProgress {
		return "", fmt.Errorf("only open inventories can be cancelled")
	}
	if err := orm.UpdateRecordByID(bypass, "stock.inventory", id,
		map[string]interface{}{"state": string(InventoryStateCancelled)}); err != nil {
		return "", err
	}
	return "Inventory cancelled.", nil
}
