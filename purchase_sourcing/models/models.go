package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	purchasemodels "nexa_addons/purchase/models"
)

// CorePartner / CoreUser are shared master data (Contacts). Supplier identity
// is NOT duplicated here; purchase.supplier.profile only layers procurement-
// specific attributes onto an existing partner.
type (
	CorePartner = basemodels.CorePartner
	CoreUser    = basemodels.CoreUser
)

type SupplierStatus string

const (
	SuppStatusDraft     SupplierStatus = "draft"
	SuppStatusPending   SupplierStatus = "pending"
	SuppStatusApproved  SupplierStatus = "approved"
	SuppStatusSuspended SupplierStatus = "suspended"
	SuppStatusBlocked   SupplierStatus = "blocked"
)

type SupplierRisk string

const (
	RiskUnclassified SupplierRisk = "unclassified"
	RiskLow          SupplierRisk = "low"
	RiskMedium       SupplierRisk = "medium"
	RiskHigh         SupplierRisk = "high"
)

type RFQState string

const (
	RFQStateDraft      RFQState = "draft"
	RFQStateApproved   RFQState = "approved"
	RFQStateSent       RFQState = "sent"
	RFQStateResponses  RFQState = "responses"
	RFQStateEvaluation RFQState = "evaluation"
	RFQStateAwarded    RFQState = "awarded"
	RFQStateCancelled  RFQState = "cancelled"
	RFQStateExpired    RFQState = "expired"
)

type InviteState string

const (
	InviteInvited   InviteState = "invited"
	InviteViewed    InviteState = "viewed"
	InviteResponded InviteState = "responded"
	InviteDeclined  InviteState = "declined"
	InviteExpired   InviteState = "expired"
)

type QuotationState string

const (
	QuoteDraft     QuotationState = "draft"
	QuoteReceived  QuotationState = "received"
	QuoteEvaluated QuotationState = "evaluated"
	QuoteAccepted  QuotationState = "accepted"
	QuoteRejected  QuotationState = "rejected"
)

type AwardState string

const (
	AwardDraft    AwardState = "draft"
	AwardProposed AwardState = "proposed"
	AwardApproved AwardState = "approved"
)

type RoundKind string

const (
	RoundInitial RoundKind = "initial"
	RoundRevised RoundKind = "revised"
	RoundCounter RoundKind = "counter"
	RoundFinal   RoundKind = "final"
)

// PurchaseSupplierProfile layers procurement data on a shared partner.
type PurchaseSupplierProfile struct {
	sdk.Model `nexa:"model=purchase.supplier.profile"`

	Name           sdk.String                    `nexa:"required,size=120,string=Reference"`
	PartnerID      sdk.Many2One[CorePartner]     `nexa:"required,index,string=Supplier"`
	SupplierCode   sdk.String                    `nexa:"size=60,string=Supplier Code"`
	Classification sdk.String                    `nexa:"size=120,string=Business Classification"`
	Status         sdk.Selection[SupplierStatus] `nexa:"string=Status,default=draft"`
	Risk           sdk.Selection[SupplierRisk]   `nexa:"string=Risk,default=unclassified"`
	Currencies     sdk.String                    `nexa:"size=120,string=Supported Currencies"`
	PaymentTerms   sdk.String                    `nexa:"size=200,string=Payment Terms"`
	DeliveryTerms  sdk.String                    `nexa:"size=200,string=Delivery Terms"`
	QualityRating  sdk.Numeric                   `nexa:"string=Quality Rating,precision=4,scale=2,default=0"`
	OnTimeDelivery sdk.Numeric                   `nexa:"string=On-Time Delivery %,precision=4,scale=2,default=0"`
	DueDiligence   sdk.Boolean                   `nexa:"string=Due Diligence Complete,default=false"`
	OnboardDate    sdk.Date                      `nexa:"string=Onboard Date"`
	Notes          sdk.Text                      `nexa:"string=Notes"`
	Active         sdk.Boolean                   `nexa:"string=Active,default=true"`
}

