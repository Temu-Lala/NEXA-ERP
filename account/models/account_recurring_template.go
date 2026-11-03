package models

import (
	"nexa/core/sdk"
)

type AccountRecurringTemplate struct {
	sdk.Model `nexa:"model=account.recurring.template"`

	Name      sdk.String                   `nexa:"required,string=Name"`
	JournalID sdk.Many2One[AccountJournal] `nexa:"string=Journal"`
	AccountID sdk.Many2One[AccountAccount] `nexa:"string=Account"`
	PartnerID sdk.Many2One[CorePartner]    `nexa:"string=Partner"`
	MoveType  sdk.String                   `nexa:"string=Move Type,default=entry"`
	Amount    sdk.Numeric                  `nexa:"string=Amount,precision=18,scale=2,default=0"`
	Interval  sdk.String                   `nexa:"string=Interval,default=monthly"`
	NextDate  sdk.Date                     `nexa:"string=Next Date"`
	Active    sdk.Boolean                  `nexa:"string=Active,default=true"`
}
