package models

import (
	"nexa/core/sdk"
)

// ProductCategory model.
type ProductCategory struct {
	sdk.Model `nexa:"model=product.category"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	Name     sdk.String                    `nexa:"required,string=Category"`
	ParentID sdk.Many2One[ProductCategory] `nexa:"string=Parent"`
	Sequence sdk.Integer                   `nexa:"string=Sequence,default=10"`
	Active   sdk.Boolean                   `nexa:"string=Active,default=true"`
}
