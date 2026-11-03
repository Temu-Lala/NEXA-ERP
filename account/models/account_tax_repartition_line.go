package models

import (
	"nexa/core/sdk"
)

type AccountTaxRepartitionLine struct {
	sdk.Model `nexa:"model=account.tax.repartition.line"`

	TaxID           sdk.Many2One[AccountTax]        `nexa:"required,index,string=Tax"`
	RepartitionType sdk.Selection[RepartitionType] `nexa:"string=Type,default=tax"`
	FactorPercent   sdk.Numeric                     `nexa:"string=Factor (%),precision=18,scale=2,default=100"`
	AccountID       sdk.Many2One[AccountAccount]    `nexa:"string=Account"`
}
