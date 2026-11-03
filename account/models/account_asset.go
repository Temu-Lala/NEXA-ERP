package models

import (
	"nexa/core/sdk"
)

type AccountAsset struct {
	sdk.Model `nexa:"model=account.asset"`

	Name            sdk.String                   `nexa:"required,string=Name"`
	OriginalValue   sdk.Numeric                  `nexa:"string=Original Value,precision=18,scale=2,default=0"`
	SalvageValue    sdk.Numeric                  `nexa:"string=Salvage Value,precision=18,scale=2,default=0"`
	BookValue       sdk.Numeric                  `nexa:"string=Book Value,precision=18,scale=2,default=0"`
	Months          sdk.Integer                  `nexa:"string=Useful Life (Months),default=36"`
	Method          sdk.String                   `nexa:"string=Method,default=straight_line"`
	AcquisitionDate sdk.Date                     `nexa:"string=Acquisition Date"`
	AccountID       sdk.Many2One[AccountAccount] `nexa:"string=Asset Account"`
	ExpenseAccountID sdk.Many2One[AccountAccount] `nexa:"string=Depreciation Expense"`
	State           sdk.String                   `nexa:"string=Status,default=draft"`
}
