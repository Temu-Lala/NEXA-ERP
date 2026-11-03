package models

import (
	"nexa/core/sdk"
)

// ProductTag is a free classification tag for search and marketing.
type ProductTag struct {
	sdk.Model `nexa:"model=product.tag"`

	Name  sdk.String  `nexa:"required,index,string=Tag"`
	Color sdk.Integer `nexa:"string=Color,default=0"`
}
