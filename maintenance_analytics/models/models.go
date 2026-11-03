package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	maintmodels "nexa_addons/maintenance/models"
)

// Cross-module reference aliases.
type (
	MaintenanceAsset     = maintmodels.MaintenanceAsset
	MaintenanceWorkOrder = maintmodels.MaintenanceWorkOrder
	CoreUser             = basemodels.CoreUser
)

// ---- Selection types -------------------------------------------------------

type MetricPeriod string

const (
	MetricPeriodDaily     MetricPeriod = "daily"
	MetricPeriodWeekly    MetricPeriod = "weekly"
	MetricPeriodMonthly   MetricPeriod = "monthly"
	MetricPeriodQuarterly MetricPeriod = "quarterly"
	MetricPeriodAnnual    MetricPeriod = "annual"
)

type RCAMethod string

const (
	RCAMethodFiveWhy  RCAMethod = "five_why"
	RCAMethodFishbone RCAMethod = "fishbone"
	RCAMethodFMEA     RCAMethod = "fmea"
	RCAMethodRCM      RCAMethod = "rcm"
	RCAMethodPareto   RCAMethod = "pareto"
	RCAMethodOther    RCAMethod = "other"
)

type RCAState string

const (
	RCAStateOpen           RCAState = "open"
	RCAStateInProgress     RCAState = "in_progress"
	RCAStateFindingsReady  RCAState = "findings_ready"
	RCAStateActionRequired RCAState = "action_required"
	RCAStateVerifiedClosed RCAState = "verified_closed"
)

type CAState string

const (
	CAStateOpen       CAState = "open"
	CAStateInProgress CAState = "in_progress"
	CAStateCompleted  CAState = "completed"
	CAStateVerified   CAState = "verified"
	CAStateOverdue    CAState = "overdue"
)

type WOHistoryEvent string

const (
	WOEventCreated      WOHistoryEvent = "created"
	WOEventAssigned     WOHistoryEvent = "assigned"
	WOEventStarted      WOHistoryEvent = "started"
	WOEventPaused       WOHistoryEvent = "paused"
	WOEventResumed      WOHistoryEvent = "resumed"
	WOEventCompleted    WOHistoryEvent = "completed"
	WOEventVerified     WOHistoryEvent = "verified"
	WOEventClosed       WOHistoryEvent = "closed"
	WOEventCancelled    WOHistoryEvent = "cancelled"
	WOEventReopened     WOHistoryEvent = "reopened"
	WOEventOnHold       WOHistoryEvent = "on_hold"
	WOEventPartsWaiting WOHistoryEvent = "parts_waiting"
)

// ---- Models ----------------------------------------------------------------

// MaintWOHistory is an append-only audit trail of every state change on a work order.
type MaintWOHistory struct {
	sdk.Model `nexa:"model=maintenance.wo.history"`

	WorkOrderID sdk.Many2One[MaintenanceWorkOrder] `nexa:"required,index,string=Work Order"`
	Event       sdk.Selection[WOHistoryEvent]      `nexa:"required,string=Event,default=created"`
	FromState   sdk.String                         `nexa:"string=From State"`
	ToState     sdk.String                         `nexa:"string=To State"`
	ChangedBy   sdk.Many2One[CoreUser]             `nexa:"required,string=Changed By"`
	ChangedAt   sdk.DateTime                       `nexa:"required,string=Changed At"`
	Note        sdk.Text                           `nexa:"string=Note"`
}

