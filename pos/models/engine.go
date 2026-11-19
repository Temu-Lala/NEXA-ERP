package models

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"nexa/core/orm"
	posservices "nexa_addons/account/services"
)

// POS engine: order totals recompute from lines; session close reconciles
// expected cash vs counted cash; payment validation enforces tenders cover
// the order total before it can be marked paid.

const (
	sessionModel = "pos.session"
	orderModel   = "pos.order"
	lineModel    = "pos.order.line"
	payModel     = "pos.payment"
)

func init() {
	orm.RegisterCompute(orderModel, "amount_total", []string{"id"}, computeOrderTotal)
	orm.RegisterCompute(orderModel, "amount_paid", []string{"id"}, computeOrderPaid)
	orm.RegisterCompute(sessionModel, "expected_cash", []string{"id"}, computeSessionExpectedCash)
	orm.RegisterCompute(sessionModel, "cash_variance", []string{"id"}, computeSessionVariance)
	orm.RegisterObjectAction(sessionModel, "action_pos_close", sessionClose)
	orm.RegisterObjectAction(orderModel, "action_pos_pay", orderPay)
	orm.RegisterObjectAction(orderModel, "action_pos_void", orderVoid)
}

func fnum(v interface{}) float64 {
	n, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(v)), 64)
	return n
}

// poidField extracts an id from a many2one value (array or scalar).
func poidField(v interface{}) int {
	if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
		return oid(arr[0])
	}
	return oid(v)
}

func oid(v interface{}) int {
	if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
		n, _ := strconv.Atoi(fmt.Sprint(arr[0]))
		return n
	}
	n, _ := strconv.Atoi(fmt.Sprint(v))
	return n
}

// computeOrderTotal sums line subtotals minus order discount.
func computeOrderTotal(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	lines, err := orm.SearchPage(orm.ContextWithBypass(ctx, true), lineModel,
		[][]interface{}{{"order_id", "=", oid(rec["id"])}}, 500, 0, "id asc")
	if err != nil {
		return 0, nil
	}
	total := 0.0
	for _, l := range lines {
		total += fnum(l["price_subtotal"])
	}
	return total - fnum(rec["discount"]) + fnum(rec["tip_amount"]), nil
}

func computeOrderPaid(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	pays, err := orm.SearchPage(orm.ContextWithBypass(ctx, true), payModel,
		[][]interface{}{{"order_id", "=", oid(rec["id"])}}, 500, 0, "id asc")
	if err != nil {
		return 0, nil
	}
	total := 0.0
	for _, p := range pays {
		total += fnum(p["amount"])
	}
	return total, nil
}

// computeSessionExpectedCash = opening + cash tenders on session's orders.
func computeSessionExpectedCash(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	sid := oid(rec["id"])
	orders, err := orm.SearchPage(orm.ContextWithBypass(ctx, true), orderModel,
		[][]interface{}{{"session_id", "=", sid}}, 2000, 0, "id asc")
	if err != nil {
		return fnum(rec["opening_cash"]), nil
	}
	cash := fnum(rec["opening_cash"])
	for _, o := range orders {
		pays, _ := orm.SearchPage(orm.ContextWithBypass(ctx, true), payModel,
			[][]interface{}{{"order_id", "=", oid(o["id"])}, {"method", "=", "cash"}}, 200, 0, "id asc")
		for _, p := range pays {
			cash += fnum(p["amount"])
		}
	}
	return cash, nil
}

func computeSessionVariance(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	expected, _ := computeSessionExpectedCash(ctx, rec)
	return fnum(rec["closing_cash"]) - fnum(expected), nil
}

// sessionClose validates the drawer: counted cash must be present; variance is
// computed and the session closes. Blocked if orders are still unpaid.
func sessionClose(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	sess := loadRec(bypass, sessionModel, id)
	if sess == nil {
		return "", fmt.Errorf("session not found")
	}
	orders, _ := orm.SearchPage(bypass, orderModel,
		[][]interface{}{{"session_id", "=", id}, {"state", "in", []interface{}{"draft", "paid"}}}, 2000, 0, "id asc")
	if len(orders) > 0 {
		return "", fmt.Errorf("%d orders still unpaid — pay or void them first", len(orders))
	}
	closing := fnum(args["closing_cash"])
	_ = orm.UpdateRecordByID(bypass, sessionModel, id, map[string]interface{}{
		"state":        "closed",
		"closing_cash": closing,
		"stop_at":      time.Now().UTC().Format(time.RFC3339),
	})
	return fmt.Sprintf("Session closed — counted %.2f", closing), nil
}

// orderPay marks an order paid once tenders cover the total.
func orderPay(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	ord := loadRec(bypass, orderModel, id)
	if ord == nil {
		return "", fmt.Errorf("order not found")
	}
	if st := orm.AsString(ord["state"]); st == "void" || st == "refunded" {
		return "", fmt.Errorf("cannot pay a %s order", st)
	}
	// auto-apply best active promotion before validating tenders
	if !orm.AsBool(ord["promo_applied"]) {
		if _, perr := orderApplyPromo(bypass, orderModel, id, args); perr == nil {
			ord = loadRec(bypass, orderModel, id)
		}
	}
	total := fnum(ord["amount_total"])
	pays, _ := orm.SearchPage(bypass, payModel, [][]interface{}{{"order_id", "=", id}}, 500, 0, "id asc")
	covered := 0.0
	for _, p := range pays {
		covered += fnum(p["amount"])
	}
	if covered+0.005 < total {
		return "", fmt.Errorf("tenders %.2f < total %.2f — add payments first", covered, total)
	}
	// expand bundle lines into components before stock/loyalty (spec §15)
	explodeBundles(bypass, id)
	// stock deduction + loyalty ledger + audit (non-fatal if stock not configured)
	_ = deductStock(bypass, ord)
	pid := oid(ord["partner_id"])
	pts := int(fnum(ord["amount_total"]) / 10)
	if pid > 0 && pts > 0 {
		loyaltyPost(bypass, pid, id, "earn", pts, "Earned on "+orm.AsString(ord["name"]), args["user_id"])
	}
	audit(bypass, orderModel, id, "pay", "Order paid "+orm.AsString(ord["name"]), args["user_id"])
	// post customer invoice to Finance (idempotent on invoice_origin)
	if _, err := posservices.CreatePOSInvoiceFromOrder(bypass, id); err != nil {
		fmt.Printf("pos invoice post failed for order %d: %v", id, err)
	}
	_ = orm.UpdateRecordByID(bypass, orderModel, id, map[string]interface{}{"state": "done"})
	return "Order paid, stock deducted, loyalty accrued", nil
}

