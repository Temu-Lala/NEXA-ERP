package models

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"nexa/core/orm"
)

// Ledger core (blueprint §4). stock.quant rows are projections maintained ONLY
// through the posting engine below; balances are never edited directly. Every
// posting runs inside one DB transaction with the affected quant rows locked
// (SELECT ... FOR UPDATE) so concurrent operations cannot allocate the same
// stock twice (§14).

const (
	moveDateLayout = "2006-01-02 15:04:05"
	qtyEps         = 1e-6
)

func nowStamp() string {
	return time.Now().Format(moveDateLayout)
}

// ---------------------------------------------------------------------------
// Value coercion helpers (shared by actions and engine)
// ---------------------------------------------------------------------------

func str(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	if v == nil {
		return ""
	}
	return orm.AsString(v)
}

func flt(v interface{}) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int64:
		return float64(x)
	case int:
		return float64(x)
	case nil:
		return 0
	default:
		if s := orm.AsString(v); s != "" {
			var tmp float64
			if _, err := fmt.Sscanf(s, "%g", &tmp); err == nil {
				return tmp
			}
		}
		return 0
	}
}

func idOf(v interface{}) int {
	i, ok := orm.CoerceInt64(v)
	if !ok {
		return 0
	}
	return int(i)
}

// ---------------------------------------------------------------------------
// Pure allocation maths (unit tested)
// ---------------------------------------------------------------------------

// QuantSnap is one candidate quant row for allocation.
type QuantSnap struct {
	ID     int
	LotID  int
	Qty    float64
	Rsvd   float64
	InDate time.Time
	Expiry time.Time // lot expiration; zero when untracked or undated (§8.4)
}

// Available returns the eligible (unreserved) quantity of the snapshot.
func (q QuantSnap) Available() float64 { return q.Qty - q.Rsvd }

// QuantAlloc is one allocated slice of a demand.
type QuantAlloc struct {
	QuantID int
	Qty     float64
}

// planAllocation allocates demand across quant snapshots FEFO where lots
// carry an expiration date (earliest expiry first, §8.4) and FIFO otherwise
// (oldest in_date first); ties resolve by id. It returns the allocation plan
// and any shortfall — a pure function so reservation behaviour is testable
// in isolation.
func planAllocation(demand float64, snaps []QuantSnap) (allocs []QuantAlloc, short float64) {
	if demand <= qtyEps {
		return nil, 0
	}
	ordered := make([]QuantSnap, len(snaps))
	copy(ordered, snaps)
	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		if (a.Expiry.IsZero()) != (b.Expiry.IsZero()) {
			return !a.Expiry.IsZero() // dated lots are consumed first
		}
		if !a.Expiry.IsZero() && !a.Expiry.Equal(b.Expiry) {
			return a.Expiry.Before(b.Expiry)
		}
		if !a.InDate.Equal(b.InDate) && !a.InDate.IsZero() && !b.InDate.IsZero() {
			return a.InDate.Before(b.InDate)
		}
		if a.InDate.IsZero() != b.InDate.IsZero() {
			return !a.InDate.IsZero()
		}
		return a.ID < b.ID
	})
	remaining := demand
	for _, s := range ordered {
		avail := s.Available()
		if avail <= qtyEps {
			continue
		}
		take := avail
		if take > remaining {
			take = remaining
		}
		allocs = append(allocs, QuantAlloc{QuantID: s.ID, Qty: take})
		remaining -= take
		if remaining <= qtyEps {
			return allocs, 0
		}
	}
	return allocs, math_max(remaining, 0)
}

func math_max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// splitCompletion decides how much of a move posts and what remains as a
// backorder demand (blueprint §5.1 partial receipts/deliveries).
func splitCompletion(demand, done float64) (posted, backorder float64) {
	if done <= qtyEps {
		return 0, demand
	}
	if done >= demand-qtyEps {
		return done, 0
	}
	return done, demand - done
}

// ---------------------------------------------------------------------------
// Transaction plumbing
// ---------------------------------------------------------------------------

func bypassCtx(ctx context.Context) context.Context { return orm.ContextWithBypass(ctx, true) }

