package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	maintmodels "nexa_addons/maintenance/models"
)

// Cross-module reference aliases.
type (
	CoreUser             = basemodels.CoreUser
	MaintenanceAsset     = maintmodels.MaintenanceAsset
	MaintenanceWorkOrder = maintmodels.MaintenanceWorkOrder
)

// ---- Selection types -------------------------------------------------------

type HazardSeverity string

const (
	HazardSeverityLow      HazardSeverity = "low"
	HazardSeverityMedium   HazardSeverity = "medium"
	HazardSeverityHigh     HazardSeverity = "high"
	HazardSeverityCritical HazardSeverity = "critical"
)

type HazardLikelihood string

const (
	HazardLikelihoodRare         HazardLikelihood = "rare"
	HazardLikelihoodUnlikely     HazardLikelihood = "unlikely"
	HazardLikelihoodPossible     HazardLikelihood = "possible"
	HazardLikelihoodLikely       HazardLikelihood = "likely"
	HazardLikelihoodAlmostCertain HazardLikelihood = "almost_certain"
)

type ControlMeasure string

const (
	ControlMeasureElimination    ControlMeasure = "elimination"
	ControlMeasureSubstitution   ControlMeasure = "substitution"
	ControlMeasureEngineering    ControlMeasure = "engineering"
	ControlMeasureAdministrative ControlMeasure = "administrative"
	ControlMeasurePPE            ControlMeasure = "ppe"
)

type PermitType string

const (
	PermitTypeGeneral       PermitType = "general"
	PermitTypeElectrical    PermitType = "electrical"
	PermitTypeMechanical    PermitType = "mechanical"
	PermitTypeConfinedSpace PermitType = "confined_space"
	PermitTypeHotWork       PermitType = "hot_work"
	PermitTypeHeight        PermitType = "height"
	PermitTypeChemical      PermitType = "chemical"
	PermitTypeColdWork      PermitType = "cold_work"
)

type PermitState string

const (
	PermitStateDraft        PermitState = "draft"
	PermitStateRequested    PermitState = "requested"
	PermitStateUnderReview  PermitState = "under_review"
	PermitStateApproved     PermitState = "approved"
	PermitStateActive       PermitState = "active"
	PermitStateWorkComplete PermitState = "work_complete"
	PermitStateClosed       PermitState = "closed"
	PermitStateRejected     PermitState = "rejected"
	PermitStateCancelled    PermitState = "cancelled"
)

type LOTOState string

const (
	LOTOStateDraft     LOTOState = "draft"
	LOTOStateInitiated LOTOState = "initiated"
	LOTOStateVerified  LOTOState = "verified"
	LOTOStateCleared   LOTOState = "cleared"
	LOTOStateReleased  LOTOState = "released"
)

type IncidentSeverity string

const (
	IncidentSeverityNearMiss         IncidentSeverity = "near_miss"
	IncidentSeverityFirstAid         IncidentSeverity = "first_aid"
	IncidentSeverityMedicalTreatment IncidentSeverity = "medical_treatment"
	IncidentSeverityLostTime         IncidentSeverity = "lost_time"
	IncidentSeverityFatality         IncidentSeverity = "fatality"
	IncidentSeverityPropertyDamage   IncidentSeverity = "property_damage"
	IncidentSeverityEnvironmental    IncidentSeverity = "environmental"
)

type IncidentState string

const (
	IncidentStateReported          IncidentState = "reported"
	IncidentStateUnderInvestigation IncidentState = "under_investigation"
	IncidentStateActionRequired    IncidentState = "action_required"
	IncidentStateClosed            IncidentState = "closed"
)

// ---- Models ----------------------------------------------------------------

