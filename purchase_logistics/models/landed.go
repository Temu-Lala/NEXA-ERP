package models

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"nexa/core/orm"
)

// Landed-cost engine: distributes a shipment's freight charges across the
// receipt (or PO) lines it covers, proportionally by line value — the spec's
// configurable basis (quantity/weight/value) starts with value, the most
// common accounting basis. Allocation is computed on demand, never stored, so
// it always reflects current charges and lines.

const (
	shipmentModel = "purchase.shipment"
	chargeModel   = "purchase.freight.charge"
	rcptLineModel = "purchase.receipt.line"
	poLineModel   = "purchase.order.line"
)

func init() {
	orm.RegisterCompute(shipmentModel, "freight_total", []string{"id"}, computeShipmentFreightTotal)
	orm.RegisterCompute(shipmentModel, "charge_count", []string{"id"}, computeShipmentChargeCount)
	orm.RegisterObjectAction(shipmentModel, "action_landed_allocation", shipmentLandedAllocation)
}

func computeShipmentFreightTotal(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return shipmentChargeSum(ctx, intID(rec["id"])), nil
}

func computeShipmentChargeCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	rows, err := orm.SearchPage(orm.ContextWithBypass(ctx, true), chargeModel,
		[][]interface{}{{"shipment_id", "=", intID(rec["id"])}}, 500, 0, "id asc")
	if err != nil {
		return 0, nil
	}
	return len(rows), nil
}

func shipmentChargeSum(ctx context.Context, shipmentID int) float64 {
	if shipmentID <= 0 {
		return 0
	}
	rows, err := orm.SearchPage(orm.ContextWithBypass(ctx, true), chargeModel,
		[][]interface{}{{"shipment_id", "=", shipmentID}}, 500, 0, "id asc")
	if err != nil {
		return 0
	}
	total := 0.0
	for _, r := range rows {
		total += asFloat(r["amount"])
	}
	return total
}

// shipmentLandedAllocation returns the per-line landed-cost breakdown as text:
// each receipt line's share of every freight charge, proportional to its
// line value. Falls back to PO lines when the shipment has no receipt yet.
func shipmentLandedAllocation(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	ship := loadShipment(bypass, id)
	if ship == nil {
		return "", fmt.Errorf("shipment not found")
	}
	charges, err := orm.SearchPage(bypass, chargeModel,
		[][]interface{}{{"shipment_id", "=", id}}, 500, 0, "id asc")
	if err != nil || len(charges) == 0 {
		return "No freight charges on this shipment.", nil
	}

	// Lines: prefer the linked receipt's lines, else the PO's lines.
	var lines []map[string]interface{}
	var lineModel string
	if rid := intID(ship["receipt_id"]); rid > 0 {
		lines, _ = orm.SearchPage(bypass, rcptLineModel,
			[][]interface{}{{"receipt_id", "=", rid}}, 500, 0, "id asc")
		lineModel = rcptLineModel
	}
	if len(lines) == 0 {
		if oid := intID(ship["order_id"]); oid > 0 {
			lines, _ = orm.SearchPage(bypass, poLineModel,
				[][]interface{}{{"order_id", "=", oid}}, 500, 0, "id asc")
			lineModel = poLineModel
		}
	}
	if len(lines) == 0 {
		return "No receipt or PO lines linked — cannot allocate.", nil
	}

	// Basis: line value (qty × price). Lines without a computable value share
	// equally so allocation never silently drops cost.
	type alloc struct {
		label string
		value float64
	}
	allocs := make([]alloc, 0, len(lines))
	totalValue := 0.0
	for _, l := range lines {
		qty := asFloat(l["quantity"])
		if qty == 0 {
			qty = asFloat(l["qty"])
		}
		price := asFloat(l["price_unit"])
		if price == 0 {
			price = asFloat(l["unit_price"])
		}
		if price == 0 {
			price = asFloat(l["price"])
		}
		v := qty * price
		if v <= 0 {
			v = 1 // equal-share fallback marker
		}
		label := orm.AsString(l["product_id"])
		if label == "" {
			label = fmt.Sprintf("line %d", intID(l["id"]))
		}
		allocs = append(allocs, alloc{label: label, value: v})
		totalValue += v
	}
	if totalValue <= 0 {
		return "Lines carry no value — cannot allocate by value basis.", nil
	}

	grand := 0.0
	for _, c := range charges {
		grand += asFloat(c["amount"])
	}
	// Per-line totals across all charges.
	perLine := make([]float64, len(allocs))
	for _, c := range charges {
		amt := asFloat(c["amount"])
		if amt == 0 {
			continue
		}
		for i, a := range allocs {
			perLine[i] += amt * a.value / totalValue
		}
	}

	// Largest-remainder rounding so shares sum exactly to the charge total.
	rounded := make([]float64, len(perLine))
	var assigned float64
	for i, v := range perLine {
		rounded[i] = round2(v)
		assigned += rounded[i]
	}
	diff := round2(grand - assigned)
	if diff != 0 && len(perLine) > 0 {
		// give the remainder to the largest line
		maxI := 0
		for i := range perLine {
			if perLine[i] > perLine[maxI] {
				maxI = i
			}
		}
		rounded[maxI] = round2(rounded[maxI] + diff)
	}

	order := make([]int, len(allocs))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(a, b int) bool { return perLine[order[a]] > perLine[order[b]] })

	out := fmt.Sprintf("Landed cost allocation — %s (%s, %.2f total charges):\n", orm.AsString(ship["name"]), lineModel, grand)
	for _, i := range order {
		out += fmt.Sprintf("  %-30s %12.2f  (%.1f%% of value)\n", allocs[i].label, rounded[i], allocs[i].value/totalValue*100)
	}
	return out, nil
}

func loadShipment(ctx context.Context, id int) map[string]interface{} {
	rows, err := orm.SearchPage(ctx, shipmentModel, [][]interface{}{{"id", "=", id}}, 1, 0, "id asc")
	if err != nil || len(rows) == 0 {
		return nil
	}
	return rows[0]
}

func asFloat(v interface{}) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(v)), 64)
	return f
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

func intID(v interface{}) int {
	n, _ := orm.CoerceInt64(v)
	return int(n)
}
