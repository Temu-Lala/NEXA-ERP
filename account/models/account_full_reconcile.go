package models

import (
	"nexa/core/sdk"
)

type AccountFullReconcile struct {
	sdk.Model `nexa:"model=account.full.reconcile"`

	Name sdk.String `nexa:"required,string=Number"`
}
