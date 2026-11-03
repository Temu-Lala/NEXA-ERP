package models

import (
	"nexa/core/sdk"
)

type CrmTag struct {
	sdk.Model `nexa:"model=crm.tag"`

	Name   sdk.String  `nexa:"required,string=Tag Name"`
	Color  sdk.Integer `nexa:"string=Color,default=0"`
	Active sdk.Boolean `nexa:"string=Active,default=true"`
}
