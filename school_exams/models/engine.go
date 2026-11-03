package models

import (
	"context"
	"math"

	"nexa/core/orm"
)

// Behaviour layer for examinations. The grading arithmetic — percentage, band lookup,
// weighted GPA, ranking — is pure and unit-tested. The computes project a score's
// percentage/grade/points, a report card's aggregates and class rank, and a transcript's
// cumulative figures, all resolved through the learner's grading scale (or the default
// scale when none is named). Report-card lines and transcript lines carry materialized
// values, so aggregates simply read stored fields.

const (
	scaleModel      = "school.grading.scale"
	bandModel       = "school.grading.band"
	examModel       = "school.exam"
	scoreModel      = "school.exam.score"
	reportModel     = "school.report.card"
	reportLineModel = "school.report.card.line"
	transcriptModel = "school.transcript"
	trxLineModel    = "school.transcript.line"
	studentModel    = "school.student"

	dateLayout   = "2006-01-02"
	datetimeFull = "2006-01-02 15:04:05"
)

// band is a single grading band (lower bound -> letter + GPA points + remark).
type band struct {
	minPct float64
	grade  string
	points float64
	remark string
}

// percentage is obtained/max as a percentage, rounded to two decimals. A non-positive
// max yields 0 so a mis-configured exam never divides by zero.
func percentage(obtained, max float64) float64 {
	if max <= 0 {
		return 0
	}
	return round2(obtained / max * 100)
}

// pickBand returns the letter grade and GPA points whose lower bound the percentage
// clears; the highest qualifying bound wins. An empty band set returns no grade.
func pickBand(pct float64, bands []band) (string, float64) {
	var best *band
	for i := range bands {
		b := bands[i]
		if pct >= b.minPct {
			if best == nil || b.minPct > best.minPct {
				best = &b
			}
		}
	}
	if best == nil {
		return "", 0
	}
	return best.grade, best.points
}

// weightedMean averages values weighted by weights (used for GPA and weighted subject
// marks). With no positive total weight it falls back to the simple mean; empty input
// returns 0. Result is rounded to two decimals.
func weightedMean(values, weights []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sumW float64
	var sumVW float64
	for i, v := range values {
		w := 1.0
		if i < len(weights) && weights[i] > 0 {
			w = weights[i]
		}
		sumW += w
		sumVW += v * w
	}
	if sumW <= 0 {
		return 0
	}
	return round2(sumVW / sumW)
}

// rankFromHigher converts a count of strictly-better peers into a 1-based rank.
func rankFromHigher(higher int) int { return higher + 1 }

func round2(f float64) float64 { return math.Round(f*100) / 100 }

func init() {
	orm.RegisterCompute(scaleModel, "band_count", []string{"id"}, computeBandCount)
	orm.RegisterCompute(examModel, "score_count", []string{"id"}, computeExamScoreCount)
	orm.RegisterCompute(examModel, "avg_pct", []string{"id"}, computeExamAvgPct)
	orm.RegisterCompute(scoreModel, "percentage", []string{"marks_obtained", "exam_id", "absent"}, computeScorePct)
	orm.RegisterCompute(scoreModel, "grade", []string{"marks_obtained", "exam_id", "absent"}, computeScoreGrade)
	orm.RegisterCompute(scoreModel, "grade_points", []string{"marks_obtained", "exam_id", "absent"}, computeScorePoints)
	orm.RegisterCompute(reportModel, "total_marks", []string{"id"}, computeReportTotal)
	orm.RegisterCompute(reportModel, "obtained_marks", []string{"id"}, computeReportObtained)
	orm.RegisterCompute(reportModel, "overall_pct", []string{"id"}, computeReportOverallPct)
	orm.RegisterCompute(reportModel, "gpa", []string{"id"}, computeReportGpa)
	orm.RegisterCompute(reportModel, "overall_grade", []string{"id"}, computeReportGrade)
	orm.RegisterCompute(reportModel, "subject_count", []string{"id"}, computeReportSubjects)
	orm.RegisterCompute(reportModel, "class_rank", []string{"id"}, computeReportRank)
	orm.RegisterCompute(transcriptModel, "total_credits", []string{"id"}, computeTrxCredits)
	orm.RegisterCompute(transcriptModel, "cumulative_gpa", []string{"id"}, computeTrxGpa)
	orm.RegisterCompute(transcriptModel, "overall_grade", []string{"id"}, computeTrxGrade)
}

// ---------------------------------------------------------------------------
// Scale / band loading
// ---------------------------------------------------------------------------

