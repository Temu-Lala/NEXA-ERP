package models

import (
	"context"
	"math"
	"strconv"

	"nexa/core/orm"
)

// Field/model constants for the criticality engine.
const (
	lineModel = "maintenance.criticality.assessment.line"

	lineAssessmentField = "assessment_id"
	lineWeightField     = "weight"
	lineScaleMaxField   = "scale_max"
	lineRatingField     = "rating"
)

func init() {
	orm.RegisterCompute(lineModel, "weighted_score", []string{lineWeightField, lineRatingField}, computeLineWeighted)
	orm.RegisterCompute("maintenance.criticality.assessment", "total_score", nil, computeAssessmentTotal)
	orm.RegisterCompute("maintenance.criticality.assessment", "score_index", nil, computeAssessmentIndex)
	orm.RegisterCompute("maintenance.criticality.assessment", "criticality_level", nil, computeAssessmentLevel)
}

// computeLineWeighted returns weight * rating for a single assessment line.
func computeLineWeighted(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	w := num(rec[lineWeightField])
	r := num(rec[lineRatingField])
	return round4(w * r), nil
}

// computeAssessmentTotal sums the weighted scores across the assessment's lines.
func computeAssessmentTotal(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	lines := assessmentLines(ctx, rec["id"])
	var total float64
	for _, ln := range lines {
		total += num(ln[lineWeightField]) * num(ln[lineRatingField])
	}
	return round4(total), nil
}

// computeAssessmentIndex returns the achieved score as a percentage of the maximum
// attainable score (sum of weight * scale_max across lines), i.e. the criticality index.
func computeAssessmentIndex(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	total, maxPoss := scoreAggregate(assessmentLines(ctx, rec["id"]))
	if maxPoss <= 0 {
		return 0.0, nil
	}
	return round2(total / maxPoss * 100), nil
}

// computeAssessmentLevel maps the criticality index onto a classification band.
func computeAssessmentLevel(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	total, maxPoss := scoreAggregate(assessmentLines(ctx, rec["id"]))
	return string(bandIndex(total, maxPoss)), nil
}

// bandIndex converts achieved/maximum scores into a criticality level band.
func bandIndex(total, maxPoss float64) CriticalityLevel {
	if maxPoss <= 0 {
		return LevelLow
	}
	idx := total / maxPoss * 100
	switch {
	case idx >= 75:
		return LevelCritical
	case idx >= 50:
		return LevelHigh
	case idx >= 25:
		return LevelNormal
	default:
		return LevelLow
	}
}

// scoreAggregate computes (achieved, maximum) across the given lines.
func scoreAggregate(lines []map[string]interface{}) (float64, float64) {
	var total, max float64
	for _, ln := range lines {
		w := num(ln[lineWeightField])
		total += w * num(ln[lineRatingField])
		max += w * num(ln[lineScaleMaxField])
	}
	return total, max
}

// assessmentLines fetches all score lines for the given assessment id.
func assessmentLines(ctx context.Context, id interface{}) []map[string]interface{} {
	iid, ok := orm.CoerceInt64(id)
	if !ok || iid <= 0 {
		return nil
	}
	bypass := orm.ContextWithBypass(ctx, true)
	domain := [][]interface{}{{lineAssessmentField, "=", int(iid)}}
	var out []map[string]interface{}
	offset := 0
	const page = 500
	for {
		rows, err := orm.SearchPage(bypass, lineModel, domain, page, offset, "id")
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

// num parses an ORM numeric/interface value into float64 (0 on failure).
func num(v interface{}) float64 {
	s := orm.AsString(v)
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }
func round4(f float64) float64 { return math.Round(f*10000) / 10000 }
