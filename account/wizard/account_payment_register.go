package wizard

import (
	"nexa/core/sdk"
	"nexa_addons/account/models"
)

type AccountPaymentRegister struct {
	sdk.Model `nexa:"model=account.payment.register"`

	InvoiceID     sdk.Many2One[models.AccountMove]    `nexa:"required,string=Invoice"`
	PartnerID     sdk.Many2One[models.CorePartner]    `nexa:"string=Partner"`
	Amount        sdk.Numeric                         `nexa:"required,string=Amount,precision=18,scale=2,default=0"`
	JournalID     sdk.Many2One[models.AccountJournal] `nexa:"string=Payment Journal"`
	PaymentDate   sdk.Date                            `nexa:"string=Payment Date"`
	Communication sdk.String                          `nexa:"string=Memo"`
	PaymentType   sdk.Selection[models.PaymentType]   `nexa:"string=Payment Type,default=inbound"`
}
