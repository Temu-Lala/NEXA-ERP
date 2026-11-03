package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	purchasemodels "nexa_addons/purchase/models"
)

type CoreUser = basemodels.CoreUser

// PurchaseOrder / PurchaseOrderLine are owned by the core purchase module.
type (
	PurchaseOrder     = purchasemodels.PurchaseOrder
	PurchaseOrderLine = purchasemodels.PurchaseOrderLine
)

// Purchase owns the matching VALIDATION only. The supplier bill (account.move)
// is referenced, never re-stored; Accounting owns the bill, AP balance, journal
// entries and payment. References are loose (comodel) to respect ownership.

type MatchState string

const (
	MatchDraft      MatchState = "draft"
	MatchInProgress MatchState = "in_progress"
	MatchDone       MatchState = "done"
)

type MatchResult string

const (
	ResultPending   MatchResult = "pending"
	ResultMatched   MatchResult = "matched"
	ResultException MatchResult = "exception"
	ResultHold      MatchResult = "on_hold"
)

type ExceptionType string

const (
	ExcPrice     ExceptionType = "price"
	ExcQuantity  ExceptionType = "quantity"
	ExcTax       ExceptionType = "tax"
	ExcCurrency  ExceptionType = "currency"
	ExcDuplicate ExceptionType = "duplicate"
	ExcNoReceipt ExceptionType = "missing_receipt"
	ExcOther     ExceptionType = "other"
)

type ExcState string

const (
	ExcStateOpen          ExcState = "open"
	ExcStateInvestigating ExcState = "investigating"
	ExcStateResolved      ExcState = "resolved"
	ExcStateWaived        ExcState = "waived"
)

type ExcSeverity string

const (
	SevLow    ExcSeverity = "low"
	SevMedium ExcSeverity = "medium"
	SevHigh   ExcSeverity = "high"
)

// MatchMethod records how many documents the reconciliation compared. Two-way
// matches an invoice to the PO, three-way adds the accepted receipt, and
// four-way also folds in the service/contract acceptance milestone.
type MatchMethod string

const (
	MethodTwo   MatchMethod = "two_way"
	MethodThree MatchMethod = "three_way"
	MethodFour  MatchMethod = "four_way"
)

// MatchEvent is the publish-only outbox vocabulary towards Accounts Payable.
// Purchase never posts or pays; it emits reviewable signals (blueprint section 12).
type MatchEvent string

const (
	EvtExcCreated MatchEvent = "exception_created"
	EvtMatched    MatchEvent = "matched"
	EvtSentToAP   MatchEvent = "sent_to_ap"
)

