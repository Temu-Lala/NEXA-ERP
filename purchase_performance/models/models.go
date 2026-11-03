package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	purchasemodels "nexa_addons/purchase/models"
)

type CoreUser = basemodels.CoreUser

// PurchaseOrder is owned by the core purchase module; scorecards reference the
// transactional records (PO / receipt / invoice) read-only to compute evidence.
type (
	PurchaseOrder = purchasemodels.PurchaseOrder
)

// Supplier performance, risk and corrective actions (blueprint §13, §15).
// Scorecards are EVIDENCE-based: each KPI is computed server-side from the PO,
// receipt and invoice transactional records, with the period, denominator and
// exclusions documented per KPI so the same rule is applied consistently across
// suppliers. A single low score never auto-blocks a supplier - it raises a
// review that a human must authorize.

type PeriodType string

const (
	PeriodMonthly   PeriodType = "monthly"
	PeriodQuarterly PeriodType = "quarterly"
	PeriodAnnual    PeriodType = "annual"
)

type PeriodState string

const (
	PStateDraft     PeriodState = "draft"
	PStateComputing PeriodState = "computing"
	PStateComputed  PeriodState = "computed"
	PStateLocked    PeriodState = "locked"
	PStateCancelled PeriodState = "cancelled"
)

type KpiKind string

const (
	KpiOnTime         KpiKind = "on_time"
	KpiFillRate       KpiKind = "fill_rate"
	KpiAcceptance     KpiKind = "acceptance_rate"
	KpiPriceVariance  KpiKind = "price_variance"
	KpiLeadTime       KpiKind = "lead_time_reliability"
	KpiInvoiceAcc     KpiKind = "invoice_accuracy"
	KpiDefect         KpiKind = "defect_rate"
	KpiResponsiveness KpiKind = "responsiveness"
	KpiContractComp   KpiKind = "contract_compliance"
	KpiCorrective     KpiKind = "corrective_closure"
)

type KpiDirection string

const (
	DirHigher KpiDirection = "higher_better"
	DirLower  KpiDirection = "lower_better"
)

type Dimension string

const (
	DimQuality    Dimension = "quality"
	DimDelivery   Dimension = "delivery"
	DimCost       Dimension = "cost"
	DimService    Dimension = "service"
	DimCompliance Dimension = "compliance"
)

type Grade string

const (
	GradeExcellent Grade = "excellent"
	GradeGood      Grade = "good"
	GradeAccept    Grade = "acceptable"
	GradePoor      Grade = "poor"
	GradeCritical  Grade = "critical"
	GradeNA        Grade = "not_rated"
)

type ScorecardState string

const (
	ScDraft      ScorecardState = "draft"
	ScCalculated ScorecardState = "calculated"
	ScReviewed   ScorecardState = "reviewed"
	ScLocked     ScorecardState = "locked"
)

type Escalation string

const (
	EscNone     Escalation = "none"
	EscReview   Escalation = "review"
	EscEscalate Escalation = "escalate"
)

type OnTimeTreatment string

const (
	OtStrict    OnTimeTreatment = "strict"
	OtWithGrace OnTimeTreatment = "with_grace"
)

type FillRateBasis string

const (
	FrOrdered FillRateBasis = "ordered"
	FrRequest FillRateBasis = "requested"
)

type CaKind string

const (
	CaQuality    CaKind = "quality"
	CaDelivery   CaKind = "delivery"
	CaCost       CaKind = "cost"
	CaService    CaKind = "service"
	CaCompliance CaKind = "compliance"
	CaContract   CaKind = "contract"
	CaOther      CaKind = "other"
)

type CaSeverity string

const (
	SevLow      CaSeverity = "low"
	SevMedium   CaSeverity = "medium"
	SevHigh     CaSeverity = "high"
	SevCritical CaSeverity = "critical"
)

type CaState string

const (
	CaDraft      CaState = "draft"
	CaOpen       CaState = "open"
	CaInProgress CaState = "in_progress"
	CaPendingV   CaState = "pending_verification"
	CaClosed     CaState = "closed"
	CaRejected   CaState = "rejected"
	CaCancelled  CaState = "cancelled"
)

