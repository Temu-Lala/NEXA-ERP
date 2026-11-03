package models

import (
	"nexa/core/sdk"
)

type AccountPaymentTermLine struct {
	sdk.Model `nexa:"model=account.payment.term.line"`

	PaymentTermID sdk.Many2One[AccountPaymentTerm]   `nexa:"required,index,string=Payment Terms"`
	Value         sdk.Selection[PaymentTermValue]    `nexa:"string=Value Type,default=percent"`
	ValueAmount   sdk.Numeric                        `nexa:"string=Value,precision=18,scale=2,default=100"`
	Days          sdk.Integer                        `nexa:"string=Days,default=0"`
	DelayType     sdk.Selection[PaymentTermDelay]    `nexa:"string=Delay Type,default=days_after"`
	Sequence      sdk.Integer                        `nexa:"string=Sequence,default=10"`
}