// PurchaseInvoiceMatchCase is a three-way matching case: PO + receipt + invoice.
type PurchaseInvoiceMatchCase struct {
	sdk.Model `nexa:"model=purchase.invoice.match.case"`

	Name            sdk.String                                  `nexa:"required,index,string=Reference"`
	OrderID         sdk.Many2One[PurchaseOrder]                 `nexa:"required,index,string=Purchase Order"`
	BillRef         sdk.Many2One[sdk.Any]                       `nexa:"string=Supplier Bill,comodel=account.move"`
	BillNumber      sdk.String                                  `nexa:"size=120,string=Invoice Reference"`
	ReceiptRef      sdk.Many2One[sdk.Any]                       `nexa:"string=Receipt,comodel=purchase.receipt"`
	PartnerID       sdk.Many2One[sdk.Any]                       `nexa:"string=Supplier,comodel=core.partner"`
	Currency        sdk.String                                  `nexa:"size=3,string=Currency,default=USD"`
	MatchDate       sdk.Date                                    `nexa:"string=Match Date,default=today"`
	Method          sdk.Selection[MatchMethod]                  `nexa:"string=Match Method,default=three_way"`
	State           sdk.Selection[MatchState]                   `nexa:"string=Matching,default=draft"`
	Result          sdk.Selection[MatchResult]                  `nexa:"string=Outcome,default=pending"`
	ReviewerID      sdk.Many2One[CoreUser]                      `nexa:"string=Reviewed By"`
	TolerancePct    sdk.Numeric                                 `nexa:"string=Price Tolerance %,precision=6,scale=2,default=0"`
	ToleranceQtyPct sdk.Numeric                                 `nexa:"string=Qty Tolerance %,precision=6,scale=2,default=0"`
	ToleranceTaxPct sdk.Numeric                                 `nexa:"string=Tax Tolerance %,precision=6,scale=2,default=0"`
	RoundingTol     sdk.Numeric                                 `nexa:"string=Rounding Tolerance,precision=18,scale=2,default=0.05"`
	BillAmount      sdk.Numeric                                 `nexa:"string=Invoice Total,precision=18,scale=2,default=0"`
	BillTaxAmount   sdk.Numeric                                 `nexa:"string=Invoice Tax,precision=18,scale=2,default=0"`
	BillDate        sdk.Date                                    `nexa:"string=Invoice Date"`
	PoAmount        sdk.Numeric                                 `nexa:"string=PO Value,precision=18,scale=2,default=0"`
	ExpectedAmount  sdk.Numeric                                 `nexa:"string=Expected (Accepted),precision=18,scale=2,default=0"`
	MatchedAmount   sdk.Numeric                                 `nexa:"string=Matched (Approvable),precision=18,scale=2,default=0"`
	Fingerprint     sdk.String                                  `nexa:"size=120,index,string=Duplicate Fingerprint"`
	Duplicate       sdk.Boolean                                 `nexa:"string=Duplicate,default=false"`
	Escalated       sdk.Boolean                                 `nexa:"string=Escalated,default=false"`
	Approved        sdk.Boolean                                 `nexa:"string=Exceptions Approved,default=false"`
	WaivedBy        sdk.Many2One[CoreUser]                      `nexa:"string=Waived By"`
	WaiverReason    sdk.Text                                    `nexa:"string=Waiver Reason"`
	SentToAp        sdk.Boolean                                 `nexa:"string=Sent To AP,default=false"`
	SentToApOn      sdk.DateTime                                `nexa:"string=Sent To AP At"`
	CommitmentRef   sdk.Many2One[sdk.Any]                       `nexa:"string=Budget Commitment,comodel=purchase.budget.commitment"`
	RequisitionRef  sdk.Many2One[sdk.Any]                       `nexa:"string=Requisition,comodel=purchase.requisition"`
	BudgetRef       sdk.Many2One[sdk.Any]                       `nexa:"string=Budget,comodel=account.budget"`
	Lines           sdk.One2Many[PurchaseInvoiceMatchLine]      `nexa:"string=Compared Lines"`
	Exceptions      sdk.One2Many[PurchaseInvoiceMatchException] `nexa:"string=Exceptions"`
	Notes           sdk.Text                                    `nexa:"string=Notes"`
	History         sdk.Text                                    `nexa:"string=Matching History"`
	Active          sdk.Boolean                                 `nexa:"string=Active,default=true"`
}