// PurchasePerformancePeriod defines the measurement window and documents the
// denominator / exclusion / partial-delivery treatment applied to every KPI so
// results are comparable across suppliers (consistency rule, blueprint §15).
type PurchasePerformancePeriod struct {
	sdk.Model `nexa:"model=purchase.performance.period"`

	Name           sdk.String                              `nexa:"required,index,size=60,string=Reference"`
	Title          sdk.String                              `nexa:"size=255,string=Title"`
	PeriodType     sdk.Selection[PeriodType]               `nexa:"string=Period Type,default=monthly"`
	DateStart      sdk.Date                                `nexa:"required,index,string=Start Date"`
	DateEnd        sdk.Date                                `nexa:"required,index,string=End Date"`
	CompanyID      sdk.Many2One[basemodels.CoreCompany]    `nexa:"string=Company"`
	OnTimeTreat    sdk.Selection[OnTimeTreatment]          `nexa:"string=On-Time Treatment,default=strict"`
	FillBasis      sdk.Selection[FillRateBasis]            `nexa:"string=Fill Rate Basis,default=ordered"`
	IncludePartial sdk.Boolean                             `nexa:"string=Count Partial Deliveries,default=true"`
	Exclusions     sdk.Text                                `nexa:"string=Exclusions"`
	Methodology    sdk.Text                                `nexa:"string=KPI Definitions"`
	Scorecards     sdk.One2Many[PurchaseSupplierScorecard] `nexa:"string=Scorecards"`
	State          sdk.Selection[PeriodState]              `nexa:"index,string=Status,default=draft"`
	Notes          sdk.Text                                `nexa:"string=Notes"`
	History        sdk.Text                                `nexa:"string=History"`
	Active         sdk.Boolean                             `nexa:"string=Active,default=true"`
}

// PurchaseSupplierScorecard is the weighted composite evaluation of a supplier
// for one period, broken into quality/delivery/cost/service/compliance
// dimensions with configurable weights by category.
type PurchaseSupplierScorecard struct {
	sdk.Model `nexa:"model=purchase.supplier.scorecard"`

	Name             sdk.String                              `nexa:"required,index,size=60,string=Reference"`
	PeriodID         sdk.Many2One[PurchasePerformancePeriod] `nexa:"required,index,string=Period"`
	SupplierID       sdk.Many2One[sdk.Any]                   `nexa:"required,index,string=Supplier,comodel=purchase.supplier"`
	CompanyID        sdk.Many2One[basemodels.CoreCompany]    `nexa:"string=Company"`
	Category         sdk.String                              `nexa:"size=120,string=Category"`
	WeightQuality    sdk.Numeric                             `nexa:"string=Weight Quality,precision=18,scale=2,default=30"`
	WeightDelivery   sdk.Numeric                             `nexa:"string=Weight Delivery,precision=18,scale=2,default=30"`
	WeightCost       sdk.Numeric                             `nexa:"string=Weight Cost,precision=18,scale=2,default=20"`
	WeightService    sdk.Numeric                             `nexa:"string=Weight Service,precision=18,scale=2,default=10"`
	WeightCompliance sdk.Numeric                             `nexa:"string=Weight Compliance,precision=18,scale=2,default=10"`
	QualityScore     sdk.Numeric                             `nexa:"string=Quality,precision=18,scale=2,default=0"`
	DeliveryScore    sdk.Numeric                             `nexa:"string=Delivery,precision=18,scale=2,default=0"`
	CostScore        sdk.Numeric                             `nexa:"string=Cost,precision=18,scale=2,default=0"`
	ServiceScore     sdk.Numeric                             `nexa:"string=Service,precision=18,scale=2,default=0"`
	ComplianceScore  sdk.Numeric                             `nexa:"string=Compliance,precision=18,scale=2,default=0"`
	CompositeScore   sdk.Numeric                             `nexa:"string=Composite,precision=18,scale=2,default=0"`
	Grade            sdk.Selection[Grade]                    `nexa:"string=Grade,default=not_rated"`
	Escalation       sdk.Selection[Escalation]               `nexa:"index,string=Escalation,default=none"`
	Metrics          sdk.One2Many[PurchasePerformanceMetric] `nexa:"string=Metrics"`
	RiskRef          sdk.Many2One[sdk.Any]                   `nexa:"string=Risk Assessment,comodel=purchase.supplier.risk.assessment"`
	EvaluatedBy      sdk.Many2One[CoreUser]                  `nexa:"string=Evaluated By"`
	EvaluatedOn      sdk.DateTime                            `nexa:"string=Evaluated On"`
	State            sdk.Selection[ScorecardState]           `nexa:"index,string=Status,default=draft"`
	Notes            sdk.Text                                `nexa:"string=Notes"`
	History          sdk.Text                                `nexa:"string=History"`
	Active           sdk.Boolean                             `nexa:"string=Active,default=true"`
}

