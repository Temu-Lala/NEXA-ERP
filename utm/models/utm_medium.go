package models

import (
	"nexa/core/sdk"
)

type UtmMedium struct {
	sdk.Model `nexa:"model=utm.medium"`

	Name   sdk.String  `nexa:"required,string=Medium"`
	Active sdk.Boolean `nexa:"string=Active,default=true"`
}
