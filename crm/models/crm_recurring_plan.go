package models

import (
	"nexa/core/sdk"
)

type CrmRecurringPlan struct {
	sdk.Model `nexa:"model=crm.recurring.plan"`

	Name           sdk.String  `nexa:"required,string=Plan Name"`
	NumberOfMonths sdk.Integer `nexa:"required,string=# Months"`
	Active         sdk.Boolean `nexa:"string=Active,default=true"`
	Sequence       sdk.Integer `nexa:"string=Sequence,default=10"`
}
