package models

import (
	"context"
	"fmt"
	"strings"

	"nexa/core/orm"
)

// Risk engine (blueprint §11.2). Severity is derived from probability and impact
// through a matrix, not hand-typed, so the register is consistent and auditable.
// The matrix multiplies ordinal levels (low=1..critical=4): a score of 12+ is
// critical, 6+ high, 3+ medium, otherwise low. Zero/unknown inputs stay low so an
// unassessed risk is never silently shown as safe.

// levelScore maps a level name to its ordinal weight.
func levelScore(level string) int {
	switch strings.TrimSpace(level) {
	case "low":
		return 1
	case "medium":
		return 2
	case "high":
		return 3
	case "critical":
		return 4
	default:
		return 0
	}
}

// computeSeverity returns the matrix level for a probability/impact pair.
func computeSeverity(probability, impact string) Level {
	ps, is := levelScore(probability), levelScore(impact)
	score := ps * is
	switch {
	case score >= 12:
		return LevelCritical
	case score >= 6:
		return LevelHigh
	case score >= 3:
		return LevelMedium
	default:
		return LevelLow
	}
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

func setModel(ctx context.Context, model string, id int, vals map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	if load(bypass, model, id) == nil {
		return fmt.Errorf("%s not found", model)
	}
	return orm.UpdateRecordByID(bypass, model, id, vals)
}
