package models

import (
	"nexa/core/sdk"
)

type AccountLockDate struct {
	sdk.Model `nexa:"model=account.lock.date"`

	Name          sdk.String                   `nexa:"required,string=Name"`
	JournalID     sdk.Many2One[AccountJournal] `nexa:"string=Journal"`
	HardLockDate  sdk.Date                     `nexa:"string=Hard Lock Date"`
	SoftLockDate  sdk.Date                     `nexa:"string=Soft Lock Date"`
}