// PurchaseRFQ is a request-for-quotation header.
type PurchaseRFQ struct {
	sdk.Model `nexa:"model=purchase.rfq"`

	Name           sdk.String                        `nexa:"required,index,string=Reference"`
	DateRFQ        sdk.Date                          `nexa:"string=Date,default=today"`
	OwnerID        sdk.Many2One[CoreUser]            `nexa:"string=Procurement Owner"`
	RequisitionRef sdk.Many2One[sdk.Any]             `nexa:"string=Requisition,comodel=purchase.requisition"`
	Category       sdk.String                        `nexa:"size=120,string=Purchase Category"`
	Currency       sdk.String                        `nexa:"size=3,string=Currency,default=USD"`
	TaxAssumption  sdk.String                        `nexa:"size=200,string=Tax Assumptions"`
	DeliveryTerms  sdk.String                        `nexa:"size=200,string=Delivery Terms"`
	Deadline       sdk.Date                          `nexa:"string=Submission Deadline"`
	ReqDocuments   sdk.Text                          `nexa:"string=Required Supplier Documents"`
	EvalCriteria   sdk.Text                          `nexa:"string=Evaluation Criteria"`
	State          sdk.Selection[RFQState]           `nexa:"string=Status,default=draft"`
	Lines          sdk.One2Many[PurchaseRFQLine]     `nexa:"string=Line Items"`
	Invited        sdk.One2Many[PurchaseRFQSupplier] `nexa:"string=Invited Suppliers"`
	Notes          sdk.Text                          `nexa:"string=Notes"`
	Active         sdk.Boolean                       `nexa:"string=Active,default=true"`
}

// PurchaseRFQLine is a requested product/service on an RFQ.
type PurchaseRFQLine struct {
	sdk.Model `nexa:"model=purchase.rfq.line"`

	RFQID       sdk.Many2One[PurchaseRFQ] `nexa:"required,index,string=RFQ"`
	Sequence    sdk.Integer               `nexa:"string=Sequence,default=10"`
	ProductID   sdk.Many2One[sdk.Any]     `nexa:"string=Product,comodel=product.product"`
	Description sdk.String                `nexa:"required,size=255,string=Description"`
	Quantity    sdk.Numeric               `nexa:"string=Quantity,precision=18,scale=3,default=1"`
	UoM         sdk.String                `nexa:"size=30,string=Unit"`
	Spec        sdk.Text                  `nexa:"string=Specification"`
	DeliveryReq sdk.Date                  `nexa:"string=Required Delivery"`
	Location    sdk.String                `nexa:"size=200,string=Delivery Location"`
}

// PurchaseRFQSupplier records an invited supplier and its response status.
type PurchaseRFQSupplier struct {
	sdk.Model `nexa:"model=purchase.rfq.supplier"`

	RFQID         sdk.Many2One[PurchaseRFQ]  `nexa:"required,index,string=RFQ"`
	PartnerID     sdk.Many2One[CorePartner]  `nexa:"required,string=Supplier"`
	InvitedDate   sdk.Date                   `nexa:"string=Invited On,default=today"`
	ResponseState sdk.Selection[InviteState] `nexa:"string=Response,default=invited"`
	RespondedOn   sdk.Date                   `nexa:"string=Responded On"`
	Note          sdk.Text                   `nexa:"string=Clarifications"`
}

