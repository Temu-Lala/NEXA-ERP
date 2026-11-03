package models

import (
	"nexa/core/sdk"
)

type AccountJournal struct {
	sdk.Model `nexa:"model=account.journal"`

	Name             sdk.String                   `nexa:"required,string=Journal Name"`
	Code             sdk.String                   `nexa:"required,unique,string=Short Code"`
	Type             sdk.Selection[JournalType]   `nexa:"string=Type,default=general"`
	DefaultAccountID sdk.Many2One[AccountAccount] `nexa:"string=Default Account"`
	Active           sdk.Boolean                  `nexa:"string=Active,default=true"`
}
