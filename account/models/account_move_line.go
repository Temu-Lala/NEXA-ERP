package models

import (
	"nexa/core/sdk"
)

type AccountMoveLine struct {
	sdk.Model `nexa:"model=account.move.line"`

	MoveID         sdk.Many2One[AccountMove]    `nexa:"required,index,string=Journal Entry"`
	AccountID      sdk.Many2One[AccountAccount] `nexa:"string=Account"`
	Name           sdk.String                   `nexa:"string=Label"`
	PartnerID      sdk.Many2One[CorePartner]    `nexa:"string=Partner"`
	AnalyticAccountID sdk.Many2One[AccountAnalyticAccount] `nexa:"string=Analytic Account"`
	ProductID      sdk.Many2One[sdk.Any]        `nexa:"string=Product,comodel=product.product"`
	TaxID          sdk.Many2One[AccountTax]     `nexa:"string=Tax"`
	TaxOriginID    sdk.Many2One[AccountTax]     `nexa:"string=Originator Tax"`
	Quantity       sdk.Numeric                  `nexa:"string=Quantity,precision=18,scale=3,default=1"`
	PriceUnit      sdk.Numeric                  `nexa:"string=Unit Price,precision=18,scale=2,default=0"`
	PriceSubtotal  sdk.Numeric                  `nexa:"string=Subtotal,precision=18,scale=2,default=0,readonly"`
	DisplayType    sdk.Selection[DisplayType]   `nexa:"string=Display Type,default=product"`
	Debit          sdk.Numeric                  `nexa:"string=Debit,precision=18,scale=2,default=0"`
	Credit         sdk.Numeric                  `nexa:"string=Credit,precision=18,scale=2,default=0"`
	Balance        sdk.Numeric                  `nexa:"string=Balance,precision=18,scale=2,default=0"`
	Reconciled     sdk.Boolean                  `nexa:"string=Reconciled,default=false"`
	AmountResidual sdk.Numeric                  `nexa:"string=Residual,precision=18,scale=2,default=0"`
}
