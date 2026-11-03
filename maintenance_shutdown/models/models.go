package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	maintmodels "nexa_addons/maintenance/models"
)

// Cross-module reference aliases.
type (
	CoreUser                 = basemodels.CoreUser
	MaintenanceAsset         = maintmodels.MaintenanceAsset
	MaintenanceAssetLocation = maintmodels.MaintenanceAssetLocation
	MaintenanceWorkOrder     = maintmodels.MaintenanceWorkOrder
)

// ---- Selection types -------------------------------------------------------

type ShutdownState string

const (
	ShutdownDraft              ShutdownState = "draft"
	ShutdownPlanning           ShutdownState = "planning"
	ShutdownApproved           ShutdownState = "approved"
	ShutdownExecution          ShutdownState = "in_execution"
	ShutdownMechanicalComplete ShutdownState = "mechanical_complete"
	ShutdownCommissioning      ShutdownState = "commissioning"
	ShutdownClosed             ShutdownState = "closed"
	ShutdownCancelled          ShutdownState = "cancelled"
)

type PackageState string

const (
	PackageDraft      PackageState = "draft"
	PackageReady      PackageState = "ready"
	PackageInProgress PackageState = "in_progress"
	PackageCompleted  PackageState = "completed"
	PackageVerified   PackageState = "verified"
)

type WindowType string

const (
	WindowRoutine     WindowType = "routine"
	WindowPlanned     WindowType = "planned_outage"
	WindowSeasonal    WindowType = "seasonal"
	WindowOpportunity WindowType = "opportunity"
	WindowEmergency   WindowType = "emergency"
)

type WindowState string

const (
	WindowStateRequested  WindowState = "requested"
	WindowStateAgreed     WindowState = "agreed"
	WindowStateInProgress WindowState = "in_progress"
	WindowStateCompleted  WindowState = "completed"
	WindowStateBreached   WindowState = "breached"
	WindowStateCancelled  WindowState = "cancelled"
)

// ---- Models ----------------------------------------------------------------

// MaintShutdownProject represents a planned plant turnaround or major shutdown outage.
type MaintShutdownProject struct {
	sdk.Model `nexa:"model=maintenance.shutdown.project"`

	Name                 sdk.String                         `nexa:"required,index,string=Turnaround Name"`
	Code                 sdk.String                         `nexa:"index,string=Project Code"`
	State                sdk.Selection[ShutdownState]       `nexa:"required,string=Status,default=draft"`
	PlannedStart         sdk.DateTime                       `nexa:"required,string=Planned Start"`
	PlannedFinish        sdk.DateTime                       `nexa:"required,string=Planned Finish"`
	ActualStart          sdk.DateTime                       `nexa:"string=Actual Start"`
	ActualFinish         sdk.DateTime                       `nexa:"string=Actual Finish"`
	TargetDowntimeHours  sdk.Numeric                        `nexa:"string=Target Downtime (hrs),precision=18,scale=2,default=0"`
	ActualDowntimeHours  sdk.Numeric                        `nexa:"string=Actual Downtime (hrs),precision=18,scale=2,default=0"`
	LeadCoordinatorID    sdk.Many2One[CoreUser]             `nexa:"required,string=Turnaround Coordinator"`
	OperationsLiaisonID  sdk.Many2One[CoreUser]             `nexa:"string=Operations Liaison"`
	SafetyLeadID         sdk.Many2One[CoreUser]             `nexa:"string=Safety Lead"`
	LocationID           sdk.Many2One[MaintenanceAssetLocation] `nexa:"string=Location / Site"`
	ProductionLineRef    sdk.String                         `nexa:"string=Affected Line / Unit"`
	BudgetAmount         sdk.Numeric                        `nexa:"string=Approved Budget ($),precision=18,scale=2,default=0"`
	ActualCost           sdk.Numeric                        `nexa:"string=Actual Cost ($),precision=18,scale=2,default=0"`
	RestartChecklistDone sdk.Boolean                        `nexa:"string=Pre-Startup Safety Review Done,default=false"`
	Notes                sdk.Text                           `nexa:"string=Notes"`
	Packages             sdk.One2Many[MaintShutdownPackage] `nexa:"string=Work Packages"`
	Milestones           sdk.One2Many[MaintShutdownMilestone] `nexa:"string=Milestones"`
}

