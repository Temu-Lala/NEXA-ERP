package models

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"nexa/core/orm"
	"nexa/core/scheduler"
)

// Performance engines (blueprint §13, §15). KPIs are computed from transactional
// PO / receipt / delivery records with an explicit numerator and denominator, so
// every score is explainable and comparable. A low composite raises a review or
// escalation for a human to authorize - it NEVER silently blocks a supplier.

const (
	periodModel   = "purchase.performance.period"
	scorecardMdl  = "purchase.supplier.scorecard"
	metricModel   = "purchase.performance.metric"
	correctiveMdl = "purchase.corrective.action"

	supplierModel = "purchase.supplier"
	orderModel    = "purchase.order"
	receiptModel  = "purchase.receipt"
	rcptLineModel = "purchase.receipt.line"
	scheduleModel = "purchase.delivery.schedule"

	perfCron = "purchase.performance.scorecard.compute"

	dateLayout = "2006-01-02"
	dtLayout   = "2006-01-02 15:04:05"

	eps = 0.0005
)

func init() {
	scheduler.RegisterCronHandler(perfCron, runPerformanceCompute)
}

// ---------------------------------------------------------------------------
// Pure KPI maths (unit tested)
// ---------------------------------------------------------------------------

// ratePct is numerator/denominator as a percentage. A zero denominator yields
// "not applicable" rather than a fabricated 0% - an absent signal must never be
// mistaken for a failed one.
func ratePct(numerator, denominator float64) (float64, bool) {
	if denominator <= eps {
		return 0, false
	}
	return clampPct(numerator / denominator * 100.0), true
}

// onTimeScore is the share of completed deliveries that met their scheduled
// date. Denominator counts only deliveries that were actually received.
func onTimeScore(onTime, total float64) (float64, bool) { return ratePct(onTime, total) }

// fillScore is delivered value against ordered value, capped at 100%: an
// over-delivery cannot score better than a perfect fill.
func fillScore(filled, ordered float64) (float64, bool) {
	if ordered <= eps {
		return 0, false
	}
	return clampPct(math.Min(filled, ordered) / ordered * 100.0), true
}

// acceptanceScore is accepted units against all inspected units (accepted +
// rejected + damaged). The denominator is the inspected quantity, not ordered,
// so undelivered lines do not depress quality.
func acceptanceScore(accepted, inspected float64) (float64, bool) {
	return ratePct(accepted, inspected)
}

// defectScore expresses a defect RATE (lower is better) as a 0-100 quality
// score. No inspected units is "not applicable", not a perfect score.
func defectScore(defective, received float64) (float64, bool) {
	if received <= eps {
		return 0, false
	}
	rate := defective / received * 100.0
	return clampPct(100.0 - rate), true
}

// normalizeDirection turns a raw measurement into a higher-is-better score using
// the KPI's declared direction.
func normalizeDirection(value float64, dir KpiDirection) float64 {
	if dir == DirLower {
		return clampPct(100.0 - value)
	}
	return clampPct(value)
}

// gradeFor maps a composite score to a grade band. No data is "not_rated" and is
// explicitly distinct from a poor score.
func gradeFor(score float64, hasData bool) Grade {
	if !hasData {
		return GradeNA
	}
	switch {
	case score >= 90:
		return GradeExcellent
	case score >= 75:
		return GradeGood
	case score >= 60:
		return GradeAccept
	case score >= 40:
		return GradePoor
	default:
		return GradeCritical
	}
}

// dimValue is one dimension's computed score and its configured weight.
type dimValue struct {
	Score   float64
	Weight  float64
	HasData bool
}

// weightedComposite averages the dimension scores by weight, considering only
// dimensions that actually have data. Weights of absent dimensions are dropped
// so a supplier is never penalised for a KPI that could not be measured.
func weightedComposite(dims []dimValue) (float64, bool) {
	total, wsum := 0.0, 0.0
	for _, d := range dims {
		if !d.HasData || d.Weight <= eps {
			continue
		}
		total += d.Score * d.Weight
		wsum += d.Weight
	}
	if wsum <= eps {
		return 0, false
	}
	return clampPct(total / wsum), true
}

// escalationFor classifies the response to a composite score. Critically low
// scores escalate for review; the function deliberately has no "block" outcome
// because blocking requires human authorization, not a numeric threshold.
func escalationFor(score float64, hasData bool, reviewAt, escalateAt float64) Escalation {
	if !hasData {
		return EscNone
	}
	switch {
	case score < escalateAt:
		return EscEscalate
	case score < reviewAt:
		return EscReview
	default:
		return EscNone
	}
}

// meanScore averages a set of present KPI scores into a dimension score.
func meanScore(scores []float64, flags []bool) (float64, bool) {
	sum, n := 0.0, 0
	for i, s := range scores {
		if i < len(flags) && flags[i] {
			sum += s
			n++
		}
	}
	if n == 0 {
		return 0, false
	}
	return clampPct(sum / float64(n)), true
}

