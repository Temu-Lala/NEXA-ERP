package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// ProductBrand is a manufacturer/brand registry for classification and filtering.
type CoreCompany = basemodels.CoreCompany

// ProductBrand model.
type ProductBrand struct {
	sdk.Model `nexa:"model=product.brand"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	Name   sdk.String  `nexa:"required,index,unique,string=Brand"`
	Logo   sdk.Text    `nexa:"string=Logo"`
	Active sdk.Boolean `nexa:"string=Active,default=true"`
}
