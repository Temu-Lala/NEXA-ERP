package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	maintmodels "nexa_addons/maintenance/models"
)

// Cross-module reference aliases.
type (
	CoreUser         = basemodels.CoreUser
	MaintenanceAsset = maintmodels.MaintenanceAsset
)

// ---- Selection types -------------------------------------------------------

type PhysicalCondition string

const (
	CondExcellent PhysicalCondition = "excellent"
	CondGood      PhysicalCondition = "good"
	CondFair      PhysicalCondition = "fair"
	CondPoor      PhysicalCondition = "poor"
	CondCritical  PhysicalCondition = "critical"
)

type OperationalPerformance string

const (
	PerfOptimal      OperationalPerformance = "optimal"
	PerfAcceptable   OperationalPerformance = "acceptable"
	PerfDegraded     OperationalPerformance = "degraded"
	PerfFailureProne OperationalPerformance = "failure_prone"
)

type ReliabilityRating string

const (
	RelRatingHigh    ReliabilityRating = "high"
	RelRatingMedium  ReliabilityRating = "medium"
	RelRatingLow     ReliabilityRating = "low"
	RelRatingVeryLow ReliabilityRating = "very_low"
)

type MaintainabilityRating string

const (
	MaintEasy          MaintainabilityRating = "easy"
	MaintModerate      MaintainabilityRating = "moderate"
	MaintDifficult     MaintainabilityRating = "difficult"
	MaintObsoleteParts MaintainabilityRating = "obsolete_parts"
)

type LifecycleAction string

const (
	ActionContinueMaintain LifecycleAction = "continue_maintenance"
	ActionMinorOverhaul    LifecycleAction = "minor_overhaul"
	ActionMajorRefurbish   LifecycleAction = "major_refurbish"
	ActionReplace          LifecycleAction = "replace"
	ActionDecommission     LifecycleAction = "decommission"
)

type ProposalState string

const (
	ProposalDraft           ProposalState = "draft"
	ProposalSubmitted       ProposalState = "submitted"
	ProposalUnderEvaluation ProposalState = "under_evaluation"
	ProposalApproved        ProposalState = "approved"
	ProposalRejected        ProposalState = "rejected"
	ProposalExecuted        ProposalState = "executed"
)

type ProposalType string

const (
	PropTypeDirectReplacement ProposalType = "replacement"
	PropTypeMajorRefurbish    ProposalType = "major_refurbishment"
	PropTypeTechnologyUpgrade ProposalType = "upgrade"
	PropTypeCapacityExpansion ProposalType = "capacity_expansion"
)

type DisposalState string

const (
	DisposalDraft          DisposalState = "draft"
	DisposalApproved       DisposalState = "approved"
	DisposalDecommissioned DisposalState = "decommissioned"
	DisposalDisposed       DisposalState = "disposed"
	DisposalCancelled      DisposalState = "cancelled"
)

type DisposalMethod string

const (
	DispMethodScrap          DisposalMethod = "scrap"
	DispMethodSale           DisposalMethod = "sale"
	DispMethodAuction        DisposalMethod = "auction"
	DispMethodTradeIn        DisposalMethod = "trade_in"
	DispMethodDonate         DisposalMethod = "donate"
	DispMethodHazardousWaste DisposalMethod = "hazardous_waste"
)

// ---- Models ----------------------------------------------------------------

// MaintAssetConditionScore captures periodic multi-factor condition assessments and asset health index.
type MaintAssetConditionScore struct {
	sdk.Model `nexa:"model=maintenance.asset.condition.score"`

	AssetID                sdk.Many2One[MaintenanceAsset]        `nexa:"required,index,string=Asset"`
	AssessmentDate         sdk.Date                              `nexa:"required,string=Assessment Date"`
	AssessedBy             sdk.Many2One[CoreUser]                `nexa:"required,string=Assessor"`
	PhysicalCondition      sdk.Selection[PhysicalCondition]      `nexa:"required,string=Physical Condition,default=good"`
	OperationalPerformance sdk.Selection[OperationalPerformance] `nexa:"required,string=Performance,default=optimal"`
	ReliabilityRating      sdk.Selection[ReliabilityRating]      `nexa:"required,string=Reliability,default=high"`
	MaintainabilityRating  sdk.Selection[MaintainabilityRating]  `nexa:"required,string=Maintainability,default=easy"`
	HealthIndex            sdk.Numeric                           `nexa:"required,string=Health Index (0-100),precision=18,scale=2,default=100"`
	EstimatedRULYears      sdk.Numeric                           `nexa:"string=Estimated Remaining Life (yrs),precision=18,scale=2,default=0"`
	RecommendedAction      sdk.Selection[LifecycleAction]        `nexa:"required,string=Recommended Action,default=continue_maintenance"`
	NextAssessmentDate     sdk.Date                              `nexa:"string=Next Assessment Due"`
	Notes                  sdk.Text                              `nexa:"string=Assessment Notes"`
}