func clampPct(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return math.Round(v*100) / 100
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

func loadRec(ctx context.Context, model string, id int) map[string]interface{} {
	if id <= 0 {
		return nil
	}
	rec, err := orm.SearchOne(ctx, model, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return nil
	}
	return rec
}

func createRec(ctx context.Context, model string, vals map[string]interface{}) (int, error) {
	inst, ok := orm.Registry[model]
	if !ok {
		return 0, fmt.Errorf("model %s unavailable", model)
	}
	return orm.Create(ctx, inst, vals)
}

func str(v interface{}) string { return orm.AsString(v) }

func strOr(v interface{}, def string) string {
	if s := strings.TrimSpace(orm.AsString(v)); s != "" {
		return s
	}
	return def
}

func flt(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int64:
		return float64(t)
	case int:
		return float64(t)
	default:
		n, _ := orm.CoerceInt64(v)
		return float64(n)
	}
}

func intID(v interface{}) int {
	n, _ := orm.CoerceInt64(v)
	return int(n)
}

func boolOf(v interface{}) bool {
	b, _ := v.(bool)
	return b
}

func nextRef(ctx context.Context, prefix string) string {
	if name, err := orm.NextSequence(ctx, prefix); err == nil && name != "" {
		return name
	}
	return fmt.Sprintf("%s/%05d", prefix, time.Now().Unix()%100000)
}

func nowStr() string { return time.Now().Format(dtLayout) }

func todayStr() string { return time.Now().Format(dateLayout) }

func parseDay(v interface{}) (time.Time, bool) {
	s := strings.TrimSpace(str(v))
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{dateLayout, dtLayout, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Truncate(24 * time.Hour), true
		}
	}
	return time.Time{}, false
}

func appendHistory(ctx context.Context, model string, id int, note string) {
	cur := loadRec(ctx, model, id)
	if cur == nil {
		return
	}
	hist := strings.TrimSpace(str(cur["history"]))
	line := fmt.Sprintf("[%s] %s", time.Now().Format(dtLayout), strings.TrimSpace(note))
	if hist == "" {
		hist = line
	} else {
		hist = hist + "\n" + line
	}
	_ = orm.UpdateRecordByID(ctx, model, id, map[string]interface{}{"history": hist})
}

// supplierForPartner resolves the procurement supplier record for a partner id.
func supplierForPartner(ctx context.Context, partnerID int) int {
	if partnerID <= 0 {
		return 0
	}
	rows, err := orm.SearchPage(ctx, supplierModel, [][]interface{}{
		{"partner_id", "=", partnerID},
	}, 1, 0, "id asc")
	if err != nil || len(rows) == 0 {
		return 0
	}
	return intID(rows[0]["id"])
}

// ---------------------------------------------------------------------------
// Scorecard computation cron
// ---------------------------------------------------------------------------

// partAgg accumulates the raw evidence samples for one supplier within a period.
type partAgg struct {
	onTimeDenom, onTimeNum float64
	ordered, delivered     float64
	accepted, rejected     float64
	damaged                float64
	anyReceipt             bool
}

// runPerformanceCompute recomputes scorecards for periods awaiting computation.
// It is idempotent: a period is moved out of draft/computing once processed, and
// any scorecard already present for a supplier in that period is refreshed
// rather than duplicated.
func runPerformanceCompute(ctx context.Context, _ map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	periods, err := orm.SearchPage(bypass, periodModel, [][]interface{}{
		{"state", "in", []interface{}{string(PStateDraft), string(PStateComputing)}},
		{"active", "=", true},
	}, 50, 0, "date_start asc")
	if err != nil {
		return err
	}
	for _, p := range periods {
		if err := computePeriod(bypass, p); err != nil {
			// a single bad period must not abort the whole run
			appendHistory(bypass, periodModel, intID(p["id"]), "Compute error: "+err.Error())
		}
	}
	return nil
}

