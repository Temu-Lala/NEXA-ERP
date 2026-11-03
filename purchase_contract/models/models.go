package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	purchasemodels "nexa_addons/purchase/models"
)

type CoreUser = basemodels.CoreUser

// PurchaseOrder / PurchaseOrderLine are owned by the core purchase module;
// contracts reference them loosely so ownership is never crossed.
type (
	PurchaseOrder = purchasemodels.PurchaseOrder
)

// Contracts & framework agreements (blueprint §10). Purchase owns the contract
// record, its price schedules, obligations and consumption tracking. A contract
// NEVER bypasses approval or budget control - it only supplies contracted rates
// and a value ceiling that the matching/commitment engines still respect.

type ContractType string

const (
	TypeFixed        ContractType = "fixed"
	TypeFramework    ContractType = "framework"
	TypeBlanket      ContractType = "blanket"
	TypeRateCard     ContractType = "rate_card"
	TypeVolume       ContractType = "volume"
	TypeSupply       ContractType = "supply"
	TypeSupport      ContractType = "support"
	TypeSubscription ContractType = "subscription"
	TypeMilestone    ContractType = "milestone"
	TypeConsignment  ContractType = "consignment"
)

type ContractState string

const (
	CStateDraft     ContractState = "draft"
	CStatePending   ContractState = "pending_approval"
	CStateActive    ContractState = "active"
	CStateSuspended ContractState = "suspended"
	CStateExpiring  ContractState = "expiring"
	CStateExpired   ContractState = "expired"
	CStateRenewed   ContractState = "renewed"
	CStateClosed    ContractState = "closed"
	CStateCancelled ContractState = "cancelled"
)

type ObligationType string

const (
	ObSLA        ObligationType = "sla"
	ObDelivery   ObligationType = "delivery"
	ObQuality    ObligationType = "quality"
	ObMonitoring ObligationType = "monitoring"
	ObReporting  ObligationType = "reporting"
	ObCompliance ObligationType = "compliance"
	ObPayment    ObligationType = "payment"
)

type MonitoringState string

const (
	MonPending  MonitoringState = "pending"
	MonMet      MonitoringState = "met"
	MonBreached MonitoringState = "breached"
	MonNA       MonitoringState = "not_applicable"
)

type ConsumptionKind string

const (
	ConsCommit   ConsumptionKind = "commit"
	ConsInvoice  ConsumptionKind = "invoice"
	ConsCredit   ConsumptionKind = "credit"
	ConsEstimate ConsumptionKind = "estimate"
)

// PurchaseContract is the master commercial agreement.
type PurchaseContract struct {
	sdk.Model `nexa:"model=purchase.contract"`

	Name             sdk.String                                  `nexa:"required,index,size=60,string=Reference"`
	Title            sdk.String                                  `nexa:"required,size=255,string=Title"`
	ContractType     sdk.Selection[ContractType]                 `nexa:"string=Type,default=framework"`
	PartnerID        sdk.Many2One[sdk.Any]                       `nexa:"required,index,string=Supplier,comodel=core.partner"`
	OwnerID          sdk.Many2One[CoreUser]                      `nexa:"string=Contract Owner"`
	CompanyID        sdk.Many2One[basemodels.CoreCompany]        `nexa:"string=Contracting Entity"`
	Category         sdk.String                                  `nexa:"size=120,string=Category"`
	Currency         sdk.String                                  `nexa:"size=3,string=Currency,default=USD"`
	DateStart        sdk.Date                                    `nexa:"string=Effective Date"`
	DateEnd          sdk.Date                                    `nexa:"index,string=Expiry Date"`
	RenewalNotice    sdk.Integer                                 `nexa:"string=Renewal Notice (days),default=60"`
	AutoRenew        sdk.Boolean                                 `nexa:"string=Auto Renew,default=false"`
	ValueCeiling     sdk.Numeric                                 `nexa:"string=Value Ceiling,precision=18,scale=2,default=0"`
	ValueCommitted   sdk.Numeric                                 `nexa:"string=Committed Value,precision=18,scale=2,default=0"`
	ValueInvoiced    sdk.Numeric                                 `nexa:"string=Invoiced Value,precision=18,scale=2,default=0"`
	ValueCredit      sdk.Numeric                                 `nexa:"string=Credited Value,precision=18,scale=2,default=0"`
	ValueUsed        sdk.Numeric                                 `nexa:"string=Used Value,precision=18,scale=2,default=0"`
	MinCommitment    sdk.Numeric                                 `nexa:"string=Minimum Commitment,precision=18,scale=2,default=0"`
	MaxQuantity      sdk.Numeric                                 `nexa:"string=Max Quantity,precision=18,scale=3,default=0"`
	PaymentTerms     sdk.Text                                    `nexa:"string=Payment Terms"`
	Warranty         sdk.Text                                    `nexa:"string=Warranty"`
	Penalties        sdk.Text                                    `nexa:"string=Penalties"`
	SlaText          sdk.Text                                    `nexa:"string=Service Levels"`
	State            sdk.Selection[ContractState]                `nexa:"index,string=Status,default=draft"`
	Version          sdk.Integer                                 `nexa:"string=Version,default=1"`
	ApprovalRef      sdk.Many2One[sdk.Any]                       `nexa:"string=Approval,comodel=approval.request"`
	ParentContractID sdk.Many2One[PurchaseContract]              `nexa:"string=Renewed From"`
	PriceLines       sdk.One2Many[PurchaseContractPriceSchedule] `nexa:"string=Price Schedules"`
	Obligations      sdk.One2Many[PurchaseContractObligation]    `nexa:"string=Obligations"`
	Consumptions     sdk.One2Many[PurchaseContractConsumption]   `nexa:"string=Consumption"`
	Versions         sdk.One2Many[PurchaseContractVersion]       `nexa:"string=Amendment History"`
	Notes            sdk.Text                                    `nexa:"string=Notes"`
	History          sdk.Text                                    `nexa:"string=History"`
	Active           sdk.Boolean                                 `nexa:"string=Active,default=true"`
}

