package models

import (
	"nexa/core/sdk"
)

type UtmSource struct {
	sdk.Model `nexa:"model=utm.source"`

	Name   sdk.String  `nexa:"required,string=Source"`
	Active sdk.Boolean `nexa:"string=Active,default=true"`
}
