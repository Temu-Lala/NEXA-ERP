package models

import (
	"context"
	"strconv"
	"strings"

	"nexa/core/orm"
	"nexa/core/scheduler"
)

// Resource engine (blueprint §8.2, §8.3). A person's capacity must be evaluated
// across every project they touch, not within one project in isolation, so the
// over-allocation signal is a per-employee roll-up of confirmed allocations. The
// daily recompute refreshes that flag; confirming an allocation refreshes it
// immediately. Capacity uses a 100% working baseline here; where HR calendars are
// available the effective capacity should replace this constant.

const (
	allocModel   = "project.resource.allocation"
	reqModel     = "project.resource.request"
	capacityCron = "project.resource.capacity"

	// fullCapacityPct is the 100% availability baseline for the period.
	fullCapacityPct = 100.0
)

func init() {
	scheduler.RegisterCronHandler(capacityCron, runCapacityRecompute)
}

// userCommittedPct sums confirmed allocation percentages for one employee across
// all projects (optionally overlapping the allocation's window is approximated by
// counting all confirmed rows, keeping the model simple and conservative).
func userCommittedPct(ctx context.Context, userID int) float64 {
	rows, err := orm.SearchPage(ctx, allocModel, [][]interface{}{
		{"user_id", "=", userID},
		{"state", "=", "confirmed"},
	}, 5000, 0, "id asc")
	if err != nil {
		return 0
	}
	var sum float64
	for _, r := range rows {
		sum += flt(r["alloc_pct"])
	}
	return sum
}

func isOverAllocated(committed float64) bool {
	return committed > fullCapacityPct+1e-9
}

// runCapacityRecompute refreshes the over-allocation flag on every confirmed
// allocation by grouping committed percentage per employee.
func runCapacityRecompute(ctx context.Context, _ map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	rows, err := orm.SearchPage(bypass, allocModel, [][]interface{}{
		{"state", "=", "confirmed"},
	}, 20000, 0, "id asc")
	if err != nil {
		return err
	}
	// committed-by-user (cached to avoid re-scanning per row)
	committed := map[int]float64{}
	over := map[int]bool{}
	for _, r := range rows {
		uid := idOf(r["user_id"])
		if uid <= 0 {
			continue
		}
		if _, ok := committed[uid]; !ok {
			c := userCommittedPct(bypass, uid)
			committed[uid] = c
			over[uid] = isOverAllocated(c)
		}
		want := over[uid]
		if boolOf(r["overalloc"]) != want {
			_ = orm.UpdateRecordByID(bypass, allocModel, idOf(r["id"]), map[string]interface{}{"overalloc": want})
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers (package-local)
// ---------------------------------------------------------------------------

func load(ctx context.Context, model string, id int) map[string]interface{} {
	if id <= 0 {
		return nil
	}
	rec, err := orm.SearchOne(ctx, model, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return nil
	}
	return rec
}

func idOf(v interface{}) int {
	n, _ := orm.CoerceInt64(v)
	return int(n)
}

func s(v interface{}) string { return orm.AsString(v) }

func boolOf(v interface{}) bool {
	b, _ := v.(bool)
	return b
}

func flt(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int64:
		return float64(t)
	case int:
		return float64(t)
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f
	default:
		n, _ := orm.CoerceInt64(v)
		return float64(n)
	}
}
