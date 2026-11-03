package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	maintmodels "nexa_addons/maintenance/models"
)

// Cross-module reference aliases.
type (
	CoreUser               = basemodels.CoreUser
	MaintenanceAsset       = maintmodels.MaintenanceAsset
	MaintenanceAssetCategory = maintmodels.MaintenanceAssetCategory
	MaintenanceWorkOrder   = maintmodels.MaintenanceWorkOrder
)

// ---- Selection types -------------------------------------------------------

type InspectionType string

const (
	InspRoutine         InspectionType = "routine"
	InspSafety          InspectionType = "safety"
	InspRegulatory      InspectionType = "regulatory"
	InspQuality         InspectionType = "quality"
	InspCondition       InspectionType = "condition"
	InspPreMaintenance  InspectionType = "pre_maintenance"
	InspPostMaintenance InspectionType = "post_maintenance"
)

type InspectionItemType string

const (
	InspItemYesNo  InspectionItemType = "yesno"
	InspItemMeasure InspectionItemType = "measurement"
	InspItemVisual  InspectionItemType = "visual"
	InspItemPhoto   InspectionItemType = "photo"
	InspItemText    InspectionItemType = "text"
)

type InspectionOutcome string

const (
	OutcomePass           InspectionOutcome = "pass"
	OutcomeFail           InspectionOutcome = "fail"
	OutcomeNA             InspectionOutcome = "na"
	OutcomeObservation    InspectionOutcome = "observation"
	OutcomeRequiresAction InspectionOutcome = "requires_action"
)

type InspectionRunState string

const (
	RunStatePlanned    InspectionRunState = "planned"
	RunStateInProgress InspectionRunState = "in_progress"
	RunStateCompleted  InspectionRunState = "completed"
	RunStateCancelled  InspectionRunState = "cancelled"
	RunStateFailed     InspectionRunState = "failed"
)

type CalibrationResult string

const (
	CalibPass         CalibrationResult = "pass"
	CalibFail         CalibrationResult = "fail"
	CalibAdjusted     CalibrationResult = "adjusted"
	CalibOutTolerance CalibrationResult = "out_of_tolerance"
)

// ---- Models ----------------------------------------------------------------

// MaintInspectionTemplate is a reusable inspection checklist template.
type MaintInspectionTemplate struct {
	sdk.Model `nexa:"model=maintenance.inspection.template"`

	Name               sdk.String                             `nexa:"required,index,string=Name"`
	Code               sdk.String                             `nexa:"index,string=Code"`
	InspType           sdk.Selection[InspectionType]          `nexa:"required,string=Inspection Type,default=routine"`
	AssetCategoryID    sdk.Many2One[MaintenanceAssetCategory]  `nexa:"string=Asset Category"`
	AssetID            sdk.Many2One[MaintenanceAsset]          `nexa:"string=Asset"`
	Description        sdk.Text                               `nexa:"string=Description"`
	Instructions       sdk.Text                               `nexa:"string=Instructions"`
	RequiresApproval   sdk.Boolean                            `nexa:"string=Requires Approval,default=false"`
	FollowUpRequired   sdk.Boolean                            `nexa:"string=Follow-Up Required,default=false"`
	EstimatedMinutes   sdk.Integer                            `nexa:"string=Estimated Duration (min),default=30"`
	Active             sdk.Boolean                            `nexa:"string=Active,default=true"`
	Items              sdk.One2Many[MaintInspectionTemplateItem] `nexa:"string=Items"`
}

// MaintInspectionTemplateItem is a single item within an inspection template.
type MaintInspectionTemplateItem struct {
	sdk.Model `nexa:"model=maintenance.inspection.template.item"`

	TemplateID   sdk.Many2One[MaintInspectionTemplate] `nexa:"required,index,string=Template"`
	Sequence     sdk.Integer                           `nexa:"string=Sequence,default=10"`
	ItemType     sdk.Selection[InspectionItemType]     `nexa:"required,string=Item Type,default=yesno"`
	Name         sdk.String                            `nexa:"required,string=Name"`
	Instructions sdk.Text                              `nexa:"string=Instructions"`
	Unit         sdk.String                            `nexa:"string=Unit"`
	MinValue     sdk.Numeric                           `nexa:"string=Min Value,precision=18,scale=4,default=0"`
	MaxValue     sdk.Numeric                           `nexa:"string=Max Value,precision=18,scale=4,default=0"`
	IsCritical   sdk.Boolean                           `nexa:"string=Critical,default=false"`
	FailAction   sdk.Text                              `nexa:"string=Fail Action"`
	Active       sdk.Boolean                           `nexa:"string=Active,default=true"`
}

