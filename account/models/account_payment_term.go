package models

import (
	"nexa/core/sdk"
)

type AccountPaymentTerm struct {
	sdk.Model `nexa:"model=account.payment.term"`

	Name   sdk.String  `nexa:"required,string=Payment Terms"`
	Note   sdk.Text    `nexa:"string=Description"`
	Days    sdk.Integer                    `nexa:"string=Due Days,default=0"`
	LineIDs sdk.One2Many[AccountPaymentTermLine] `nexa:"string=Terms"`
	Active  sdk.Boolean                    `nexa:"string=Active,default=true"`
}
