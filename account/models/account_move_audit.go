package models

import (
	"nexa/core/sdk"
)

type AccountMoveAudit struct {
	sdk.Model `nexa:"model=account.move.audit"`

	MoveID   sdk.Many2One[AccountMove] `nexa:"required,string=Journal Entry"`
	UserID   sdk.Many2One[CoreUser]    `nexa:"string=User"`
	Action   sdk.String                `nexa:"required,string=Action"`
	Snapshot sdk.Text                  `nexa:"string=Snapshot"`
	Date     sdk.DateTime              `nexa:"string=Date"`
}
