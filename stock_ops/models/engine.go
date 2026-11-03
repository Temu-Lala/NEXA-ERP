package models

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"nexa/core/orm"
)

// Operations behaviour layer: pure putaway ranking and barcode matching
// (unit tested, no DB), plus the candidate loader that feeds them. Like all
// companions this never touches quants — it directs work, the anchor ledger
// moves stock.

const (
	taskModel   = "stock.operation.task"
	waveModel   = "stock.wave"
	waveLineMdl = "stock.wave.line"
	scanLogMdl  = "stock.ops.scan.log"
	dtLayout    = "2006-01-02 15:04:05"
)

// ---------------------------------------------------------------------------
// Pure putaway ranking (§3.3, §5.4)
// ---------------------------------------------------------------------------

// PutawaySlot is one candidate storage location with its current load, in
// product-weight/volume terms. Capacity <= 0 means unlimited.
type PutawaySlot struct {
	LocID       int
	Name        string
	PosPriority int
	CapacityKG  float64
	CapacityM3  float64
	LoadKG      float64
	LoadM3      float64
}

func (s PutawaySlot) freeKG() float64 { return headroom(s.CapacityKG, s.LoadKG) }
func (s PutawaySlot) freeM3() float64 { return headroom(s.CapacityM3, s.LoadM3) }

func headroom(capacity, load float64) float64 {
	if capacity <= 0 {
		return -1 // unlimited
	}
	return capacity - load
}

func slotFits(s PutawaySlot, needKG, needM3 float64) bool {
	if f := s.freeKG(); f >= 0 && f+1e-9 < needKG {
		return false
	}
	if f := s.freeM3(); f >= 0 && f+1e-9 < needM3 {
		return false
	}
	return true
}

// rankPutaway filters out locations that cannot hold the load and orders the
// rest: lowest pos_priority first (closest / fastest pick face), then
// best-fit on remaining weight capacity (tight slots fill first, big ones
// stay flexible; unlimited capacities rank last), then id for determinism.
func rankPutaway(slots []PutawaySlot, needKG, needM3 float64) []PutawaySlot {
	var out []PutawaySlot
	for _, s := range slots {
		if slotFits(s, needKG, needM3) {
			out = append(out, s)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.PosPriority != b.PosPriority {
			return a.PosPriority < b.PosPriority
		}
		ak, bk := a.freeKG(), b.freeKG()
		if (ak >= 0) != (bk >= 0) {
			return ak >= 0 // finite headroom before unlimited
		}
		if ak >= 0 && bk >= 0 && ak != bk {
			return ak < bk // best fit first
		}
		return a.LocID < b.LocID
	})
	return out
}

// ---------------------------------------------------------------------------
// Pure scan matching (§16.3)
// ---------------------------------------------------------------------------

// scanMatches compares a scanned code with the product barcode and internal
// reference: trimmed, case-insensitive; an empty scan never matches.
func scanMatches(scanned, barcode, reference string) bool {
	s := normalizeCode(scanned)
	if s == "" {
		return false
	}
	return s == normalizeCode(barcode) || s == normalizeCode(reference)
}

func normalizeCode(c string) string {
	return strings.ToLower(strings.TrimSpace(c))
}

// expectedCodes lists what a scan may legally match for a product.
func expectedCodes(barcode, reference string) string {
	var parts []string
	if barcode != "" {
		parts = append(parts, barcode)
	}
	if reference != "" {
		parts = append(parts, reference)
	}
	return strings.Join(parts, " / ")
}

// ---------------------------------------------------------------------------
// Candidate loading
// ---------------------------------------------------------------------------

// putawayCandidates loads active, unblocked internal locations with current
// on-hand weight/volume per location. warehouseID>0 restricts to one site.
func putawayCandidates(ctx context.Context, warehouseID int) []PutawaySlot {
	sql := `
		SELECT l.id, l.name, COALESCE(l.pos_priority,0),
		       COALESCE(l.capacity_kg,0), COALESCE(l.capacity_m3,0),
		       COALESCE(SUM(q.quantity * COALESCE(p.weight,0)),0),
		       COALESCE(SUM(q.quantity * COALESCE(p.volume,0)),0)
		FROM stock_location l
		LEFT JOIN stock_quant q ON q.location_id = l.id
		LEFT JOIN product_product p ON p.id = q.product_id
		WHERE COALESCE(l.usage,'') = 'internal' AND COALESCE(l.active, true)
		  AND NOT COALESCE(l.blocked, false)`
	args := []interface{}{}
	if warehouseID > 0 {
		sql += ` AND l.warehouse_id = $1`
		args = append(args, warehouseID)
	}
	sql += ` GROUP BY l.id, l.name ORDER BY l.id`
	rows, err := orm.DB.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []PutawaySlot
	for rows.Next() {
		var s PutawaySlot
		if err := rows.Scan(&s.LocID, &s.Name, &s.PosPriority, &s.CapacityKG,
			&s.CapacityM3, &s.LoadKG, &s.LoadM3); err != nil {
			continue
		}
		out = append(out, s)
	}
	return out
}

func nowStamp() string { return time.Now().Format(dtLayout) }

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

func createRec(ctx context.Context, model string, vals map[string]interface{}) (int, error) {
	inst, ok := orm.Registry[model]
	if !ok {
		return 0, fmt.Errorf("model %s unavailable", model)
	}
	return orm.Create(ctx, inst, vals)
}
