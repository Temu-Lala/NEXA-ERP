package wizard

import (
	"nexa/core/sdk"
	"nexa_addons/account/models"
)

type AccountMoveReversal struct {
	sdk.Model `nexa:"model=account.move.reversal"`

	MoveID sdk.Many2One[models.AccountMove] `nexa:"required,string=Move"`
	Date   sdk.Date                         `nexa:"string=Reversal Date"`
	Reason sdk.String                       `nexa:"string=Reason"`
}
