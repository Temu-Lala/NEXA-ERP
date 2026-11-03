package models

import (
	basemodels "nexa/addons/base/models"
	fleetmodels "nexa_addons/fleet/models"
	"nexa/core/sdk"
)

// Cross-module imports aliased at top
type CoreUser = basemodels.CoreUser
type FleetVehicle = fleetmodels.FleetVehicle
type FleetDepot = fleetmodels.FleetDepot

// ---- Selection Types -------------------------------------------------------

type DriverStatus string

const (
	DriverActive    DriverStatus = "active"
	DriverOnLeave   DriverStatus = "on_leave"
	DriverSuspended DriverStatus = "suspended"
	DriverInactive  DriverStatus = "inactive"
)

type MedicalFitnessStatus string

const (
	FitnessFit          MedicalFitnessStatus = "fit"
	FitnessConditional  MedicalFitnessStatus = "conditional"
	FitnessUnfit        MedicalFitnessStatus = "unfit"
	FitnessPendingCheck MedicalFitnessStatus = "pending_check"
)

type QualificationType string

const (
	QualLicense            QualificationType = "license"
	QualDangerousGoods     QualificationType = "dg_dangerous_goods"
	QualDefensiveDriving   QualificationType = "defensive_driving"
	QualFirstAid           QualificationType = "first_aid"
	QualHeavyVehicle       QualificationType = "heavy_vehicle"
	QualPassengerTransport QualificationType = "passenger_transport"
	QualForkliftCert       QualificationType = "forklift_cert"
)

type AssignmentType string

const (
	AssignPermanent   AssignmentType = "permanent"
	AssignTemporary   AssignmentType = "temporary"
	AssignTrip        AssignmentType = "trip"
	AssignPool        AssignmentType = "pool"
	AssignReplacement AssignmentType = "replacement"
	AssignCustodian   AssignmentType = "custodian"
)

type AssignmentState string

const (
	AssignStateRequested AssignmentState = "requested"
	AssignStateApproved  AssignmentState = "approved"
	AssignStateActive    AssignmentState = "active"
	AssignStateCompleted AssignmentState = "completed"
	AssignStateCancelled AssignmentState = "cancelled"
)

type HandoverType string

const (
	HandoverCheckout HandoverType = "checkout"
	HandoverCheckin  HandoverType = "checkin"
)

type CleanlinessLevel string

const (
	CleanClean    CleanlinessLevel = "clean"
	CleanModerate CleanlinessLevel = "moderate"
	CleanDirty    CleanlinessLevel = "dirty"
)

// ---- Models ----------------------------------------------------------------

// FleetDriver represents an authorized vehicle operator.
type FleetDriver struct {
	sdk.Model `nexa:"model=fleet.driver"`

	UserID              sdk.Many2One[CoreUser]             `nexa:"index,string=Linked User"`
	Name                sdk.String                         `nexa:"required,index,string=Driver Name"`
	DriverCode          sdk.String                         `nexa:"index,string=Driver Code"`
	Status              sdk.Selection[DriverStatus]        `nexa:"string=Status,default=active"`
	Phone               sdk.String                         `nexa:"string=Phone"`
	Email               sdk.String                         `nexa:"string=Email"`
	LicenseNumber       sdk.String                         `nexa:"required,index,string=License Number"`
	LicenseClass        sdk.String                         `nexa:"required,string=License Class"`
	LicenseIssueDate    sdk.Date                           `nexa:"string=License Issue Date"`
	LicenseExpiryDate   sdk.Date                           `nexa:"required,index,string=License Expiry Date"`
	LicenseVerified     sdk.Boolean                        `nexa:"string=License Verified,default=false"`
	MedicalStatus       sdk.Selection[MedicalFitnessStatus] `nexa:"string=Medical Fitness Status,default=fit"`
	MedicalExpiryDate   sdk.Date                           `nexa:"string=Medical Expiry Date"`
	SafetyInductionDate sdk.Date                           `nexa:"string=Safety Induction Date"`
	DepotID             sdk.Many2One[FleetDepot]           `nexa:"string=Depot"`
	ShiftType           sdk.String                         `nexa:"string=Shift Type,default=day"`
	TotalTrips          sdk.Integer                        `nexa:"string=Total Trips,default=0"`
	IncidentCount       sdk.Integer                        `nexa:"string=Incident Count,default=0"`
	Notes               sdk.Text                           `nexa:"string=Notes"`
	Active              sdk.Boolean                        `nexa:"string=Active,default=true"`
}

