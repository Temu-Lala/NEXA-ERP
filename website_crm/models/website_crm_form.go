package models

import (
	"nexa/core/sdk"
)

type WebsiteCrmForm struct {
	sdk.Model `nexa:"model=website.crm.form"`

	Name   sdk.String        `nexa:"required,string=Form Name"`
	TeamID sdk.Many2One[any] `nexa:"comodel=crm.team,string=Sales Team"`
	Active sdk.Boolean       `nexa:"string=Active,default=true"`
}