func computePeriod(bypass context.Context, period map[string]interface{}) error {
	pid := intID(period["id"])
	start, okS := parseDay(period["date_start"])
	end, okE := parseDay(period["date_end"])
	if !okS || !okE {
		return fmt.Errorf("period has invalid date bounds")
	}
	_ = orm.UpdateRecordByID(bypass, periodModel, pid, map[string]interface{}{"state": string(PStateComputing)})

	grace := period["on_time_treat"] == string(OtWithGrace)
	orderPartner := loadOrderPartners(bypass)

	// Delivery on-time evidence from schedules.
	aggs := map[int]*partAgg{}
	scheds, _ := orm.SearchPage(bypass, scheduleModel, [][]interface{}{
		{"state", "=", "received"},
	}, 5000, 0, "id asc")
	for _, s := range scheds {
		partner := orderPartner[intID(s["order_id"])]
		sid := supplierForPartner(bypass, partner)
		if sid <= 0 {
			continue
		}
		rec, hasRec := parseDay(s["received_date"])
		sch, hasSch := parseDay(s["scheduled_date"])
		if !hasRec || !hasSch || rec.Before(start) || rec.After(end) {
			continue
		}
		a := aggFor(aggs, sid)
		a.onTimeDenom++
		if rec.After(sch) && !grace {
			// late
		} else {
			a.onTimeNum++
		}
	}

	// Quantity evidence from receipts and their lines.
	receipts, _ := orm.SearchPage(bypass, receiptModel, [][]interface{}{
		{"state", "in", []interface{}{"accepted", "partially_accepted", "rejected"}},
	}, 5000, 0, "id asc")
	for _, r := range receipts {
		rd, ok := parseDay(r["receipt_date"])
		if !ok || rd.Before(start) || rd.After(end) {
			continue
		}
		partner := orderPartner[intID(r["order_id"])]
		sid := supplierForPartner(bypass, partner)
		if sid <= 0 {
			continue
		}
		lines, _ := orm.SearchPage(bypass, rcptLineModel, [][]interface{}{
			{"receipt_id", "=", intID(r["id"])},
		}, 1000, 0, "id asc")
		a := aggFor(aggs, sid)
		a.anyReceipt = true
		for _, l := range lines {
			a.ordered += flt(l["ordered_qty"])
			a.delivered += flt(l["delivered_qty"])
			a.accepted += flt(l["accepted_qty"])
			a.rejected += flt(l["rejected_qty"])
			a.damaged += flt(l["damaged_qty"])
		}
	}

	// Corrective-closure evidence (our own records).
	closure := correctiveClosure(bypass, pid)

	created := 0
	for sid, a := range aggs {
		if upsertScorecard(bypass, pid, sid, a, closure[sid], grace) {
			created++
		}
	}
	_ = orm.UpdateRecordByID(bypass, periodModel, pid, map[string]interface{}{"state": string(PStateComputed)})
	appendHistory(bypass, periodModel, pid, fmt.Sprintf("Computed %d supplier scorecard(s) for %s..%s.", created, str(period["date_start"]), str(period["date_end"])))
	return nil
}

func aggFor(m map[int]*partAgg, sid int) *partAgg {
	if a, ok := m[sid]; ok {
		return a
	}
	a := &partAgg{}
	m[sid] = a
	return a
}

func loadOrderPartners(ctx context.Context) map[int]int {
	out := map[int]int{}
	rows, _ := orm.SearchPage(ctx, orderModel, [][]interface{}{}, 10000, 0, "id asc")
	for _, r := range rows {
		out[intID(r["id"])] = intID(r["partner_id"])
	}
	return out
}

// correctiveClosure returns closed / total corrective actions per supplier that
// fall inside the period window.
func correctiveClosure(ctx context.Context, periodID int) map[int][2]float64 {
	out := map[int][2]float64{}
	rows, _ := orm.SearchPage(ctx, correctiveMdl, [][]interface{}{
		{"state", "!=", string(CaCancelled)},
	}, 5000, 0, "id asc")
	for _, r := range rows {
		sid := intID(r["supplier_id"])
		if sid <= 0 {
			continue
		}
		pair := out[sid]
		pair[1]++
		if str(r["state"]) == string(CaClosed) {
			pair[0]++
		}
		out[sid] = pair
	}
	return out
}