// bandsForScale loads a scale's bands. A non-positive scaleID resolves to the default
// scale so grading still works when an exam/report does not name a scheme explicitly.
func bandsForScale(ctx context.Context, scaleID int) []band {
	bypass := orm.ContextWithBypass(ctx, true)
	if scaleID <= 0 {
		scaleID = defaultScaleID(bypass)
	}
	if scaleID <= 0 {
		return nil
	}
	rows, err := orm.Search(bypass, bandModel, [][]interface{}{{"scale_id", "=", scaleID}})
	if err != nil {
		return nil
	}
	bands := make([]band, 0, len(rows))
	for _, r := range rows {
		bands = append(bands, band{
			minPct: floatOr(r["min_percentage"]),
			grade:  strOr(r["grade"]),
			points: floatOr(r["grade_points"]),
			remark: strOr(r["remark"]),
		})
	}
	return bands
}

func defaultScaleID(ctx context.Context) int {
	rows, err := orm.Search(ctx, scaleModel, [][]interface{}{{"is_default", "=", true}})
	if err != nil || len(rows) == 0 {
		return 0
	}
	return intOr(rows[0]["id"])
}

// passPctForScale returns a scale's pass mark (default scale when scaleID<=0).
func passPctForScale(ctx context.Context, scaleID int) float64 {
	bypass := orm.ContextWithBypass(ctx, true)
	if scaleID <= 0 {
		scaleID = defaultScaleID(bypass)
	}
	if scaleID <= 0 {
		return 0
	}
	rec, err := orm.SearchOne(bypass, scaleModel, map[string]interface{}{"id": scaleID})
	if err != nil || rec == nil {
		return 0
	}
	return floatOr(rec["pass_percentage"])
}

// ---------------------------------------------------------------------------
// Exam / score computes
// ---------------------------------------------------------------------------

func computeBandCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), bandModel, [][]interface{}{
		{"scale_id", "=", intOr(rec["id"])},
	})
	if err != nil {
		return 0, nil
	}
	return n, nil
}

func computeExamScoreCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), scoreModel, [][]interface{}{
		{"exam_id", "=", intOr(rec["id"])},
	})
	if err != nil {
		return 0, nil
	}
	return n, nil
}

func computeExamAvgPct(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	examID := intOr(rec["id"])
	bypass := orm.ContextWithBypass(ctx, true)
	exam, err := orm.SearchOne(bypass, examModel, map[string]interface{}{"id": examID})
	if err != nil || exam == nil {
		return 0, nil
	}
	max := floatOr(exam["max_marks"])
	scores, err := orm.Search(bypass, scoreModel, [][]interface{}{{"exam_id", "=", examID}})
	if err != nil || len(scores) == 0 {
		return 0, nil
	}
	weights := make([]float64, 0, len(scores))
	for range scores {
		weights = append(weights, 1)
	}
	vals := make([]float64, 0, len(scores))
	for _, s := range scores {
		if boolOr(s["absent"]) {
			continue
		}
		vals = append(vals, percentage(floatOr(s["marks_obtained"]), max))
	}
	return weightedMean(vals, weights), nil
}

func scoreContext(ctx context.Context, rec map[string]interface{}) (pct float64, absent bool, scaleID int) {
	bypass := orm.ContextWithBypass(ctx, true)
	if boolOr(rec["absent"]) {
		return 0, true, 0
	}
	exam, err := orm.SearchOne(bypass, examModel, map[string]interface{}{"id": intOr(rec["exam_id"])})
	if err != nil || exam == nil {
		return 0, false, 0
	}
	pct = percentage(floatOr(rec["marks_obtained"]), floatOr(exam["max_marks"]))
	return pct, false, intOr(exam["scale_id"])
}

func computeScorePct(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	pct, absent, _ := scoreContext(ctx, rec)
	if absent {
		return 0, nil
	}
	return pct, nil
}

func computeScoreGrade(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	pct, absent, scaleID := scoreContext(ctx, rec)
	if absent {
		return "AB", nil
	}
	grade, _ := pickBand(pct, bandsForScale(ctx, scaleID))
	return grade, nil
}

func computeScorePoints(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	pct, absent, scaleID := scoreContext(ctx, rec)
	if absent {
		return 0, nil
	}
	_, points := pickBand(pct, bandsForScale(ctx, scaleID))
	return points, nil
}

// ---------------------------------------------------------------------------
// Report card computes (read materialized lines)
// ---------------------------------------------------------------------------

type reportLine struct {
	max, obtained, weight, pct, points float64
}

func loadReportLines(ctx context.Context, reportID int) []reportLine {
	rows, err := orm.Search(orm.ContextWithBypass(ctx, true), reportLineModel, [][]interface{}{
		{"report_card_id", "=", reportID},
	})
	if err != nil {
		return nil
	}
	out := make([]reportLine, 0, len(rows))
	for _, r := range rows {
		out = append(out, reportLine{
			max:      floatOr(r["max_marks"]),
			obtained: floatOr(r["obtained_marks"]),
			weight:   floatOr(r["weight"]),
			pct:      floatOr(r["percentage"]),
			points:   floatOr(r["grade_points"]),
		})
	}
	return out
}

