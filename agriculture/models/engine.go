package models

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"nexa/core/orm"
)

// Agriculture engine: activity amounts, plan cost rollups, farm area totals —
// all live computes feeding field-level profitability.

const (
	farmModel  = "agriculture.farm"
	fieldModel = "agriculture.field"
	planModel  = "agriculture.crop.plan"
	actModel   = "agriculture.activity"
)

func init() {
	orm.RegisterCompute(actModel, "amount", []string{"quantity", "unit_cost"}, computeActAmount)
	orm.RegisterCompute(planModel, "total_cost", []string{"id"}, computePlanCost)
	orm.RegisterCompute(farmModel, "area_ha_total", []string{"id"}, computeFarmArea)
}

func fnum(v interface{}) float64 {
	n, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(v)), 64)
	return n
}

func oid(v interface{}) int {
	if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
		n, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprint(arr[0])))
		return n
	}
	n, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprint(v)))
	return n
}

func computeActAmount(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	return fnum(rec["quantity"]) * fnum(rec["unit_cost"]), nil
}

func computePlanCost(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	acts, err := orm.SearchPage(orm.ContextWithBypass(ctx, true), actModel,
		[][]interface{}{{"plan_id", "=", oid(rec["id"])}}, 2000, 0, "id asc")
	if err != nil {
		return 0, nil
	}
	total := 0.0
	for _, a := range acts {
		total += fnum(a["quantity"]) * fnum(a["unit_cost"])
	}
	return total, nil
}

func computeFarmArea(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	fields, err := orm.SearchPage(orm.ContextWithBypass(ctx, true), fieldModel,
		[][]interface{}{{"farm_id", "=", oid(rec["id"])}}, 500, 0, "id asc")
	if err != nil {
		return 0, nil
	}
	total := 0.0
	for _, f := range fields {
		total += fnum(f["area_ha"])
	}
	return total, nil
}
