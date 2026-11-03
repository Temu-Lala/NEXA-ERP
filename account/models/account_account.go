package models

import (
	"nexa/core/sdk"
)

type AccountAccount struct {
	sdk.Model `nexa:"model=account.account"`

	Code        sdk.String                 `nexa:"required,unique,string=Code"`
	Name        sdk.String                 `nexa:"required,string=Account Name"`
	AccountType sdk.Selection[AccountType] `nexa:"required,string=Type"`
	Reconcile   sdk.Boolean                `nexa:"string=Allow Reconciliation,default=false"`
	Deprecated  sdk.Boolean                `nexa:"string=Deprecated,default=false"`
	GroupID     sdk.Many2One[AccountGroup] `nexa:"string=Group"`
	Active      sdk.Boolean                `nexa:"string=Active,default=true"`
}