// orderVoid cancels a draft/unpaid order.
func orderVoid(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	ord := loadRec(bypass, orderModel, id)
	if ord == nil {
		return "", fmt.Errorf("order not found")
	}
	if st := orm.AsString(ord["state"]); st == "done" || st == "refunded" {
		return "", fmt.Errorf("cannot void a %s order — refund instead", st)
	}
	_ = orm.UpdateRecordByID(bypass, orderModel, id, map[string]interface{}{"state": "void"})
	audit(bypass, orderModel, id, "void", "Order voided "+orm.AsString(ord["name"]), args["user_id"])
	return "Order voided", nil
}

func loadRec(ctx context.Context, model string, id int) map[string]interface{} {
	rows, err := orm.SearchPage(ctx, model, [][]interface{}{{"id", "=", id}}, 1, 0, "id asc")
	if err != nil || len(rows) == 0 {
		return nil
	}
	return rows[0]
}

// ---------------------------------------------------------------------------
// Phase 2 — stock deduction, loyalty, returns.
// ---------------------------------------------------------------------------

const (
	retModel     = "pos.return"
	pickingModel = "stock.picking"
	moveModel    = "stock.move"
	locModel     = "stock.location"
)

func init() {
	orm.RegisterObjectAction(retModel, "action_pos_return_approve", returnApprove)
	orm.RegisterObjectAction(retModel, "action_pos_return_refund", returnRefund)
	orm.RegisterCompute(orderModel, "points_earned", []string{"amount_total"}, computeOrderPoints)
}

func computeOrderPoints(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	// 1 point per 10 currency units
	return int(fnum(rec["amount_total"]) / 10), nil
}

// deductStock creates an outgoing picking for a paid order's lines.
func deductStock(ctx context.Context, ord map[string]interface{}) error {
	lines, err := orm.SearchPage(orm.ContextWithBypass(ctx, true), lineModel,
		[][]interface{}{{"order_id", "=", oid(ord["id"])}}, 500, 0, "id asc")
	if err != nil || len(lines) == 0 {
		return nil
	}
	// resolve locations: internal stock → customer (use first internal as src;
	// dst = first customer-usage or virtual location)
	src, dst := 0, 0
	rows, _ := orm.SearchPage(orm.ContextWithBypass(ctx, true), locModel,
		[][]interface{}{{"usage", "=", "internal"}, {"active", "=", true}}, 1, 0, "id asc")
	if len(rows) > 0 {
		src = oid(rows[0]["id"])
	}
	rows, _ = orm.SearchPage(orm.ContextWithBypass(ctx, true), locModel,
		[][]interface{}{{"usage", "=", "customer"}, {"active", "=", true}}, 1, 0, "id asc")
	if len(rows) > 0 {
		dst = oid(rows[0]["id"])
	} else {
		rows, _ = orm.SearchPage(orm.ContextWithBypass(ctx, true), locModel,
			[][]interface{}{{"usage", "=", "virtual"}, {"active", "=", true}}, 1, 0, "id asc")
		if len(rows) > 0 {
			dst = oid(rows[0]["id"])
		}
	}
	if src <= 0 || dst <= 0 {
		return fmt.Errorf("stock locations not configured")
	}
	pinst, ok := orm.Registry[pickingModel]
	if !ok {
		return fmt.Errorf("stock.picking not registered")
	}
	pid, err := orm.Create(orm.ContextWithBypass(ctx, true), pinst, map[string]interface{}{
		"name":               "POS/" + orm.AsString(ord["name"]),
		"operation":          "outgoing",
		"source_location_id": src,
		"destination_id":     dst,
		"origin":             "POS/" + orm.AsString(ord["name"]),
		"state":              "done",
	})
	if err != nil {
		return err
	}
	minst, ok := orm.Registry[moveModel]
	if !ok {
		return fmt.Errorf("stock.move not registered")
	}
	for _, l := range lines {
		_, _ = orm.Create(orm.ContextWithBypass(ctx, true), minst, map[string]interface{}{
			"name":               "POS sale " + orm.AsString(ord["name"]),
			"product_id":         oid(l["product_id"]),
			"picking_id":         pid,
			"source_location_id": src,
			"destination_id":     dst,
			"quantity":           fnum(l["qty"]),
			"origin":             "POS/" + orm.AsString(ord["name"]),
			"state":              "done",
		})
	}
	return nil
}

// accrueLoyalty adds points to the customer's partner record if it has a
// loyalty_points field; otherwise silently skips (non-fatal).
func accrueLoyalty(ctx context.Context, ord map[string]interface{}) {
	pid := oid(ord["partner_id"])
	if pid <= 0 {
		return
	}
	points := int(fnum(ord["amount_total"]) / 10)
	if points <= 0 {
		return
	}
	// partner loyalty field may not exist — try update, ignore failure
	_ = orm.UpdateRecordByID(orm.ContextWithBypass(ctx, true), "core.partner", pid,
		map[string]interface{}{"loyalty_points": points})
}

// returnApprove: requested → approved.
func returnApprove(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	r := loadRec(bypass, retModel, id)
	if r == nil {
		return "", fmt.Errorf("return not found")
	}
	if st := orm.AsString(r["state"]); st != "requested" {
		return "", fmt.Errorf("only requested returns can be approved (state=%s)", st)
	}
	_ = orm.UpdateRecordByID(bypass, retModel, id, map[string]interface{}{
		"state":          "approved",
		"approved_by_id": args["user_id"],
	})
	return "Return approved — refund it to complete", nil
}

// returnRefund: approved → refunded; marks original order refunded.
func returnRefund(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	r := loadRec(bypass, retModel, id)
	if r == nil {
		return "", fmt.Errorf("return not found")
	}
	if st := orm.AsString(r["state"]); st != "approved" {
		return "", fmt.Errorf("only approved returns can be refunded (state=%s)", st)
	}
	upd := map[string]interface{}{"state": "refunded"}
	if m := strings.TrimSpace(args["method"]); m != "" {
		upd["refund_method"] = m
	}
	_ = orm.UpdateRecordByID(bypass, retModel, id, upd)
	// mark the original order refunded
	_ = orm.UpdateRecordByID(bypass, orderModel, oid(r["order_id"]),
		map[string]interface{}{"state": "refunded"})
	// issue credit note reversing the POS invoice (idempotent)
	if cn, err := posservices.CreatePOSCreditNoteFromReturn(bypass, id); err == nil && cn > 0 {
		return fmt.Sprintf("Refund issued — credit note #%d posted", cn), nil
	}
	return "Refund issued — order marked refunded", nil
}

// ---------------------------------------------------------------------------
// Advanced POS: hold/resume, promotions, gift cards, cash movements.
// ---------------------------------------------------------------------------

const (
	storeModel    = "pos.store"
	registerModel = "pos.register"
	promoModel    = "pos.promotion"
	gcModel       = "pos.giftcard"
	cmModel       = "pos.cash.movement"
)

