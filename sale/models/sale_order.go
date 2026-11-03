package models

import (
	"nexa/core/sdk"
)

type SaleOrder struct {
	sdk.Model `nexa:"model=sale.order"`

	Name          sdk.String                    `nexa:"required,string=Order Reference"`
	PartnerID     sdk.Many2One[CorePartner]     `nexa:"required,string=Customer"`
	UserID        sdk.Many2One[CoreUser]        `nexa:"string=Salesperson"`
	OpportunityID sdk.Many2One[CrmLead]         `nexa:"string=Opportunity"`
	DateOrder     sdk.DateTime                  `nexa:"string=Order Date"`
	State         sdk.Selection[SaleOrderState] `nexa:"string=Status,default=draft"`
	InvoiceStatus sdk.Selection[InvoiceStatus]  `nexa:"string=Invoice Status,default=no"`
	AmountUntaxed sdk.Numeric                   `nexa:"string=Untaxed Amount,precision=18,scale=2,default=0"`
	AmountTotal   sdk.Numeric                   `nexa:"string=Total,precision=18,scale=2,default=0"`
	Note          sdk.Text                      `nexa:"string=Terms and Conditions"`
	OrderLine     sdk.One2Many[SaleOrderLine]   `nexa:"string=Order Lines"`
	CompanyID     sdk.Many2One[CoreCompany]     `nexa:"string=Company"`
}
