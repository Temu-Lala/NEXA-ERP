package models

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"nexa/core/orm"
	"nexa/core/scheduler"
)

// Condition-monitoring engine (blueprint §9.2). The
// "maintenance.condition.evaluate.thresholds" cron was declared in
// data/downtime_cron_data.xml but had no handler. It compares the latest
// parameter reading for each active rule against its configured threshold,
// raising a de-duplicated condition alert on breach (with debounce and
// suppression) and auto-resolving alerts once the value returns inside the
// hysteresis band. Critical breaches can raise a corrective work order.

const (
	dtDateLayout = "2006-01-02"
	dtDateTime   = "2006-01-02 15:04:05"

	cronConditionEval = "maintenance.condition.evaluate.thresholds"

	mCondRule    = "maintenance.condition.rule"
	mCondReading = "maintenance.condition.reading"
	mCondAlert   = "maintenance.condition.alert"
	mAsset       = "maintenance.asset"
	mWorkOrderV2 = "maintenance.work.order"

	floatEps = 1e-9
)

func init() {
	scheduler.RegisterCronHandler(cronConditionEval, runConditionEvaluation)
}

func runConditionEvaluation(ctx context.Context, _ map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	rules := fetchCondAll(bypass, mCondRule, [][]interface{}{{"active", "=", true}}, "id asc")
	for _, rule := range rules {
		evaluateRule(bypass, rule)
	}
	return nil
}

func evaluateRule(ctx context.Context, rule map[string]interface{}) {
	ruleID := atoiCond(strCond(rule["id"]))
	param := strCond(rule["parameter_name"])
	if param == "" {
		return
	}
	op := ConditionRuleOperator(strOrCond(rule["operator"], "gt"))
	thr := numCond(rule["threshold_value"])
	hyst := numCond(rule["hysteresis_value"])
	debounce := atoiCond(strOrCond(rule["debounce_minutes"], "0"))

	for _, assetID := range resolveRuleAssets(ctx, rule) {
		readings := fetchCondAll(ctx, mCondReading, [][]interface{}{
			{"asset_id", "=", assetID},
			{"parameter_name", "=", param},
		}, "reading_time desc, id desc")
		if len(readings) == 0 {
			continue
		}
		latest := readings[0]
		val := numCond(latest["value"])
		breach := compareCondition(val, op, thr)

		active := fetchCondAll(ctx, mCondAlert, [][]interface{}{
			{"rule_id", "=", ruleID},
			{"asset_id", "=", assetID},
			{"state", "=", string(AlertStateActive)},
		}, "id desc")

		if breach {
			if len(active) > 0 {
				continue // alert already open for this rule+asset
			}
			if debounce > 0 && recentlyAlerted(ctx, ruleID, assetID, debounce) {
				continue
			}
			raiseAlert(ctx, rule, ruleID, assetID, latest, param, val, thr)
		} else if len(active) > 0 && withinClear(val, op, thr, hyst) {
			resolveAlert(ctx, atoiCond(strCond(active[0]["id"])))
		}
	}
}

// raiseAlert creates an active condition alert, tags the triggering reading and
// optionally opens a corrective work order for critical severities.
func raiseAlert(ctx context.Context, rule map[string]interface{}, ruleID, assetID int, reading map[string]interface{}, param string, val, thr float64) {
	severity := AlertSeverity(strOrCond(rule["alert_severity"], "warning"))
	msg := strCond(rule["alert_message"])
	if msg == "" {
		msg = fmt.Sprintf("Threshold breach: %s = %.4f (%s %.4f)", param, val, string(opOf(rule)), thr)
	}
	alert := orm.Registry[mCondAlert]
	if alert == nil {
		return
	}
	name := fmt.Sprintf("ALERT/%s/%s/A%d", param, time.Now().UTC().Format(dtDateLayout), assetID)
	aid, err := orm.Create(ctx, alert, map[string]interface{}{
		"name":            name,
		"asset_id":        assetID,
		"rule_id":         ruleID,
		"state":           string(AlertStateActive),
		"alert_severity":  string(severity),
		"alert_time":      time.Now().UTC().Format(dtDateTime),
		"triggered_value": val,
		"threshold_value": thr,
		"parameter_name":  param,
		"unit":            strCond(rule["unit"]),
		"alert_message":   msg,
	})
	if err != nil {
		return
	}
	_ = orm.UpdateRecordByID(ctx, mCondReading, atoiCond(strCond(reading["id"])), map[string]interface{}{
		"triggered_alert": true,
		"alert_id":        aid,
	})
	if boolCond(rule["create_maintenance_alert"]) && (severity == AlertSevCritical || severity == AlertSevEmergency) {
		linkWorkOrder(ctx, aid, assetID, rule, param, msg)
	}
}