// PurchaseContractVersion is an append-only amendment snapshot: superseded terms
// are never overwritten, so the exact wording governed at any point is retained.
type PurchaseContractVersion struct {
	sdk.Model `nexa:"model=purchase.contract.version"`

	ContractID    sdk.Many2One[PurchaseContract] `nexa:"required,index,string=Contract"`
	Version       sdk.Integer                    `nexa:"string=Version,default=1"`
	Title         sdk.String                     `nexa:"size=255,string=Title"`
	Snapshot      sdk.Text                       `nexa:"string=Snapshot"`
	ChangeSummary sdk.Text                       `nexa:"string=Change Summary"`
	ChangedBy     sdk.Many2One[CoreUser]         `nexa:"string=Changed By"`
	ChangedOn     sdk.DateTime                   `nexa:"string=Changed On"`
	Note          sdk.Text                       `nexa:"string=Note"`
}

// PurchaseContractPriceSchedule holds contracted unit rates with quantity tiers
// and optional indexation, used to validate PO/invoice prices server-side.
type PurchaseContractPriceSchedule struct {
	sdk.Model `nexa:"model=purchase.contract.price.schedule"`

	ContractID    sdk.Many2One[PurchaseContract] `nexa:"required,index,string=Contract"`
	ProductID     sdk.Many2One[sdk.Any]          `nexa:"string=Product,comodel=product.product"`
	Category      sdk.String                     `nexa:"size=120,string=Category"`
	Uom           sdk.String                     `nexa:"size=30,string=Unit"`
	QuantityFrom  sdk.Numeric                    `nexa:"string=Qty From,precision=18,scale=3,default=0"`
	QuantityTo    sdk.Numeric                    `nexa:"string=Qty To,precision=18,scale=3,default=0"`
	UnitPrice     sdk.Numeric                    `nexa:"string=Unit Price,precision=18,scale=2,default=0"`
	Currency      sdk.String                     `nexa:"size=3,string=Currency,default=USD"`
	IndexationPct sdk.Numeric                    `nexa:"string=Indexation %,precision=6,scale=2,default=0"`
	ValidFrom     sdk.Date                       `nexa:"string=Valid From"`
	ValidTo       sdk.Date                       `nexa:"string=Valid To"`
	Note          sdk.Text                       `nexa:"string=Note"`
}

// PurchaseContractObligation records SLA/monitoring commitments to be reviewed.
type PurchaseContractObligation struct {
	sdk.Model `nexa:"model=purchase.contract.obligation"`

	ContractID      sdk.Many2One[PurchaseContract] `nexa:"required,index,string=Contract"`
	Name            sdk.String                     `nexa:"required,size=120,string=Obligation"`
	ObligationType  sdk.Selection[ObligationType]  `nexa:"string=Type,default=sla"`
	Description     sdk.Text                       `nexa:"string=Description"`
	Target          sdk.String                     `nexa:"size=120,string=Target"`
	Frequency       sdk.String                     `nexa:"size=40,string=Frequency"`
	DueDate         sdk.Date                       `nexa:"string=Next Due"`
	MonitoringState sdk.Selection[MonitoringState] `nexa:"string=Status,default=pending"`
	LastReviewed    sdk.Date                       `nexa:"string=Last Reviewed"`
	Note            sdk.Text                       `nexa:"string=Note"`
}

// PurchaseContractConsumption is the append-only drawdown ledger against the
// value ceiling. Committed/invoiced/credited movements accumulate; nothing is
// overwritten. Ceiling enforcement blocks or warns without silently approving.
type PurchaseContractConsumption struct {
	sdk.Model `nexa:"model=purchase.contract.consumption"`

	ContractID sdk.Many2One[PurchaseContract] `nexa:"required,index,string=Contract"`
	Kind       sdk.Selection[ConsumptionKind] `nexa:"required,index,string=Movement,default=commit"`
	OrderRef   sdk.Many2One[sdk.Any]          `nexa:"string=Purchase Order,comodel=purchase.order"`
	ProductID  sdk.Many2One[sdk.Any]          `nexa:"string=Product,comodel=product.product"`
	Reference  sdk.String                     `nexa:"size=60,string=Source Ref"`
	Date       sdk.DateTime                   `nexa:"string=Date"`
	Quantity   sdk.Numeric                    `nexa:"string=Quantity,precision=18,scale=3,default=0"`
	UnitPrice  sdk.Numeric                    `nexa:"string=Unit Price,precision=18,scale=2,default=0"`
	Amount     sdk.Numeric                    `nexa:"string=Amount,precision=18,scale=2,default=0"`
	Note       sdk.Text                       `nexa:"string=Note"`
	Active     sdk.Boolean                    `nexa:"string=Active,default=true"`
}

func init() {
	sdk.MustRegister("purchase_contract",
		&PurchaseContract{},
		&PurchaseContractVersion{},
		&PurchaseContractPriceSchedule{},
		&PurchaseContractObligation{},
		&PurchaseContractConsumption{},
	)
}
