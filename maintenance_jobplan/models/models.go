package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	maintmodels "nexa_addons/maintenance/models"
)

// Cross-module reference aliases.
type (
	CoreUser                  = basemodels.CoreUser
	MaintenanceAsset          = maintmodels.MaintenanceAsset
	MaintenanceAssetCategory  = maintmodels.MaintenanceAssetCategory
	MaintenanceWorkOrder      = maintmodels.MaintenanceWorkOrder
	MaintStrategy             = maintmodels.MaintStrategy
)

// ---- Selection types -------------------------------------------------------

type MaintJobPlanStatus string

const (
	JobPlanDraft      MaintJobPlanStatus = "draft"
	JobPlanActive     MaintJobPlanStatus = "active"
	JobPlanArchived   MaintJobPlanStatus = "archived"
	JobPlanSuperseded MaintJobPlanStatus = "superseded"
)

type MaintTaskType string

const (
	TaskTypeTask           MaintTaskType = "task"
	TaskTypeInspection     MaintTaskType = "inspection_point"
	TaskTypeMeasurement    MaintTaskType = "measurement"
	TaskTypeSafetyCheck    MaintTaskType = "safety_check"
	TaskTypeSignOff        MaintTaskType = "sign_off"
)

type MaintTaskResult string

const (
	TaskResultPending     MaintTaskResult = "pending"
	TaskResultPass        MaintTaskResult = "pass"
	TaskResultFail        MaintTaskResult = "fail"
	TaskResultNA          MaintTaskResult = "na"
	TaskResultObservation MaintTaskResult = "observation"
)

type MaintChecklistItemType string

const (
	ChecklistYesNo      MaintChecklistItemType = "yesno"
	ChecklistMeasure    MaintChecklistItemType = "measurement"
	ChecklistText       MaintChecklistItemType = "text"
	ChecklistPhoto      MaintChecklistItemType = "photo_required"
)

// ---- Models ----------------------------------------------------------------

// MaintJobPlan is a reusable, versioned maintenance procedure.
type MaintJobPlan struct {
	sdk.Model `nexa:"model=maintenance.job.plan"`

	Name              sdk.String                            `nexa:"required,string=Name"`
	Code              sdk.String                            `nexa:"index,string=Code"`
	Version           sdk.String                            `nexa:"string=Version,default=1.0"`
	AssetCategoryID   sdk.Many2One[MaintenanceAssetCategory] `nexa:"string=Asset Category"`
	AssetID           sdk.Many2One[MaintenanceAsset]         `nexa:"string=Asset"`
	Strategy          sdk.Selection[MaintStrategy]           `nexa:"string=Strategy,default=preventive"`
	Status            sdk.Selection[MaintJobPlanStatus]      `nexa:"required,string=Status,default=draft"`
	Description       sdk.Text                               `nexa:"string=Description"`
	SafetyHazards     sdk.Text                               `nexa:"string=Safety Hazards"`
	RequiredTools     sdk.Text                               `nexa:"string=Required Tools"`
	EstimatedMinutes  sdk.Integer                            `nexa:"string=Estimated Duration (min),default=0"`
	RequiredSkills    sdk.Text                               `nexa:"string=Required Skills"`
	EffectiveDate     sdk.Date                               `nexa:"string=Effective Date"`
	ExpiryDate        sdk.Date                               `nexa:"string=Expiry Date"`
	ApprovedBy        sdk.Many2One[CoreUser]                 `nexa:"string=Approved By"`
	ApprovedDate      sdk.Date                               `nexa:"string=Approval Date"`
	Notes             sdk.Text                               `nexa:"string=Notes"`
	Active            sdk.Boolean                            `nexa:"string=Active,default=true"`
	Tasks             sdk.One2Many[MaintJobPlanTask]         `nexa:"string=Tasks"`
	Checklists        sdk.One2Many[MaintJobPlanChecklist]    `nexa:"string=Checklists"`
}

// MaintJobPlanTask is a numbered task step within a job plan.
type MaintJobPlanTask struct {
	sdk.Model `nexa:"model=maintenance.job.plan.task"`

	JobPlanID                sdk.Many2One[MaintJobPlan]         `nexa:"required,index,string=Job Plan"`
	Sequence                 sdk.Integer                        `nexa:"string=Sequence,default=10"`
	TaskType                 sdk.Selection[MaintTaskType]       `nexa:"required,string=Task Type,default=task"`
	Name                     sdk.String                         `nexa:"required,string=Name"`
	Instructions             sdk.Text                           `nexa:"string=Instructions"`
	ExpectedDurationMinutes  sdk.Integer                        `nexa:"string=Expected Duration (min),default=0"`
	RequiredSkill            sdk.String                         `nexa:"string=Required Skill"`
	MeasurementUnit          sdk.String                         `nexa:"string=Measurement Unit"`
	MinValue                 sdk.Numeric                        `nexa:"string=Min Value,precision=18,scale=4,default=0"`
	MaxValue                 sdk.Numeric                        `nexa:"string=Max Value,precision=18,scale=4,default=0"`
	RequiresSignOff          sdk.Boolean                        `nexa:"string=Requires Sign-Off,default=false"`
	SafetyNote               sdk.Text                           `nexa:"string=Safety Note"`
	Active                   sdk.Boolean                        `nexa:"string=Active,default=true"`
}