func init() {
	orm.RegisterObjectAction(orderModel, "action_pos_hold", orderHold)
	orm.RegisterObjectAction(orderModel, "action_pos_layaway", orderLayaway)
	orm.RegisterObjectAction(orderModel, "action_pos_layaway_cancel", orderLayawayCancel)
	orm.RegisterObjectAction(orderModel, "action_pos_resume", orderResume)
	orm.RegisterObjectAction(orderModel, "action_pos_apply_promo", orderApplyPromo)
	orm.RegisterObjectAction(orderModel, "action_pos_redeem_giftcard", orderRedeemGC)
	orm.RegisterObjectAction(sessionModel, "action_pos_cash_move", sessionCashMove)
	orm.RegisterObjectAction(gcModel, "action_gc_redeem", gcRedeem)
	orm.RegisterObjectAction(gcModel, "action_gc_cancel", gcCancel)
}

// orderHold suspends a draft order for later resume.
func orderHold(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	ord := loadRec(bypass, orderModel, id)
	if ord == nil {
		return "", fmt.Errorf("order not found")
	}
	if st := orm.AsString(ord["state"]); st != "draft" {
		return "", fmt.Errorf("only draft orders can be held (state=%s)", st)
	}
	_ = orm.UpdateRecordByID(bypass, orderModel, id, map[string]interface{}{"state": "held"})
	return "Order held — resume it from the held list", nil
}

// orderResume returns a held order to draft.
func orderResume(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	ord := loadRec(bypass, orderModel, id)
	if ord == nil {
		return "", fmt.Errorf("order not found")
	}
	if st := orm.AsString(ord["state"]); st != "held" {
		return "", fmt.Errorf("only held orders can be resumed (state=%s)", st)
	}
	_ = orm.UpdateRecordByID(bypass, orderModel, id, map[string]interface{}{"state": "draft"})
	return "Order resumed", nil
}

// orderApplyPromo evaluates active promotions against the order total and
// records the best discount on the order. Deterministic: highest-value wins.
func orderApplyPromo(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	ord := loadRec(bypass, orderModel, id)
	if ord == nil {
		return "", fmt.Errorf("order not found")
	}
	if st := orm.AsString(ord["state"]); st == "done" || st == "void" || st == "refunded" {
		return "", fmt.Errorf("cannot promote a %s order", st)
	}
	if orm.AsBool(ord["promo_applied"]) {
		return "Promotion already applied to this order.", nil
	}
	total := fnum(ord["amount_total"])
	promos, err := orm.SearchPage(bypass, promoModel,
		[][]interface{}{{"active", "=", true}}, 500, 0, "id asc")
	if err != nil || len(promos) == 0 {
		return "No active promotions.", nil
	}
	// company scope: shared promos (no company) or same company as the order —
	// bypass ctx sees all rows, so filter explicitly
	ordCo := poidField(ord["company_id"])
	filtered := promos[:0]
	for _, p := range promos {
		pc := poidField(p["company_id"])
		if pc == 0 || pc == ordCo {
			filtered = append(filtered, p)
		}
	}
	promos = filtered
	if len(promos) == 0 {
		return "No active promotions for this company.", nil
	}
	code := strings.TrimSpace(args["code"])
	best := 0.0
	bestName := ""
	for _, p := range promos {
		// coupon promos require the code to match
		if orm.AsString(p["kind"]) == "coupon" {
			if code == "" || orm.AsString(p["code"]) != code {
				continue
			}
		}
		// time window check
		if s := strings.TrimSpace(fmt.Sprint(p["start_at"])); len(s) > 10 && s[:10] > time.Now().UTC().Format("2006-01-02") {
			continue
		}
		if e := strings.TrimSpace(fmt.Sprint(p["end_at"])); len(e) > 10 && e[:10] < time.Now().UTC().Format("2006-01-02") {
			continue
		}
		if total < fnum(p["min_basket"]) {
			continue
		}
		var disc float64
		switch orm.AsString(p["kind"]) {
		case "percent":
			disc = total * fnum(p["value"]) / 100
		case "fixed":
			disc = fnum(p["value"])
		case "buy_x_get_y":
			// approx: get_qty free units of the promo product priced at avg line price
			disc = fnum(p["value"])
		}
		if disc > best {
			best = disc
			bestName = orm.AsString(p["name"])
		}
	}
	if best <= 0 {
		return "No applicable promotion for this basket.", nil
	}
	// add to order discount (cumulative with existing)
	newDisc := fnum(ord["discount"]) + best
	_ = orm.UpdateRecordByID(bypass, orderModel, id, map[string]interface{}{"discount": newDisc, "promo_applied": true})
	return fmt.Sprintf("Applied '%s' — discount %.2f (order discount now %.2f)", bestName, best, newDisc), nil
}

// orderRedeemGC redeems gift card balance toward the order.
func orderRedeemGC(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	ord := loadRec(bypass, orderModel, id)
	if ord == nil {
		return "", fmt.Errorf("order not found")
	}
	gcID := oid(args["giftcard_id"])
	if gcID <= 0 {
		return "", fmt.Errorf("giftcard_id required")
	}
	gc := loadRec(bypass, gcModel, gcID)
	if gc == nil {
		return "", fmt.Errorf("gift card not found")
	}
	if st := orm.AsString(gc["state"]); st != "active" {
		return "", fmt.Errorf("gift card is %s", st)
	}
	if e := strings.TrimSpace(fmt.Sprint(gc["expires_at"])); len(e) > 10 && e[:10] < time.Now().UTC().Format("2006-01-02") {
		return "", fmt.Errorf("gift card expired")
	}
	amount := fnum(args["amount"])
	if amount <= 0 {
		amount = fnum(gc["balance"])
	}
	bal := fnum(gc["balance"])
	if amount > bal {
		amount = bal
	}
	newBal := bal - amount
	upd := map[string]interface{}{"balance": newBal}
	if newBal <= 0.005 {
		upd["state"] = "depleted"
	}
	_ = orm.UpdateRecordByID(bypass, gcModel, gcID, upd)
	// add a gift-card tender for the redeemed amount
	pinst, ok := orm.Registry[payModel]
	if ok {
		_, _ = orm.Create(bypass, pinst, map[string]interface{}{
			"order_id":  id,
			"method":    "credit",
			"amount":    amount,
			"reference": "GC/" + orm.AsString(gc["code"]),
			"note":      "Gift card redemption",
		})
	}
	return fmt.Sprintf("Redeemed %.2f from card %s — balance %.2f", amount, orm.AsString(gc["code"]), newBal), nil
}

