package models

import (
	"nexa/core/sdk"
)

type AccountPartialReconcile struct {
	sdk.Model `nexa:"model=account.partial.reconcile"`

	DebitMoveID     sdk.Many2One[AccountMoveLine]     `nexa:"required,index,string=Debit Move Line"`
	CreditMoveID    sdk.Many2One[AccountMoveLine]     `nexa:"required,index,string=Credit Move Line"`
	Amount          sdk.Numeric                       `nexa:"string=Amount,precision=18,scale=2,default=0"`
	FullReconcileID sdk.Many2One[AccountFullReconcile] `nexa:"string=Full Reconcile"`
}
