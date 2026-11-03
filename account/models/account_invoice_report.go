package models

import (
	"nexa/core/sdk"
)

type AccountInvoiceReport struct {
	sdk.Model `nexa:"model=account.invoice.report"`

	MoveID         sdk.Many2One[AccountMove]   `nexa:"index,string=Invoice"`
	Name           sdk.String                  `nexa:"string=Number"`
	PartnerID      sdk.Many2One[CorePartner]   `nexa:"string=Partner"`
	MoveType       sdk.Selection[MoveType]     `nexa:"string=Type"`
	InvoiceDate    sdk.Date                    `nexa:"string=Invoice Date"`
	State          sdk.Selection[MoveState]    `nexa:"string=Status"`
	PaymentState   sdk.Selection[PaymentState] `nexa:"string=Payment"`
	AmountUntaxed  sdk.Numeric                 `nexa:"string=Untaxed,precision=18,scale=2,default=0"`
	AmountTax      sdk.Numeric                 `nexa:"string=Tax,precision=18,scale=2,default=0"`
	AmountTotal    sdk.Numeric                 `nexa:"string=Total,precision=18,scale=2,default=0"`
	AmountResidual sdk.Numeric                 `nexa:"string=Residual,precision=18,scale=2,default=0"`
}