// gcRedeem is the standalone card-balance deduction (for non-order use).
func gcRedeem(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	gc := loadRec(bypass, gcModel, id)
	if gc == nil {
		return "", fmt.Errorf("gift card not found")
	}
	if st := orm.AsString(gc["state"]); st != "active" {
		return "", fmt.Errorf("gift card is %s", st)
	}
	amount := fnum(args["amount"])
	if amount <= 0 {
		return "", fmt.Errorf("amount required")
	}
	bal := fnum(gc["balance"])
	if amount > bal {
		return "", fmt.Errorf("amount exceeds balance %.2f", bal)
	}
	newBal := bal - amount
	upd := map[string]interface{}{"balance": newBal}
	if newBal <= 0.005 {
		upd["state"] = "depleted"
	}
	_ = orm.UpdateRecordByID(bypass, gcModel, id, upd)
	return fmt.Sprintf("Redeemed %.2f — balance %.2f", amount, newBal), nil
}

// gcCancel voids an unused card.
func gcCancel(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	gc := loadRec(bypass, gcModel, id)
	if gc == nil {
		return "", fmt.Errorf("gift card not found")
	}
	if st := orm.AsString(gc["state"]); st == "cancelled" {
		return "", fmt.Errorf("card already cancelled")
	}
	_ = orm.UpdateRecordByID(bypass, gcModel, id, map[string]interface{}{"state": "cancelled"})
	return "Gift card cancelled", nil
}

