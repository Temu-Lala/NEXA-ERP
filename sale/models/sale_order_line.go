package models

import (
	"nexa/core/sdk"
)

type SaleOrderLine struct {
	sdk.Model `nexa:"model=sale.order.line"`

	OrderID       sdk.Many2One[SaleOrder]      `nexa:"required,index,string=Order"`
	ProductID     sdk.Many2One[ProductProduct] `nexa:"string=Product"`
	Name          sdk.String                   `nexa:"required,string=Description"`
	ProductUomQty sdk.Float64                  `nexa:"string=Quantity,default=1"`
	PriceUnit     sdk.Numeric                  `nexa:"string=Unit Price,default=0"`
	PriceSubtotal sdk.Numeric                  `nexa:"string=Subtotal,default=0,readonly"`
	Sequence      sdk.Integer                  `nexa:"string=Sequence,default=10"`
}
