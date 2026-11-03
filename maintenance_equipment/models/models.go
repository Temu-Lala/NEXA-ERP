package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	maintmodels "nexa_addons/maintenance/models"
)

// Cross-module reference aliases.
type (
	MaintenanceAsset    = maintmodels.MaintenanceAsset
	MaintenanceWorkOrder = maintmodels.MaintenanceWorkOrder
	CoreUser            = basemodels.CoreUser
)

type PlanState string

const (
	PlanDraft  PlanState = "draft"
	PlanActive PlanState = "active"
	PlanPaused PlanState = "paused"
	PlanDone   PlanState = "done"
)

type TriggerType string

const (
	TriggerCalendar  TriggerType = "calendar"
	TriggerHours     TriggerType = "operating_hours"
	TriggerDistance  TriggerType = "distance"
	TriggerCycles    TriggerType = "usage_count"
	TriggerCondition TriggerType = "condition"
	TriggerOnce      TriggerType = "one_time"
)

type MeterType string

const (
	MeterHours    MeterType = "hours"
	MeterDistance MeterType = "distance"
	MeterCycles   MeterType = "cycles"
	MeterOther    MeterType = "other"
)

type OccurrenceState string

const (
	OccurrencePending   OccurrenceState = "pending"
	OccurrenceGenerated OccurrenceState = "generated"
	OccurrenceSkipped   OccurrenceState = "skipped"
	OccurrenceCancelled OccurrenceState = "cancelled"
)

// MaintenanceMeter defines a usage counter (hours, mileage, cycles) on an asset.
type MaintenanceMeter struct {
	sdk.Model `nexa:"model=maintenance.meter"`

	Name         sdk.String                     `nexa:"required,string=Meter"`
	AssetID      sdk.Many2One[MaintenanceAsset] `nexa:"required,index,string=Asset"`
	MeterType    sdk.Selection[MeterType]       `nexa:"string=Type,default=hours"`
	Unit         sdk.String                     `nexa:"string=Unit"`
	CurrentValue sdk.Numeric                    `nexa:"string=Current Value,precision=18,scale=2,default=0"`
	Rollover     sdk.Boolean                    `nexa:"string=Rollover Allowed,default=false"`
	Active       sdk.Boolean                    `nexa:"string=Active,default=true"`
}

// MaintenanceMeterReading is a timestamped usage reading against a meter.
type MaintenanceMeterReading struct {
	sdk.Model `nexa:"model=maintenance.meter.reading"`

	MeterID    sdk.Many2One[MaintenanceMeter] `nexa:"required,index,string=Meter"`
	AssetID    sdk.Many2One[MaintenanceAsset] `nexa:"index,string=Asset"`
	Date       sdk.Date                       `nexa:"required,string=Date"`
	Value      sdk.Numeric                    `nexa:"required,string=Reading,precision=18,scale=2,default=0"`
	RecordedBy sdk.Many2One[CoreUser]         `nexa:"string=Recorded By"`
	Note       sdk.Text                       `nexa:"string=Note"`
}

// MaintenancePlan is a preventive-maintenance schedule and recurrence rule.
type MaintenancePlan struct {
	sdk.Model `nexa:"model=maintenance.plan"`

	Name               sdk.String                               `nexa:"required,index,string=Plan"`
	AssetID            sdk.Many2One[MaintenanceAsset]           `nexa:"index,string=Asset"`
	Strategy           sdk.Selection[maintmodels.MaintStrategy] `nexa:"string=Maintenance Strategy,default=preventive"`
	Trigger            sdk.Selection[TriggerType]               `nexa:"required,string=Trigger Method,default=calendar"`
	MeterID            sdk.Many2One[MaintenanceMeter]           `nexa:"string=Usage Meter"`
	IntervalValue      sdk.Integer                              `nexa:"string=Interval Value,default=0"`
	IntervalUnit       sdk.String                               `nexa:"string=Interval Unit,default=days"`
	ConditionThreshold sdk.Numeric                              `nexa:"string=Condition Threshold,precision=18,scale=2,default=0"`
	ResponsibleID      sdk.Many2One[CoreUser]                   `nexa:"string=Responsible Team Lead"`
	EstimatedHours     sdk.Numeric                              `nexa:"string=Estimated Hours,precision=18,scale=2,default=0"`
	RequiredSkills     sdk.String                               `nexa:"string=Required Skills"`
	SafetyReq          sdk.Text                                 `nexa:"string=Safety Requirements"`
	Instructions       sdk.Text                                 `nexa:"string=Task Template / Instructions"`
	LastDoneDate       sdk.Date                                 `nexa:"string=Last Completion"`
	NextDueDate        sdk.Date                                 `nexa:"string=Next Due Date"`
	State              sdk.Selection[PlanState]                 `nexa:"string=Status,default=active"`
	Active             sdk.Boolean                              `nexa:"string=Active,default=true"`
}