// PurchaseQuotation is a supplier's commercial offer against an RFQ.
type PurchaseQuotation struct {
	sdk.Model `nexa:"model=purchase.quotation"`

	Name          sdk.String                          `nexa:"required,index,string=Reference"`
	RFQID         sdk.Many2One[PurchaseRFQ]           `nexa:"required,index,string=RFQ"`
	PartnerID     sdk.Many2One[CorePartner]           `nexa:"required,string=Supplier"`
	QuotationRef  sdk.String                          `nexa:"size=60,string=Supplier Quote No."`
	DateQuote     sdk.Date                            `nexa:"string=Date,default=today"`
	ValidUntil    sdk.Date                            `nexa:"string=Validity"`
	Currency      sdk.String                          `nexa:"size=3,string=Currency,default=USD"`
	ExchangeRate  sdk.Numeric                         `nexa:"string=Exchange Rate,precision=18,scale=6,default=1"`
	PaymentTerms  sdk.String                          `nexa:"size=200,string=Payment Terms"`
	DeliveryTerms sdk.String                          `nexa:"size=200,string=Delivery Terms"`
	LeadTimeDays  sdk.Integer                         `nexa:"string=Lead Time (days),default=0"`
	Freight       sdk.Numeric                         `nexa:"string=Freight & Charges,precision=18,scale=2,default=0"`
	TaxAmount     sdk.Numeric                         `nexa:"string=Tax,precision=18,scale=2,default=0"`
	TotalLanded   sdk.Numeric                         `nexa:"string=Landed Total,precision=18,scale=2,default=0"`
	State         sdk.Selection[QuotationState]       `nexa:"string=Status,default=draft"`
	Lines         sdk.One2Many[PurchaseQuotationLine] `nexa:"string=Lines"`
	Notes         sdk.Text                            `nexa:"string=Notes"`
	Active        sdk.Boolean                         `nexa:"string=Active,default=true"`
}

// PurchaseQuotationLine is an offered price line on a supplier quotation.
type PurchaseQuotationLine struct {
	sdk.Model `nexa:"model=purchase.quotation.line"`

	QuotationID  sdk.Many2One[PurchaseQuotation] `nexa:"required,index,string=Quotation"`
	RFQLineID    sdk.Many2One[PurchaseRFQLine]   `nexa:"string=RFQ Line"`
	Sequence     sdk.Integer                     `nexa:"string=Sequence,default=10"`
	ProductID    sdk.Many2One[sdk.Any]           `nexa:"string=Product,comodel=product.product"`
	SupplierCode sdk.String                      `nexa:"size=60,string=Supplier Product Code"`
	Description  sdk.String                      `nexa:"required,size=255,string=Description"`
	Quantity     sdk.Numeric                     `nexa:"string=Quantity,precision=18,scale=3,default=1"`
	UnitPrice    sdk.Numeric                     `nexa:"string=Unit Price,precision=18,scale=2,default=0"`
	Discount     sdk.Numeric                     `nexa:"string=Discount,precision=18,scale=2,default=0"`
	LineTotal    sdk.Numeric                     `nexa:"string=Line Total,precision=18,scale=2,default=0"`
	PromiseDate  sdk.Date                        `nexa:"string=Promised Delivery"`
}

// PurchaseEvaluation captures weighted scoring of quotations for an RFQ.
type PurchaseEvaluation struct {
	sdk.Model `nexa:"model=purchase.evaluation"`

	Name           sdk.String                `nexa:"required,size=60,string=Reference"`
	RFQID          sdk.Many2One[PurchaseRFQ] `nexa:"required,index,string=RFQ"`
	EvaluatorID    sdk.Many2One[CoreUser]    `nexa:"string=Evaluator"`
	EvalDate       sdk.Date                  `nexa:"string=Date,default=today"`
	WeightPrice    sdk.Numeric               `nexa:"string=Weight: Price,precision=6,scale=2,default=0"`
	WeightLanded   sdk.Numeric               `nexa:"string=Weight: Landed Cost,precision=6,scale=2,default=0"`
	WeightDelivery sdk.Numeric               `nexa:"string=Weight: Delivery,precision=6,scale=2,default=0"`
	WeightQuality  sdk.Numeric               `nexa:"string=Weight: Quality,precision=6,scale=2,default=0"`
	WeightTerms    sdk.Numeric               `nexa:"string=Weight: Terms,precision=6,scale=2,default=0"`
	WeightRisk     sdk.Numeric               `nexa:"string=Weight: Risk,precision=6,scale=2,default=0"`
	Summary        sdk.Text                  `nexa:"string=Assumptions & Summary"`
	Notes          sdk.Text                  `nexa:"string=Notes"`
}

