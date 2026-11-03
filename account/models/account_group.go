package models

import (
	"nexa/core/sdk"
)

type AccountGroup struct {
	sdk.Model `nexa:"model=account.group"`

	Name       sdk.String              `nexa:"required,string=Name"`
	CodePrefix sdk.String              `nexa:"string=Code Prefix"`
	ParentID   sdk.Many2One[AccountGroup] `nexa:"string=Parent Group"`
	Active     sdk.Boolean             `nexa:"string=Active,default=true"`
}
