package models

import (
	"context"
	"strings"

	"nexa/core/orm"
)

// Performance behaviour layer: the pure rating / achievement / roll-up maths lives
// here (unit-testable, no database) plus the stored / live computes that project a
// goal or KPI achievement percent, a calibration delta and a 360 weighted score
// onto their records. Nothing here mutates pay — a rating is an evaluation, never a
// salary instruction.

const (
	dateLayout = "2006-01-02"
)

const (
	goalModel      = "hr.appraisal.goal"
	appraisalModel = "hr.appraisal"
	kpiModel       = "hr.performance.kpi"
	feedbackModel  = "hr.appraisal.feedback"
	calibLineModel = "hr.calibration.line"
)

func init() {
	orm.RegisterCompute(goalModel, "achievement_pct", []string{"target_value", "actual_value"}, computeGoalAchievement)
	orm.RegisterCompute(kpiModel, "achievement_pct", []string{"target_value", "actual_value"}, computeKpiAchievement)
	orm.RegisterCompute(calibLineModel, "delta", []string{"proposed_rating", "calibrated_rating"}, computeCalibDelta)
	orm.RegisterCompute(appraisalModel, "peer_score", []string{"id"}, computePeerScore)
}

// ---------------------------------------------------------------------------
// Pure maths
// ---------------------------------------------------------------------------

// round2 snaps to 2 decimals with integer-safe half-up rounding.
func round2(f float64) float64 {
	scaled := f * 100
	if scaled >= 0 {
		return float64(int64(scaled+0.5)) / 100
	}
	return float64(int64(scaled-0.5)) / 100
}

// achievementPct is actual as a percent of target. A non-positive target yields 0
// so an un-set objective cannot divide by zero or report a spurious 100%.
func achievementPct(target, actual float64) float64 {
	if target <= 0 {
		return 0
	}
	return round2(actual / target * 100)
}

// weightedAverage is the sum(score*weight) / sum(weight) over paired slices; a
// zero (or mismatched) total weight yields 0. Mismatched lengths are compared only
// over the shared prefix so a partially-filled record never panics.
func weightedAverage(scores, weights []float64) float64 {
	n := len(scores)
	if len(weights) < n {
		n = len(weights)
	}
	var sum, wsum float64
	for i := 0; i < n; i++ {
		if weights[i] <= 0 {
			continue
		}
		sum += scores[i] * weights[i]
		wsum += weights[i]
	}
	if wsum <= 0 {
		return 0
	}
	return round2(sum / wsum)
}

// calibrationDelta is calibrated minus proposed (signed), rounded to 2 decimals.
func calibrationDelta(proposed, calibrated float64) float64 {
	return round2(calibrated - proposed)
}

// ---------------------------------------------------------------------------
// Field helpers
// ---------------------------------------------------------------------------

func intOr(v interface{}) int {
	n, ok := orm.CoerceInt64(v)
	if !ok {
		return 0
	}
	return int(n)
}

func floatOr(v interface{}) float64 {
	f, ok := orm.CoerceFloat64(v)
	if !ok {
		return 0
	}
	return f
}

// dateStr normalises a date / datetime value to a YYYY-MM-DD string.
func dateStr(v interface{}) string {
	s := strings.TrimSpace(orm.AsString(v))
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

// ---------------------------------------------------------------------------
// Computes
// ---------------------------------------------------------------------------

func computeGoalAchievement(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	return achievementPct(floatOr(rec["target_value"]), floatOr(rec["actual_value"])), nil
}

func computeKpiAchievement(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	return achievementPct(floatOr(rec["target_value"]), floatOr(rec["actual_value"])), nil
}

func computeCalibDelta(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	return calibrationDelta(floatOr(rec["proposed_rating"]), floatOr(rec["calibrated_rating"])), nil
}

// computePeerScore rolls up an appraisal's SUBMITTED 360 feedback into a weighted
// average. It is a live (non-stored) compute so it always reflects current inputs.
func computePeerScore(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	appraisalID := intOr(rec["id"])
	if appraisalID <= 0 {
		return 0.0, nil
	}
	domain := [][]interface{}{{"appraisal_id", "=", appraisalID}, {"submitted", "=", true}}
	rows, err := orm.Search(orm.ContextWithBypass(ctx, true), feedbackModel, domain)
	if err != nil {
		return 0.0, nil
	}
	scores := make([]float64, 0, len(rows))
	weights := make([]float64, 0, len(rows))
	for _, r := range rows {
		scores = append(scores, floatOr(r["score"]))
		weights = append(weights, floatOr(r["weight"]))
	}
	return weightedAverage(scores, weights), nil
}