// MaintJobPlanChecklist is a checklist item within a job plan.
type MaintJobPlanChecklist struct {
	sdk.Model `nexa:"model=maintenance.job.plan.checklist"`

	JobPlanID     sdk.Many2One[MaintJobPlan]              `nexa:"required,index,string=Job Plan"`
	Sequence      sdk.Integer                             `nexa:"string=Sequence,default=10"`
	ItemType      sdk.Selection[MaintChecklistItemType]   `nexa:"required,string=Item Type,default=yesno"`
	Question      sdk.String                              `nexa:"required,string=Question"`
	IsRequired    sdk.Boolean                             `nexa:"string=Required,default=true"`
	ExpectedValue sdk.String                              `nexa:"string=Expected Value"`
	Unit          sdk.String                              `nexa:"string=Unit"`
	FailAction    sdk.Text                                `nexa:"string=Fail Action"`
	Active        sdk.Boolean                             `nexa:"string=Active,default=true"`
}

// MaintWorkOrderTask is an executed task step on a work order.
type MaintWorkOrderTask struct {
	sdk.Model `nexa:"model=maintenance.work.order.task"`

	WorkOrderID     sdk.Many2One[MaintenanceWorkOrder]  `nexa:"required,index,string=Work Order"`
	JobPlanID       sdk.Many2One[MaintJobPlan]          `nexa:"string=Job Plan"`
	Sequence        sdk.Integer                         `nexa:"string=Sequence,default=10"`
	TaskType        sdk.Selection[MaintTaskType]        `nexa:"string=Task Type,default=task"`
	Name            sdk.String                          `nexa:"required,string=Name"`
	Instructions    sdk.Text                            `nexa:"string=Instructions"`
	Result          sdk.Selection[MaintTaskResult]      `nexa:"string=Result,default=pending"`
	ActualValue     sdk.Numeric                         `nexa:"string=Actual Value,precision=18,scale=4,default=0"`
	MeasurementUnit sdk.String                          `nexa:"string=Measurement Unit"`
	ObservationNote sdk.Text                            `nexa:"string=Observation Note"`
	CompletedBy     sdk.Many2One[CoreUser]              `nexa:"string=Completed By"`
	CompletedAt     sdk.DateTime                        `nexa:"string=Completed At"`
	SignedOffBy     sdk.Many2One[CoreUser]              `nexa:"string=Signed Off By"`
	SignedOffAt     sdk.DateTime                        `nexa:"string=Signed Off At"`
}

// MaintWorkOrderChecklist is an executed checklist item on a work order.
type MaintWorkOrderChecklist struct {
	sdk.Model `nexa:"model=maintenance.work.order.checklist"`

	WorkOrderID     sdk.Many2One[MaintenanceWorkOrder]      `nexa:"required,index,string=Work Order"`
	ChecklistItemID sdk.Many2One[MaintJobPlanChecklist]     `nexa:"string=Checklist Item"`
	Sequence        sdk.Integer                             `nexa:"string=Sequence,default=10"`
	Question        sdk.String                              `nexa:"required,string=Question"`
	ItemType        sdk.Selection[MaintChecklistItemType]   `nexa:"string=Item Type,default=yesno"`
	IsRequired      sdk.Boolean                             `nexa:"string=Required,default=true"`
	Response        sdk.String                              `nexa:"string=Response"`
	NumericValue    sdk.Numeric                             `nexa:"string=Numeric Value,precision=18,scale=4,default=0"`
	Result          sdk.Selection[MaintTaskResult]          `nexa:"string=Result,default=pending"`
	ObservationNote sdk.Text                                `nexa:"string=Observation Note"`
	CompletedBy     sdk.Many2One[CoreUser]                  `nexa:"string=Completed By"`
	CompletedAt     sdk.DateTime                            `nexa:"string=Completed At"`
}

func init() {
	sdk.MustRegister("maintenance_jobplan",
		&MaintJobPlan{},
		&MaintJobPlanTask{},
		&MaintJobPlanChecklist{},
		&MaintWorkOrderTask{},
		&MaintWorkOrderChecklist{},
	)
}
