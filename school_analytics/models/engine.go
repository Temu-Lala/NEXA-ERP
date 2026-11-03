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

// Behaviour layer for school analytics. The metric arithmetic is pure and unit-tested:
// a safe rate percentage, list average and sum (both rounded to two decimals), a trend
// direction against a tolerance, a gauge band classification and a threshold comparison
// for alerts. The computes project a KPI's latest snapshot value, a snapshot's formatted
// display value (read through its KPI's unit), a dashboard's tile count and whether an
// alert condition is currently met. A daily cron recomputes every active KPI by
// aggregating its configured source model through the ORM registry and writes a fresh
// snapshot, then re-evaluates the active alert rules.

const (
	kpiModel       = "school.analytics.kpi"
	snapshotModel  = "school.analytics.snapshot"
	dashboardModel = "school.analytics.dashboard"
	widgetModel    = "school.analytics.dashboard.widget"
	alertModel     = "school.analytics.alert.rule"

	refreshCronCode = "school.analytics.refresh"

	dateLayout       = "2006-01-02"
	defaultTolerance = 0.0001
)

// ratePct is the numerator's share of the denominator as a percentage, guarding a zero
// denominator to 0.
func ratePct(numerator, denominator int) float64 {
	if denominator <= 0 {
		return 0
	}
	return round2(float64(numerator) / float64(denominator) * 100)
}

// averageOf is the arithmetic mean of a slice, 0 for an empty slice, rounded to 2dp.
func averageOf(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	return round2(sumOf(values) / float64(len(values)))
}

// sumOf totals a slice, rounded to 2dp.
func sumOf(values []float64) float64 {
	var s float64
	for _, v := range values {
		s += v
	}
	return round2(s)
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}

// trendDirection compares a current and previous value against a tolerance.
func trendDirection(current, previous, tolerance float64) string {
	tol := tolerance
	if tol <= 0 {
		tol = defaultTolerance
	}
	d := current - previous
	switch {
	case d > tol:
		return "up"
	case d < -tol:
		return "down"
	default:
		return "flat"
	}
}

// band classifies a value into a gauge band: below warnBelow is critical, at or above
// goodAt is good, otherwise warning.
func band(value, warnBelow, goodAt float64) string {
	switch {
	case value < warnBelow:
		return "critical"
	case value >= goodAt:
		return "good"
	default:
		return "warning"
	}
}

// compareTriggers evaluates whether a value satisfies an alert comparison.
func compareTriggers(value, threshold float64, cmp string) bool {
	switch cmp {
	case string(CmpLt):
		return value < threshold
	case string(CmpLte):
		return value <= threshold
	case string(CmpGt):
		return value > threshold
	case string(CmpGte):
		return value >= threshold
	default:
		return false
	}
}

// parseInList splits a comma-separated numerator value list into trimmed, non-empty
// tokens.
func parseInList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// formatMetric renders a numeric value for display according to its unit.
func formatMetric(v float64, unit string) string {
	switch unit {
	case string(UnitPercent):
		return fmt.Sprintf("%.1f%%", v)
	case string(UnitAverage), string(UnitSum):
		return fmt.Sprintf("%.2f", v)
	default: // count
		return fmt.Sprintf("%d", int(math.Round(v)))
	}
}

// ---------------------------------------------------------------------------
// Computes
// ---------------------------------------------------------------------------

func init() {
	orm.RegisterCompute(kpiModel, "latest_value", []string{"id"}, computeKpiLatestValue)
	orm.RegisterCompute(snapshotModel, "formatted_value", []string{"numeric_value", "kpi_id"}, computeSnapshotFormattedValue)
	orm.RegisterCompute(dashboardModel, "widget_count", []string{"id"}, computeDashboardWidgetCount)
	orm.RegisterCompute(alertModel, "meets_now", []string{"kpi_id", "threshold", "comparison"}, computeAlertMeetsNow)
	scheduler.RegisterCronHandler(refreshCronCode, runAnalyticsRefresh)
}

func computeKpiLatestValue(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	v, _, ok := latestSnapshotValue(orm.ContextWithBypass(ctx, true), intOr(rec["id"]))
	if !ok {
		return 0.0, nil
	}
	return v, nil
}

func computeSnapshotFormattedValue(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	unit := "count"
	if kpi := intOr(rec["kpi_id"]); kpi > 0 {
		if k, err := orm.SearchOne(orm.ContextWithBypass(ctx, true), kpiModel, map[string]interface{}{"id": kpi}); err == nil && k != nil {
			unit = strOr(k["unit"])
		}
	}
	return formatMetric(floatOr(rec["numeric_value"]), unit), nil
}

func computeDashboardWidgetCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), widgetModel, [][]interface{}{{"dashboard_id", "=", intOr(rec["id"])}})
	if err != nil {
		return 0, nil
	}
	return n, nil
}

func computeAlertMeetsNow(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	v, _, ok := latestSnapshotValue(orm.ContextWithBypass(ctx, true), intOr(rec["kpi_id"]))
	if !ok {
		return false, nil
	}
	return compareTriggers(v, floatOr(rec["threshold"]), strOr(rec["comparison"])), nil
}

// ---------------------------------------------------------------------------
// Snapshot + aggregation helpers (shared by computes, actions and the cron)
// ---------------------------------------------------------------------------

