package models

import (
	"nexa/core/sdk"
)

type AccountTax struct {
	sdk.Model `nexa:"model=account.tax"`

	Name       sdk.String                   `nexa:"required,string=Tax Name"`
	Amount     sdk.Numeric                  `nexa:"string=Amount (%),precision=18,scale=4,default=0"`
	TypeTaxUse sdk.Selection[TaxUse]        `nexa:"string=Tax Type,default=sale"`
	AccountID  sdk.Many2One[AccountAccount] `nexa:"string=Tax Account"`
	Active     sdk.Boolean                  `nexa:"string=Active,default=true"`
	RepartitionLineIDs sdk.One2Many[AccountTaxRepartitionLine] `nexa:"string=Repartition Lines"`
}