// MaintRiskAssessment captures job risk assessments with hazard identification.
type MaintRiskAssessment struct {
	sdk.Model `nexa:"model=maintenance.risk.assessment"`

	Name         sdk.String                    `nexa:"required,index,string=Reference"`
	WorkOrderID  sdk.Many2One[MaintenanceWorkOrder] `nexa:"index,string=Work Order"`
	AssetID      sdk.Many2One[MaintenanceAsset]    `nexa:"index,string=Asset"`
	AssessedBy   sdk.Many2One[CoreUser]            `nexa:"required,string=Assessed By"`
	AssessedDate sdk.Date                          `nexa:"required,string=Assessed Date"`
	ReviewedBy   sdk.Many2One[CoreUser]            `nexa:"string=Reviewed By"`
	ReviewedDate sdk.Date                          `nexa:"string=Reviewed Date"`
	ApprovedBy   sdk.Many2One[CoreUser]            `nexa:"string=Approved By"`
	ApprovedDate sdk.Date                          `nexa:"string=Approved Date"`
	OverallRisk  sdk.Selection[HazardSeverity]     `nexa:"string=Overall Risk,default=low"`
	RequiresPTW  sdk.Boolean                       `nexa:"string=Requires Permit-to-Work,default=false"`
	RequiresLOTO sdk.Boolean                       `nexa:"string=Requires LOTO,default=false"`
	PPERequired  sdk.Text                          `nexa:"string=PPE Required"`
	EmergencyProcedure sdk.Text                   `nexa:"string=Emergency Procedure"`
	Notes        sdk.Text                          `nexa:"string=Notes"`
	Active       sdk.Boolean                       `nexa:"string=Active,default=true"`
	Hazards      sdk.One2Many[MaintRiskHazard]     `nexa:"string=Hazards"`
}

// MaintRiskHazard is a single hazard line within a risk assessment.
type MaintRiskHazard struct {
	sdk.Model `nexa:"model=maintenance.risk.hazard"`

	AssessmentID       sdk.Many2One[MaintRiskAssessment] `nexa:"required,index,string=Assessment"`
	Sequence           sdk.Integer                       `nexa:"string=Sequence,default=10"`
	HazardDescription  sdk.String                        `nexa:"required,string=Hazard Description"`
	Severity           sdk.Selection[HazardSeverity]     `nexa:"required,string=Severity,default=medium"`
	Likelihood         sdk.Selection[HazardLikelihood]   `nexa:"required,string=Likelihood,default=possible"`
	RiskScore          sdk.Integer                       `nexa:"string=Risk Score,default=0"`
	ControlMeasure     sdk.Selection[ControlMeasure]     `nexa:"required,string=Control Measure,default=administrative"`
	ControlDescription sdk.Text                          `nexa:"required,string=Control Description"`
	ResidualSeverity   sdk.Selection[HazardSeverity]     `nexa:"string=Residual Severity,default=low"`
	ResidualLikelihood sdk.Selection[HazardLikelihood]   `nexa:"string=Residual Likelihood,default=rare"`
	ResidualRiskScore  sdk.Integer                       `nexa:"string=Residual Risk Score,default=0"`
}

// MaintPermit manages permit-to-work workflows with approval and isolation records.
type MaintPermit struct {
	sdk.Model `nexa:"model=maintenance.permit"`

	Name                sdk.String                        `nexa:"required,index,string=Permit Number"`
	PermitType          sdk.Selection[PermitType]         `nexa:"required,string=Permit Type,default=general"`
	WorkOrderID         sdk.Many2One[MaintenanceWorkOrder] `nexa:"index,string=Work Order"`
	AssetID             sdk.Many2One[MaintenanceAsset]    `nexa:"index,string=Asset"`
	State               sdk.Selection[PermitState]        `nexa:"required,string=State,default=draft"`
	RequestedBy         sdk.Many2One[CoreUser]            `nexa:"required,string=Requested By"`
	RequestedDate       sdk.DateTime                      `nexa:"string=Requested Date"`
	ValidFrom           sdk.DateTime                      `nexa:"string=Valid From"`
	ValidTo             sdk.DateTime                      `nexa:"string=Valid To"`
	WorkLocation        sdk.Text                          `nexa:"string=Work Location"`
	WorkDescription     sdk.Text                          `nexa:"string=Work Description"`
	HazardsIdentified   sdk.Text                          `nexa:"string=Hazards Identified"`
	PrecautionsTaken    sdk.Text                          `nexa:"string=Precautions Taken"`
	PPERequired         sdk.Text                          `nexa:"string=PPE Required"`
	IsolationsRequired  sdk.Boolean                       `nexa:"string=Isolations Required,default=false"`
	EmergencyContact    sdk.String                        `nexa:"string=Emergency Contact"`
	IssuedBy            sdk.Many2One[CoreUser]            `nexa:"string=Issued By"`
	IssuedAt            sdk.DateTime                      `nexa:"string=Issued At"`
	ApprovedBy          sdk.Many2One[CoreUser]            `nexa:"string=Approved By"`
	ApprovedAt          sdk.DateTime                      `nexa:"string=Approved At"`
	ClosedBy            sdk.Many2One[CoreUser]            `nexa:"string=Closed By"`
	ClosedAt            sdk.DateTime                      `nexa:"string=Closed At"`
	AreaClearedBy       sdk.Many2One[CoreUser]            `nexa:"string=Area Cleared By"`
	AreaClearedAt       sdk.DateTime                      `nexa:"string=Area Cleared At"`
	CancellationReason  sdk.Text                          `nexa:"string=Cancellation Reason"`
	Notes               sdk.Text                          `nexa:"string=Notes"`
	IsolationRecords    sdk.One2Many[MaintLOTORecord]     `nexa:"string=Isolation Records"`
}

