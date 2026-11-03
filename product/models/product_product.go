package models

import (
	"nexa/core/sdk"
)

type ProductProduct struct {
	sdk.Model `nexa:"model=product.product"`

	Name                     sdk.String                       `nexa:"required,string=Product Name"`
	Image                    sdk.Text                         `nexa:"string=Image"`
	DefaultCode              sdk.String                       `nexa:"string=Internal Reference"`
	Barcode                  sdk.String                       `nexa:"index,string=Barcode"`
	Type                     sdk.Selection[ProductType]       `nexa:"string=Product Type,default=consu"`
	State                    sdk.Selection[ProductState]      `nexa:"string=Lifecycle,default=draft"`
	CategID                  sdk.Many2One[ProductCategory]    `nexa:"string=Category"`
	BrandID                  sdk.Many2One[ProductBrand]       `nexa:"string=Brand"`
	TagIDs                   sdk.Many2Many[ProductTag]        `nexa:"string=Tags,table=product_tag_rel,left=product_id,right=tag_id"`
	ListPrice                sdk.Numeric                      `nexa:"string=Sales Price,precision=18,scale=2,default=0"`
	StandardPrice            sdk.Numeric                      `nexa:"string=Cost,precision=18,scale=2,default=0"`
	CostMethod               sdk.Selection[ProductCostMethod] `nexa:"string=Cost Method,default=standard"`
	Tracking                 sdk.Selection[ProductTracking]   `nexa:"string=Tracking,default=none"`
	Description              sdk.Text                         `nexa:"string=Description"`
	DescriptionSale          sdk.Text                         `nexa:"string=Sales Description"`
	InternalNotes            sdk.Text                         `nexa:"string=Internal Notes"`
	SalesUomID               sdk.Many2One[sdk.Any]            `nexa:"comodel=uom.uom,string=Unit of Measure"`
	PurchaseUomID            sdk.Many2One[sdk.Any]            `nexa:"comodel=uom.uom,string=Purchase UoM"`
	Weight                   sdk.Numeric                      `nexa:"string=Weight,precision=18,scale=4,default=0"`
	Volume                   sdk.Numeric                      `nexa:"string=Volume,precision=18,scale=4,default=0"`
	WarrantyMonths           sdk.Integer                      `nexa:"string=Warranty (months),default=0"`
	ReorderMin               sdk.Float                        `nexa:"string=Min Quantity,default=0"`
	ReorderMax               sdk.Float                        `nexa:"string=Max Quantity,default=0"`
	PurchaseOk               sdk.Boolean                      `nexa:"string=Can be Purchased,default=true"`
	SaleOk                   sdk.Boolean                      `nexa:"string=Can be Sold,default=true"`
	PropertyAccountIncomeID  sdk.Many2One[sdk.Any]            `nexa:"comodel=account.account,string=Income Account"`
	PropertyAccountExpenseID sdk.Many2One[sdk.Any]            `nexa:"comodel=account.account,string=Expense Account"`
	Active                   sdk.Boolean                      `nexa:"string=Active,default=true"`
}
