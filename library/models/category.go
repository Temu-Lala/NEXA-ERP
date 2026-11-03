package models

import (
	"nexa/core/sdk"
)

type LibraryCategory struct {
	sdk.Model `nexa:"model=library.category"`

	Name        sdk.String  `nexa:"required,string=Name"`
	Code        sdk.String  `nexa:"string=Code"`
	Description sdk.Text    `nexa:"string=Description"`
	Active      sdk.Boolean `nexa:"string=Active,default=true"`
}
