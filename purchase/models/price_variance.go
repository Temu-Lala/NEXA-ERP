package models

import (
	"context"
	"strconv"
	"strings"

	"nexa/core/orm"
)

// Price variance (spec §14.2): compares an order line's unit price against the
// most recent *other* purchase order line for the same product — the honest
// benchmark available without a cross-model supplier mapping. Positive
// variance = paying more than last time.

func init() {
	deps := []string{"price_unit", "product_id", "order_id"}
	orm.RegisterCompute("purchase.order.line", "benchmark_price", deps, computeLineBenchmarkPrice)
	orm.RegisterCompute("purchase.order.line", "price_variance_pct", deps, computeLinePriceVariance)
}

func computeLineBenchmarkPrice(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return lastLinePrice(ctx, rec, ""), nil
}

func computeLinePriceVariance(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	last := lastLinePrice(ctx, rec, "")
	if last <= 0 {
		return 0, nil
	}
	cur := numFrom(rec["price_unit"])
	return (cur - last) / last * 100, nil
}

// lastLinePrice finds the most recent price_unit on another order line for the
// same product. ownID excludes the current row; used by both computes.
func lastLinePrice(ctx context.Context, rec map[string]interface{}, _ string) float64 {
	pid := ormID(rec["product_id"])
	if pid <= 0 {
		return 0
	}
	own := intID(rec["id"])
	rows, err := orm.SearchPage(orm.ContextWithBypass(ctx, true), "purchase.order.line",
		[][]interface{}{{"product_id", "=", pid}, {"id", "!=", own}}, 500, 0, "id desc")
	if err != nil {
		return 0
	}
	// Most recent by order line id — take the first with a positive price.
	for _, r := range rows {
		if p := numFrom(r["price_unit"]); p > 0 {
			return p
		}
	}
	return 0
}

func numFrom(v interface{}) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(orm.AsString(v)), 64)
	return f
}

func ormID(v interface{}) int {
	if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
		n, _ := strconv.Atoi(orm.AsString(arr[0]))
		return n
	}
	n, _ := strconv.Atoi(orm.AsString(v))
	return n
}

func intID(v interface{}) int {
	n, _ := strconv.Atoi(orm.AsString(v))
	return n
}
