package models

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"nexa/core/orm"
	"nexa/core/scheduler"
)

// Automation layer for the preventive-maintenance companion. Two scheduled
// handlers realise the blueprint's "schedule generation engine" (§4.3) and the
// asset-meter rollup: they turn active MaintenancePlans into idempotent work
// orders and keep per-asset meter summaries current. The crons were already
// declared (data/maint_cron_data.xml) but had no handler until now.

const (
	dateLayout = "2006-01-02"
	dtLayout   = "2006-01-02 15:04:05"

	cronPMGenerate  = "maintenance.plan.generate.occurrences"
	cronMeterRollup = "maintenance.meter.rollup.summaries"

	mPlan        = "maintenance.plan"
	mOccurrence  = "maintenance.plan.occurrence"
	mWorkOrder   = "maintenance.work.order"
	mMeter       = "maintenance.meter"
	mMeterRead   = "maintenance.meter.reading"
	mMeterSumm   = "maintenance.asset.meter.summary"
	maxGenPerRun = 12
)

func init() {
	scheduler.RegisterCronHandler(cronPMGenerate, runPMGeneration)
	scheduler.RegisterCronHandler(cronMeterRollup, runMeterRollup)
}

// runPMGeneration evaluates every active plan and, for plans that are due,
// creates one work order per not-yet-generated occurrence (idempotent via the
// unique occurrence_key) and advances the plan's next due date.
func runPMGeneration(ctx context.Context, _ map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	now := time.Now().UTC()
	today := now.Format(dateLayout)

	plans := fetchAll(bypass, mPlan, [][]interface{}{
		{"active", "=", true},
		{"state", "=", string(PlanActive)},
	}, "id asc")

	for _, plan := range plans {
		switch TriggerType(strOr(plan["trigger"], "calendar")) {
		case TriggerCalendar:
			generateCalendarOccurrences(bypass, plan, now)
		case TriggerOnce:
			generateOneTime(bypass, plan, today)
		case TriggerHours, TriggerDistance, TriggerCycles:
			generateMeterOccurrences(bypass, plan, today)
		default:
			// condition-based plans are driven by the downtime threshold engine.
		}
	}
	return nil
}

// generateCalendarOccurrences walks the recurrence from the plan's next due
// date forward up to today, emitting one work order per occurrence.
func generateCalendarOccurrences(ctx context.Context, plan map[string]interface{}, now time.Time) {
	planID := atoi(str(plan["id"]))
	assetID := atoi(str(plan["asset_id"]))
	if assetID <= 0 {
		return
	}
	base := parseDate(str(plan["next_due_date"]))
	if base.IsZero() {
		base = now.UTC().Truncate(24 * time.Hour)
	}
	iv := atoi(strOr(plan["interval_value"], "1"))
	if iv <= 0 {
		iv = 1
	}
	unit := strOr(plan["interval_unit"], "days")

	generated := 0
	for !base.After(now) && generated < maxGenPerRun {
		pd := base.Format(dateLayout)
		key := fmt.Sprintf("%d:cal:%s", planID, pd)
		if generateOccurrenceAndWO(ctx, plan, assetID, key, pd, 0) {
			generated++
		}
		base = addInterval(base, iv, unit)
	}
	// Advance the plan's horizon to the next future occurrence.
	if err := orm.UpdateRecordByID(ctx, mPlan, planID, map[string]interface{}{
		"next_due_date": base.Format(dateLayout),
	}); err != nil {
		_ = err
	}
}

// generateOneTime emits a single work order the first time the plan becomes due.
func generateOneTime(ctx context.Context, plan map[string]interface{}, today string) {
	planID := atoi(str(plan["id"]))
	assetID := atoi(str(plan["asset_id"]))
	if assetID <= 0 {
		return
	}
	key := fmt.Sprintf("%d:once", planID)
	if generateOccurrenceAndWO(ctx, plan, assetID, key, today, 0) {
		_ = orm.UpdateRecordByID(ctx, mPlan, planID, map[string]interface{}{
			"state": string(PlanDone),
		})
	}
}