// PurchaseInvoiceMatchLine compares a PO line, its receipt and the invoiced
// quantity/price, recording variances against tolerance.
type PurchaseInvoiceMatchLine struct {
	sdk.Model `nexa:"model=purchase.invoice.match.line"`

	MatchID         sdk.Many2One[PurchaseInvoiceMatchCase] `nexa:"required,index,string=Match Case"`
	OrderLineID     sdk.Many2One[PurchaseOrderLine]        `nexa:"string=PO Line"`
	ProductID       sdk.Many2One[sdk.Any]                  `nexa:"string=Product,comodel=product.product"`
	Description     sdk.String                             `nexa:"size=255,string=Description"`
	OrderedQty      sdk.Numeric                            `nexa:"string=Ordered,precision=18,scale=3,default=0"`
	ReceivedQty     sdk.Numeric                            `nexa:"string=Received,precision=18,scale=3,default=0"`
	AcceptedQty     sdk.Numeric                            `nexa:"string=Accepted,precision=18,scale=3,default=0"`
	InvoicedQty     sdk.Numeric                            `nexa:"string=Invoiced,precision=18,scale=3,default=0"`
	PoUnitPrice     sdk.Numeric                            `nexa:"string=PO Unit Price,precision=18,scale=2,default=0"`
	InvUnitPrice    sdk.Numeric                            `nexa:"string=Invoice Unit Price,precision=18,scale=2,default=0"`
	PriceVariance   sdk.Numeric                            `nexa:"string=Price Variance,precision=18,scale=2,default=0"`
	QtyVariance     sdk.Numeric                            `nexa:"string=Qty Variance,precision=18,scale=3,default=0"`
	PoAmount        sdk.Numeric                            `nexa:"string=PO Amount,precision=18,scale=2,default=0"`
	InvAmount       sdk.Numeric                            `nexa:"string=Invoice Amount,precision=18,scale=2,default=0"`
	AmountVariance  sdk.Numeric                            `nexa:"string=Amount Variance,precision=18,scale=2,default=0"`
	WithinTolerance sdk.Boolean                            `nexa:"string=Within Tolerance,default=true"`
	Note            sdk.Text                               `nexa:"string=Note"`
}

// PurchaseInvoiceMatchException records a price/quantity/tax/document discrepancy
// and its authorized resolution. The resolver must not bypass financial controls.
type PurchaseInvoiceMatchException struct {
	sdk.Model `nexa:"model=purchase.invoice.match.exception"`

	Name        sdk.String                             `nexa:"required,size=60,string=Reference"`
	MatchID     sdk.Many2One[PurchaseInvoiceMatchCase] `nexa:"required,index,string=Match Case"`
	MatchLineID sdk.Many2One[PurchaseInvoiceMatchLine] `nexa:"string=Match Line"`
	ExcType     sdk.Selection[ExceptionType]           `nexa:"string=Type,default=other"`
	Severity    sdk.Selection[ExcSeverity]             `nexa:"string=Severity,default=medium"`
	Description sdk.Text                               `nexa:"string=Description"`
	DiffAmount  sdk.Numeric                            `nexa:"string=Difference Amount,precision=18,scale=2,default=0"`
	State       sdk.Selection[ExcState]                `nexa:"string=Status,default=open"`
	ResolvedBy  sdk.Many2One[CoreUser]                 `nexa:"string=Resolved By"`
	Resolution  sdk.Text                               `nexa:"string=Resolution"`
	ApprovalRef sdk.Many2One[sdk.Any]                  `nexa:"string=Approval,comodel=approval.request"`
	Active      sdk.Boolean                            `nexa:"string=Active,default=true"`
}

// PurchaseInvoiceMatchEvent is the publish-only outbox towards Accounts
// Payable. It records an exception/match/hand-off signal for downstream
// consumption; Purchase never posts the bill or authorizes a payment here.
type PurchaseInvoiceMatchEvent struct {
	sdk.Model `nexa:"model=purchase.invoice.match.event"`

	Name        sdk.String                             `nexa:"required,index,size=60,string=Reference"`
	EventType   sdk.Selection[MatchEvent]              `nexa:"required,index,string=Event"`
	MatchID     sdk.Many2One[PurchaseInvoiceMatchCase] `nexa:"index,string=Match Case"`
	Amount      sdk.Numeric                            `nexa:"string=Amount,precision=18,scale=2,default=0"`
	Fingerprint sdk.String                             `nexa:"size=120,string=Fingerprint"`
	Message     sdk.Text                               `nexa:"string=Details"`
	Consumed    sdk.Boolean                            `nexa:"string=Consumed by AP,default=false"`
	Date        sdk.DateTime                           `nexa:"string=Published At"`
}

func init() {
	sdk.MustRegister("purchase_match",
		&PurchaseInvoiceMatchCase{},
		&PurchaseInvoiceMatchLine{},
		&PurchaseInvoiceMatchException{},
		&PurchaseInvoiceMatchEvent{},
	)
}