// MaintLOTORecord maintains lockout/tagout procedure records for isolation.
type MaintLOTORecord struct {
	sdk.Model `nexa:"model=maintenance.loto.record"`

	Name           sdk.String                        `nexa:"required,index,string=Reference"`
	PermitID       sdk.Many2One[MaintPermit]         `nexa:"index,string=Permit"`
	WorkOrderID    sdk.Many2One[MaintenanceWorkOrder] `nexa:"index,string=Work Order"`
	AssetID        sdk.Many2One[MaintenanceAsset]    `nexa:"required,index,string=Asset"`
	State          sdk.Selection[LOTOState]          `nexa:"required,string=State,default=draft"`
	IsolationPoint sdk.String                        `nexa:"required,string=Isolation Point"`
	IsolationType  sdk.String                        `nexa:"string=Isolation Type"`
	LockTagRef     sdk.String                        `nexa:"string=Lock / Tag Ref"`
	LockedBy       sdk.Many2One[CoreUser]            `nexa:"required,string=Locked By"`
	LockedAt       sdk.DateTime                      `nexa:"string=Locked At"`
	VerifiedBy     sdk.Many2One[CoreUser]            `nexa:"string=Verified By"`
	VerifiedAt     sdk.DateTime                      `nexa:"string=Verified At"`
	ReleasedBy     sdk.Many2One[CoreUser]            `nexa:"string=Released By"`
	ReleasedAt     sdk.DateTime                      `nexa:"string=Released At"`
	Notes          sdk.Text                          `nexa:"string=Notes"`
}

// MaintSafetyIncident records safety incidents and near-misses linked to maintenance work.
type MaintSafetyIncident struct {
	sdk.Model `nexa:"model=maintenance.safety.incident"`

	Name               sdk.String                        `nexa:"required,index,string=Reference"`
	WorkOrderID        sdk.Many2One[MaintenanceWorkOrder] `nexa:"index,string=Work Order"`
	AssetID            sdk.Many2One[MaintenanceAsset]    `nexa:"index,string=Asset"`
	IncidentDate       sdk.DateTime                      `nexa:"required,string=Incident Date"`
	ReportedBy         sdk.Many2One[CoreUser]            `nexa:"required,string=Reported By"`
	Severity           sdk.Selection[IncidentSeverity]   `nexa:"required,string=Severity,default=near_miss"`
	State              sdk.Selection[IncidentState]      `nexa:"required,string=State,default=reported"`
	Location           sdk.Text                          `nexa:"string=Location"`
	Description        sdk.Text                          `nexa:"required,string=Description"`
	PersonsInvolved    sdk.Text                          `nexa:"string=Persons Involved"`
	ImmediateAction    sdk.Text                          `nexa:"string=Immediate Action"`
	RootCauseSummary   sdk.Text                          `nexa:"string=Root Cause Summary"`
	CorrectiveActions  sdk.Text                          `nexa:"string=Corrective Actions"`
	InvestigatedBy     sdk.Many2One[CoreUser]            `nexa:"string=Investigated By"`
	InvestigationDate  sdk.Date                          `nexa:"string=Investigation Date"`
	ClosedBy           sdk.Many2One[CoreUser]            `nexa:"string=Closed By"`
	ClosedDate         sdk.Date                          `nexa:"string=Closed Date"`
	Notes              sdk.Text                          `nexa:"string=Notes"`
}

func init() {
	sdk.MustRegister("maintenance_safety",
		&MaintRiskAssessment{},
		&MaintRiskHazard{},
		&MaintPermit{},
		&MaintLOTORecord{},
		&MaintSafetyIncident{},
	)
}
