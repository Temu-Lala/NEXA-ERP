package models

import (
	"nexa/core/sdk"
)

type PurchaseOrderLine struct {
	sdk.Model `nexa:"model=purchase.order.line"`

	OrderID       sdk.Many2One[PurchaseOrder]  `nexa:"required,index,string=Order"`
	ProductID     sdk.Many2One[ProductProduct] `nexa:"string=Product"`
	Name          sdk.String                   `nexa:"required,string=Description"`
	ProductQty    sdk.Float                    `nexa:"string=Quantity,default=1"`
	QtyReceived   sdk.Numeric                  `nexa:"string=Received Quantity,precision=18,scale=3,default=0"`
	PriceUnit     sdk.Numeric                  `nexa:"string=Unit Price,precision=18,scale=2,default=0"`
	PriceSubtotal sdk.Numeric                  `nexa:"string=Subtotal,precision=18,scale=2,default=0"`
	Sequence      sdk.Integer                  `nexa:"string=Sequence,default=10"`
}
