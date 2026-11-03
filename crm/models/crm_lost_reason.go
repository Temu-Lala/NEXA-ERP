package models

import (
	"nexa/core/sdk"
)

type CrmLostReason struct {
	sdk.Model `nexa:"model=crm.lost.reason"`

	Name   sdk.String  `nexa:"required,string=Description"`
	Active sdk.Boolean `nexa:"string=Active,default=true"`
}
