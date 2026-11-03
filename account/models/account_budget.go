package models

import (
	"nexa/core/sdk"
)

type AccountBudget struct {
	sdk.Model `nexa:"model=account.budget"`

	Name        sdk.String                   `nexa:"required,string=Name"`
	DateFrom    sdk.Date                     `nexa:"string=Start Date"`
	DateTo      sdk.Date                     `nexa:"string=End Date"`
	CompanyID   sdk.Many2One[CoreCompany]    `nexa:"string=Company"`
	AccountID   sdk.Many2One[AccountAccount] `nexa:"string=Account"`
	Amount      sdk.Numeric                  `nexa:"string=Budget Amount,precision=18,scale=2,default=0"`
	ActualAmount sdk.Numeric                 `nexa:"string=Actual,precision=18,scale=2,default=0"`
	Variance    sdk.Numeric                  `nexa:"string=Variance,precision=18,scale=2,default=0"`
	State       sdk.String                   `nexa:"string=Status,default=draft"`
}