// upsertScorecard builds the KPI metric rows, the dimension scores, the
// weighted composite, the grade and the (non-blocking) escalation for one
// supplier, reusing an existing scorecard for the period when present.
func upsertScorecard(ctx context.Context, periodID, supplierID int, a *partAgg, closure [2]float64, grace bool) bool {
	type kpi struct {
		kind  KpiKind
		dim   Dimension
		dir   KpiDirection
		num   float64
		den   float64
		val   float64
		score float64
		has   bool
	}
	inspect := a.accepted + a.rejected + a.damaged
	defective := a.rejected + a.damaged

	onT, onTOK := onTimeScore(a.onTimeNum, a.onTimeDenom)
	fill, fillOK := fillScore(a.delivered, a.ordered)
	acc, accOK := acceptanceScore(a.accepted, inspect)
	def, defOK := defectScore(defective, a.delivered)
	clo, cloOK := ratePct(closure[0], closure[1])

	kpis := []kpi{
		{KpiOnTime, DimDelivery, DirHigher, a.onTimeNum, a.onTimeDenom, pct(a.onTimeNum, a.onTimeDenom), round2(onT), onTOK},
		{KpiFillRate, DimDelivery, DirHigher, math.Min(a.delivered, a.ordered), a.ordered, pct(a.delivered, a.ordered), round2(fill), fillOK},
		{KpiAcceptance, DimQuality, DirHigher, a.accepted, inspect, pct(a.accepted, inspect), round2(acc), accOK},
		{KpiDefect, DimQuality, DirLower, defective, a.delivered, pct(defective, a.delivered), round2(def), defOK},
		{KpiCorrective, DimCompliance, DirHigher, closure[0], closure[1], pct(closure[0], closure[1]), round2(clo), cloOK},
	}

	dimScores := map[Dimension][]float64{}
	dimFlags := map[Dimension][]bool{}
	for _, k := range kpis {
		dimScores[k.dim] = append(dimScores[k.dim], k.score)
		dimFlags[k.dim] = append(dimFlags[k.dim], k.has)
	}
	dimScore := map[Dimension]float64{}
	dimHas := map[Dimension]bool{}
	for d := range dimScores {
		s, ok := meanScore(dimScores[d], dimFlags[d])
		dimScore[d] = s
		dimHas[d] = ok
	}

	wq, wd, wc, wsv, wcp := defaultWeights(ctx, periodID, supplierID)
	dims := []dimValue{
		{dimScore[DimQuality], wq, dimHas[DimQuality]},
		{dimScore[DimDelivery], wd, dimHas[DimDelivery]},
		{dimScore[DimCost], wc, dimHas[DimCost]},
		{dimScore[DimService], wsv, dimHas[DimService]},
		{dimScore[DimCompliance], wcp, dimHas[DimCompliance]},
	}
	composite, hasComp := weightedComposite(dims)
	grade := gradeFor(composite, hasComp)
	esc := escalationFor(composite, hasComp, 70, 50)

	existing := existingScorecard(ctx, periodID, supplierID)
	vals := map[string]interface{}{
		"period_id":         periodID,
		"supplier_id":       supplierID,
		"quality_score":     dimScore[DimQuality],
		"delivery_score":    dimScore[DimDelivery],
		"cost_score":        dimScore[DimCost],
		"service_score":     dimScore[DimService],
		"compliance_score":  dimScore[DimCompliance],
		"composite_score":   composite,
		"grade":             string(grade),
		"escalation":        string(esc),
		"state":             string(ScCalculated),
		"weight_quality":    wq,
		"weight_delivery":   wd,
		"weight_cost":       wc,
		"weight_service":    wsv,
		"weight_compliance": wcp,
	}
	var scID int
	if existing > 0 {
		scID = existing
		_ = orm.UpdateRecordByID(ctx, scorecardMdl, scID, vals)
		// replace prior metric rows (recomputation, not history)
		deactivateMetrics(ctx, scID)
	} else {
		vals["name"] = nextRef(ctx, "SC")
		id, err := createRec(ctx, scorecardMdl, vals)
		if err != nil {
			return false
		}
		scID = id
	}

	for _, k := range kpis {
		if k.den <= eps {
			continue
		}
		_, _ = createRec(ctx, metricModel, map[string]interface{}{
			"name":         nextRef(ctx, "KPI"),
			"scorecard_id": scID,
			"period_id":    periodID,
			"supplier_id":  supplierID,
			"kpi":          string(k.kind),
			"dimension":    string(k.dim),
			"direction":    string(k.dir),
			"numerator":    round3(k.num),
			"denominator":  round3(k.den),
			"value":        round2(k.val),
			"score":        k.score,
			"has_data":     k.has,
			"date":         nowStr(),
		})
	}
	return true
}

func defaultWeights(ctx context.Context, periodID, supplierID int) (float64, float64, float64, float64, float64) {
	if sc := existingScorecard(ctx, periodID, supplierID); sc > 0 {
		if r := loadRec(ctx, scorecardMdl, sc); r != nil {
			return flt(r["weight_quality"]), flt(r["weight_delivery"]), flt(r["weight_cost"]), flt(r["weight_service"]), flt(r["weight_compliance"])
		}
	}
	return 30, 30, 20, 10, 10
}

func existingScorecard(ctx context.Context, periodID, supplierID int) int {
	rows, err := orm.SearchPage(ctx, scorecardMdl, [][]interface{}{
		{"period_id", "=", periodID}, {"supplier_id", "=", supplierID},
	}, 1, 0, "id asc")
	if err != nil || len(rows) == 0 {
		return 0
	}
	return intID(rows[0]["id"])
}

func deactivateMetrics(ctx context.Context, scorecardID int) {
	_, _ = orm.UnlinkWhere(ctx, metricModel, [][]interface{}{
		{"scorecard_id", "=", scorecardID},
	})
}

func pct(num, den float64) float64 {
	if den <= eps {
		return 0
	}
	return round2(num / den * 100)
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
func round3(v float64) float64 { return math.Round(v*1000) / 1000 }
