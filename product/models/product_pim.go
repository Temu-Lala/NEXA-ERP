package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// Product PIM layer (spec): attributes, variants, barcodes, suppliers,
// price history — the catalog depth behind product.product.

type CoreUser = basemodels.CoreUser

// ProductAttribute is a reusable typed attribute definition (Color, Size…).
type ProductAttribute struct {
	sdk.Model `nexa:"model=product.attribute"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	Name          sdk.String              `nexa:"required,string=Attribute"`
	Code          sdk.String              `nexa:"string=Code"`
	Kind          sdk.Selection[AttrKind] `nexa:"string=Type,default=selection,selection=text:Text,number:Number,boolean:Boolean,date:Date,selection:Selection"`
	Values        sdk.Text                `nexa:"string=Allowed Values (one per line)"`
	CreateVariant sdk.Boolean             `nexa:"string=Creates Variants,default=false"`
	Active        sdk.Boolean             `nexa:"string=Active,default=true"`
}

type AttrKind string

const (
	AttrText      AttrKind = "text"
	AttrNumber    AttrKind = "number"
	AttrBoolean   AttrKind = "boolean"
	AttrDate      AttrKind = "date"
	AttrSelection AttrKind = "selection"
)

// ProductVariant is a sellable version of a product (size/color combo).
type ProductVariant struct {
	sdk.Model `nexa:"model=product.variant"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	ProductID     sdk.Many2One[ProductProduct] `nexa:"required,index,ondelete=cascade,string=Product"`
	Name          sdk.String                   `nexa:"required,string=Variant"`
	Sku           sdk.String                   `nexa:"string=SKU"`
	Barcode       sdk.String                   `nexa:"index,string=Barcode"`
	ListPrice     sdk.Numeric                  `nexa:"string=Price,precision=18,scale=2,default=0"`
	StandardPrice sdk.Numeric                  `nexa:"string=Cost,precision=18,scale=2,default=0"`
	Attributes    sdk.Text                     `nexa:"string=Attributes (JSON)"`
	Active        sdk.Boolean                  `nexa:"string=Active,default=true"`
}

// ProductBarcode is an alternate identifier for a product or variant.
type ProductBarcode struct {
	sdk.Model `nexa:"model=product.barcode"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	ProductID sdk.Many2One[ProductProduct] `nexa:"index,string=Product"`
	VariantID sdk.Many2One[ProductVariant] `nexa:"index,string=Variant"`
	Code      sdk.String                   `nexa:"required,index,string=Barcode"`
	Symbology sdk.String                   `nexa:"string=Barcode Type"`
	Source    sdk.String                   `nexa:"string=Source"`
	IsPrimary sdk.Boolean                  `nexa:"string=Primary,default=false"`
	Active    sdk.Boolean                  `nexa:"string=Active,default=true"`
}

// ProductSupplier links a product to a vendor with purchase terms.
type ProductSupplier struct {
	sdk.Model `nexa:"model=product.supplier"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	ProductID    sdk.Many2One[ProductProduct] `nexa:"required,index,ondelete=cascade,string=Product"`
	PartnerID    sdk.Many2One[sdk.Any]        `nexa:"required,index,string=Supplier,comodel=core.partner"`
	SupplierSku  sdk.String                   `nexa:"string=Supplier SKU"`
	SupplierCode sdk.String                   `nexa:"string=Supplier Part No"`
	Price        sdk.Numeric                  `nexa:"string=Purchase Price,precision=18,scale=2,default=0"`
	MinQty       sdk.Numeric                  `nexa:"string=Min Order Qty,precision=18,scale=3,default=1"`
	LeadDays     sdk.Integer                  `nexa:"string=Lead Time (days),default=0"`
	Priority     sdk.Integer                  `nexa:"string=Priority,default=10"`
	Preferred    sdk.Boolean                  `nexa:"string=Preferred,default=false"`
	Active       sdk.Boolean                  `nexa:"string=Active,default=true"`
}

// ProductPriceHistory is an immutable price-change audit record.
type ProductPriceHistory struct {
	sdk.Model `nexa:"model=product.price.history"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	ProductID  sdk.Many2One[ProductProduct] `nexa:"required,index,string=Product"`
	VariantID  sdk.Many2One[ProductVariant] `nexa:"index,string=Variant"`
	Field      sdk.String                   `nexa:"string=Field"`
	OldValue   sdk.Numeric                  `nexa:"string=Old,precision=18,scale=2,default=0"`
	NewValue   sdk.Numeric                  `nexa:"string=New,precision=18,scale=2,default=0"`
	CurrencyID sdk.Many2One[sdk.Any]        `nexa:"string=Currency,comodel=currency.currency"`
	Reason     sdk.String                   `nexa:"string=Reason"`
	UserID     sdk.Many2One[CoreUser]       `nexa:"string=Changed By"`
	At         sdk.DateTime                 `nexa:"string=At"`
}

// ProductBundle is a fixed kit sold as one SKU; components deduct on sale.
type ProductBundle struct {
	sdk.Model `nexa:"model=product.bundle"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	Name      sdk.String                      `nexa:"required,string=Bundle"`
	ProductID sdk.Many2One[ProductProduct]    `nexa:"required,index,string=Sellable Product"`
	Active    sdk.Boolean                     `nexa:"string=Active,default=true"`
	Lines     sdk.One2Many[ProductBundleLine] `nexa:"string=Components"`
}

// ProductBundleLine is one component inside a bundle.
type ProductBundleLine struct {
	sdk.Model `nexa:"model=product.bundle.line"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	BundleID  sdk.Many2One[ProductBundle]  `nexa:"required,index,ondelete=cascade,string=Bundle"`
	ProductID sdk.Many2One[ProductProduct] `nexa:"required,index,string=Component"`
	Qty       sdk.Numeric                  `nexa:"string=Qty,precision=18,scale=3,default=1"`
}