// linkWorkOrder opens a draft corrective work order and links it to the alert.
func linkWorkOrder(ctx context.Context, alertID, assetID int, rule map[string]interface{}, param, msg string) {
	wo := orm.Registry[mWorkOrderV2]
	if wo == nil {
		return
	}
	name := fmt.Sprintf("COND/%s/A%d", time.Now().UTC().Format(dtDateLayout), assetID)
	prio := strOrCond(rule["maintenance_priority"], "high")
	wid, err := orm.Create(ctx, wo, map[string]interface{}{
		"name":      name,
		"title":     fmt.Sprintf("Condition alert: %s", param),
		"asset_id":  assetID,
		"work_type": "condition_based",
		"priority":  prio,
		"state":     "draft",
		"notes":     msg,
	})
	if err != nil {
		return
	}
	_ = orm.UpdateRecordByID(ctx, mCondAlert, alertID, map[string]interface{}{"work_order_id": wid})
}

func resolveAlert(ctx context.Context, alertID int) {
	if alertID <= 0 {
		return
	}
	_ = orm.UpdateRecordByID(ctx, mCondAlert, alertID, map[string]interface{}{
		"state":         string(AlertStateResolved),
		"resolved_time": time.Now().UTC().Format(dtDateTime),
	})
}

// recentlyAlerted suppresses re-raising while an alert (any state) for the
// rule+asset was created within the debounce window.
func recentlyAlerted(ctx context.Context, ruleID, assetID, debounceMinutes int) bool {
	cutoff := time.Now().UTC().Add(-time.Duration(debounceMinutes) * time.Minute).Format(dtDateTime)
	alerts := fetchCondAll(ctx, mCondAlert, [][]interface{}{
		{"rule_id", "=", ruleID},
		{"asset_id", "=", assetID},
	}, "alert_time desc, id desc")
	if len(alerts) == 0 {
		return false
	}
	last := strCond(alerts[0]["alert_time"])
	// if the last alert was cleared (resolved) and the value re-breached inside
	// the debounce window, hold off to avoid alert storms.
	if last >= cutoff {
		if sup := strCond(alerts[0]["suppressed_until"]); sup != "" && sup >= time.Now().UTC().Format(dtDateTime) {
			return true
		}
		return true
	}
	return false
}

func resolveRuleAssets(ctx context.Context, rule map[string]interface{}) []int {
	if aid := atoiCond(strCond(rule["asset_id"])); aid > 0 {
		return []int{aid}
	}
	if cid := atoiCond(strCond(rule["asset_category_id"])); cid > 0 {
		assets := fetchCondAll(ctx, mAsset, [][]interface{}{
			{"category_id", "=", cid},
			{"active", "=", true},
		}, "id asc")
		return collectIDs(assets)
	}
	assets := fetchCondAll(ctx, mAsset, [][]interface{}{{"active", "=", true}}, "id asc")
	return collectIDs(assets)
}

func collectIDs(rows []map[string]interface{}) []int {
	out := make([]int, 0, len(rows))
	for _, r := range rows {
		if id := atoiCond(strCond(r["id"])); id > 0 {
			out = append(out, id)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// pure comparisons (unit-testable)
// ---------------------------------------------------------------------------

func compareCondition(val float64, op ConditionRuleOperator, thr float64) bool {
	switch op {
	case CondOpGt:
		return val > thr
	case CondOpGte:
		return val >= thr-floatEps
	case CondOpLt:
		return val < thr
	case CondOpLte:
		return val <= thr+floatEps
	case CondOpEq:
		return math.Abs(val-thr) <= floatEps
	case CondOpNeq:
		return math.Abs(val-thr) > floatEps
	default:
		return false
	}
}

// withinClear decides if an open alert may be resolved given the hysteresis band.
func withinClear(val float64, op ConditionRuleOperator, thr, hyst float64) bool {
	switch op {
	case CondOpGt, CondOpGte:
		return val <= thr-hyst
	case CondOpLt, CondOpLte:
		return val >= thr+hyst
	default:
		return !compareCondition(val, op, thr)
	}
}

func opOf(rule map[string]interface{}) ConditionRuleOperator {
	return ConditionRuleOperator(strOrCond(rule["operator"], "gt"))
}

// ---------------------------------------------------------------------------
// local helpers
// ---------------------------------------------------------------------------

func fetchCondAll(ctx context.Context, model string, domain [][]interface{}, order string) []map[string]interface{} {
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

func strCond(v interface{}) string { return orm.AsString(v) }

func strOrCond(v interface{}, def string) string {
	s := trimCond(orm.AsString(v))
	if s == "" {
		return def
	}
	return s
}

func trimCond(s string) string {
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

func atoiCond(s string) int {
	n, _ := strconv.Atoi(trimCond(s))
	return n
}

func numCond(v interface{}) float64 {
	f, _ := strconv.ParseFloat(trimCond(orm.AsString(v)), 64)
	return f
}

func boolCond(v interface{}) bool { return orm.AsBool(v) }