// generateMeterOccurrences fires when the bound usage meter has advanced past
// the last recorded threshold by at least one interval increment.
func generateMeterOccurrences(ctx context.Context, plan map[string]interface{}, today string) {
	planID := atoi(str(plan["id"]))
	assetID := atoi(str(plan["asset_id"]))
	meterID := atoi(str(plan["meter_id"]))
	if assetID <= 0 || meterID <= 0 {
		return
	}
	meter, err := orm.SearchOne(ctx, mMeter, map[string]interface{}{"id": meterID})
	if err != nil || meter == nil {
		return
	}
	cur := num(meter["current_value"])
	iv := num(strOr(plan["interval_value"], "1"))
	if iv <= 0 {
		iv = 1
	}
	last := lastMeterThreshold(ctx, planID)
	next := last + iv
	if next <= 0 || cur < next {
		return
	}
	key := fmt.Sprintf("%d:meter:%.2f", planID, next)
	generateOccurrenceAndWO(ctx, plan, assetID, key, today, next)
}

// lastMeterThreshold returns the highest meter threshold already materialised
// for a plan (0 when the plan has never generated an occurrence).
func lastMeterThreshold(ctx context.Context, planID int) float64 {
	occ := fetchAll(ctx, mOccurrence, [][]interface{}{
		{"plan_id", "=", planID},
		{"state", "=", string(OccurrenceGenerated)},
	}, "meter_threshold desc, id desc")
	if len(occ) == 0 {
		return 0
	}
	return num(occ[0]["meter_threshold"])
}

// generateOccurrenceAndWO creates a planned work order and its idempotent
// occurrence record. Returns false when the occurrence key already exists.
func generateOccurrenceAndWO(ctx context.Context, plan map[string]interface{}, assetID int, key, plannedDate string, meterThreshold float64) bool {
	if n, _ := orm.SearchCount(ctx, mOccurrence, [][]interface{}{{"occurrence_key", "=", key}}); n > 0 {
		return false
	}
	planID := atoi(str(plan["id"]))
	woID := createPreventiveWO(ctx, plan, assetID, plannedDate)
	if woID <= 0 {
		return false
	}
	occ := orm.Registry[mOccurrence]
	if occ == nil {
		return false
	}
	_, err := orm.Create(ctx, occ, map[string]interface{}{
		"plan_id":         planID,
		"asset_id":        assetID,
		"occurrence_key":  key,
		"state":           string(OccurrenceGenerated),
		"planned_date":    plannedDate,
		"meter_threshold": meterThreshold,
		"work_order_id":   woID,
		"generated_at":    time.Now().UTC().Format(dtLayout),
		"notes":           "Auto-generated by preventive-maintenance scheduler",
	})
	return err == nil
}

func createPreventiveWO(ctx context.Context, plan map[string]interface{}, assetID int, plannedDate string) int {
	wo := orm.Registry[mWorkOrder]
	if wo == nil {
		return 0
	}
	name := fmt.Sprintf("PM/%s/P%d", plannedDate, atoi(str(plan["id"])))
	vals := map[string]interface{}{
		"name":            name,
		"title":           strOr(plan["name"], "Preventive Maintenance"),
		"asset_id":        assetID,
		"work_type":       strOr(plan["strategy"], string(TriggerCalendar)),
		"priority":        "normal",
		"state":           "planned",
		"planned_start":   plannedDate,
		"planned_finish":  plannedDate,
		"repair_plan":     str(plan["instructions"]),
		"estimated_hours": num(strOr(plan["estimated_hours"], "0")),
		"notes":           "Generated from maintenance plan (auto-scheduler)",
	}
	if resp := atoi(str(plan["responsible_id"])); resp > 0 {
		// best-effort: keep technician blank, responsible is a team lead reference
		_ = resp
	}
	id, err := orm.Create(ctx, wo, vals)
	if err != nil {
		return 0
	}
	return id
}

