package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	productmodels "nexa_addons/product/models"
)

// Cross-module reference aliases.
type (
	CorePartner    = basemodels.CorePartner
	ProductProduct = productmodels.ProductProduct
)

// ProductSupplierInfo stores a supplier's own code, price and terms for a product.
type ProductSupplierInfo struct {
	sdk.Model `nexa:"model=product.supplierinfo"`

	PartnerID   sdk.Many2One[CorePartner]    `nexa:"required,index,string=Supplier"`
	ProductID   sdk.Many2One[ProductProduct] `nexa:"required,index,string=Product"`
	ProductCode sdk.String                   `nexa:"string=Supplier Product Code"`
	ProductName sdk.String                   `nexa:"string=Supplier Product Name"`
	Price       sdk.Numeric                  `nexa:"string=Supplier Price,precision=18,scale=2,default=0"`
	CurrencyId  sdk.String                   `nexa:"string=Currency"`
	MinQty      sdk.Float                    `nexa:"string=Min Quantity,default=0"`
	Delay       sdk.Integer                  `nexa:"string=Lead Time (days),default=1"`
	DateStart   sdk.Date                     `nexa:"string=Valid From"`
	DateEnd     sdk.Date                     `nexa:"string=Valid Until"`
	Sequence    sdk.Integer                  `nexa:"string=Preferred Rank,default=10"`
	Active      sdk.Boolean                  `nexa:"string=Active,default=true"`
}

func init() {
	sdk.MustRegister("product_supplier",
		&ProductSupplierInfo{},
	)
}
