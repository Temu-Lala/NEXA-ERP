package models

import (
	basemodels "nexa/addons/base/models"
	fleetmodels "nexa_addons/fleet/models"
	maintmodels "nexa_addons/maintenance/models"
	"nexa/core/sdk"
)

// Cross-module aliases
type (
	CoreUser             = basemodels.CoreUser
	FleetVehicle         = fleetmodels.FleetVehicle
	FleetVehicleClass    = fleetmodels.FleetVehicleClass
	MaintenanceWorkOrder = maintmodels.MaintenanceWorkOrder
)

// ---- Selection Types -------------------------------------------------------

type DefectSeverity string

const (
	SeverityLow       DefectSeverity = "low"
	SeverityMedium    DefectSeverity = "medium"
	SeverityHighRestr DefectSeverity = "high_restricted"
	SeverityCritical  DefectSeverity = "critical_grounded"
)

type DefectState string

const (
	DefectReported       DefectState = "reported"
	DefectTriaged        DefectState = "triaged"
	DefectWorkOrder      DefectState = "work_order_created"
	DefectInRepair       DefectState = "in_repair"
	DefectRepaired       DefectState = "repaired"
	DefectVerifiedClosed DefectState = "verified_cleared"
)

type InspectionType string

const (
	InspPreTrip    InspectionType = "pre_trip"
	InspPostTrip   InspectionType = "post_trip"
	InspWeekly     InspectionType = "weekly"
	InspMonthly    InspectionType = "monthly"
	InspRegulatory InspectionType = "regulatory"
)

type InspectionOutcome string

const (
	OutcomePass            InspectionOutcome = "pass"
	OutcomePassWithDefects InspectionOutcome = "pass_with_defects"
	OutcomeFailGrounded    InspectionOutcome = "fail_grounded"
)

type ScheduleTriggerType string

const (
	TriggerDistance ScheduleTriggerType = "distance"
	TriggerTime     ScheduleTriggerType = "time"
	TriggerHours    ScheduleTriggerType = "engine_hours"
)

// ---- Models ----------------------------------------------------------------

// FleetVehicleDefect records vehicle defects, safety issues, and repair requests.
type FleetVehicleDefect struct {
	sdk.Model `nexa:"model=fleet.vehicle.defect"`

	Name             sdk.String                   `nexa:"required,index,string=Defect Reference"`
	VehicleID        sdk.Many2One[FleetVehicle]   `nexa:"required,index,string=Vehicle"`
	ReportedBy       sdk.Many2One[CoreUser]       `nexa:"required,string=Reported By"`
	DateReported     sdk.DateTime                 `nexa:"required,string=Date Reported"`
	Source           sdk.String                   `nexa:"string=Source,default=driver_report"`
	Category         sdk.String                   `nexa:"required,string=Category,default=brakes"`
	Severity         sdk.Selection[DefectSeverity]`nexa:"string=Severity,default=medium"`
	GroundVehicle    sdk.Boolean                  `nexa:"string=Ground Vehicle,default=false"`
	State            sdk.Selection[DefectState]   `nexa:"string=Status,default=reported"`
	Description      sdk.Text                     `nexa:"required,string=Description"`
	MaintWorkOrderID sdk.Many2One[MaintenanceWorkOrder] `nexa:"string=Maintenance Work Order"`
	VerifiedBy       sdk.Many2One[CoreUser]       `nexa:"string=Verified By"`
	VerifiedDate     sdk.Date                     `nexa:"string=Verified Date"`
	Notes            sdk.Text                     `nexa:"string=Notes"`
}

// FleetInspectionTemplate defines standard inspection checklists.
type FleetInspectionTemplate struct {
	sdk.Model `nexa:"model=fleet.inspection.template"`

	Name           sdk.String                     `nexa:"required,index,string=Template Name"`
	Code           sdk.String                     `nexa:"index,string=Template Code"`
	InspType       sdk.Selection[InspectionType]  `nexa:"string=Inspection Type,default=pre_trip"`
	VehicleClassID sdk.Many2One[FleetVehicleClass]`nexa:"string=Vehicle Class"`
	Instructions   sdk.Text                       `nexa:"string=Instructions"`
	Active         sdk.Boolean                    `nexa:"string=Active,default=true"`
}

// FleetVehicleInspection logs execution of a checklist on a vehicle.
type FleetVehicleInspection struct {
	sdk.Model `nexa:"model=fleet.vehicle.inspection"`

	Name                sdk.String                       `nexa:"required,index,string=Inspection Number"`
	TemplateID          sdk.Many2One[FleetInspectionTemplate] `nexa:"required,string=Inspection Template"`
	VehicleID           sdk.Many2One[FleetVehicle]       `nexa:"required,index,string=Vehicle"`
	InspectorID         sdk.Many2One[CoreUser]           `nexa:"required,string=Inspector"`
	InspectionDate      sdk.DateTime                     `nexa:"required,string=Inspection Date"`
	InspType            sdk.Selection[InspectionType]    `nexa:"string=Inspection Type,default=pre_trip"`
	Odometer            sdk.Integer                      `nexa:"string=Odometer,default=0"`
	OverallOutcome      sdk.Selection[InspectionOutcome] `nexa:"string=Overall Outcome,default=pass"`
	CriticalDefectFound sdk.Boolean                      `nexa:"string=Critical Defect Found,default=false"`
	State               sdk.String                       `nexa:"string=Status,default=completed"`
	Notes               sdk.Text                         `nexa:"string=Notes"`
}

// FleetServiceSchedule defines preventive maintenance intervals by odometer, time, or hours.
type FleetServiceSchedule struct {
	sdk.Model `nexa:"model=fleet.service.schedule"`

	Name                sdk.String                          `nexa:"required,index,string=Schedule Name"`
	VehicleID           sdk.Many2One[FleetVehicle]          `nexa:"index,string=Vehicle"`
	VehicleClassID      sdk.Many2One[FleetVehicleClass]     `nexa:"index,string=Vehicle Class"`
	TriggerType         sdk.Selection[ScheduleTriggerType]  `nexa:"string=Trigger Type,default=distance"`
	IntervalKm          sdk.Integer                         `nexa:"string=Interval (Km),default=10000"`
	IntervalMonths      sdk.Integer                         `nexa:"string=Interval (Months),default=6"`
	IntervalHours       sdk.Integer                         `nexa:"string=Interval (Hours),default=0"`
	LastServiceOdometer sdk.Integer                         `nexa:"string=Last Service Odometer,default=0"`
	LastServiceDate     sdk.Date                            `nexa:"string=Last Service Date"`
	NextDueOdometer     sdk.Integer                         `nexa:"string=Next Due Odometer,default=0"`
	NextDueDate         sdk.Date                            `nexa:"string=Next Due Date"`
	DueThresholdPct     sdk.Numeric                         `nexa:"string=Due Threshold (%),precision=5,scale=2,default=90"`
	IsDue               sdk.Boolean                         `nexa:"string=Is Due,default=false"`
	IsOverdue           sdk.Boolean                         `nexa:"string=Is Overdue,default=false"`
	Notes               sdk.Text                            `nexa:"string=Notes"`
}

func init() {
	sdk.MustRegister("fleet_maintenance",
		&FleetVehicleDefect{},
		&FleetInspectionTemplate{},
		&FleetVehicleInspection{},
		&FleetServiceSchedule{},
	)
}
