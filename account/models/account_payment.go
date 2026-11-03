package models

import (
	"nexa/core/sdk"
)

type AccountPayment struct {
	sdk.Model `nexa:"model=account.payment"`

	Name        sdk.String                      `nexa:"string=Number"`
	PaymentType sdk.Selection[PaymentType]      `nexa:"required,string=Payment Type,default=inbound"`
	PartnerType sdk.Selection[PartnerType]      `nexa:"string=Partner Type,default=customer"`
	PartnerID   sdk.Many2One[CorePartner]       `nexa:"string=Partner"`
	Amount      sdk.Numeric                     `nexa:"required,string=Amount,precision=18,scale=2,default=0"`
	Date        sdk.Date                        `nexa:"string=Date"`
	JournalID   sdk.Many2One[AccountJournal]    `nexa:"string=Journal"`
	Memo        sdk.String                      `nexa:"string=Memo"`
	State       sdk.Selection[PaymentRecordState] `nexa:"string=Status,default=draft"`
	MoveID      sdk.Many2One[AccountMove]       `nexa:"string=Journal Entry"`
	InvoiceID   sdk.Many2One[AccountMove]       `nexa:"string=Invoice / Bill"`
}