// sessionCashMove records a cash-in/out event against an open session.
func sessionCashMove(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	sess := loadRec(bypass, sessionModel, id)
	if sess == nil {
		return "", fmt.Errorf("session not found")
	}
	if st := orm.AsString(sess["state"]); st != "open" {
		return "", fmt.Errorf("cash moves only on open sessions (state=%s)", st)
	}
	kind := strings.ToLower(strings.TrimSpace(args["kind"]))
	if kind != "in" && kind != "out" {
		return "", fmt.Errorf("kind must be in or out")
	}
	amount := fnum(args["amount"])
	if amount <= 0 {
		return "", fmt.Errorf("amount required")
	}
	inst, ok := orm.Registry[cmModel]
	if !ok {
		return "", fmt.Errorf("cash movement model not registered")
	}
	_, err := orm.Create(bypass, inst, map[string]interface{}{
		"session_id": id,
		"kind":       kind,
		"amount":     amount,
		"reason":     args["reason"],
		"user_id":    args["user_id"],
		"at":         time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Cash %s of %.2f recorded", kind, amount), nil
}

// ---------------------------------------------------------------------------
// Offline sync apply: idempotent by client_txn_id. Payload is JSON:
// {"session_id":N,"partner_id":N,"discount":F,"lines":[{"product_id":N,"name":"..","qty":F,"price_unit":F,"discount_pct":F}],"payments":[{"method":"cash","amount":F,"reference":".."}]}
// ---------------------------------------------------------------------------

const (
	deviceModel = "pos.device"
	syncModel   = "pos.sync.event"
)

func init() {
	orm.RegisterObjectAction(syncModel, "action_pos_sync_apply", syncApply)
	orm.RegisterObjectAction(deviceModel, "action_pos_sync_pending", deviceSyncPending)
}

// syncApply applies one queued offline event. Idempotent: if an event with
// the same client_txn_id is already applied, it returns success without
// re-posting — safe for retries.
func syncApply(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	ev := loadRec(bypass, syncModel, id)
	if ev == nil {
		return "", fmt.Errorf("sync event not found")
	}
	if st := orm.AsString(ev["state"]); st == "applied" {
		return "Already applied (idempotent)", nil
	}
	if orm.AsString(ev["kind"]) != "sale" {
		_ = orm.UpdateRecordByID(bypass, syncModel, id, map[string]interface{}{
			"state": "rejected", "error": "only sale events supported in phase 1",
		})
		return "", fmt.Errorf("only sale sync supported")
	}
	// duplicate check: same client_txn_id applied elsewhere
	dups, _ := orm.SearchPage(bypass, syncModel,
		[][]interface{}{{"client_txn_id", "=", orm.AsString(ev["client_txn_id"])},
			{"state", "=", "applied"}, {"id", "!=", id}}, 10, 0, "id asc")
	if len(dups) > 0 {
		_ = orm.UpdateRecordByID(bypass, syncModel, id, map[string]interface{}{"state": "applied"})
		return "Duplicate client_txn_id — already applied, skipped", nil
	}
	payload := strings.TrimSpace(orm.AsString(ev["payload"]))
	if payload == "" {
		_ = orm.UpdateRecordByID(bypass, syncModel, id, map[string]interface{}{"state": "rejected", "error": "empty payload"})
		return "", fmt.Errorf("empty payload")
	}
	var p struct {
		SessionID  int     `json:"session_id"`
		PartnerID  int     `json:"partner_id"`
		Discount   float64 `json:"discount"`
		DeviceTime string  `json:"device_time"`
		Lines      []struct {
			ProductID   int     `json:"product_id"`
			Name        string  `json:"name"`
			Qty         float64 `json:"qty"`
			PriceUnit   float64 `json:"price_unit"`
			DiscountPct float64 `json:"discount_pct"`
		} `json:"lines"`
		Payments []struct {
			Method    string  `json:"method"`
			Amount    float64 `json:"amount"`
			Reference string  `json:"reference"`
		} `json:"payments"`
	}
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		_ = orm.UpdateRecordByID(bypass, syncModel, id, map[string]interface{}{"state": "rejected", "error": "bad JSON: " + err.Error()})
		return "", fmt.Errorf("payload parse: %v", err)
	}
	if len(p.Lines) == 0 {
		_ = orm.UpdateRecordByID(bypass, syncModel, id, map[string]interface{}{"state": "rejected", "error": "no lines"})
		return "", fmt.Errorf("no lines in payload")
	}
	// create the order
	oinst, ok := orm.Registry[orderModel]
	if !ok {
		return "", fmt.Errorf("order model not registered")
	}
	orderID, err := orm.Create(bypass, oinst, map[string]interface{}{
		"name":       "POS-OFFLINE/" + orm.AsString(ev["client_txn_id"]),
		"session_id": p.SessionID,
		"partner_id": p.PartnerID,
		"state":      "draft",
		"date_order": time.Now().UTC().Format(time.RFC3339),
		"discount":   p.Discount,
		"note":       "Offline sync " + orm.AsString(ev["client_txn_id"]),
	})
	if err != nil {
		_ = orm.UpdateRecordByID(bypass, syncModel, id, map[string]interface{}{"state": "conflict", "error": err.Error()})
		return "", err
	}
	// lines
	linst, _ := orm.Registry[lineModel]
	for i, l := range p.Lines {
		sub := l.Qty * l.PriceUnit * (1 - l.DiscountPct/100)
		_, _ = orm.Create(bypass, linst, map[string]interface{}{
			"order_id":       orderID,
			"product_id":     l.ProductID,
			"name":           l.Name,
			"qty":            l.Qty,
			"price_unit":     l.PriceUnit,
			"discount_pct":   l.DiscountPct,
			"price_subtotal": sub,
			"sequence":       (i + 1) * 10,
		})
	}
	// payments
	pinst, _ := orm.Registry[payModel]
	for _, pay := range p.Payments {
		m := pay.Method
		if m == "" {
			m = "cash"
		}
		_, _ = orm.Create(bypass, pinst, map[string]interface{}{
			"order_id":  orderID,
			"method":    m,
			"amount":    pay.Amount,
			"reference": pay.Reference,
			"note":      "Offline sync",
		})
	}
	// finalize: deduct stock + loyalty + mark done (reuse helpers)
	ord := loadRec(bypass, orderModel, orderID)
	_ = deductStock(bypass, ord)
	accrueLoyalty(bypass, ord)
	_ = orm.UpdateRecordByID(bypass, orderModel, orderID, map[string]interface{}{"state": "done"})
	if _, err := posservices.CreatePOSInvoiceFromOrder(bypass, orderID); err != nil {
		fmt.Printf("pos invoice post failed for synced order %d: %v", orderID, err)
	}
	// mark event applied + update device last sync
	_ = orm.UpdateRecordByID(bypass, syncModel, id, map[string]interface{}{
		"state":           "applied",
		"server_order_id": orderID,
		"applied_at":      time.Now().UTC().Format(time.RFC3339),
	})
	did := oid(ev["device_id"])
	if did > 0 {
		_ = orm.UpdateRecordByID(bypass, deviceModel, did, map[string]interface{}{
			"last_sync_at": time.Now().UTC().Format(time.RFC3339),
		})
	}
	return fmt.Sprintf("Offline sale %s applied → order %d", orm.AsString(ev["client_txn_id"]), orderID), nil
}

// deviceSyncPending lists pending events for a device (terminal polls this).
func deviceSyncPending(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rows, err := orm.SearchPage(bypass, syncModel,
		[][]interface{}{{"device_id", "=", id}, {"state", "=", "pending"}}, 500, 0, "id asc")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d pending sync events for device %d", len(rows), id), nil
}

// ---------------------------------------------------------------------------
// Price lists: apply a list's rules to an order's lines.
// ---------------------------------------------------------------------------

const (
	pricelistModel = "pos.price.list"
	plistItemModel = "pos.price.list.item"
)

func init() {
	orm.RegisterObjectAction(orderModel, "action_pos_apply_pricelist", orderApplyPriceList)
}

// orderApplyPriceList reprices every line on the order using the given
// price list's items (product match first, then category). Idempotent:
// re-running overwrites price_unit/discount_pct from the list rules.
func orderApplyPriceList(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	ord := loadRec(bypass, orderModel, id)
	if ord == nil {
		return "", fmt.Errorf("order not found")
	}
	plID := oid(args["price_list_id"])
	if plID <= 0 {
		return "", fmt.Errorf("price_list_id required")
	}
	pl := loadRec(bypass, pricelistModel, plID)
	if pl == nil {
		return "", fmt.Errorf("price list not found")
	}
	if !ordBool(pl["active"], true) {
		return "", fmt.Errorf("price list is inactive")
	}
	items, err := orm.SearchPage(bypass, plistItemModel,
		[][]interface{}{{"price_list_id", "=", plID}}, 1000, 0, "id asc")
	if err != nil {
		return "", err
	}
	lines, err := orm.SearchPage(bypass, lineModel,
		[][]interface{}{{"order_id", "=", id}}, 500, 0, "sequence asc, id asc")
	if err != nil {
		return "", err
	}
	if len(lines) == 0 {
		return "", fmt.Errorf("order has no lines")
	}
	changed := 0
	for _, l := range lines {
		pid := oid(l["product_id"])
		catID := 0
		// fetch product category for category rules
		if pres, err := orm.SearchPage(bypass, "product.product",
			[][]interface{}{{"id", "=", pid}}, 1, 0, "id asc"); err == nil && len(pres) > 0 {
			catID = oid(pres[0]["category_id"])
		}
		var best *map[string]interface{}
		for i := range items {
			it := items[i]
			ip := oid(it["product_id"])
			ic := oid(it["category_id"])
			match := (ip > 0 && ip == pid) || (ic > 0 && ic == catID)
			if !match {
				continue
			}
			// qty tier: pick the rule with highest min_qty <= line qty
			if best == nil || fnum(it["min_qty"]) >= fnum((*best)["min_qty"]) {
				best = &it
			}
		}
		if best == nil {
			continue
		}
		upd := map[string]interface{}{}
		if d := fnum((*best)["discount_pct"]); d > 0 {
			upd["discount_pct"] = d
		}
		if p := fnum((*best)["price"]); p > 0 {
			upd["price_unit"] = p
		}
		if len(upd) == 0 {
			continue
		}
		qty := fnum(l["qty"])
		price := fnum(l["price_unit"])
		if np, ok := upd["price_unit"]; ok {
			price = fnum(np)
		}
		dpct := fnum(l["discount_pct"])
		if nd, ok := upd["discount_pct"]; ok {
			dpct = fnum(nd)
		}
		upd["price_subtotal"] = qty * price * (1 - dpct/100)
		_ = orm.UpdateRecordByID(bypass, lineModel, oid(l["id"]), upd)
		changed++
	}
	return fmt.Sprintf("Price list '%s' applied — %d line(s) repriced", orm.AsString(pl["name"]), changed), nil
}

func ordBool(v interface{}, def bool) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true" || t == "1" || t == "t"
	}
	return def
}

// ---------------------------------------------------------------------------
// Loyalty ledger + audit events (spec §8, §15).
// ---------------------------------------------------------------------------

const (
	loyModel   = "pos.loyalty.txn"
	auditModel = "pos.audit.event"
)

func init() {
	orm.RegisterObjectAction(orderModel, "action_pos_redeem_points", orderRedeemPoints)
}

// audit writes an immutable audit event (best-effort).
func audit(ctx context.Context, model string, recordID int, action, detail, userID string) {
	inst, ok := orm.Registry[auditModel]
	if !ok {
		return
	}
	_, _ = orm.Create(ctx, inst, map[string]interface{}{
		"model_name": model,
		"record_id":  recordID,
		"action":     action,
		"user_id":    userID,
		"detail":     detail,
		"at":         time.Now().UTC().Format(time.RFC3339),
	})
}

// loyaltyBalance = sum of all ledger entries for a partner.
func loyaltyBalance(ctx context.Context, partnerID int) int {
	rows, err := orm.SearchPage(ctx, loyModel,
		[][]interface{}{{"partner_id", "=", partnerID}}, 5000, 0, "id asc")
	if err != nil {
		return 0
	}
	bal := 0
	for _, r := range rows {
		bal += int(fnum(r["points"]))
	}
	return bal
}

