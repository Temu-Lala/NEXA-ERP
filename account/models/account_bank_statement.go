package models

import (
	"nexa/core/sdk"
)

type AccountBankStatement struct {
	sdk.Model `nexa:"model=account.bank.statement"`

	Name       sdk.String                         `nexa:"required,string=Reference"`
	JournalID  sdk.Many2One[AccountJournal]       `nexa:"required,string=Journal"`
	Date       sdk.Date                           `nexa:"string=Date"`
	BalanceEnd sdk.Numeric                        `nexa:"string=Ending Balance,precision=18,scale=2,default=0"`
	State      sdk.Selection[BankStatementState]  `nexa:"string=Status,default=open"`
	LineIDs    sdk.One2Many[AccountBankStatementLine] `nexa:"string=Statement Lines"`
}

type AccountBankStatementLine struct {
	sdk.Model `nexa:"model=account.bank.statement.line"`

	StatementID  sdk.Many2One[AccountBankStatement] `nexa:"required,index,string=Statement"`
	Date         sdk.Date                           `nexa:"string=Date"`
	Name         sdk.String                         `nexa:"string=Label"`
	Amount       sdk.Numeric                        `nexa:"string=Amount,precision=18,scale=2,default=0"`
	PartnerID    sdk.Many2One[CorePartner]          `nexa:"string=Partner"`
	IsReconciled sdk.Boolean                        `nexa:"string=Reconciled,default=false"`
	MoveLineID   sdk.Many2One[AccountMoveLine]      `nexa:"string=Matched Move Line"`
}

type AccountReconcileModel struct {
	sdk.Model `nexa:"model=account.reconcile.model"`

	Name              sdk.String                 `nexa:"required,string=Name"`
	MatchLabel        sdk.String                 `nexa:"string=Label Contains"`
	MatchAmount       sdk.Numeric                `nexa:"string=Amount Tolerance,precision=18,scale=2,default=0.01"`
	WriteoffAccountID sdk.Many2One[AccountAccount] `nexa:"string=Write-off Account"`
	Active            sdk.Boolean                `nexa:"string=Active,default=true"`
}
