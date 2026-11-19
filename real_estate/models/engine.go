package models

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"nexa/core/orm"
)

// Real estate engine: lease activation occupies the unit; rent payments mark
// paid with date; overdue detection is a computed flag on payments.

const (
	leaseModel   = "realestate.lease"
	unitModel    = "realestate.unit"
	paymentModel = "realestate.rent.payment"
)

func init() {
	orm.RegisterObjectAction(leaseModel, "action_lease_activate", leaseActivate)
	orm.RegisterObjectAction(leaseModel, "action_lease_end", leaseEnd)
	orm.RegisterObjectAction(paymentModel, "action_rent_pay", rentPay)
	orm.RegisterCompute(paymentModel, "is_overdue", []string{"state", "due_date"}, computeRentOverdue)
}

func fnum(v interface{}) float64 {
	n, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(v)), 64)
	return n
}

func oid(v interface{}) int {
	if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
		n, _ := strconv.Atoi(fmt.Sprint(arr[0]))
		return n
	}
	n, _ := strconv.Atoi(fmt.Sprint(v))
	return n
}

func loadRec(ctx context.Context, model string, id int) map[string]interface{} {
	rows, err := orm.SearchPage(ctx, model, [][]interface{}{{"id", "=", id}}, 1, 0, "id asc")
	if err != nil || len(rows) == 0 {
		return nil
	}
	return rows[0]
}

// leaseActivate: draft → active, unit → occupied.
func leaseActivate(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	l := loadRec(bypass, leaseModel, id)
	if l == nil {
		return "", fmt.Errorf("lease not found")
	}
	if st := orm.AsString(l["state"]); st != "draft" {
		return "", fmt.Errorf("only draft leases can be activated (state=%s)", st)
	}
	unitID := oid(l["unit_id"])
	// refuse if unit already occupied by another active lease
	others, _ := orm.SearchPage(bypass, leaseModel,
		[][]interface{}{{"unit_id", "=", unitID}, {"state", "=", "active"}, {"id", "!=", id}}, 10, 0, "id asc")
	if len(others) > 0 {
		return "", fmt.Errorf("unit already has an active lease")
	}
	_ = orm.UpdateRecordByID(bypass, leaseModel, id, map[string]interface{}{"state": "active"})
	_ = orm.UpdateRecordByID(bypass, unitModel, unitID, map[string]interface{}{"state": "occupied"})
	return "Lease activated — unit occupied", nil
}

// leaseEnd: active → terminated/expired, unit → vacant.
func leaseEnd(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	l := loadRec(bypass, leaseModel, id)
	if l == nil {
		return "", fmt.Errorf("lease not found")
	}
	if st := orm.AsString(l["state"]); st != "active" {
		return "", fmt.Errorf("only active leases can be ended (state=%s)", st)
	}
	end := strings.ToLower(strings.TrimSpace(args["end_state"]))
	if end != "expired" && end != "terminated" {
		end = "terminated"
	}
	_ = orm.UpdateRecordByID(bypass, leaseModel, id, map[string]interface{}{"state": end})
	_ = orm.UpdateRecordByID(bypass, unitModel, oid(l["unit_id"]), map[string]interface{}{"state": "vacant"})
	return "Lease " + end + " — unit vacant", nil
}

// rentPay marks a pending/partial/overdue payment as paid.
func rentPay(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	p := loadRec(bypass, paymentModel, id)
	if p == nil {
		return "", fmt.Errorf("payment not found")
	}
	if st := orm.AsString(p["state"]); st == "paid" || st == "waived" {
		return "", fmt.Errorf("payment already %s", st)
	}
	_ = orm.UpdateRecordByID(bypass, paymentModel, id, map[string]interface{}{
		"state":     "paid",
		"paid_date": time.Now().UTC().Format("2006-01-02"),
		"reference": args["reference"],
	})
	return "Rent payment recorded", nil
}

// computeRentOverdue: pending/partial payment past due_date → overdue flag.
func computeRentOverdue(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	st := orm.AsString(rec["state"])
	if st == "paid" || st == "waived" {
		return false, nil
	}
	due := strings.TrimSpace(fmt.Sprint(rec["due_date"]))
	if len(due) < 10 {
		return false, nil
	}
	d, err := time.Parse("2006-01-02", due[:10])
	if err != nil {
		return false, nil
	}
	return d.Before(time.Now()), nil
}