// MaintReplacementProposal manages capital replacement and major overhaul proposals.
type MaintReplacementProposal struct {
	sdk.Model `nexa:"model=maintenance.replacement.proposal"`

	Name                      sdk.String                     `nexa:"required,index,string=Proposal Number"`
	Title                     sdk.String                     `nexa:"required,string=Title"`
	AssetID                   sdk.Many2One[MaintenanceAsset] `nexa:"required,index,string=Asset"`
	State                     sdk.Selection[ProposalState]   `nexa:"required,string=Status,default=draft"`
	ProposalType              sdk.Selection[ProposalType]    `nexa:"required,string=Proposal Type,default=replacement"`
	Priority                  sdk.String                     `nexa:"string=Priority,default=medium"`
	CurrentAgeYears           sdk.Numeric                    `nexa:"string=Current Age (yrs),precision=18,scale=2,default=0"`
	EstimatedRULYears         sdk.Numeric                    `nexa:"string=Remaining Useful Life (yrs),precision=18,scale=2,default=0"`
	CumulativeMaintenanceCost sdk.Numeric                    `nexa:"string=Historical Maintenance Cost,precision=18,scale=2,default=0"`
	EstimatedAnnualMaintCost  sdk.Numeric                    `nexa:"string=Est. Future Annual Maint ($),precision=18,scale=2,default=0"`
	ReplacementCostEstimate   sdk.Numeric                    `nexa:"string=Estimated Replacement / Capex ($),precision=18,scale=2,default=0"`
	EstimatedSalvageValue     sdk.Numeric                    `nexa:"string=Estimated Salvage Value ($),precision=18,scale=2,default=0"`
	PaybackYears              sdk.Numeric                    `nexa:"string=Estimated Payback (yrs),precision=18,scale=2,default=0"`
	CapexProjectRef           sdk.String                     `nexa:"string=Capex Project Reference"`
	RequestedBy               sdk.Many2One[CoreUser]         `nexa:"required,string=Requested By"`
	RequestedDate             sdk.Date                       `nexa:"string=Requested Date"`
	ApprovedBy                sdk.Many2One[CoreUser]         `nexa:"string=Approved By"`
	ApprovedDate              sdk.Date                       `nexa:"string=Approval Date"`
	BusinessJustification     sdk.Text                       `nexa:"required,string=Business Case & Justification"`
	Notes                     sdk.Text                       `nexa:"string=Notes"`
}

// MaintAssetDisposal tracks the formal decommissioning, environmental signoff and disposal of assets.
type MaintAssetDisposal struct {
	sdk.Model `nexa:"model=maintenance.asset.disposal"`

	Name                     sdk.String                     `nexa:"required,index,string=Disposal Reference"`
	AssetID                  sdk.Many2One[MaintenanceAsset] `nexa:"required,index,string=Asset"`
	State                    sdk.Selection[DisposalState]   `nexa:"required,string=Status,default=draft"`
	DisposalMethod           sdk.Selection[DisposalMethod]  `nexa:"required,string=Disposal Method,default=scrap"`
	Reason                   sdk.Text                       `nexa:"required,string=Disposal Justification"`
	DecommissionDate         sdk.Date                       `nexa:"string=Decommission Date"`
	DisposedDate             sdk.Date                       `nexa:"string=Disposal Date"`
	EstimatedSalvageValue    sdk.Numeric                    `nexa:"string=Est. Salvage Value ($),precision=18,scale=2,default=0"`
	ActualSalvageValue       sdk.Numeric                    `nexa:"string=Actual Proceeds ($),precision=18,scale=2,default=0"`
	EnvironmentalSignoff     sdk.Boolean                    `nexa:"string=Environmental Clearance,default=false"`
	SafetyIsolationVerified  sdk.Boolean                    `nexa:"string=Safety Isolation Verified,default=false"`
	EnvironmentalSignoffBy   sdk.Many2One[CoreUser]         `nexa:"string=Environmental Signoff By"`
	AuthorizedBy             sdk.Many2One[CoreUser]         `nexa:"string=Authorized By"`
	FinalMeterReading        sdk.Numeric                    `nexa:"string=Final Meter Reading,precision=18,scale=2,default=0"`
	CustodyHandoverTo        sdk.String                     `nexa:"string=Custody Handover To"`
	DisposalCertificateRef   sdk.String                     `nexa:"string=Disposal Certificate Ref"`
	Notes                    sdk.Text                       `nexa:"string=Notes"`
}

func init() {
	sdk.MustRegister("maintenance_lifecycle",
		&MaintAssetConditionScore{},
		&MaintReplacementProposal{},
		&MaintAssetDisposal{},
	)
}
