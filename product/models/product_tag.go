package models

import (
	"nexa/core/sdk"
)

// ProductTag is a free classification tag for search and marketing.

// ProductTag model.
type ProductTag struct {
	sdk.Model `nexa:"model=product.tag"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	Name  sdk.String  `nexa:"required,index,string=Tag"`
	Color sdk.Integer `nexa:"string=Color,default=0"`
}
