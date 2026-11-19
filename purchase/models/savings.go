package models

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"nexa/core/orm"
	"nexa/core/sdk"
)

// Spend analytics: savings records. A savings record documents a verified
// price reduction — baseline vs achieved price — so realized savings are
// counted only when evidence exists (spec §14.3: projected savings never
// count as realized until verified).

type SavingsCategory string

const (
	SavNegotiated    SavingsCategory = "negotiated"
	SavVolume        SavingsCategory = "volume_discount"
	SavEarlyPay      SavingsCategory = "early_payment"
	SavFreight       SavingsCategory = "freight"
	SavConsolidation SavingsCategory = "consolidation"
	SavProcess       SavingsCategory = "process"
	SavAvoided       SavingsCategory = "avoided"
	SavSourcing      SavingsCategory = "competitive_sourcing"
)

// PurchaseSavingsRecord is one verified saving event.
type PurchaseSavingsRecord struct {
	sdk.Model `nexa:"model=purchase.savings.record"`
	CompanyID   sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	Name          sdk.String                     `nexa:"required,string=Reference"`
	Category      sdk.Selection[SavingsCategory] `nexa:"required,string=Category,selection=negotiated:Negotiated Price,volume_discount:Volume Discount,early_payment:Early-Payment Discount,freight:Freight Savings,consolidation:Supplier Consolidation,process:Process Cost,avoided:Avoided Cost,competitive_sourcing:Competitive Sourcing"`
	SupplierRef   sdk.Many2One[sdk.Any]          `nexa:"string=Supplier,comodel=purchase.supplier"`
	OrderRef      sdk.Many2One[sdk.Any]          `nexa:"string=Purchase Order,comodel=purchase.order"`
	ProductRef    sdk.Many2One[sdk.Any]          `nexa:"string=Product,comodel=product.product"`
	BaselinePrice sdk.Numeric                    `nexa:"string=Baseline Price,precision=18,scale=4,default=0"`
	AchievedPrice sdk.Numeric                    `nexa:"string=Achieved Price,precision=18,scale=4,default=0"`
	Quantity      sdk.Numeric                    `nexa:"string=Quantity,precision=18,scale=3,default=0"`
	Period        sdk.String                     `nexa:"string=Period,help='YYYY-MM'"`
	Realized      sdk.Boolean                    `nexa:"string=Realized,default=false"`
	Evidence      sdk.String                     `nexa:"string=Evidence Reference"`
	Note          sdk.Text                       `nexa:"string=Note"`
}

func init() {
	orm.RegisterCompute("purchase.savings.record", "realized_amount", []string{"baseline_price", "achieved_price", "quantity", "realized"}, computeSavingsRealized)
}

// computeSavingsRealized returns (baseline - achieved) * qty only when the
// saving is marked realized — projected savings stay at zero per spec.
func computeSavingsRealized(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	if rec["realized"] != true {
		return 0, nil
	}
	f := func(v interface{}) float64 {
		n, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(v)), 64)
		return n
	}
	diff := f(rec["baseline_price"]) - f(rec["achieved_price"])
	if diff < 0 {
		diff = 0
	}
	return diff * f(rec["quantity"]), nil
}