// MaintShutdownPackage groups related work orders within a shutdown project.
type MaintShutdownPackage struct {
	sdk.Model `nexa:"model=maintenance.shutdown.package"`

	ProjectID           sdk.Many2One[MaintShutdownProject] `nexa:"required,index,string=Shutdown Project"`
	Name                sdk.String                         `nexa:"required,string=Package Name"`
	Code                sdk.String                         `nexa:"string=Package Code"`
	Sequence            sdk.Integer                        `nexa:"string=Sequence,default=10"`
	State               sdk.Selection[PackageState]        `nexa:"required,string=Status,default=draft"`
	PackageLeadID       sdk.Many2One[CoreUser]             `nexa:"string=Package Lead"`
	Trade               sdk.String                         `nexa:"string=Trade / Discipline"`
	CriticalPath        sdk.Boolean                        `nexa:"string=On Critical Path,default=false"`
	EstimatedLaborHours sdk.Numeric                        `nexa:"string=Est. Labor (hrs),precision=18,scale=2,default=0"`
	ActualLaborHours    sdk.Numeric                        `nexa:"string=Actual Labor (hrs),precision=18,scale=2,default=0"`
	Description         sdk.Text                           `nexa:"string=Description"`
	WorkOrderLinks      sdk.One2Many[MaintShutdownWOLink]  `nexa:"string=Work Orders"`
}

// MaintShutdownWOLink associates a work order to a shutdown work package.
type MaintShutdownWOLink struct {
	sdk.Model `nexa:"model=maintenance.shutdown.wo.link"`

	PackageID   sdk.Many2One[MaintShutdownPackage] `nexa:"required,index,string=Work Package"`
	WorkOrderID sdk.Many2One[MaintenanceWorkOrder] `nexa:"required,index,string=Work Order"`
	Sequence    sdk.Integer                        `nexa:"string=Execution Order,default=10"`
	Critical    sdk.Boolean                        `nexa:"string=Critical Path,default=false"`
	Notes       sdk.Text                           `nexa:"string=Notes"`
}

// MaintShutdownMilestone tracks major phase gates during a turnaround (e.g. Isolation, Mechanical Complete).
type MaintShutdownMilestone struct {
	sdk.Model `nexa:"model=maintenance.shutdown.milestone"`

	ProjectID    sdk.Many2One[MaintShutdownProject] `nexa:"required,index,string=Shutdown Project"`
	Name         sdk.String                         `nexa:"required,string=Milestone Name"`
	TargetDate   sdk.DateTime                       `nexa:"required,string=Target Date"`
	AchievedDate sdk.DateTime                       `nexa:"string=Achieved Date"`
	Completed    sdk.Boolean                        `nexa:"string=Completed,default=false"`
	SignoffBy    sdk.Many2One[CoreUser]             `nexa:"string=Sign-off By"`
	Sequence     sdk.Integer                        `nexa:"string=Sequence,default=10"`
	Notes        sdk.Text                           `nexa:"string=Notes"`
}

// MaintMaintenanceWindow represents an agreed operational window for maintenance work.
type MaintMaintenanceWindow struct {
	sdk.Model `nexa:"model=maintenance.window"`

	Name                 sdk.String                         `nexa:"required,index,string=Window Reference"`
	AssetID              sdk.Many2One[MaintenanceAsset]     `nexa:"index,string=Asset"`
	LocationID           sdk.Many2One[MaintenanceAssetLocation] `nexa:"string=Location"`
	ProductionLineRef    sdk.String                         `nexa:"string=Production Line"`
	WindowType           sdk.Selection[WindowType]          `nexa:"required,string=Window Type,default=routine"`
	State                sdk.Selection[WindowState]         `nexa:"required,string=Status,default=requested"`
	ScheduledStart       sdk.DateTime                       `nexa:"required,string=Scheduled Start"`
	ScheduledFinish      sdk.DateTime                       `nexa:"required,string=Scheduled Finish"`
	ActualStart          sdk.DateTime                       `nexa:"string=Actual Start"`
	ActualFinish         sdk.DateTime                       `nexa:"string=Actual Finish"`
	AllowedDowntimeHours sdk.Numeric                        `nexa:"string=Allowed Downtime (hrs),precision=18,scale=2,default=0"`
	ActualDowntimeHours  sdk.Numeric                        `nexa:"string=Actual Downtime (hrs),precision=18,scale=2,default=0"`
	OperationsApprovedBy sdk.Many2One[CoreUser]             `nexa:"string=Operations Approved By"`
	OperationsApprovedAt sdk.DateTime                       `nexa:"string=Operations Approved At"`
	ShutdownProjectID    sdk.Many2One[MaintShutdownProject] `nexa:"string=Shutdown Project"`
	Notes                sdk.Text                           `nexa:"string=Notes"`
}

func init() {
	sdk.MustRegister("maintenance_shutdown",
		&MaintShutdownProject{},
		&MaintShutdownPackage{},
		&MaintShutdownWOLink{},
		&MaintShutdownMilestone{},
		&MaintMaintenanceWindow{},
	)
}
