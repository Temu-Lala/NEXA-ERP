package models

import (
	"context"

	"nexa/core/orm"
)

// Maverick spend (spec §14.2): a purchase order is "maverick" when it was
// created without any sourcing provenance — no requisition, RFQ or contract
// reference — and is past draft. Derived on read, never stored.

func init() {
	orm.RegisterCompute("purchase.order", "is_maverick", []string{"state", "requisition_ref", "rfq_ref", "contract_ref"}, computePOIsMaverick)
}

func computePOIsMaverick(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	state := orm.AsString(rec["state"])
	if state == "draft" || state == "cancel" || state == "cancelled" {
		return false, nil
	}
	for _, f := range []string{"requisition_ref", "rfq_ref", "contract_ref"} {
		if v := rec[f]; v != nil {
			if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
				return false, nil
			}
			if n, ok := v.(int64); ok && n > 0 {
				return false, nil
			}
			if n, ok := v.(int); ok && n > 0 {
				return false, nil
			}
			if s := orm.AsString(v); s != "" && s != "0" && s != "false" {
				return false, nil
			}
		}
	}
	return true, nil
}