// runMeterRollup recomputes per-asset/meter usage summaries from readings.
func runMeterRollup(ctx context.Context, _ map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	meters := fetchAll(bypass, mMeter, [][]interface{}{{"active", "=", true}}, "id asc")
	for _, meter := range meters {
		mid := atoi(str(meter["id"]))
		aid := atoi(str(meter["asset_id"]))
		if mid <= 0 || aid <= 0 {
			continue
		}
		readings := fetchAll(bypass, mMeterRead, [][]interface{}{{"meter_id", "=", mid}}, "date asc, id asc")
		var (
			count         int
			sum, min, max float64
			latestDate    string
			latestVal     float64
		)
		for i, r := range readings {
			v := num(r["value"])
			sum += v
			if i == 0 || v < min {
				min = v
			}
			if i == 0 || v > max {
				max = v
			}
			latestDate = str(r["date"])
			latestVal = v
			count++
		}
		avg := 0.0
		if count > 0 {
			avg = sum / float64(count)
		}
		thr, overdue := nextMeterDue(bypass, mid, num(meter["current_value"]))
		vals := map[string]interface{}{
			"latest_reading_date": latestDate,
			"latest_value":        latestVal,
			"reading_count":       count,
			"min_value":           min,
			"max_value":           max,
			"avg_value":           avg,
			"next_due_threshold":  thr,
			"is_overdue":          overdue,
			"updated_at":          time.Now().UTC().Format(dtLayout),
		}
		upsertMeterSummary(bypass, aid, mid, vals)
	}
	return nil
}

// nextMeterDue derives the earliest outstanding usage threshold for plans bound
// to this meter and whether the current reading already exceeds it.
func nextMeterDue(ctx context.Context, meterID int, cur float64) (float64, bool) {
	plans := fetchAll(ctx, mPlan, [][]interface{}{
		{"meter_id", "=", meterID},
		{"active", "=", true},
		{"state", "=", string(PlanActive)},
	}, "id asc")
	best := 0.0
	for _, p := range plans {
		iv := num(strOr(p["interval_value"], "1"))
		if iv <= 0 {
			continue
		}
		next := lastMeterThreshold(ctx, atoi(str(p["id"]))) + iv
		if next <= 0 {
			continue
		}
		if best == 0 || next < best {
			best = next
		}
	}
	if best > 0 && cur >= best {
		return best, true
	}
	return best, false
}

func upsertMeterSummary(ctx context.Context, assetID, meterID int, vals map[string]interface{}) {
	existing := fetchAll(ctx, mMeterSumm, [][]interface{}{
		{"asset_id", "=", assetID},
		{"meter_id", "=", meterID},
	}, "id asc")
	if len(existing) > 0 {
		_ = orm.UpdateRecordByID(ctx, mMeterSumm, atoi(str(existing[0]["id"])), vals)
		return
	}
	summ := orm.Registry[mMeterSumm]
	if summ == nil {
		return
	}
	vals["asset_id"] = assetID
	vals["meter_id"] = meterID
	_, _ = orm.Create(ctx, summ, vals)
}

// ---------------------------------------------------------------------------
// local helpers
// ---------------------------------------------------------------------------

func fetchAll(ctx context.Context, model string, domain [][]interface{}, order string) []map[string]interface{} {
	var out []map[string]interface{}
	offset := 0
	const page = 500
	for {
		rows, err := orm.SearchPage(ctx, model, domain, page, offset, order)
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

func parseDate(s string) time.Time {
	s = first8(s)
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func first8(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func addInterval(t time.Time, n int, unit string) time.Time {
	switch unit {
	case "weeks":
		return t.AddDate(0, 0, n*7)
	case "months":
		return t.AddDate(0, n, 0)
	case "years":
		return t.AddDate(n, 0, 0)
	default: // days
		return t.AddDate(0, 0, n)
	}
}

func str(v interface{}) string { return orm.AsString(v) }

func strOr(v interface{}, def string) string {
	s := trim(orm.AsString(v))
	if s == "" {
		return def
	}
	return s
}

func trim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 {
		c := s[len(s)-1]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			s = s[:len(s)-1]
			continue
		}
		break
	}
	return s
}

func atoi(s string) int {
	n, _ := strconv.Atoi(trim(s))
	return n
}

func num(v interface{}) float64 {
	f, _ := strconv.ParseFloat(trim(orm.AsString(v)), 64)
	return f
}
