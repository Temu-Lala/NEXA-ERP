package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	maintmodels "nexa_addons/maintenance/models"
	relimodels "nexa_addons/maintenance_reliability/models"
)

// Cross-module reference aliases.
type (
	MaintenanceAsset          = maintmodels.MaintenanceAsset
	MaintenanceAssetCategory  = maintmodels.MaintenanceAssetCategory
	MaintenanceWorkOrder      = maintmodels.MaintenanceWorkOrder
	MaintenanceDowntimeEvent  = relimodels.MaintenanceDowntimeEvent
	CoreUser                  = basemodels.CoreUser
)

// ---- Selection types -------------------------------------------------------

type DowntimeCategory string

const (
	DtCatMechanicalFailure  DowntimeCategory = "mechanical_failure"
	DtCatElectricalFailure  DowntimeCategory = "electrical_failure"
	DtCatInstrumentFailure  DowntimeCategory = "instrument_failure"
	DtCatProcessUpset       DowntimeCategory = "process_upset"
	DtCatPlannedMaintenance DowntimeCategory = "planned_maintenance"
	DtCatExternal           DowntimeCategory = "external"
	DtCatUtilityFailure     DowntimeCategory = "utility_failure"
	DtCatOperatorError      DowntimeCategory = "operator_error"
	DtCatOther              DowntimeCategory = "other"
)

type ProductionUnit string

const (
	ProdUnitUnits   ProductionUnit = "units"
	ProdUnitKg      ProductionUnit = "kg"
	ProdUnitTonnes  ProductionUnit = "tonnes"
	ProdUnitLitres  ProductionUnit = "litres"
	ProdUnitM3      ProductionUnit = "m3"
	ProdUnitHours   ProductionUnit = "hours"
	ProdUnitBatches ProductionUnit = "batches"
	ProdUnitOther   ProductionUnit = "other"
)

type AlertSeverity string

const (
	AlertSevInfo      AlertSeverity = "info"
	AlertSevWarning   AlertSeverity = "warning"
	AlertSevCritical  AlertSeverity = "critical"
	AlertSevEmergency AlertSeverity = "emergency"
)

type AlertState string

const (
	AlertStateActive       AlertState = "active"
	AlertStateAcknowledged AlertState = "acknowledged"
	AlertStateResolved     AlertState = "resolved"
	AlertStateSuppressed   AlertState = "suppressed"
)

type ReadingSource string

const (
	ReadingSrcManual ReadingSource = "manual"
	ReadingSrcImport ReadingSource = "import"
	ReadingSrcAPI    ReadingSource = "api"
	ReadingSrcSensor ReadingSource = "sensor"
)

type ConditionRuleOperator string

const (
	CondOpGt  ConditionRuleOperator = "gt"
	CondOpGte ConditionRuleOperator = "gte"
	CondOpLt  ConditionRuleOperator = "lt"
	CondOpLte ConditionRuleOperator = "lte"
	CondOpEq  ConditionRuleOperator = "eq"
	CondOpNeq ConditionRuleOperator = "neq"
)

// ---- Models ----------------------------------------------------------------

// MaintDowntimeCategory is a classification for downtime events.
type MaintDowntimeCategory struct {
	sdk.Model `nexa:"model=maintenance.downtime.category"`

	Name        sdk.String                        `nexa:"required,index,string=Name"`
	Code        sdk.String                        `nexa:"string=Code"`
	DowntimeCat sdk.Selection[DowntimeCategory]   `nexa:"required,string=Downtime Category,default=other"`
	Description sdk.Text                          `nexa:"string=Description"`
	Active      sdk.Boolean                       `nexa:"string=Active,default=true"`
}

// MaintDowntimeEventV2 is an enhanced downtime record with production impact details.
type MaintDowntimeEventV2 struct {
	sdk.Model `nexa:"model=maintenance.downtime.v2"`

	Name                    sdk.String                          `nexa:"required,index,string=Name"`
	AssetID                 sdk.Many2One[MaintenanceAsset]      `nexa:"required,index,string=Asset"`
	WorkOrderID             sdk.Many2One[MaintenanceWorkOrder]  `nexa:"index,string=Work Order"`
	DowntimeEventID         sdk.Many2One[MaintenanceDowntimeEvent] `nexa:"index,string=Base Downtime Event"`
	CategoryID              sdk.Many2One[MaintDowntimeCategory] `nexa:"index,string=Category"`
	StartTime               sdk.DateTime                        `nexa:"required,string=Start Time"`
	EndTime                 sdk.DateTime                        `nexa:"string=End Time"`
	TotalDurationHours      sdk.Numeric                         `nexa:"string=Total Duration (hrs),precision=18,scale=2,default=0"`
	RepairDurationHours     sdk.Numeric                         `nexa:"string=Repair Duration (hrs),precision=18,scale=2,default=0"`
	WaitingForPartsHours    sdk.Numeric                         `nexa:"string=Waiting for Parts (hrs),precision=18,scale=2,default=0"`
	WaitingForTechHours     sdk.Numeric                         `nexa:"string=Waiting for Technician (hrs),precision=18,scale=2,default=0"`
	ProductionLineStopped   sdk.Boolean                         `nexa:"string=Production Line Stopped,default=false"`
	ProductionLineRef       sdk.String                          `nexa:"string=Production Line Reference"`
	LostProductionQty       sdk.Numeric                         `nexa:"string=Lost Production Qty,precision=18,scale=2,default=0"`
	LostProductionUnit      sdk.Selection[ProductionUnit]       `nexa:"string=Lost Production Unit,default=units"`
	EstimatedLossCost       sdk.Numeric                         `nexa:"string=Estimated Loss Cost,precision=18,scale=2,default=0"`
	OperationsApprovedBy    sdk.Many2One[CoreUser]              `nexa:"string=Operations Approved By"`
	OperationsApprovedAt    sdk.DateTime                        `nexa:"string=Operations Approved At"`
	ReturnToServiceBy       sdk.Many2One[CoreUser]              `nexa:"string=Return to Service By"`
	ReturnToServiceAt       sdk.DateTime                        `nexa:"string=Return to Service At"`
	RestartChecksRequired   sdk.Boolean                         `nexa:"string=Restart Checks Required,default=false"`
	RestartChecksCompleted  sdk.Boolean                         `nexa:"string=Restart Checks Completed,default=false"`
	Notes                   sdk.Text                            `nexa:"string=Notes"`
}