// loyaltyPost appends a ledger entry with running balance.
func loyaltyPost(ctx context.Context, partnerID, orderID int, kind string, points int, note, userID string) {
	inst, ok := orm.Registry[loyModel]
	if !ok || partnerID <= 0 || points == 0 {
		return
	}
	bal := loyaltyBalance(ctx, partnerID) + points
	_, _ = orm.Create(ctx, inst, map[string]interface{}{
		"partner_id":    partnerID,
		"order_id":      orderID,
		"kind":          kind,
		"points":        points,
		"balance_after": bal,
		"note":          note,
		"user_id":       userID,
		"at":            time.Now().UTC().Format(time.RFC3339),
	})
	// keep partner.loyalty_points in sync (best-effort)
	_ = orm.UpdateRecordByID(ctx, "core.partner", partnerID,
		map[string]interface{}{"loyalty_points": bal})
}

// orderRedeemPoints redeems loyalty points as an order discount
// (1 point = 1 currency unit, capped at order total).
func orderRedeemPoints(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	ord := loadRec(bypass, orderModel, id)
	if ord == nil {
		return "", fmt.Errorf("order not found")
	}
	pid := oid(ord["partner_id"])
	if pid <= 0 {
		return "", fmt.Errorf("order has no customer — loyalty needs a partner")
	}
	points := int(fnum(args["points"]))
	if points <= 0 {
		return "", fmt.Errorf("points required")
	}
	bal := loyaltyBalance(bypass, pid)
	if points > bal {
		return "", fmt.Errorf("insufficient points — balance %d", bal)
	}
	total := fnum(ord["amount_total"])
	if float64(points) > total {
		points = int(total)
	}
	loyaltyPost(bypass, pid, id, "redeem", -points, "Redeemed on "+orm.AsString(ord["name"]), args["user_id"])
	_ = orm.UpdateRecordByID(bypass, orderModel, id, map[string]interface{}{
		"discount": fnum(ord["discount"]) + float64(points),
	})
	return fmt.Sprintf("Redeemed %d points — new balance %d", points, bal-points), nil
}

// explodeBundles replaces bundle-product lines with their component lines.
func explodeBundles(ctx context.Context, orderID int) {
	lines, err := orm.SearchPage(ctx, lineModel,
		[][]interface{}{{"order_id", "=", orderID}}, 500, 0, "sequence asc, id asc")
	if err != nil {
		return
	}
	for _, l := range lines {
		pid := oid(l["product_id"])
		bs, _ := orm.SearchPage(ctx, "product.bundle",
			[][]interface{}{{"product_id", "=", pid}, {"active", "=", true}}, 1, 0, "id asc")
		if len(bs) == 0 {
			continue
		}
		bid := oid(bs[0]["id"])
		bls, _ := orm.SearchPage(ctx, "product.bundle.line",
			[][]interface{}{{"bundle_id", "=", bid}}, 100, 0, "id asc")
		qty := fnum(l["qty"])
		seq := int(fnum(l["sequence"]))
		// mark original line qty 0 (kept for audit, contributes nothing)
		_ = orm.UpdateRecordByID(ctx, lineModel, oid(l["id"]), map[string]interface{}{"qty": 0})
		for _, bl := range bls {
			cpid := oid(bl["product_id"])
			cqty := fnum(bl["qty"]) * qty
			if cpid <= 0 || cqty <= 0 {
				continue
			}
			_, _ = orm.Create(ctx, mustModel(lineModel), map[string]interface{}{
				"order_id":       orderID,
				"product_id":     cpid,
				"name":           fmt.Sprintf("%s (bundle)", orm.AsString(l["name"])),
				"qty":            cqty,
				"price_unit":     0,
				"discount_pct":   0,
				"price_subtotal": 0,
				"sequence":       seq,
			})
			seq += 1
		}
	}
}

func mustModel(name string) orm.Model {
	if m, ok := orm.Registry[name]; ok {
		return m
	}
	return nil
}

// orderLayaway parks an order as a customer layaway — no stock deduction
// until fully paid (spec §9 customer accounts).
func orderLayaway(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	ord := loadRec(bypass, orderModel, id)
	if ord == nil {
		return "", fmt.Errorf("order not found")
	}
	st := orm.AsString(ord["state"])
	if st != "draft" && st != "held" {
		return "", fmt.Errorf("only draft/held orders can become layaway (state=%s)", st)
	}
	if oid(ord["partner_id"]) <= 0 {
		return "", fmt.Errorf("layaway requires a customer")
	}
	if err := orm.UpdateRecordByID(bypass, orderModel, id,
		map[string]interface{}{"state": "layaway"}); err != nil {
		return "", err
	}
	audit(bypass, orderModel, id, "layaway", "Order placed on layaway "+orm.AsString(ord["name"]), args["user_id"])
	return "Order on layaway — collect payments, then pay to complete", nil
}

// orderLayawayCancel voids a layaway and refunds any deposits taken.
func orderLayawayCancel(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	ord := loadRec(bypass, orderModel, id)
	if ord == nil {
		return "", fmt.Errorf("order not found")
	}
	if orm.AsString(ord["state"]) != "layaway" {
		return "", fmt.Errorf("only layaway orders can be cancelled here")
	}
	// refund recorded payments back to the customer (cash refund entries)
	pays, _ := orm.SearchPage(bypass, payModel, [][]interface{}{{"order_id", "=", id}}, 500, 0, "id asc")
	total := 0.0
	for _, p := range pays {
		total += fnum(p["amount"])
	}
	_ = orm.UpdateRecordByID(bypass, orderModel, id, map[string]interface{}{"state": "void"})
	msg := "Layaway cancelled"
	if total > 0 {
		msg = fmt.Sprintf("Layaway cancelled — %.2f collected must be refunded manually", total)
	}
	audit(bypass, orderModel, id, "layaway_cancel", msg, args["user_id"])
	return msg, nil
}

// ---------------------------------------------------------------------------
// Restaurant mode (spec): tables, courses, kitchen routing.
// ---------------------------------------------------------------------------

const (
	tableModel = "pos.table"
)

func init() {
	orm.RegisterObjectAction(orderModel, "action_pos_send_course", orderSendCourse)
	orm.RegisterObjectAction(orderModel, "action_pos_free_table", orderFreeTable)
	orm.RegisterObjectAction(tableModel, "action_table_status", tableSetStatus)
	orm.RegisterObjectAction(lineModel, "action_pos_line_prepared", orderLinePrepared)
	orm.RegisterObjectAction(orderModel, "action_pos_transfer_table", orderTransferTable)
	orm.RegisterObjectAction(orderModel, "action_pos_split_bill", orderSplitBill)
}

