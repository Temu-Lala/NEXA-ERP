package models

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"nexa/core/orm"
)

// ITAM engine: license utilization, warranty-expiry flag, asset assignment.

const (
	assetModel = "itam.asset"
	licModel   = "itam.license"
)

func init() {
	orm.RegisterCompute(licModel, "utilization_pct", []string{"seats_total", "seats_used"}, computeLicUtil)
	orm.RegisterCompute(assetModel, "warranty_expiring", []string{"warranty_until"}, computeWarrantyExpiring)
	orm.RegisterObjectAction(assetModel, "action_asset_assign", assetAssign)
	orm.RegisterObjectAction(assetModel, "action_asset_retire", assetRetire)
}

func oid(v interface{}) int {
	if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
		n, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprint(arr[0])))
		return n
	}
	n, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprint(v)))
	return n
}

func loadRec(ctx context.Context, model string, id int) map[string]interface{} {
	rows, err := orm.SearchPage(ctx, model, [][]interface{}{{"id", "=", id}}, 1, 0, "id asc")
	if err != nil || len(rows) == 0 {
		return nil
	}
	return rows[0]
}

func computeLicUtil(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	total := float64(oid(rec["seats_total"]))
	used := float64(oid(rec["seats_used"]))
	if total <= 0 {
		return 0, nil
	}
	return used / total * 100, nil
}

// warranty_expiring = warranty ends within 60 days.
func computeWarrantyExpiring(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	w := strings.TrimSpace(fmt.Sprint(rec["warranty_until"]))
	if len(w) < 10 {
		return false, nil
	}
	d, err := time.Parse("2006-01-02", w[:10])
	if err != nil {
		return false, nil
	}
	return d.Before(time.Now().AddDate(0, 0, 60)) && d.After(time.Now()), nil
}

// assetAssign: in_stock → assigned, sets assignee.
func assetAssign(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	a := loadRec(bypass, assetModel, id)
	if a == nil {
		return "", fmt.Errorf("asset not found")
	}
	st := orm.AsString(a["state"])
	if st != "in_stock" && st != "in_repair" {
		return "", fmt.Errorf("cannot assign from state %s", st)
	}
	uid := oid(args["user_id"])
	upd := map[string]interface{}{"state": "assigned"}
	if uid > 0 {
		upd["assigned_to_id"] = uid
	}
	_ = orm.UpdateRecordByID(bypass, assetModel, id, upd)
	return "Asset assigned", nil
}

// assetRetire: any state → retired (or disposed with disposal_date).
func assetRetire(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	a := loadRec(bypass, assetModel, id)
	if a == nil {
		return "", fmt.Errorf("asset not found")
	}
	state := "retired"
	if strings.EqualFold(strings.TrimSpace(args["dispose"]), "true") || strings.EqualFold(strings.TrimSpace(args["dispose"]), "1") {
		state = "disposed"
	}
	upd := map[string]interface{}{"state": state}
	if state == "disposed" {
		upd["disposal_date"] = time.Now().UTC().Format("2006-01-02")
	}
	_ = orm.UpdateRecordByID(bypass, assetModel, id, upd)
	return "Asset " + state, nil
}