// MaintConditionRule defines a threshold-based alert rule for an asset parameter.
type MaintConditionRule struct {
	sdk.Model `nexa:"model=maintenance.condition.rule"`

	Name                  sdk.String                           `nexa:"required,index,string=Name"`
	AssetID               sdk.Many2One[MaintenanceAsset]       `nexa:"index,string=Asset"`
	AssetCategoryID       sdk.Many2One[MaintenanceAssetCategory] `nexa:"index,string=Asset Category"`
	ParameterName         sdk.String                           `nexa:"required,string=Parameter Name"`
	Unit                  sdk.String                           `nexa:"string=Unit"`
	Operator              sdk.Selection[ConditionRuleOperator] `nexa:"required,string=Operator,default=gt"`
	ThresholdValue        sdk.Numeric                          `nexa:"required,string=Threshold Value,precision=18,scale=4,default=0"`
	HysteresisValue       sdk.Numeric                          `nexa:"string=Hysteresis Value,precision=18,scale=4,default=0"`
	AlertSeverity         sdk.Selection[AlertSeverity]         `nexa:"required,string=Alert Severity,default=warning"`
	AlertMessage          sdk.Text                             `nexa:"string=Alert Message"`
	CreateMaintenanceAlert sdk.Boolean                         `nexa:"string=Create Maintenance Alert,default=true"`
	MaintenancePriority   sdk.String                           `nexa:"string=Maintenance Priority,default=normal"`
	DebounceMinutes       sdk.Integer                          `nexa:"string=Debounce Minutes,default=0"`
	Active                sdk.Boolean                          `nexa:"string=Active,default=true"`
}

// MaintConditionReading captures a sensor or manual parameter reading for an asset.
type MaintConditionReading struct {
	sdk.Model `nexa:"model=maintenance.condition.reading"`

	AssetID       sdk.Many2One[MaintenanceAsset]      `nexa:"required,index,string=Asset"`
	RuleID        sdk.Many2One[MaintConditionRule]    `nexa:"index,string=Condition Rule"`
	ParameterName sdk.String                          `nexa:"required,string=Parameter Name"`
	ReadingTime   sdk.DateTime                        `nexa:"required,string=Reading Time"`
	Value         sdk.Numeric                         `nexa:"required,string=Value,precision=18,scale=4,default=0"`
	Unit          sdk.String                          `nexa:"string=Unit"`
	Source        sdk.Selection[ReadingSource]        `nexa:"required,string=Source,default=manual"`
	ExternalRef   sdk.String                          `nexa:"string=External Reference"`
	TriggeredAlert sdk.Boolean                        `nexa:"string=Triggered Alert,default=false"`
	AlertID       sdk.Many2One[MaintConditionAlert]   `nexa:"index,string=Condition Alert"`
	RecordedBy    sdk.Many2One[CoreUser]              `nexa:"string=Recorded By"`
	Notes         sdk.Text                            `nexa:"string=Notes"`
}

// MaintConditionAlert is a threshold breach alert raised from a condition reading.
type MaintConditionAlert struct {
	sdk.Model `nexa:"model=maintenance.condition.alert"`

	Name           sdk.String                         `nexa:"required,index,string=Name"`
	AssetID        sdk.Many2One[MaintenanceAsset]     `nexa:"required,index,string=Asset"`
	RuleID         sdk.Many2One[MaintConditionRule]   `nexa:"required,index,string=Condition Rule"`
	State          sdk.Selection[AlertState]          `nexa:"required,string=State,default=active"`
	AlertSeverity  sdk.Selection[AlertSeverity]       `nexa:"required,string=Severity,default=warning"`
	AlertTime      sdk.DateTime                       `nexa:"required,string=Alert Time"`
	ResolvedTime   sdk.DateTime                       `nexa:"string=Resolved Time"`
	TriggeredValue sdk.Numeric                        `nexa:"string=Triggered Value,precision=18,scale=4,default=0"`
	ThresholdValue sdk.Numeric                        `nexa:"string=Threshold Value,precision=18,scale=4,default=0"`
	ParameterName  sdk.String                         `nexa:"string=Parameter Name"`
	Unit           sdk.String                         `nexa:"string=Unit"`
	AlertMessage   sdk.Text                           `nexa:"string=Alert Message"`
	AcknowledgedBy sdk.Many2One[CoreUser]             `nexa:"string=Acknowledged By"`
	AcknowledgedAt sdk.DateTime                       `nexa:"string=Acknowledged At"`
	ResolvedBy     sdk.Many2One[CoreUser]             `nexa:"string=Resolved By"`
	SuppressedUntil sdk.DateTime                      `nexa:"string=Suppressed Until"`
	WorkOrderID    sdk.Many2One[MaintenanceWorkOrder]  `nexa:"index,string=Work Order"`
	Notes          sdk.Text                           `nexa:"string=Notes"`
}

func init() {
	sdk.MustRegister("maintenance_downtime",
		&MaintDowntimeCategory{},
		&MaintDowntimeEventV2{},
		&MaintConditionRule{},
		&MaintConditionReading{},
		&MaintConditionAlert{},
	)
}