// FleetDriverQualification tracks certifications, endorsements, and credentials.
type FleetDriverQualification struct {
	sdk.Model `nexa:"model=fleet.driver.qualification"`

	DriverID          sdk.Many2One[FleetDriver]        `nexa:"required,index,string=Driver"`
	QualType          sdk.Selection[QualificationType] `nexa:"required,string=Qualification Type,default=license"`
	CertificateNumber sdk.String                       `nexa:"required,string=Certificate Number"`
	IssuingAuthority  sdk.String                       `nexa:"string=Issuing Authority"`
	IssueDate         sdk.Date                         `nexa:"required,string=Issue Date"`
	ExpiryDate        sdk.Date                         `nexa:"index,string=Expiry Date"`
	IsValid           sdk.Boolean                      `nexa:"string=Is Valid,default=true"`
	Notes             sdk.Text                         `nexa:"string=Notes"`
}

// FleetVehicleAssignment allocates a vehicle to a driver for a defined duration or scope.
type FleetVehicleAssignment struct {
	sdk.Model `nexa:"model=fleet.vehicle.assignment"`

	Name                 sdk.String                     `nexa:"required,index,string=Assignment Reference"`
	VehicleID            sdk.Many2One[FleetVehicle]     `nexa:"required,index,string=Vehicle"`
	DriverID             sdk.Many2One[FleetDriver]      `nexa:"required,index,string=Driver"`
	AssignmentType       sdk.Selection[AssignmentType]  `nexa:"string=Assignment Type,default=temporary"`
	State                sdk.Selection[AssignmentState] `nexa:"string=State,default=requested"`
	StartDate            sdk.DateTime                   `nexa:"required,string=Start Date"`
	EndDate              sdk.DateTime                   `nexa:"string=End Date"`
	ActualStartDate      sdk.DateTime                   `nexa:"string=Actual Start Date"`
	ActualEndDate        sdk.DateTime                   `nexa:"string=Actual End Date"`
	PlannedOdometerStart sdk.Integer                    `nexa:"string=Planned Odometer Start,default=0"`
	ActualOdometerStart  sdk.Integer                    `nexa:"string=Actual Odometer Start,default=0"`
	ActualOdometerEnd    sdk.Integer                    `nexa:"string=Actual Odometer End,default=0"`
	TotalDistance        sdk.Integer                    `nexa:"string=Total Distance,default=0"`
	Purpose              sdk.String                     `nexa:"string=Purpose"`
	Department           sdk.String                     `nexa:"string=Department"`
	ApprovedBy           sdk.Many2One[CoreUser]         `nexa:"string=Approved By"`
	Notes                sdk.Text                       `nexa:"string=Notes"`
}

// FleetVehicleHandover records pre/post assignment vehicle condition checklists.
type FleetVehicleHandover struct {
	sdk.Model `nexa:"model=fleet.vehicle.handover"`

	AssignmentID             sdk.Many2One[FleetVehicleAssignment] `nexa:"required,index,string=Assignment"`
	VehicleID                sdk.Many2One[FleetVehicle]           `nexa:"required,index,string=Vehicle"`
	DriverID                 sdk.Many2One[FleetDriver]            `nexa:"required,index,string=Driver"`
	HandoverType             sdk.Selection[HandoverType]          `nexa:"string=Handover Type,default=checkout"`
	Timestamp                sdk.DateTime                         `nexa:"required,string=Timestamp"`
	Odometer                 sdk.Integer                          `nexa:"required,string=Odometer,default=0"`
	FuelLevelPct             sdk.Numeric                          `nexa:"string=Fuel Level (%),precision=5,scale=2,default=100"`
	KeysReturned             sdk.Boolean                          `nexa:"string=Keys Returned,default=true"`
	Cleanliness              sdk.Selection[CleanlinessLevel]      `nexa:"string=Cleanliness,default=clean"`
	SpareTirePresent         sdk.Boolean                          `nexa:"string=Spare Tire Present,default=true"`
	JackToolsPresent         sdk.Boolean                          `nexa:"string=Jack and Tools Present,default=true"`
	FirstAidKitPresent       sdk.Boolean                          `nexa:"string=First Aid Kit Present,default=true"`
	FireExtinguisherPresent  sdk.Boolean                          `nexa:"string=Fire Extinguisher Present,default=true"`
	DamageNotes              sdk.Text                             `nexa:"string=Damage Notes"`
	InspectorID              sdk.Many2One[CoreUser]               `nexa:"string=Inspector"`
	DriverConfirmed          sdk.Boolean                          `nexa:"string=Driver Confirmed,default=true"`
	Notes                    sdk.Text                             `nexa:"string=Notes"`
}

func init() {
	sdk.MustRegister("fleet_driver",
		&FleetDriver{},
		&FleetDriverQualification{},
		&FleetVehicleAssignment{},
		&FleetVehicleHandover{},
	)
}