// MaintInspectionRun is an execution of an inspection template against an asset.
type MaintInspectionRun struct {
	sdk.Model `nexa:"model=maintenance.inspection.run"`

	Name                sdk.String                             `nexa:"required,index,string=Reference"`
	TemplateID          sdk.Many2One[MaintInspectionTemplate]  `nexa:"required,index,string=Template"`
	AssetID             sdk.Many2One[MaintenanceAsset]          `nexa:"required,index,string=Asset"`
	WorkOrderID         sdk.Many2One[MaintenanceWorkOrder]      `nexa:"string=Work Order"`
	State               sdk.Selection[InspectionRunState]       `nexa:"required,string=State,default=planned"`
	ScheduledDate       sdk.Date                               `nexa:"string=Scheduled Date"`
	StartedAt           sdk.DateTime                           `nexa:"string=Started At"`
	CompletedAt         sdk.DateTime                           `nexa:"string=Completed At"`
	InspectorID         sdk.Many2One[CoreUser]                 `nexa:"required,string=Inspector"`
	ReviewedBy          sdk.Many2One[CoreUser]                 `nexa:"string=Reviewed By"`
	ReviewedAt          sdk.DateTime                           `nexa:"string=Reviewed At"`
	OverallOutcome      sdk.Selection[InspectionOutcome]        `nexa:"string=Overall Outcome,default=pass"`
	Summary             sdk.Text                               `nexa:"string=Summary"`
	FollowUpWorkOrderID sdk.Many2One[MaintenanceWorkOrder]      `nexa:"string=Follow-Up Work Order"`
	Notes               sdk.Text                               `nexa:"string=Notes"`
	Results             sdk.One2Many[MaintInspectionResult]    `nexa:"string=Results"`
}

// MaintInspectionResult is a captured result for a single template item in a run.
type MaintInspectionResult struct {
	sdk.Model `nexa:"model=maintenance.inspection.result"`

	RunID           sdk.Many2One[MaintInspectionRun]         `nexa:"required,index,string=Inspection Run"`
	TemplateItemID  sdk.Many2One[MaintInspectionTemplateItem] `nexa:"string=Template Item"`
	Sequence        sdk.Integer                              `nexa:"string=Sequence,default=10"`
	ItemType        sdk.Selection[InspectionItemType]        `nexa:"string=Item Type,default=yesno"`
	Name            sdk.String                               `nexa:"required,string=Name"`
	Outcome         sdk.Selection[InspectionOutcome]         `nexa:"string=Outcome,default=pass"`
	MeasuredValue   sdk.Numeric                              `nexa:"string=Measured Value,precision=18,scale=4,default=0"`
	Unit            sdk.String                               `nexa:"string=Unit"`
	TextResponse    sdk.String                               `nexa:"string=Text Response"`
	ObservationNote sdk.Text                                 `nexa:"string=Observation Note"`
	IsCritical      sdk.Boolean                              `nexa:"string=Critical,default=false"`
}

// MaintCalibrationRecord captures calibration data for a measurement asset.
type MaintCalibrationRecord struct {
	sdk.Model `nexa:"model=maintenance.calibration.record"`

	Name                sdk.String                         `nexa:"required,index,string=Reference"`
	AssetID             sdk.Many2One[MaintenanceAsset]      `nexa:"required,index,string=Asset"`
	CalibratorID        sdk.Many2One[CoreUser]             `nexa:"string=Calibrator (Internal)"`
	CalibratedBy        sdk.String                         `nexa:"string=Calibrated By (External)"`
	CalibrationDate     sdk.Date                           `nexa:"required,string=Calibration Date"`
	NextCalibrationDate sdk.Date                           `nexa:"string=Next Calibration Date"`
	CertificateRef      sdk.String                         `nexa:"string=Certificate Reference"`
	CertificateExpiry   sdk.Date                           `nexa:"string=Certificate Expiry"`
	AsFoundReading      sdk.Numeric                        `nexa:"string=As-Found Reading,precision=18,scale=4,default=0"`
	AsLeftReading       sdk.Numeric                        `nexa:"string=As-Left Reading,precision=18,scale=4,default=0"`
	Tolerance           sdk.Numeric                        `nexa:"string=Tolerance,precision=18,scale=4,default=0"`
	Unit                sdk.String                         `nexa:"string=Unit"`
	Result              sdk.Selection[CalibrationResult]   `nexa:"required,string=Result,default=pass"`
	EquipmentRef        sdk.String                         `nexa:"string=Reference Equipment"`
	Notes               sdk.Text                           `nexa:"string=Notes"`
	WorkOrderID         sdk.Many2One[MaintenanceWorkOrder]  `nexa:"string=Work Order"`
}

func init() {
	sdk.MustRegister("maintenance_inspection",
		&MaintInspectionTemplate{},
		&MaintInspectionTemplateItem{},
		&MaintInspectionRun{},
		&MaintInspectionResult{},
		&MaintCalibrationRecord{},
	)
}