func withTx(ctx context.Context, fn func(tx orm.TxWrapper) error) error {
	tx, err := orm.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// ---------------------------------------------------------------------------
// Quant access inside a transaction
// ---------------------------------------------------------------------------

type quantRow struct {
	id       int
	qty      float64
	rsvd     float64
	inDate   sql.NullString
	lotID    sql.NullInt64
	location int
}

// locationAndDescendants returns the id set of a location plus all its
// physical descendants (recursive CTE over stock_location). A zero root
// returns just {0} so callers can treat "no location" uniformly.
func locationAndDescendants(ctx context.Context, tx orm.TxWrapper, rootID int) ([]int, error) {
	if rootID <= 0 {
		return []int{rootID}, nil
	}
	rows, err := tx.QueryContext(ctx, `
		WITH RECURSIVE tree AS (
			SELECT id FROM stock_location WHERE id = $1
			UNION ALL
			SELECT l.id FROM stock_location l JOIN tree t ON l.parent_id = t.id
			WHERE COALESCE(l.usage,'') <> 'view'
		)
		SELECT id FROM tree ORDER BY id`, rootID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		ids = []int{rootID}
	}
	return ids, rows.Err()
}

// lockQuants locks quant rows for a product across the given location ids,
// optionally filtered by exact lot (lotID<=0 → any lot), ordered oldest
// receipt first for deterministic FIFO consumption.
func lockQuants(ctx context.Context, tx orm.TxWrapper, productID int, locIDs []int, lotID int) ([]quantRow, error) {
	if len(locIDs) == 0 {
		return nil, nil
	}
	query := `SELECT id, quantity, reserved_quantity, COALESCE(in_date::text,''), COALESCE(lot_id,0), location_id
	          FROM stock_quant
	          WHERE product_id = $1 AND location_id = ANY($2)`
	args := []interface{}{productID, pgIntArr(locIDs)}
	if lotID > 0 {
		query += ` AND lot_id = $3`
		args = append(args, lotID)
	}
	query += ` ORDER BY in_date NULLS LAST, id FOR UPDATE`
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []quantRow
	for rows.Next() {
		var q quantRow
		var inDate string
		var lot int64
		if err := rows.Scan(&q.id, &q.qty, &q.rsvd, &inDate, &lot, &q.location); err != nil {
			return nil, err
		}
		if inDate != "" {
			q.inDate = sql.NullString{String: inDate, Valid: true}
		}
		if lot > 0 {
			q.lotID = sql.NullInt64{Int64: lot, Valid: true}
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// lotGuard carries the reservation-relevant facts of a lot (blueprint §8.4–§8.5).
type lotGuard struct {
	State   string
	Expiry  time.Time // zero = no expiration date
	Removal time.Time // zero = no removal (FEFO-forced) date
}

func parseDateOnly(s string) time.Time {
	if len(s) >= 10 {
		if t, err := time.Parse("2006-01-02", s[:10]); err == nil {
			return t
		}
	}
	return time.Time{}
}

// lotGuardInfo loads disposition/expiry data for the given lots in one pass.
// It runs inside the caller's transaction but takes no row locks: reservation
// decisions read lot master data, which is tiny and changes rarely.
func lotGuardInfo(ctx context.Context, tx orm.TxWrapper, lotIDs []int) map[int]lotGuard {
	out := map[int]lotGuard{}
	if len(lotIDs) == 0 {
		return out
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT id, COALESCE(state,'released'), COALESCE(expiration_on::text,''), COALESCE(removal_on::text,'')
		FROM stock_lot WHERE id = ANY($1)`, pgIntArr(lotIDs))
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var st, exp, rem string
		if err := rows.Scan(&id, &st, &exp, &rem); err != nil {
			continue
		}
		out[id] = lotGuard{State: st, Expiry: parseDateOnly(exp), Removal: parseDateOnly(rem)}
	}
	return out
}

// eligibleForReservation reports whether stock of this lot may be reserved
// today: released disposition, not past its expiration or removal date. A lot
// expiring today is still usable through the end of its expiration day.
func eligibleForReservation(g lotGuard, today time.Time) bool {
	if g.State != string(LotStateReleased) {
		return false
	}
	day := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	if !g.Expiry.IsZero() && day.After(g.Expiry) {
		return false
	}
	if !g.Removal.IsZero() && day.After(g.Removal) {
		return false
	}
	return true
}

func pgIntArr(ids []int) []byte {
	// Formats an int slice for use with = ANY($n) against bigint[].
	buf := []byte("{")
	for i, id := range ids {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, []byte(fmt.Sprint(id))...)
	}
	return append(buf, '}')
}

// decreaseQuant consumes qty from quants at the source (or descendants). It
// first releases reserved stock (up to releaseRsvd), then consumes unreserved
// availability. Negative balances are blocked unless allowNegative (policy
// invariants 4 and 5).
func decreaseQuant(ctx context.Context, tx orm.TxWrapper, productID, rootLocID, lotID int, qty, releaseRsvd float64, allowNegative bool) error {
	if qty <= qtyEps {
		return nil
	}
	locIDs, err := locationAndDescendants(ctx, tx, rootLocID)
	if err != nil {
		return err
	}
	rows, err := lockQuants(ctx, tx, productID, locIDs, lotID)
	if err != nil {
		return err
	}
	remaining := qty
	reserveToRelease := releaseRsvd
	for _, q := range rows {
		if remaining <= qtyEps {
			break
		}
		takeFromReserved := 0.0
		if reserveToRelease > qtyEps && q.rsvd > qtyEps {
			takeFromReserved = q.rsvd
			if takeFromReserved > reserveToRelease {
				takeFromReserved = reserveToRelease
			}
			if takeFromReserved > remaining {
				takeFromReserved = remaining
			}
		}
		free := q.qty - q.rsvd
		takeFree := 0.0
		if free > qtyEps {
			takeFree = free
			if need := remaining - takeFromReserved; takeFree > need {
				takeFree = need
			}
		}
		total := takeFromReserved + takeFree
		if total <= qtyEps {
			continue
		}
		newQty := q.qty - total
		newRsvd := q.rsvd - takeFromReserved
		if _, err := tx.ExecContext(ctx,
			`UPDATE stock_quant SET quantity = $1, reserved_quantity = $2 WHERE id = $3`,
			newQty, newRsvd, q.id); err != nil {
			return err
		}
		remaining -= total
		reserveToRelease -= takeFromReserved
	}
	if remaining > qtyEps {
		if !allowNegative {
			return fmt.Errorf("insufficient stock for product %d at location %d: %.3f still unavailable", productID, rootLocID, remaining)
		}
		// Policy allows negative: park the shortfall on the newest row or a
		// fresh one so the balance projection still reconciles with the ledger.
		var target int
		if len(rows) > 0 {
			target = rows[0].id
		} else {
			var err error
			target, err = insertQuant(ctx, tx, productID, rootLocID, lotID, 0)
			if err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE stock_quant SET quantity = quantity - $1 WHERE id = $2`, remaining, target); err != nil {
			return err
		}
	}
	return nil
}

// increaseQuant adds qty to the quant at the exact destination location and
// lot, stamping in_date on first receipt.
func increaseQuant(ctx context.Context, tx orm.TxWrapper, productID, locID, lotID int, qty float64) error {
	if qty <= qtyEps {
		return nil
	}
	rows, err := lockQuants(ctx, tx, productID, []int{locID}, lotID)
	if err != nil {
		return err
	}
	if len(rows) > 0 {
		_, err := tx.ExecContext(ctx,
			`UPDATE stock_quant SET quantity = quantity + $1 WHERE id = $2`, qty, rows[0].id)
		return err
	}
	newID, err := insertQuant(ctx, tx, productID, locID, lotID, qty)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE stock_quant SET in_date = now() WHERE id = $1`, newID)
	return err
}

func insertQuant(ctx context.Context, tx orm.TxWrapper, productID, locID, lotID int, qty float64) (int, error) {
	var lot interface{}
	if lotID > 0 {
		lot = lotID
	}
	var id int
	err := tx.QueryRowContext(ctx, `
		INSERT INTO stock_quant (product_id, location_id, lot_id, quantity, reserved_quantity)
		VALUES ($1, $2, $3, $4, 0) RETURNING id`,
		productID, locID, lot, qty).Scan(&id)
	return id, err
}

// ---------------------------------------------------------------------------
// Ledger row writers (move / move-line) inside the same transaction
// ---------------------------------------------------------------------------

func insertMove(ctx context.Context, tx orm.TxWrapper, m map[string]interface{}) (int, error) {
	var id int
	err := tx.QueryRowContext(ctx, `
		INSERT INTO stock_move (name, product_id, picking_id, source_location_id, destination_id,
		                        quantity, reserved_quantity, state, date_due, date_done, lot_id,
		                        procurement_group_id, origin)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING id`,
		strArg(m["name"]), m["product_id"], nilIfZero(m["picking_id"]), nilIfZero(m["source_location_id"]),
		nilIfZero(m["destination_id"]), m["quantity"], m["reserved_quantity"], m["state"],
		strArg(m["date_due"]), strArg(m["date_done"]), nilIfZero(m["lot_id"]),
		nilIfZero(m["procurement_group_id"]), strArg(m["origin"]),
	).Scan(&id)
	return id, err
}

func insertMoveLine(ctx context.Context, tx orm.TxWrapper, line map[string]interface{}) (int, error) {
	var id int
	err := tx.QueryRowContext(ctx, `
		INSERT INTO stock_move_line (move_id, picking_id, product_id, lot_id, serial_id,
		                             quantity, source_location_id, result_location_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
		mustInt(line["move_id"]), nilIfZero(line["picking_id"]), line["product_id"],
		nilIfZero(line["lot_id"]), nilIfZero(line["serial_id"]), line["quantity"],
		nilIfZero(line["source_location_id"]), nilIfZero(line["result_location_id"]),
	).Scan(&id)
	return id, err
}

func strArg(v interface{}) interface{} {
	s := str(v)
	if s == "" {
		return nil
	}
	return s
}

func nilIfZero(v interface{}) interface{} {
	if idOf(v) == 0 {
		return nil
	}
	return idOf(v)
}

func mustInt(v interface{}) interface{} { return idOf(v) }