// PurchaseAward records the approved supplier-selection decision.
type PurchaseAward struct {
	sdk.Model `nexa:"model=purchase.award"`

	Name               sdk.String                                 `nexa:"required,index,string=Reference"`
	RFQID              sdk.Many2One[PurchaseRFQ]                  `nexa:"required,index,string=RFQ"`
	EvaluationID       sdk.Many2One[PurchaseEvaluation]           `nexa:"string=Evaluation"`
	QuotationID        sdk.Many2One[PurchaseQuotation]            `nexa:"string=Selected Quotation"`
	PartnerID          sdk.Many2One[CorePartner]                  `nexa:"required,string=Selected Supplier"`
	AwardDate          sdk.Date                                   `nexa:"string=Date,default=today"`
	AwardedValue       sdk.Numeric                                `nexa:"string=Awarded Value,precision=18,scale=2,default=0"`
	SelectionRationale sdk.Text                                   `nexa:"string=Reason for Selection"`
	PolicyExceptions   sdk.Text                                   `nexa:"string=Policy Exceptions"`
	ApproverID         sdk.Many2One[CoreUser]                     `nexa:"string=Approved By"`
	PORef              sdk.Many2One[purchasemodels.PurchaseOrder] `nexa:"string=Purchase Order"`
	State              sdk.Selection[AwardState]                  `nexa:"string=Status,default=draft"`
	Notes              sdk.Text                                   `nexa:"string=Notes"`
	Active             sdk.Boolean                                `nexa:"string=Active,default=true"`
}

// PurchaseQuotationRound records a negotiation round against a supplier
// quotation (blueprint §8). Sensitive negotiation text is stored here and is
// access-restricted; a finalized award locks the winning round so commercial
// terms cannot be silently changed after award.
type PurchaseQuotationRound struct {
	sdk.Model `nexa:"model=purchase.quotation.round"`

	Name         sdk.String                      `nexa:"required,index,string=Reference"`
	RFQID        sdk.Many2One[PurchaseRFQ]       `nexa:"required,index,string=RFQ"`
	QuotationID  sdk.Many2One[PurchaseQuotation] `nexa:"required,string=Quotation"`
	PartnerID    sdk.Many2One[CorePartner]       `nexa:"string=Supplier"`
	RoundNo      sdk.Integer                     `nexa:"string=Round,default=1"`
	Kind         sdk.Selection[RoundKind]        `nexa:"string=Type,default=initial"`
	RoundDate    sdk.Date                        `nexa:"string=Date,default=today"`
	OfferedTotal sdk.Numeric                     `nexa:"string=Offered Landed Total,precision=18,scale=2,default=0"`
	Outcome      sdk.String                      `nexa:"size=120,string=Outcome"`
	Participants sdk.String                      `nexa:"size=255,string=Participants"`
	TermsNote    sdk.Text                        `nexa:"string=Commercial Notes (restricted)"`
	Locked       sdk.Boolean                     `nexa:"string=Locked,default=false"`
	Active       sdk.Boolean                     `nexa:"string=Active,default=true"`
}

func init() {
	sdk.MustRegister("purchase_sourcing",
		&PurchaseSupplierProfile{},
		&PurchaseRFQ{},
		&PurchaseRFQLine{},
		&PurchaseRFQSupplier{},
		&PurchaseQuotation{},
		&PurchaseQuotationLine{},
		&PurchaseEvaluation{},
		&PurchaseAward{},
		&PurchaseQuotationRound{},
	)
}