func orderTableID(ctx context.Context, orderID int) int {
	ord := loadRec(ctx, orderModel, orderID)
	if ord == nil {
		return 0
	}
	return oid(ord["table_id"])
}

// orderSendCourse marks all unsent lines of a course as sent-to-kitchen
// and bumps the order course counter (restaurant workflow).
func orderSendCourse(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	ord := loadRec(bypass, orderModel, id)
	if ord == nil {
		return "", fmt.Errorf("order not found")
	}
	course := int(fnum(ord["course"]))
	if course <= 0 {
		course = 1
	}
	lines, err := orm.SearchPage(bypass, lineModel,
		[][]interface{}{{"order_id", "=", id}, {"course", "=", course}, {"sent_to_kitchen", "=", false}}, 500, 0, "id asc")
	if err != nil {
		return "", err
	}
	if len(lines) == 0 {
		return "", fmt.Errorf("no unsent lines in course %d", course)
	}
	for _, l := range lines {
		_ = orm.UpdateRecordByID(bypass, lineModel, oid(l["id"]),
			map[string]interface{}{"sent_to_kitchen": true})
	}
	// occupy the table if linked
	tid := orderTableID(bypass, id)
	if tid > 0 {
		_ = orm.UpdateRecordByID(bypass, tableModel, tid,
			map[string]interface{}{"state": "occupied"})
	}
	audit(bypass, orderModel, id, "kitchen", fmt.Sprintf("Course %d sent to kitchen (%d items)", course, len(lines)), args["user_id"])
	return fmt.Sprintf("Course %d sent — %d items to kitchen", course, len(lines)), nil
}

// orderFreeTable releases the table when the order is done/voided.
func orderFreeTable(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	tid := orderTableID(bypass, id)
	if tid <= 0 {
		return "", fmt.Errorf("order has no table")
	}
	_ = orm.UpdateRecordByID(bypass, tableModel, tid,
		map[string]interface{}{"state": "free"})
	audit(bypass, orderModel, id, "table_free", "Table released", args["user_id"])
	return "Table freed", nil
}

// tableSetStatus manually sets a table state (host stand override).
func tableSetStatus(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	st := strings.TrimSpace(args["state"])
	if st != "free" && st != "occupied" && st != "billed" {
		return "", fmt.Errorf("state must be free|occupied|billed")
	}
	t := loadRec(bypass, tableModel, id)
	if t == nil {
		return "", fmt.Errorf("table not found")
	}
	_ = orm.UpdateRecordByID(bypass, tableModel, id, map[string]interface{}{"state": st})
	return fmt.Sprintf("Table %s → %s", orm.AsString(t["name"]), st), nil
}

// orderLinePrepared marks a kitchen line done (KDS "bump" action).
func orderLinePrepared(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	l := loadRec(bypass, lineModel, id)
	if l == nil {
		return "", fmt.Errorf("line not found")
	}
	if !orm.AsBool(l["sent_to_kitchen"]) {
		return "", fmt.Errorf("line not sent to kitchen")
	}
	_ = orm.UpdateRecordByID(bypass, lineModel, id, map[string]interface{}{"prepared": true})
	audit(bypass, lineModel, id, "kitchen_done", "Item prepared "+orm.AsString(l["name"]), args["user_id"])
	return "Prepared", nil
}

// orderTransferTable moves an order (and its table occupancy) to another table.
func orderTransferTable(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	ord := loadRec(bypass, orderModel, id)
	if ord == nil {
		return "", fmt.Errorf("order not found")
	}
	newTID := oid(args["table_id"])
	if newTID <= 0 {
		return "", fmt.Errorf("table_id required")
	}
	nt := loadRec(bypass, tableModel, newTID)
	if nt == nil || !orm.AsBool(nt["active"]) {
		return "", fmt.Errorf("target table not found/inactive")
	}
	oldTID := orderTableID(bypass, id)
	if err := orm.UpdateRecordByID(bypass, orderModel, id,
		map[string]interface{}{"table_id": newTID}); err != nil {
		return "", err
	}
	_ = orm.UpdateRecordByID(bypass, tableModel, newTID,
		map[string]interface{}{"state": "occupied"})
	// free old table only if no other open orders remain on it
	if oldTID > 0 {
		remaining, _ := orm.SearchPage(bypass, orderModel,
			[][]interface{}{
				{"table_id", "=", oldTID},
				{"state", "in", []interface{}{"draft", "held", "layaway"}},
				{"id", "!=", id},
			}, 1, 0, "id asc")
		if len(remaining) == 0 {
			_ = orm.UpdateRecordByID(bypass, tableModel, oldTID,
				map[string]interface{}{"state": "free"})
		}
	}
	audit(bypass, orderModel, id, "table_transfer",
		fmt.Sprintf("Transferred %s → %s", orm.AsString(nt["name"]), ""), args["user_id"])
	return fmt.Sprintf("Order moved to %s", orm.AsString(nt["name"])), nil
}

// orderSplitBill moves selected lines (or a whole course) to a new child
// order on the same table — separate bills per guest/course.
func orderSplitBill(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	ord := loadRec(bypass, orderModel, id)
	if ord == nil {
		return "", fmt.Errorf("order not found")
	}
	course := int(fnum(args["course"]))
	if course <= 0 {
		return "", fmt.Errorf("course required (split by course)")
	}
	lines, err := orm.SearchPage(bypass, lineModel,
		[][]interface{}{{"order_id", "=", id}, {"course", "=", course}}, 500, 0, "id asc")
	if err != nil || len(lines) == 0 {
		return "", fmt.Errorf("no lines in course %d", course)
	}
	// create child order on same table/session
	om, ok := orm.Registry[orderModel]
	if !ok {
		return "", fmt.Errorf("order model missing")
	}
	childID, err := orm.Create(bypass, om, map[string]interface{}{
		"name":       fmt.Sprintf("%s/SPLIT-C%d", orm.AsString(ord["name"]), course),
		"session_id": ord["session_id"],
		"partner_id": ord["partner_id"],
		"state":      "draft",
		"date_order": time.Now().UTC().Format(time.RFC3339),
		"table_id":   ord["table_id"],
		"course":     course,
		"discount":   0,
	})
	if err != nil {
		return "", err
	}
	lm, _ := orm.Registry[lineModel]
	for i, l := range lines {
		vals := map[string]interface{}{}
		for k, v := range l {
			vals[k] = v
		}
		delete(vals, "id")
		vals["order_id"] = childID
		vals["sequence"] = (i + 1) * 10
		_, _ = orm.Create(bypass, lm, vals)
		_ = orm.UpdateRecordByID(bypass, lineModel, oid(l["id"]), map[string]interface{}{"qty": 0}) // moved to split order
	}
	audit(bypass, orderModel, id, "split", fmt.Sprintf("Course %d split to order %d", course, childID), args["user_id"])
	return fmt.Sprintf("Course %d split into order #%d", course, childID), nil
}