// latestSnapshotValue returns the most recent snapshot value for a KPI (by id order).
func latestSnapshotValue(ctx context.Context, kpiID int) (float64, int, bool) {
	if kpiID <= 0 {
		return 0, 0, false
	}
	rows, err := orm.Search(ctx, snapshotModel, [][]interface{}{{"kpi_id", "=", kpiID}})
	if err != nil || len(rows) == 0 {
		return 0, 0, false
	}
	best := rows[0]
	for _, r := range rows[1:] {
		if intOr(r["id"]) > intOr(best["id"]) {
			best = r
		}
	}
	return floatOr(best["numeric_value"]), intOr(best["id"]), true
}

// aggregateKpi performs the KPI's configured aggregation over its source model. Source
// models are addressed by name through the ORM registry, so this module measures other
// modules without importing them. Returns the value plus the numerator/denominator.
func aggregateKpi(ctx context.Context, kpi map[string]interface{}) (float64, int, int, error) {
	source := strOr(kpi["source_model"])
	if source == "" {
		return 0, 0, 0, fmt.Errorf("KPI %q has no source model configured", strOr(kpi["code"]))
	}
	baseDomain := buildBaseDomain(kpi)

	switch strOr(kpi["aggregation"]) {
	case string(AggAverage), string(AggSum):
		rows, err := orm.Search(ctx, source, baseDomain)
		if err != nil {
			return 0, 0, 0, err
		}
		vf := strOr(kpi["value_field"])
		vals := make([]float64, 0, len(rows))
		for _, r := range rows {
			vals = append(vals, floatOr(r[vf]))
		}
		n := len(vals)
		if strOr(kpi["aggregation"]) == string(AggSum) {
			return sumOf(vals), n, n, nil
		}
		return averageOf(vals), n, n, nil
	case string(AggRate):
		total, err := orm.SearchCount(ctx, source, baseDomain)
		if err != nil {
			return 0, 0, 0, err
		}
		numDomain := append(append([][]interface{}{}, baseDomain...), rateNumeratorFilter(kpi))
		num, err := orm.SearchCount(ctx, source, numDomain)
		if err != nil {
			return 0, 0, total, err
		}
		return ratePct(num, total), num, total, nil
	default: // count
		total, err := orm.SearchCount(ctx, source, baseDomain)
		if err != nil {
			return 0, 0, 0, err
		}
		return float64(total), total, total, nil
	}
}

// buildBaseDomain assembles the KPI's optional single equality filter.
func buildBaseDomain(kpi map[string]interface{}) [][]interface{} {
	field := strOr(kpi["filter_field"])
	value := strOr(kpi["filter_value"])
	domain := [][]interface{}{}
	if field != "" && value != "" {
		domain = append(domain, []interface{}{field, "=", coerceScalar(value)})
	}
	return domain
}

// rateNumeratorFilter builds the "field in (values)" filter for a rate KPI.
func rateNumeratorFilter(kpi map[string]interface{}) []interface{} {
	field := strOr(kpi["num_field"])
	list := parseInList(strOr(kpi["num_values"]))
	values := make([]interface{}, 0, len(list))
	for _, v := range list {
		values = append(values, coerceScalar(v))
	}
	return []interface{}{field, "in", values}
}

// coerceScalar interprets a filter string as a bool, an integer or a plain string.
func coerceScalar(s string) interface{} {
	switch strings.ToLower(s) {
	case "true":
		return true
	case "false":
		return false
	}
	if i, err := parseIntStrict(s); err == nil {
		return i
	}
	return s
}

func parseIntStrict(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	if err != nil {
		return 0, err
	}
	return i, nil
}

// runAnalyticsRefresh is the daily worker: recompute every active KPI into a fresh
// snapshot, then re-evaluate every active alert rule.
func runAnalyticsRefresh(ctx context.Context, _ map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	kpis, err := orm.Search(bypass, kpiModel, [][]interface{}{{"active", "=", true}})
	if err != nil {
		return err
	}
	for _, k := range kpis {
		_, _, _, _ = refreshKpiSnapshot(bypass, k)
	}
	rules, err := orm.Search(bypass, alertModel, [][]interface{}{{"active", "=", true}})
	if err != nil {
		return err
	}
	for _, r := range rules {
		evaluateAlert(bypass, r)
	}
	return nil
}

// refreshKpiSnapshot computes a KPI and writes a new snapshot row, returning the value.
func refreshKpiSnapshot(ctx context.Context, kpi map[string]interface{}) (float64, int, int, error) {
	value, num, den, err := aggregateKpi(ctx, kpi)
	if err != nil {
		return 0, 0, 0, err
	}
	_, _ = orm.Create(ctx, orm.RegistryModel(snapshotModel), map[string]interface{}{
		"kpi_id":        intOr(kpi["id"]),
		"snapshot_date": time.Now().UTC().Format(dateLayout),
		"numeric_value": value,
		"numerator":     num,
		"denominator":   den,
		"company_id":    intOr(kpi["company_id"]),
	})
	return value, num, den, nil
}

// evaluateAlert stamps the trigger date/counter when the rule's condition is met.
func evaluateAlert(ctx context.Context, rule map[string]interface{}) {
	v, _, ok := latestSnapshotValue(ctx, intOr(rule["kpi_id"]))
	if !ok {
		return
	}
	if !compareTriggers(v, floatOr(rule["threshold"]), strOr(rule["comparison"])) {
		return
	}
	_ = orm.UpdateRecordByID(ctx, alertModel, intOr(rule["id"]), map[string]interface{}{
		"last_triggered": time.Now().UTC().Format(dateLayout),
		"trigger_count":  intOr(rule["trigger_count"]) + 1,
	})
}

// ---------------------------------------------------------------------------
// Shared coercion helpers (ORM values arrive as interface{})
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
	return 0
}
