package models

import (
	"nexa/core/sdk"
)

// ProductBrand is a manufacturer/brand registry for classification and filtering.
type ProductBrand struct {
	sdk.Model `nexa:"model=product.brand"`

	Name   sdk.String  `nexa:"required,index,unique,string=Brand"`
	Logo   sdk.Text    `nexa:"string=Logo"`
	Active sdk.Boolean `nexa:"string=Active,default=true"`
}