// MaintReliabilityMetric stores computed MTBF, MTTR and availability per asset per period.
type MaintReliabilityMetric struct {
	sdk.Model `nexa:"model=maintenance.reliability.metric"`

	AssetID               sdk.Many2One[MaintenanceAsset] `nexa:"required,index,string=Asset"`
	MetricPeriod          sdk.Selection[MetricPeriod]    `nexa:"required,string=Period,default=monthly"`
	PeriodStart           sdk.Date                       `nexa:"required,string=Period Start"`
	PeriodEnd             sdk.Date                       `nexa:"required,string=Period End"`
	TotalOperatingHours   sdk.Numeric                    `nexa:"string=Total Operating Hours,precision=18,scale=2,default=0"`
	FailureCount          sdk.Integer                    `nexa:"string=Failure Count,default=0"`
	MTBF                  sdk.Numeric                    `nexa:"string=MTBF (hrs),precision=18,scale=2,default=0"`
	TotalRepairHours      sdk.Numeric                    `nexa:"string=Total Repair Hours,precision=18,scale=2,default=0"`
	MTTR                  sdk.Numeric                    `nexa:"string=MTTR (hrs),precision=18,scale=2,default=0"`
	Availability          sdk.Numeric                    `nexa:"string=Availability,precision=18,scale=4,default=0"`
	PlannedDowntimeHours  sdk.Numeric                    `nexa:"string=Planned Downtime (hrs),precision=18,scale=2,default=0"`
	UnplannedDowntimeHours sdk.Numeric                   `nexa:"string=Unplanned Downtime (hrs),precision=18,scale=2,default=0"`
	PMCompliancePct       sdk.Numeric                    `nexa:"string=PM Compliance (%),precision=18,scale=2,default=0"`
	BacklogHours          sdk.Numeric                    `nexa:"string=Backlog Hours,precision=18,scale=2,default=0"`
	EmergencyWOCount      sdk.Integer                    `nexa:"string=Emergency WO Count,default=0"`
	RepeatFailureCount    sdk.Integer                    `nexa:"string=Repeat Failure Count,default=0"`
	TotalMaintenanceCost  sdk.Numeric                    `nexa:"string=Total Maintenance Cost,precision=18,scale=2,default=0"`
	ComputedAt            sdk.DateTime                   `nexa:"string=Computed At"`
	Notes                 sdk.Text                       `nexa:"string=Notes"`
}

// MaintRootCauseAnalysis records an RCA investigation with findings and corrective actions.
type MaintRootCauseAnalysis struct {
	sdk.Model `nexa:"model=maintenance.rca"`

	Name                  sdk.String                         `nexa:"required,index,string=Name"`
	WorkOrderID           sdk.Many2One[MaintenanceWorkOrder] `nexa:"index,string=Work Order"`
	AssetID               sdk.Many2One[MaintenanceAsset]     `nexa:"index,string=Asset"`
	State                 sdk.Selection[RCAState]            `nexa:"required,string=State,default=open"`
	Method                sdk.Selection[RCAMethod]           `nexa:"required,string=Method,default=five_why"`
	FailureSummary        sdk.Text                           `nexa:"required,string=Failure Summary"`
	InvestigatedBy        sdk.Many2One[CoreUser]             `nexa:"required,string=Investigated By"`
	StartDate             sdk.Date                           `nexa:"string=Start Date"`
	FindingsDate          sdk.Date                           `nexa:"string=Findings Date"`
	RootCauseSummary      sdk.Text                           `nexa:"string=Root Cause Summary"`
	ContributingFactors   sdk.Text                           `nexa:"string=Contributing Factors"`
	WhyLevel1             sdk.Text                           `nexa:"string=Why 1"`
	WhyLevel2             sdk.Text                           `nexa:"string=Why 2"`
	WhyLevel3             sdk.Text                           `nexa:"string=Why 3"`
	WhyLevel4             sdk.Text                           `nexa:"string=Why 4"`
	WhyLevel5             sdk.Text                           `nexa:"string=Why 5"`
	ImmediateCorrection   sdk.Text                           `nexa:"string=Immediate Correction"`
	Notes                 sdk.Text                           `nexa:"string=Notes"`
	CorrectiveActions     sdk.One2Many[MaintCorrectiveAction] `nexa:"string=Corrective Actions"`
	LinkedWorkOrders      sdk.One2Many[MaintRCAWorkOrderLink] `nexa:"string=Linked Work Orders"`
}