// MaintPlanOccurrence is an idempotent record of a single scheduled maintenance
// occurrence — prevents duplicate WO generation.
type MaintPlanOccurrence struct {
	sdk.Model `nexa:"model=maintenance.plan.occurrence"`

	PlanID         sdk.Many2One[MaintenancePlan]      `nexa:"required,index,string=Plan"`
	AssetID        sdk.Many2One[MaintenanceAsset]     `nexa:"required,index,string=Asset"`
	OccurrenceKey  sdk.String                         `nexa:"required,index,unique=true,string=Occurrence Key"`
	State          sdk.Selection[OccurrenceState]     `nexa:"required,string=State,default=pending"`
	PlannedDate    sdk.Date                           `nexa:"string=Planned Date"`
	MeterThreshold sdk.Numeric                        `nexa:"string=Meter Threshold,precision=18,scale=2,default=0"`
	WorkOrderID    sdk.Many2One[MaintenanceWorkOrder] `nexa:"index,string=Work Order"`
	GeneratedAt    sdk.DateTime                       `nexa:"string=Generated At"`
	GeneratedBy    sdk.Many2One[CoreUser]             `nexa:"string=Generated By"`
	SkipReason     sdk.Text                           `nexa:"string=Skip Reason"`
	Notes          sdk.Text                           `nexa:"string=Notes"`
}

// MaintPlanVersion is an append-only version history record for MaintenancePlan changes.
type MaintPlanVersion struct {
	sdk.Model `nexa:"model=maintenance.plan.version"`

	PlanID                 sdk.Many2One[MaintenancePlan] `nexa:"required,index,string=Plan"`
	VersionNumber          sdk.String                   `nexa:"required,string=Version Number"`
	ChangeDate             sdk.DateTime                 `nexa:"required,string=Change Date"`
	ChangedBy              sdk.Many2One[CoreUser]       `nexa:"required,string=Changed By"`
	ChangeSummary          sdk.Text                     `nexa:"string=Change Summary"`
	SnapshotStrategy       sdk.String                   `nexa:"string=Snapshot Strategy"`
	SnapshotTrigger        sdk.String                   `nexa:"string=Snapshot Trigger"`
	SnapshotIntervalValue  sdk.Integer                  `nexa:"string=Snapshot Interval Value,default=0"`
	SnapshotIntervalUnit   sdk.String                   `nexa:"string=Snapshot Interval Unit"`
	SnapshotEstimatedHours sdk.Numeric                  `nexa:"string=Snapshot Estimated Hours,precision=18,scale=2,default=0"`
}

// MaintAssetMeterSummary is a computed summary of the latest readings and due
// status per asset+meter combination.
type MaintAssetMeterSummary struct {
	sdk.Model `nexa:"model=maintenance.asset.meter.summary"`

	AssetID          sdk.Many2One[MaintenanceAsset] `nexa:"required,index,string=Asset"`
	MeterID          sdk.Many2One[MaintenanceMeter] `nexa:"required,index,string=Meter"`
	LatestReadingDate sdk.Date                      `nexa:"string=Latest Reading Date"`
	LatestValue      sdk.Numeric                    `nexa:"string=Latest Value,precision=18,scale=2,default=0"`
	ReadingCount     sdk.Integer                    `nexa:"string=Reading Count,default=0"`
	MinValue         sdk.Numeric                    `nexa:"string=Min Value,precision=18,scale=2,default=0"`
	MaxValue         sdk.Numeric                    `nexa:"string=Max Value,precision=18,scale=2,default=0"`
	AvgValue         sdk.Numeric                    `nexa:"string=Avg Value,precision=18,scale=2,default=0"`
	NextDueThreshold sdk.Numeric                    `nexa:"string=Next Due Threshold,precision=18,scale=2,default=0"`
	IsOverdue        sdk.Boolean                    `nexa:"string=Is Overdue,default=false"`
	UpdatedAt        sdk.DateTime                   `nexa:"string=Updated At"`
}

func init() {
	sdk.MustRegister("maintenance_equipment",
		&MaintenanceMeter{},
		&MaintenanceMeterReading{},
		&MaintenancePlan{},
		&MaintPlanOccurrence{},
		&MaintPlanVersion{},
		&MaintAssetMeterSummary{},
	)
}
