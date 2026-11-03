package models

import (
	"nexa/core/sdk"
	productmodels "nexa_addons/product/models"
)

// ProductProduct -> product.product
type ProductProduct = productmodels.ProductProduct

// ProductBundle is a kit or bundle that groups component products into one offer.
type ProductBundle struct {
	sdk.Model `nexa:"model=product.bundle"`

	Name      sdk.String                      `nexa:"required,index,string=Bundle Name"`
	ProductID sdk.Many2One[ProductProduct]    `nexa:"string=Bundle Product"`
	Lines     sdk.One2Many[ProductBundleLine] `nexa:"string=Components"`
	Active    sdk.Boolean                     `nexa:"string=Active,default=true"`
	Note      sdk.Text                        `nexa:"string=Note"`
}

// ProductBundleLine is a component product and quantity within a bundle.
type ProductBundleLine struct {
	sdk.Model `nexa:"model=product.bundle.line"`

	BundleID  sdk.Many2One[ProductBundle]  `nexa:"required,index,string=Bundle"`
	ProductID sdk.Many2One[ProductProduct] `nexa:"required,string=Component"`
	Qty       sdk.Float                    `nexa:"string=Quantity,default=1"`
	Sequence  sdk.Integer                  `nexa:"string=Sequence,default=10"`
}

func init() {
	sdk.MustRegister("product_bundle",
		&ProductBundle{},
		&ProductBundleLine{},
	)
}