// MaintCorrectiveAction is a corrective action item linked to an RCA.
type MaintCorrectiveAction struct {
	sdk.Model `nexa:"model=maintenance.corrective.action"`

	RCAID         sdk.Many2One[MaintRootCauseAnalysis] `nexa:"required,index,string=RCA"`
	Sequence      sdk.Integer                          `nexa:"string=Sequence,default=10"`
	Description   sdk.String                           `nexa:"required,string=Description"`
	State         sdk.Selection[CAState]               `nexa:"required,string=State,default=open"`
	AssignedTo    sdk.Many2One[CoreUser]               `nexa:"required,string=Assigned To"`
	DueDate       sdk.Date                             `nexa:"string=Due Date"`
	CompletedDate sdk.Date                             `nexa:"string=Completed Date"`
	VerifiedBy    sdk.Many2One[CoreUser]               `nexa:"string=Verified By"`
	VerifiedDate  sdk.Date                             `nexa:"string=Verified Date"`
	Evidence      sdk.Text                             `nexa:"string=Evidence"`
	Notes         sdk.Text                             `nexa:"string=Notes"`
}

// MaintRCAWorkOrderLink links an RCA to related work orders.
type MaintRCAWorkOrderLink struct {
	sdk.Model `nexa:"model=maintenance.rca.wo.link"`

	RCAID        sdk.Many2One[MaintRootCauseAnalysis] `nexa:"required,index,string=RCA"`
	WorkOrderID  sdk.Many2One[MaintenanceWorkOrder]   `nexa:"required,index,string=Work Order"`
	Relationship sdk.String                           `nexa:"string=Relationship"`
	Notes        sdk.Text                             `nexa:"string=Notes"`
}

// MaintCostSummary aggregates maintenance cost data for an asset over a period.
type MaintCostSummary struct {
	sdk.Model `nexa:"model=maintenance.cost.summary"`

	AssetID          sdk.Many2One[MaintenanceAsset] `nexa:"required,index,string=Asset"`
	PeriodStart      sdk.Date                       `nexa:"required,string=Period Start"`
	PeriodEnd        sdk.Date                       `nexa:"required,string=Period End"`
	MetricPeriod     sdk.Selection[MetricPeriod]    `nexa:"required,string=Period Type,default=monthly"`
	LaborCost        sdk.Numeric                    `nexa:"string=Labor Cost,precision=18,scale=2,default=0"`
	PartsCost        sdk.Numeric                    `nexa:"string=Parts Cost,precision=18,scale=2,default=0"`
	ContractorCost   sdk.Numeric                    `nexa:"string=Contractor Cost,precision=18,scale=2,default=0"`
	OtherCost        sdk.Numeric                    `nexa:"string=Other Cost,precision=18,scale=2,default=0"`
	TotalCost        sdk.Numeric                    `nexa:"string=Total Cost,precision=18,scale=2,default=0"`
	CorrectiveCost   sdk.Numeric                    `nexa:"string=Corrective Cost,precision=18,scale=2,default=0"`
	PreventiveCost   sdk.Numeric                    `nexa:"string=Preventive Cost,precision=18,scale=2,default=0"`
	EmergencyCost    sdk.Numeric                    `nexa:"string=Emergency Cost,precision=18,scale=2,default=0"`
	WarrantyRecovery sdk.Numeric                    `nexa:"string=Warranty Recovery,precision=18,scale=2,default=0"`
	WorkOrderCount   sdk.Integer                    `nexa:"string=Work Order Count,default=0"`
	ComputedAt       sdk.DateTime                   `nexa:"string=Computed At"`
	Notes            sdk.Text                       `nexa:"string=Notes"`
}

func init() {
	sdk.MustRegister("maintenance_analytics",
		&MaintWOHistory{},
		&MaintReliabilityMetric{},
		&MaintRootCauseAnalysis{},
		&MaintCorrectiveAction{},
		&MaintRCAWorkOrderLink{},
		&MaintCostSummary{},
	)
}
