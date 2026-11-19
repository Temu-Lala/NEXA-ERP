package models

import (
	"nexa/core/sdk"
)

type AccountPaymentTerm struct {
	sdk.Model `nexa:"model=account.payment.term"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	Name    sdk.String                           `nexa:"required,string=Payment Terms"`
	Note    sdk.Text                             `nexa:"string=Description"`
	Days    sdk.Integer                          `nexa:"string=Due Days,default=0"`
	LineIDs sdk.One2Many[AccountPaymentTermLine] `nexa:"string=Terms"`
	Active  sdk.Boolean                          `nexa:"string=Active,default=true"`
}