// PurchasePerformanceMetric is a single evidence-based KPI measurement tied to a
// scorecard. Keeping the numerator/denominator documents exactly how the score
// was derived so results are auditable and comparable.
type PurchasePerformanceMetric struct {
	sdk.Model `nexa:"model=purchase.performance.metric"`

	Name        sdk.String                              `nexa:"required,index,size=60,string=Reference"`
	ScorecardID sdk.Many2One[PurchaseSupplierScorecard] `nexa:"required,index,string=Scorecard"`
	PeriodID    sdk.Many2One[PurchasePerformancePeriod] `nexa:"index,string=Period"`
	SupplierID  sdk.Many2One[sdk.Any]                   `nexa:"index,string=Supplier,comodel=purchase.supplier"`
	Kpi         sdk.Selection[KpiKind]                  `nexa:"required,index,string=KPI"`
	Dimension   sdk.Selection[Dimension]                `nexa:"string=Dimension,default=quality"`
	Direction   sdk.Selection[KpiDirection]             `nexa:"string=Direction,default=higher_better"`
	Numerator   sdk.Numeric                             `nexa:"string=Numerator,precision=18,scale=3,default=0"`
	Denominator sdk.Numeric                             `nexa:"string=Denominator,precision=18,scale=3,default=0"`
	Value       sdk.Numeric                             `nexa:"string=Raw Value,precision=18,scale=2,default=0"`
	Score       sdk.Numeric                             `nexa:"string=Score,precision=18,scale=2,default=0"`
	HasData     sdk.Boolean                             `nexa:"string=Has Data,default=false"`
	Detail      sdk.Text                                `nexa:"string=Evidence"`
	Date        sdk.DateTime                            `nexa:"string=Date"`
}

// PurchaseCorrectiveAction tracks the plan, owner, due date and closure of a
// corrective/preventive action raised from a low score, discrepancy or claim.
// Closure requires independent verification (verifier != responsible).
type PurchaseCorrectiveAction struct {
	sdk.Model `nexa:"model=purchase.corrective.action"`

	Name            sdk.String                              `nexa:"required,index,size=60,string=Reference"`
	Title           sdk.String                              `nexa:"required,size=255,string=Title"`
	SupplierID      sdk.Many2One[sdk.Any]                   `nexa:"required,index,string=Supplier,comodel=purchase.supplier"`
	ScorecardID     sdk.Many2One[PurchaseSupplierScorecard] `nexa:"index,string=Source Scorecard"`
	Kind            sdk.Selection[CaKind]                   `nexa:"string=Category,default=quality"`
	Severity        sdk.Selection[CaSeverity]               `nexa:"index,string=Severity,default=medium"`
	RootCause       sdk.Text                                `nexa:"string=Root Cause"`
	Containment     sdk.Text                                `nexa:"string=Containment"`
	CorrectivePlan  sdk.Text                                `nexa:"string=Corrective Plan"`
	ResponsibleID   sdk.Many2One[CoreUser]                  `nexa:"string=Responsible"`
	DateRaised      sdk.Date                                `nexa:"index,string=Raised,default=today"`
	DueDate         sdk.Date                                `nexa:"index,string=Due Date"`
	ClosureDate     sdk.Date                                `nexa:"string=Closure Date"`
	ClosureEvidence sdk.Text                                `nexa:"string=Closure Evidence"`
	VerifiedBy      sdk.Many2One[CoreUser]                  `nexa:"string=Verified By"`
	State           sdk.Selection[CaState]                  `nexa:"index,string=Status,default=draft"`
	Notes           sdk.Text                                `nexa:"string=Notes"`
	History         sdk.Text                                `nexa:"string=History"`
	Active          sdk.Boolean                             `nexa:"string=Active,default=true"`
}

func init() {
	sdk.MustRegister("purchase_performance",
		&PurchasePerformancePeriod{},
		&PurchaseSupplierScorecard{},
		&PurchasePerformanceMetric{},
		&PurchaseCorrectiveAction{},
	)
}
