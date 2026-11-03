package models

import (
	"nexa/core/sdk"
)

type AccountMove struct {
	sdk.Model `nexa:"model=account.move"`

	Name            sdk.String                   `nexa:"required,string=Number"`
	MoveType        sdk.Selection[MoveType]      `nexa:"string=Type,default=entry"`
	PartnerID       sdk.Many2One[CorePartner]    `nexa:"string=Partner"`
	CurrencyID      sdk.Many2One[CoreCurrency]   `nexa:"string=Currency"`
	JournalID       sdk.Many2One[AccountJournal] `nexa:"string=Journal"`
	CompanyID       sdk.Many2One[CoreCompany]    `nexa:"string=Company"`
	Date            sdk.Date                     `nexa:"string=Accounting Date"`
	InvoiceDate     sdk.Date                     `nexa:"string=Invoice Date"`
	InvoiceDateDue  sdk.Date                     `nexa:"string=Due Date"`
	PaymentTermID   sdk.Many2One[AccountPaymentTerm] `nexa:"string=Payment Terms"`
	State           sdk.Selection[MoveState]     `nexa:"string=Status,default=draft"`
	InvoiceOrigin   sdk.String                   `nexa:"string=Source Document"`
	Ref             sdk.String                   `nexa:"string=Reference"`
	ReversedEntryID sdk.Many2One[AccountMove]    `nexa:"string=Reversal Of"`
	AmountUntaxed   sdk.Numeric                  `nexa:"string=Untaxed Amount,precision=18,scale=2,default=0"`
	AmountTax       sdk.Numeric                  `nexa:"string=Tax,precision=18,scale=2,default=0"`
	AmountTotal     sdk.Numeric                  `nexa:"string=Total,precision=18,scale=2,default=0"`
	AmountResidual  sdk.Numeric                  `nexa:"string=Amount Due,precision=18,scale=2,default=0"`
	PaymentState    sdk.Selection[PaymentState]  `nexa:"string=Payment Status,default=not_paid"`
	LineIDs         sdk.One2Many[AccountMoveLine] `nexa:"string=Journal Items"`
	Narration       sdk.Text                     `nexa:"string=Notes"`
}
