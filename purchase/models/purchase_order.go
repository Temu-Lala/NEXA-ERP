package models

import (
	"nexa/core/sdk"
)

type PurchaseOrder struct {
	sdk.Model `nexa:"model=purchase.order"`

	Name          sdk.String                         `nexa:"required,string=Reference"`
	PartnerID     sdk.Many2One[CorePartner]          `nexa:"required,string=Vendor"`
	UserID        sdk.Many2One[CoreUser]             `nexa:"string=Buyer"`
	DateOrder     sdk.DateTime                       `nexa:"string=Order Deadline"`
	State         sdk.Selection[PurchaseOrderState]  `nexa:"string=Status,default=draft"`
	InvoiceStatus sdk.Selection[BillingStatus]       `nexa:"string=Billing Status,default=no"`
	AmountTotal   sdk.Numeric                        `nexa:"string=Total,precision=18,scale=2,default=0"`
	Notes         sdk.Text                           `nexa:"string=Terms and Conditions"`
	OrderLine     sdk.One2Many[PurchaseOrderLine]    `nexa:"string=Order Lines"`
	CompanyID     sdk.Many2One[CoreCompany]          `nexa:"string=Company"`
}
