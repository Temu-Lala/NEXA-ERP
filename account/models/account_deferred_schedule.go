package models

import (
	"nexa/core/sdk"
)

type AccountDeferredSchedule struct {
	sdk.Model `nexa:"model=account.deferred.schedule"`

	Name              sdk.String                   `nexa:"required,string=Name"`
	MoveID            sdk.Many2One[AccountMove]    `nexa:"string=Journal Entry"`
	AccountID         sdk.Many2One[AccountAccount] `nexa:"string=Recognition Account"`
	StartDate         sdk.Date                     `nexa:"string=Start Date"`
	EndDate           sdk.Date                     `nexa:"string=End Date"`
	Amount            sdk.Numeric                  `nexa:"string=Amount,precision=18,scale=2,default=0"`
	RecognizedAmount  sdk.Numeric                  `nexa:"string=Recognized,precision=18,scale=2,default=0"`
	State             sdk.String                   `nexa:"string=Status,default=draft"`
}
