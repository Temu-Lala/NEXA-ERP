package models

import (
	"nexa/core/sdk"
)

type AccountFullReconcile struct {
	sdk.Model `nexa:"model=account.full.reconcile"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	Name sdk.String `nexa:"required,string=Number"`
}
