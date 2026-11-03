package models

import (
	"nexa/core/sdk"
)

type IapAccount struct {
	sdk.Model `nexa:"model=iap.account"`

	Name         sdk.String  `nexa:"required,string=Account Name"`
	ServiceName  sdk.String  `nexa:"required,string=Service"`
	AccountToken sdk.String  `nexa:"string=Account Token"`
	Balance      sdk.Numeric `nexa:"string=Balance,precision=18,scale=2,default=0"`
}
