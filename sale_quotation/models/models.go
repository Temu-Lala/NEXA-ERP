package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	salemodels "nexa_addons/sale/models"
)

// Quotation lifecycle extensions around the owned sale.order document (which
// carries draft → sent (quotation) → sale (confirmed order) states). Approval
// decisions reuse the shared approval engine; credit data stays in Accounting.

type (
	SaleOrder     = salemodels.SaleOrder
	SaleOrderLine = salemodels.SaleOrderLine
	CoreUser      = basemodels.CoreUser
)

type QuotaApprovalState string

const (
	QApprNotRequired QuotaApprovalState = "not_required"
	QApprPending     QuotaApprovalState = "pending"
	QApprApproved    QuotaApprovalState = "approved"
	QApprRejected    QuotaApprovalState = "rejected"
	QApprEscalated   QuotaApprovalState = "escalated"
)

type CreditDecision string

const (
	CreditNone       CreditDecision = "not_checked"
	CreditApproved   CreditDecision = "approved"
	CreditConditions CreditDecision = "approved_with_conditions"
	CreditReview     CreditDecision = "manual_review"
	CreditBlocked    CreditDecision = "blocked"
)

type ExpiryState string

const (
	ExpOpen       ExpiryState = "open"
	ExpAccepted   ExpiryState = "accepted"
	ExpRejected   ExpiryState = "rejected"
	ExpExpired    ExpiryState = "expired"
	ExpWithdrawn  ExpiryState = "withdrawn"
	ExpSuperseded ExpiryState = "superseded"
)

// SaleQuotationApproval records the commercial approval gate for a quotation:
// the reason it was triggered (discount, margin floor, credit) and the decision,
// referencing the shared approval.request rather than duplicating an engine.
type SaleQuotationApproval struct {
	sdk.Model `nexa:"model=sale.quotation.approval"`

	Name         sdk.String                        `nexa:"required,index,string=Reference"`
	OrderID      sdk.Many2One[SaleOrder]           `nexa:"required,index,string=Quotation"`
	Trigger      sdk.String                        `nexa:"size=60,string=Trigger"`
	DiscountPct  sdk.Numeric                       `nexa:"string=Discount %,precision=6,scale=2,default=0"`
	MarginPct    sdk.Numeric                       `nexa:"string=Est. Margin %,precision=6,scale=2,default=0"`
	RequestedAmt sdk.Numeric                       `nexa:"string=Amount,precision=18,scale=2,default=0"`
	ApproverID   sdk.Many2One[CoreUser]            `nexa:"string=Approver"`
	State        sdk.Selection[QuotaApprovalState] `nexa:"string=Status,default=pending"`
	DecisionDate sdk.Date                          `nexa:"string=Decision Date"`
	Reason       sdk.Text                          `nexa:"string=Reason"`
	ApprovalRef  sdk.Many2One[sdk.Any]             `nexa:"string=Approval Request,comodel=approval.request"`
	Active       sdk.Boolean                       `nexa:"string=Active,default=true"`
}

// SaleQuotationRevision keeps an immutable snapshot of an issued quotation so a
// change of commercial terms creates a controlled revision instead of overwriting
// the document the customer saw.
type SaleQuotationRevision struct {
	sdk.Model `nexa:"model=sale.quotation.revision"`

	Name          sdk.String              `nexa:"required,index,string=Reference"`
	OrderID       sdk.Many2One[SaleOrder] `nexa:"required,index,string=Quotation"`
	Revision      sdk.Integer             `nexa:"required,string=Revision #,default=1"`
	IssueDate     sdk.Date                `nexa:"string=Issued On,default=today"`
	Subtotal      sdk.Numeric             `nexa:"string=Subtotal,precision=18,scale=2,default=0"`
	DiscountTotal sdk.Numeric             `nexa:"string=Discount Total,precision=18,scale=2,default=0"`
	TaxTotal      sdk.Numeric             `nexa:"string=Tax Total,precision=18,scale=2,default=0"`
	GrandTotal    sdk.Numeric             `nexa:"string=Grand Total,precision=18,scale=2,default=0"`
	Snapshot      sdk.Text                `nexa:"string=Document Snapshot"`
	IsCurrent     sdk.Boolean             `nexa:"string=Current Revision,default=true"`
	AcceptedBy    sdk.Many2One[CoreUser]  `nexa:"string=Accepted By"`
	AcceptedOn    sdk.DateTime            `nexa:"string=Accepted On"`
	Notes         sdk.Text                `nexa:"string=Change Notes"`
}

// SaleQuotationHistory records quotation state transitions with actor and reason.
type SaleQuotationHistory struct {
	sdk.Model `nexa:"model=sale.quotation.status.history"`

	OrderID   sdk.Many2One[SaleOrder] `nexa:"required,index,string=Quotation"`
	FromState sdk.String              `nexa:"size=30,string=From"`
	ToState   sdk.String              `nexa:"required,size=30,string=To"`
	UserID    sdk.Many2One[CoreUser]  `nexa:"string=Changed By"`
	Reason    sdk.String              `nexa:"size=300,string=Reason"`
	Timestamp sdk.DateTime            `nexa:"string=At"`
}

// SaleCreditCheck stores the outcome of a credit-policy check requested before
// order confirmation. Accounting owns balances; Sales keeps only the decision.
type SaleCreditCheck struct {
	sdk.Model `nexa:"model=sale.credit.check"`

	Name              sdk.String                    `nexa:"required,index,string=Reference"`
	OrderID           sdk.Many2One[SaleOrder]       `nexa:"required,index,string=Order"`
	PartnerID         sdk.Many2One[sdk.Any]         `nexa:"string=Customer,comodel=core.partner"`
	CheckDate         sdk.Date                      `nexa:"string=Date,default=today"`
	CreditLimit       sdk.Numeric                   `nexa:"string=Credit Limit,precision=18,scale=2,default=0"`
	OutstandingAR     sdk.Numeric                   `nexa:"string=Outstanding Receivables,precision=18,scale=2,default=0"`
	OverdueAmount     sdk.Numeric                   `nexa:"string=Overdue,precision=18,scale=2,default=0"`
	RequestedExposure sdk.Numeric                   `nexa:"string=Order Exposure,precision=18,scale=2,default=0"`
	Decision          sdk.Selection[CreditDecision] `nexa:"string=Decision,default=not_checked"`
	ReviewerID        sdk.Many2One[CoreUser]        `nexa:"string=Reviewed By"`
	Conditions        sdk.Text                      `nexa:"string=Conditions"`
	Notes             sdk.Text                      `nexa:"string=Notes"`
}

func init() {
	sdk.MustRegister("sale_quotation",
		&SaleQuotationApproval{},
		&SaleQuotationRevision{},
		&SaleQuotationHistory{},
		&SaleCreditCheck{},
	)
}
