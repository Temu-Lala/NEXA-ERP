package models

import (
	"nexa/core/sdk"
)

type AccountTaxReturn struct {
	sdk.Model `nexa:"model=account.tax.return"`

	Name         sdk.String  `nexa:"required,string=Name"`
	PeriodStart  sdk.Date    `nexa:"string=Period Start"`
	PeriodEnd    sdk.Date    `nexa:"string=Period End"`
	State        sdk.String  `nexa:"string=Status,default=draft"`
	Amount       sdk.Numeric `nexa:"string=Amount,precision=18,scale=2,default=0"`
}
