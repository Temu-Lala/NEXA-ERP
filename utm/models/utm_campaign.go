package models

import (
	"nexa/core/sdk"
)

type UtmCampaign struct {
	sdk.Model `nexa:"model=utm.campaign"`

	Name   sdk.String  `nexa:"required,string=Campaign"`
	Active sdk.Boolean `nexa:"string=Active,default=true"`
}
