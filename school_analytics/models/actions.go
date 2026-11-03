package models

import (
	"context"
	"fmt"

	"nexa/core/orm"
)

// Object actions expose the aggregation engine on demand. Refreshing a KPI recomputes it
// from its source model and writes a fresh snapshot; refreshing a snapshot re-runs its
// KPI; refreshing a dashboard re-runs every KPI its tiles reference; and evaluating an
// alert rule re-checks the KPI's latest value against the threshold and stamps the trigger
// when met. All mutations run with access bypass and require a school officer.

func init() {
	orm.RegisterObjectAction(kpiModel, "kpi_refresh", actKpiRefresh)
	orm.RegisterObjectAction(snapshotModel, "snapshot_refresh", actSnapshotRefresh)
	orm.RegisterObjectAction(dashboardModel, "dashboard_refresh", actDashboardRefresh)
	orm.RegisterObjectAction(alertModel, "alert_evaluate", actAlertEvaluate)
}

func actKpiRefresh(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	if err := requireOfficerErr(ctx, "refresh a KPI"); err != nil {
		return "", err
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, kpiModel, id)
	if err != nil {
		return "", err
	}
	value, _, _, err := refreshKpiSnapshot(bypass, rec)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s = %s", strOr(rec["name"]), formatMetric(value, strOr(rec["unit"]))), nil
}

func actSnapshotRefresh(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	if err := requireOfficerErr(ctx, "refresh a snapshot"); err != nil {
		return "", err
	}
	bypass := orm.ContextWithBypass(ctx, true)
	snap, err := getRecord(bypass, snapshotModel, id)
	if err != nil {
		return "", err
	}
	kpi, err := getRecord(bypass, kpiModel, intOr(snap["kpi_id"]))
	if err != nil {
		return "", err
	}
	value, _, _, err := refreshKpiSnapshot(bypass, kpi)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s refreshed to %s", strOr(kpi["name"]), formatMetric(value, strOr(kpi["unit"]))), nil
}

func actDashboardRefresh(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	if err := requireOfficerErr(ctx, "refresh a dashboard"); err != nil {
		return "", err
	}
	bypass := orm.ContextWithBypass(ctx, true)
	if _, err := getRecord(bypass, dashboardModel, id); err != nil {
		return "", err
	}
	widgets, err := orm.Search(bypass, widgetModel, [][]interface{}{{"dashboard_id", "=", id}})
	if err != nil {
		return "", err
	}
	refreshed := 0
	for _, w := range widgets {
		kpi, err := getRecord(bypass, kpiModel, intOr(w["kpi_id"]))
		if err != nil {
			continue
		}
		if _, _, _, err := refreshKpiSnapshot(bypass, kpi); err == nil {
			refreshed++
		}
	}
	return fmt.Sprintf("Refreshed %d KPI(s)", refreshed), nil
}

func actAlertEvaluate(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	if err := requireOfficerErr(ctx, "evaluate an alert rule"); err != nil {
		return "", err
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rule, err := getRecord(bypass, alertModel, id)
	if err != nil {
		return "", err
	}
	v, _, ok := latestSnapshotValue(bypass, intOr(rule["kpi_id"]))
	if !ok {
		return "", fmt.Errorf("no snapshot is available for this KPI yet — refresh the KPI first")
	}
	before := intOr(rule["trigger_count"])
	evaluateAlert(bypass, rule)
	if compareTriggers(v, floatOr(rule["threshold"]), strOr(rule["comparison"])) {
		return fmt.Sprintf("Condition met (value %.2f); alert recorded", v), nil
	}
	if intOr(rule["trigger_count"]) != before {
		return "Alert recorded", nil
	}
	return fmt.Sprintf("Condition not met (value %.2f)", v), nil
}

// ---------------------------------------------------------------------------
// Shared action helpers
// ---------------------------------------------------------------------------

func getRecord(ctx context.Context, model string, id int) (map[string]interface{}, error) {
	rec, err := orm.SearchOne(ctx, model, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return nil, fmt.Errorf("record not found")
	}
	return rec, nil
}

func requireOfficer(ctx context.Context, uid int) bool {
	return orm.UserHasGroupXML(ctx, uid, "school.group_school_clerk") ||
		orm.UserHasGroupXML(ctx, uid, "school.group_school_manager")
}

func requireOfficerErr(ctx context.Context, what string) error {
	uid := orm.SecurityUID(ctx)
	if uid <= 0 || !requireOfficer(ctx, uid) {
		return fmt.Errorf("insufficient permission to %s", what)
	}
	return nil
}