func reportAggregate(ctx context.Context, rec map[string]interface{}) (sumMax, sumObt, pct, gpa float64, subjects int, scaleID int) {
	id := intOr(rec["id"])
	lines := loadReportLines(ctx, id)
	for _, l := range lines {
		sumMax += l.max
		sumObt += l.obtained
	}
	pct = percentage(sumObt, sumMax)
	points := make([]float64, 0, len(lines))
	weights := make([]float64, 0, len(lines))
	for _, l := range lines {
		points = append(points, l.points)
		w := l.weight
		if w <= 0 {
			w = 1
		}
		weights = append(weights, w)
	}
	gpa = weightedMean(points, weights)
	return sumMax, sumObt, pct, gpa, len(lines), intOr(rec["scale_id"])
}

func computeReportTotal(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	sumMax, _, _, _, _, _ := reportAggregate(ctx, rec)
	return sumMax, nil
}
func computeReportObtained(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	_, sumObt, _, _, _, _ := reportAggregate(ctx, rec)
	return sumObt, nil
}
func computeReportOverallPct(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	_, _, pct, _, _, _ := reportAggregate(ctx, rec)
	return pct, nil
}
func computeReportGpa(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	_, _, _, gpa, _, _ := reportAggregate(ctx, rec)
	return gpa, nil
}
func computeReportSubjects(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	_, _, _, _, n, _ := reportAggregate(ctx, rec)
	return n, nil
}
func computeReportGrade(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	_, _, pct, _, _, scaleID := reportAggregate(ctx, rec)
	grade, _ := pickBand(pct, bandsForScale(ctx, scaleID))
	return grade, nil
}

// computeReportRank ranks the learner among approved report cards in the same
// section/term/year: the count of peers with a strictly higher overall percentage + 1.
func computeReportRank(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	id := intOr(rec["id"])
	sectionID := intOr(rec["section_id"])
	termID := intOr(rec["term_id"])
	yearID := intOr(rec["academic_year_id"])
	if sectionID <= 0 || yearID <= 0 {
		return 0, nil
	}
	_, _, selfPct, _, _, _ := reportAggregate(ctx, rec)
	bypass := orm.ContextWithBypass(ctx, true)
	domain := [][]interface{}{
		{"section_id", "=", sectionID},
		{"academic_year_id", "=", yearID},
		{"state", "=", string(RptApproved)},
	}
	if termID > 0 {
		domain = append(domain, []interface{}{"term_id", "=", termID})
	}
	peers, err := orm.Search(bypass, reportModel, domain)
	if err != nil {
		return 0, nil
	}
	higher := 0
	for _, p := range peers {
		if intOr(p["id"]) == id {
			continue
		}
		_, _, pct, _, _, _ := reportAggregate(ctx, p)
		if pct > selfPct {
			higher++
		}
	}
	return rankFromHigher(higher), nil
}

// ---------------------------------------------------------------------------
// Transcript computes
// ---------------------------------------------------------------------------

func loadTrxLines(ctx context.Context, trxID int) []reportLine {
	rows, err := orm.Search(orm.ContextWithBypass(ctx, true), trxLineModel, [][]interface{}{
		{"transcript_id", "=", trxID},
	})
	if err != nil {
		return nil
	}
	out := make([]reportLine, 0, len(rows))
	for _, r := range rows {
		out = append(out, reportLine{
			weight: floatOr(r["credits"]),
			pct:    floatOr(r["percentage"]),
			points: floatOr(r["grade_points"]),
		})
	}
	return out
}

func computeTrxCredits(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	var sum float64
	for _, l := range loadTrxLines(ctx, intOr(rec["id"])) {
		sum += l.weight
	}
	return round2(sum), nil
}

func computeTrxGpa(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	lines := loadTrxLines(ctx, intOr(rec["id"]))
	points := make([]float64, 0, len(lines))
	credits := make([]float64, 0, len(lines))
	for _, l := range lines {
		points = append(points, l.points)
		credits = append(credits, l.weight)
	}
	return weightedMean(points, credits), nil
}

func computeTrxGrade(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	lines := loadTrxLines(ctx, intOr(rec["id"]))
	pcts := make([]float64, 0, len(lines))
	credits := make([]float64, 0, len(lines))
	for _, l := range lines {
		pcts = append(pcts, l.pct)
		credits = append(credits, l.weight)
	}
	avg := weightedMean(pcts, credits)
	grade, _ := pickBand(avg, bandsForScale(ctx, intOr(rec["scale_id"])))
	return grade, nil
}

// ---------------------------------------------------------------------------
// Shared coercion helpers (self-contained to this module)
// ---------------------------------------------------------------------------

func intOr(v interface{}) int {
	n, ok := orm.CoerceInt64(v)
	if !ok {
		return 0
	}
	return int(n)
}

func strOr(v interface{}) string { return orm.AsString(v) }

func boolOr(v interface{}) bool { return orm.AsBool(v) }

func floatOr(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	}
	if i, ok := orm.CoerceInt64(v); ok {
		return float64(i)
	}
	return 0
}
