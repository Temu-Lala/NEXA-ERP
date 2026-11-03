package models

import (
	"nexa/core/sdk"
)

type AccountFiscalPosition struct {
	sdk.Model `nexa:"model=account.fiscal.position"`

	Name      sdk.String              `nexa:"required,string=Fiscal Position"`
	Active    sdk.Boolean             `nexa:"string=Active,default=true"`
	AutoApply sdk.Boolean             `nexa:"string=Detect Automatically"`
	CountryID sdk.Many2One[CoreCountry] `nexa:"string=Country"`
	Note      sdk.Text                `nexa:"string=Notes"`
	TaxLineIDs sdk.One2Many[AccountFiscalPositionTax] `nexa:"string=Tax Mapping"`
}

type AccountFiscalPositionTax struct {
	sdk.Model `nexa:"model=account.fiscal.position.tax"`

	PositionID sdk.Many2One[AccountFiscalPosition] `nexa:"required,index,string=Fiscal Position"`
	TaxSrcID   sdk.Many2One[AccountTax]            `nexa:"required,string=Tax on Product"`
	TaxDestID  sdk.Many2One[AccountTax]            `nexa:"string=Tax to Apply"`
}