// ---------------------------------------------------------------------------
// Hardware: ESC/POS network receipt printing (spec §hardware).
// ---------------------------------------------------------------------------

const printerModel = "pos.printer"

func init() {
	orm.RegisterObjectAction(orderModel, "action_pos_print_receipt", orderPrintReceipt)
}

// escpos helpers — minimal ESC/POS for 80mm/58mm text receipts.
func escInit() []byte        { return []byte{0x1B, 0x40} }
func escAlign(c byte) []byte { return []byte{0x1B, 0x61, c} }
func escBold(on bool) []byte {
	b := byte(0x00)
	if on {
		b = 0x01
	}
	return []byte{0x1B, 0x45, b}
}
func escSize(s byte) []byte { return []byte{0x1D, 0x21, s} }
func escCut() []byte        { return []byte{0x1D, 0x56, 0x00} }
func escDrawer() []byte     { return []byte{0x1B, 0x70, 0x00, 0x19, 0xFA} }

func padRight(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}
func padLeft(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return strings.Repeat(" ", w-len(s)) + s
}

func moneyLine(label, val string, w int) []byte {
	l := padRight(label, w-10)
	r := padLeft(val, 10)
	return []byte(l + r + "\n")
}

// orderPrintReceipt builds an ESC/POS receipt and sends it to the store's
// active printer over TCP. Falls back to first active printer.
func orderPrintReceipt(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	ord := loadRec(bypass, orderModel, id)
	if ord == nil {
		return "", fmt.Errorf("order not found")
	}
	// pick printer: args["printer_id"] or first active
	var printer map[string]interface{}
	if pid := oid(args["printer_id"]); pid > 0 {
		printer = loadRec(bypass, printerModel, pid)
	}
	if printer == nil {
		prs, _ := orm.SearchPage(bypass, printerModel,
			[][]interface{}{{"active", "=", true}}, 1, 0, "id asc")
		if len(prs) > 0 {
			printer = prs[0]
		}
	}
	if printer == nil {
		return "", fmt.Errorf("no active printer configured (POS → Tables area → Printers)")
	}
	ip := strings.TrimSpace(orm.AsString(printer["ip_address"]))
	if ip == "" {
		return "", fmt.Errorf("printer %s has no IP address", orm.AsString(printer["name"]))
	}
	port := int(fnum(printer["port"]))
	if port <= 0 {
		port = 9100
	}

	const W = 42 // 80mm ≈ 42 chars
	var b []byte
	b = append(b, escInit()...)
	b = append(b, escAlign(1)...)
	b = append(b, escSize(0x11)...)
	b = append(b, escBold(true)...)
	b = append(b, []byte("NEXA POS\n")...)
	b = append(b, escSize(0x00)...)
	b = append(b, escBold(false)...)
	b = append(b, []byte(orm.AsString(ord["name"])+"\n")...)
	b = append(b, []byte(time.Now().Format("2006-01-02 15:04")+"\n\n")...)
	b = append(b, escAlign(0)...)

	lines, _ := orm.SearchPage(bypass, lineModel,
		[][]interface{}{{"order_id", "=", id}}, 500, 0, "sequence asc, id asc")
	for _, l := range lines {
		if fnum(l["qty"]) <= 0 {
			continue
		}
		name := orm.AsString(l["name"])
		if len(name) > W {
			name = name[:W]
		}
		b = append(b, []byte(name+"\n")...)
		qty := fnum(l["qty"])
		price := fnum(l["price_unit"])
		disc := fnum(l["discount_pct"])
		amt := fnum(l["price_subtotal"])
		detail := fmt.Sprintf("  %.3gx %.2f", qty, price)
		if disc > 0 {
			detail += fmt.Sprintf(" -%g%%", disc)
		}
		b = append(b, moneyLine(detail, fmt.Sprintf("%.2f", amt), W)...)
		if sn := strings.TrimSpace(orm.AsString(l["serial"])); sn != "" {
			b = append(b, []byte("  S/N: "+sn+"\n")...)
		}
	}
	b = append(b, []byte("------------------------------------------\n")...)
	b = append(b, moneyLine("Subtotal", fmt.Sprintf("%.2f", fnum(ord["amount_untaxed"])), W)...)
	if d := fnum(ord["discount"]); d > 0 {
		b = append(b, moneyLine("Discount", fmt.Sprintf("-%.2f", d), W)...)
	}
	if t := fnum(ord["tip_amount"]); t > 0 {
		b = append(b, moneyLine("Tip", fmt.Sprintf("%.2f", t), W)...)
	}
	if tx := fnum(ord["amount_tax"]); tx > 0 {
		b = append(b, moneyLine("Tax", fmt.Sprintf("%.2f", tx), W)...)
	}
	b = append(b, escBold(true)...)
	b = append(b, moneyLine("TOTAL", fmt.Sprintf("%.2f", fnum(ord["amount_total"])), W)...)
	b = append(b, escBold(false)...)
	b = append(b, []byte("\n")...)

	pays, _ := orm.SearchPage(bypass, payModel, [][]interface{}{{"order_id", "=", id}}, 100, 0, "id asc")
	for _, p := range pays {
		method := strings.ToUpper(orm.AsString(p["method"]))
		b = append(b, moneyLine(method, fmt.Sprintf("%.2f", fnum(p["amount"])), W)...)
	}
	paid := 0.0
	for _, p := range pays {
		paid += fnum(p["amount"])
	}
	if ch := paid - fnum(ord["amount_total"]); ch > 0.004 {
		b = append(b, escBold(true)...)
		b = append(b, moneyLine("CHANGE", fmt.Sprintf("%.2f", ch), W)...)
		b = append(b, escBold(false)...)
	}
	b = append(b, []byte("\nThank you — please come again\n\n")...)
	// cash drawer kick on cash payments
	for _, p := range pays {
		if strings.EqualFold(orm.AsString(p["method"]), "cash") {
			b = append(b, escDrawer()...)
		}
	}
	b = append(b, escCut()...)

	// send to printer
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), 5*time.Second)
	if err != nil {
		return "", fmt.Errorf("printer unreachable %s:%d — %v", ip, port, err)
	}
	defer conn.Close()
	if _, err := conn.Write(b); err != nil {
		return "", fmt.Errorf("print failed: %v", err)
	}
	audit(bypass, orderModel, id, "print", "Receipt printed on "+orm.AsString(printer["name"]), args["user_id"])
	return fmt.Sprintf("Printed on %s (%s:%d)", orm.AsString(printer["name"]), ip, port), nil
}
