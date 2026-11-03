package models

import (
	"nexa/core/sdk"
	productmodels "nexa_addons/product/models"
)

// Cross-module reference aliases.
type (
	ProductProduct  = productmodels.ProductProduct
	ProductCategory = productmodels.ProductCategory
)

// ComputeMethod is how a price rule derives its amount.
type ComputeMethod string

const (
	ComputeFixed      ComputeMethod = "fixed"
	ComputePercentage ComputeMethod = "percentage"
)

// ProductPricelist is a named, effective-dated set of pricing rules.
type ProductPricelist struct {
	sdk.Model `nexa:"model=product.pricelist"`

	Name       sdk.String                         `nexa:"required,index,string=Price List"`
	CurrencyId sdk.String                         `nexa:"string=Currency"`
	Items      sdk.One2Many[ProductPricelistItem] `nexa:"string=Rules"`
	Active     sdk.Boolean                        `nexa:"string=Active,default=true"`
	Note       sdk.Text                           `nexa:"string=Note"`
}

// ProductPricelistItem is a single pricing rule within a price list.
type ProductPricelistItem struct {
	sdk.Model `nexa:"model=product.pricelist.item"`

	PricelistID sdk.Many2One[ProductPricelist] `nexa:"required,index,string=Price List"`
	ProductID   sdk.Many2One[ProductProduct]   `nexa:"string=Product"`
	CategID     sdk.Many2One[ProductCategory]  `nexa:"string=Category"`
	ComputedBy  sdk.Selection[ComputeMethod]   `nexa:"string=Compute Method,default=fixed"`
	FixedPrice  sdk.Numeric                    `nexa:"string=Fixed Price,precision=18,scale=2,default=0"`
	Percent     sdk.Float                      `nexa:"string=Discount %,default=0"`
	MinQty      sdk.Float                      `nexa:"string=Min Quantity,default=0"`
	DateStart   sdk.Date                       `nexa:"string=Start Date"`
	DateEnd     sdk.Date                       `nexa:"string=End Date"`
	Sequence    sdk.Integer                    `nexa:"string=Priority,default=10"`
	Active      sdk.Boolean                    `nexa:"string=Active,default=true"`
}

func init() {
	sdk.MustRegister("product_pricing",
		&ProductPricelist{},
		&ProductPricelistItem{},
	)
}
