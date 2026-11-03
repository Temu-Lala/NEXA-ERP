package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// Customer identity lives in the shared contacts master (core.partner). These
// models layer SALES-specific settings and team organization on top of it —
// they never duplicate the partner record itself.

type (
	CorePartner = basemodels.CorePartner
	CoreUser    = basemodels.CoreUser
	CoreCompany = basemodels.CoreCompany
)

type CustomerType string

const (
	CustTypeIndividual CustomerType = "individual"
	CustTypeBusiness   CustomerType = "business"
)

type PayerStatus string

const (
	PayerNormal  PayerStatus = "normal"
	PayerWatch   PayerStatus = "watch"
	PayerBlocked PayerStatus = "blocked"
)

// SaleCustomerProfile holds commercial sales settings for a partner: price
// list, terms, assigned rep and territory. Credit limits are stored as a
// policy reference only — Accounting owns the authoritative balance.
type SaleCustomerProfile struct {
	sdk.Model `nexa:"model=sale.customer.profile"`

	Name            sdk.String                  `nexa:"required,size=120,string=Reference"`
	PartnerID       sdk.Many2One[CorePartner]   `nexa:"required,index,string=Customer"`
	CustomerType    sdk.Selection[CustomerType] `nexa:"string=Customer Type,default=business"`
	Code            sdk.String                  `nexa:"size=60,string=Customer Code"`
	Segment         sdk.String                  `nexa:"size=60,string=Segment"`
	Currency        sdk.String                  `nexa:"size=3,string=Currency,default=USD"`
	PriceListRef    sdk.Many2One[sdk.Any]       `nexa:"string=Price List,comodel=product.pricelist"`
	PaymentTerms    sdk.String                  `nexa:"size=200,string=Payment Terms"`
	DeliveryTerms   sdk.String                  `nexa:"size=200,string=Delivery Terms"`
	TaxTreatment    sdk.String                  `nexa:"size=200,string=Tax Treatment"`
	SalespersonID   sdk.Many2One[CoreUser]      `nexa:"string=Assigned Salesperson"`
	TeamID          sdk.Many2One[SaleTeam]      `nexa:"string=Sales Team"`
	Territory       sdk.String                  `nexa:"size=120,string=Territory"`
	ShippingAddress sdk.Text                    `nexa:"string=Preferred Shipping Address"`
	CreditPolicyRef sdk.Many2One[sdk.Any]       `nexa:"string=Credit Policy,comodel=account.move"`
	PayerStatus     sdk.Selection[PayerStatus]  `nexa:"string=Payer Status,default=normal"`
	CompanyID       sdk.Many2One[CoreCompany]   `nexa:"string=Company"`
	Notes           sdk.Text                    `nexa:"string=Notes"`
	Active          sdk.Boolean                 `nexa:"string=Active,default=true"`
}

// SaleTeam is a sales team with a leader and an optional parent (region).
type SaleTeam struct {
	sdk.Model `nexa:"model=sale.team"`

	Name      sdk.String                   `nexa:"required,index,string=Team Name"`
	LeaderID  sdk.Many2One[CoreUser]       `nexa:"string=Team Leader"`
	ParentID  sdk.Many2One[SaleTeam]       `nexa:"string=Parent Team"`
	CompanyID sdk.Many2One[CoreCompany]    `nexa:"string=Company"`
	Members   sdk.One2Many[SaleTeamMember] `nexa:"string=Members"`
	Active    sdk.Boolean                  `nexa:"string=Active,default=true"`
}

// SaleTeamMember assigns a salesperson to a team for a period, with an
// optional quota target for that period.
type SaleTeamMember struct {
	sdk.Model `nexa:"model=sale.team.member"`

	TeamID      sdk.Many2One[SaleTeam] `nexa:"required,index,string=Team"`
	UserID      sdk.Many2One[CoreUser] `nexa:"required,string=Salesperson"`
	Role        sdk.String             `nexa:"size=60,string=Role"`
	Territory   sdk.String             `nexa:"size=120,string=Territory"`
	DateFrom    sdk.Date               `nexa:"string=From"`
	DateTo      sdk.Date               `nexa:"string=To"`
	QuotaAmount sdk.Numeric            `nexa:"string=Period Quota,precision=18,scale=2,default=0"`
	Active      sdk.Boolean            `nexa:"string=Active,default=true"`
}

func init() {
	sdk.MustRegister("sale_customer",
		&SaleCustomerProfile{},
		&SaleTeam{},
		&SaleTeamMember{},
	)
}
