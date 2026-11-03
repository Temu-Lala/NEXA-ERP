package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// Cross-module reference aliases.
type CoreUser = basemodels.CoreUser

// ---- Selection types -------------------------------------------------------

type VehicleLifecycleState string

const (
	LifecycleDraft         VehicleLifecycleState = "draft"
	LifecycleOrdered       VehicleLifecycleState = "ordered"
	LifecycleReceived      VehicleLifecycleState = "received"
	LifecycleCommissioning VehicleLifecycleState = "commissioning"
	LifecycleActive        VehicleLifecycleState = "active"
	LifecycleRetired       VehicleLifecycleState = "retired"
	LifecycleDisposed      VehicleLifecycleState = "disposed"
)

type VehicleConditionState string

const (
	ConditionOperational VehicleConditionState = "operational"
	ConditionRestricted  VehicleConditionState = "restricted"
	ConditionDefective   VehicleConditionState = "defective"
	ConditionUnsafe      VehicleConditionState = "unsafe"
	ConditionUnderRepair VehicleConditionState = "under_repair"
)

type VehicleAvailabilityState string

const (
	AvailabilityAvailable   VehicleAvailabilityState = "available"
	AvailabilityReserved    VehicleAvailabilityState = "reserved"
	AvailabilityAssigned    VehicleAvailabilityState = "assigned"
	AvailabilityOnTrip      VehicleAvailabilityState = "on_trip"
	AvailabilityUnavailable VehicleAvailabilityState = "unavailable"
)

type VehicleComplianceState string

const (
	ComplianceCompliant VehicleComplianceState = "compliant"
	ComplianceExpiring  VehicleComplianceState = "expiring"
	ComplianceOverdue   VehicleComplianceState = "overdue"
	ComplianceBlocked   VehicleComplianceState = "blocked"
)

type VehicleOwnershipType string

const (
	OwnershipOwned      VehicleOwnershipType = "owned"
	OwnershipLeased     VehicleOwnershipType = "leased"
	OwnershipRented     VehicleOwnershipType = "rented"
	OwnershipFinanced   VehicleOwnershipType = "financed"
	OwnershipThirdParty VehicleOwnershipType = "third_party"
)

type VehicleFuelType string

const (
	FuelDiesel       VehicleFuelType = "diesel"
	FuelPetrol       VehicleFuelType = "petrol"
	FuelElectric     VehicleFuelType = "electric"
	FuelHybridPetrol VehicleFuelType = "hybrid_petrol"
	FuelHybridDiesel VehicleFuelType = "hybrid_diesel"
	FuelCNG          VehicleFuelType = "cng"
	FuelLPG          VehicleFuelType = "lpg"
	FuelHydrogen     VehicleFuelType = "hydrogen"
	FuelOther        VehicleFuelType = "other"
)

type VehicleTransmission string

const (
	TransManual    VehicleTransmission = "manual"
	TransAutomatic VehicleTransmission = "automatic"
	TransAMT       VehicleTransmission = "amt"
	TransCVT       VehicleTransmission = "cvt"
)

type OdometerSource string

const (
	OdoSourceManual     OdometerSource = "manual"
	OdoSourceTelematics OdometerSource = "telematics"
	OdoSourceInspection OdometerSource = "inspection"
	OdoSourceFuelLog    OdometerSource = "fuel_log"
	OdoSourceService    OdometerSource = "service"
)

type FleetState string

const (
	FleetStateNew         FleetState = "new"
	FleetStateActive      FleetState = "active"
	FleetStateMaintenance FleetState = "maintenance"
	FleetStateRetired     FleetState = "retired"
)

// ---- Models ----------------------------------------------------------------

// FleetVehicleBrand represents an automotive make/manufacturer (e.g., Toyota, Ford, Scania).
type FleetVehicleBrand struct {
	sdk.Model `nexa:"model=fleet.vehicle.brand"`

	Name    sdk.String  `nexa:"required,string=Brand Name"`
	Code    sdk.String  `nexa:"string=Brand Code"`
	LogoURL sdk.String  `nexa:"string=Logo URL"`
	Active  sdk.Boolean `nexa:"string=Active,default=true"`
}

// FleetVehicleModel represents a specific commercial model/trim line.
type FleetVehicleModel struct {
	sdk.Model `nexa:"model=fleet.vehicle.model"`

	Name           sdk.String                      `nexa:"required,string=Model Name"`
	BrandID        sdk.Many2One[FleetVehicleBrand] `nexa:"required,string=Brand"`
	VehicleClassID sdk.Many2One[FleetVehicleClass] `nexa:"string=Vehicle Class"`
	ModelYear      sdk.Integer                     `nexa:"string=Model Year"`
	Active         sdk.Boolean                     `nexa:"string=Active,default=true"`
}

// FleetVehicleClass categorizes vehicles by body style and regulatory classification.
type FleetVehicleClass struct {
	sdk.Model `nexa:"model=fleet.vehicle.class"`

	Name                            sdk.String  `nexa:"required,string=Class Name"`
	Code                            sdk.String  `nexa:"string=Code"`
	RequiresCDL                     sdk.Boolean `nexa:"string=Requires CDL,default=false"`
	DefaultInspectionIntervalKm     sdk.Integer `nexa:"string=Default Inspection Interval (km),default=5000"`
	DefaultInspectionIntervalMonths sdk.Integer `nexa:"string=Default Inspection Interval (Months),default=6"`
	Active                          sdk.Boolean `nexa:"string=Active,default=true"`
}

// FleetDepot represents a base yard, garage, terminal, or physical parking depot.
type FleetDepot struct {
	sdk.Model `nexa:"model=fleet.depot"`

	Name      sdk.String             `nexa:"required,string=Depot Name"`
	Code      sdk.String             `nexa:"string=Code"`
	Address   sdk.String             `nexa:"string=Address"`
	City      sdk.String             `nexa:"string=City"`
	Capacity  sdk.Integer            `nexa:"string=Capacity,default=0"`
	ManagerID sdk.Many2One[CoreUser] `nexa:"string=Manager"`
	Active    sdk.Boolean            `nexa:"string=Active,default=true"`
}

// FleetVehicle is the central enterprise vehicle record.
type FleetVehicle struct {
	sdk.Model `nexa:"model=fleet.vehicle"`

	Name         sdk.String `nexa:"required,index,string=Fleet Number"`
	PlateNumber  sdk.String `nexa:"required,index,string=License Plate"`
	VIN          sdk.String `nexa:"index,string=VIN"`
	EngineNumber sdk.String `nexa:"string=Engine Number"`
	QRBarcode    sdk.String `nexa:"string=QR/Barcode"`

	BrandID        sdk.Many2One[FleetVehicleBrand] `nexa:"string=Brand"`
	ModelID        sdk.Many2One[FleetVehicleModel] `nexa:"string=Model"`
	VehicleClassID sdk.Many2One[FleetVehicleClass] `nexa:"string=Vehicle Class"`
	ModelYear      sdk.Integer                     `nexa:"string=Model Year"`

	LifecycleState    sdk.Selection[VehicleLifecycleState]    `nexa:"string=Lifecycle State,default=active"`
	ConditionState    sdk.Selection[VehicleConditionState]    `nexa:"string=Condition State,default=operational"`
	AvailabilityState sdk.Selection[VehicleAvailabilityState] `nexa:"string=Availability State,default=available"`
	ComplianceState   sdk.Selection[VehicleComplianceState]   `nexa:"string=Compliance State,default=compliant"`

	OwnershipType sdk.Selection[VehicleOwnershipType] `nexa:"string=Ownership Type,default=owned"`
	FuelType      sdk.Selection[VehicleFuelType]      `nexa:"string=Fuel Type,default=diesel"`
	Transmission  sdk.Selection[VehicleTransmission]  `nexa:"string=Transmission,default=manual"`

	EngineCapacityCC       sdk.Integer `nexa:"string=Engine Capacity (cc),default=0"`
	PowerKW                sdk.Integer `nexa:"string=Power (kW),default=0"`
	SeatingCapacity        sdk.Integer `nexa:"string=Seating Capacity,default=5"`
	PayloadCapacityKg      sdk.Numeric `nexa:"string=Payload Capacity (kg),precision=18,scale=2,default=0"`
	TowingCapacityKg       sdk.Numeric `nexa:"string=Towing Capacity (kg),precision=18,scale=2,default=0"`
	GrossVehicleWeightKg   sdk.Numeric `nexa:"string=Gross Vehicle Weight (kg),precision=18,scale=2,default=0"`
	FuelTankCapacityLitres sdk.Numeric `nexa:"string=Fuel Tank Capacity (L),precision=18,scale=2,default=0"`
	BatteryCapacityKWh     sdk.Numeric `nexa:"string=Battery Capacity (kWh),precision=18,scale=2,default=0"`
	ChargingConnector      sdk.String  `nexa:"string=Charging Connector"`
	EmissionStandard       sdk.String  `nexa:"string=Emission Standard"`

	Odometer     sdk.Integer `nexa:"string=Current Odometer,default=0"`
	OdometerUnit sdk.String  `nexa:"string=Odometer Unit,default=km"`

	DepotID        sdk.Many2One[FleetDepot] `nexa:"string=Depot"`
	LocationText   sdk.String               `nexa:"string=Location"`
	DepartmentName sdk.String               `nexa:"string=Department"`
	ManagerID      sdk.Many2One[CoreUser]   `nexa:"string=Manager"`
	DriverID       sdk.Many2One[CoreUser]   `nexa:"string=Driver"`

	PurchaseDate       sdk.Date    `nexa:"string=Purchase Date"`
	InServiceDate      sdk.Date    `nexa:"string=In Service Date"`
	ExpectedRetireDate sdk.Date    `nexa:"string=Expected Retire Date"`
	PurchaseCost       sdk.Numeric `nexa:"string=Purchase Cost,precision=18,scale=2,default=0"`
	Currency           sdk.String  `nexa:"string=Currency,default=USD"`
	FixedAssetRef      sdk.String  `nexa:"string=Fixed Asset Reference"`

	Notes  sdk.Text    `nexa:"string=Notes"`
	Active sdk.Boolean `nexa:"string=Active,default=true"`

	// Backwards compatibility fields
	Brand  sdk.String                `nexa:"string=Brand (Legacy)"`
	Series sdk.String                `nexa:"string=Model (Legacy)"`
	State  sdk.Selection[FleetState] `nexa:"string=Status (Legacy),default=active"`
}

// FleetOdometerReading tracks cumulative distance counter snapshots.
type FleetOdometerReading struct {
	sdk.Model `nexa:"model=fleet.odometer.reading"`

	VehicleID        sdk.Many2One[FleetVehicle]    `nexa:"required,index,string=Vehicle"`
	ReadingDate      sdk.DateTime                  `nexa:"required,string=Reading Date"`
	Value            sdk.Integer                   `nexa:"required,string=Odometer Value,default=0"`
	Unit             sdk.String                    `nexa:"string=Unit,default=km"`
	Difference       sdk.Integer                   `nexa:"string=Difference,default=0"`
	Source           sdk.Selection[OdometerSource] `nexa:"string=Source,default=manual"`
	RecordedBy       sdk.Many2One[CoreUser]        `nexa:"string=Recorded By"`
	IsCorrection     sdk.Boolean                   `nexa:"string=Is Correction,default=false"`
	RolloverDetected sdk.Boolean                   `nexa:"string=Rollover Detected,default=false"`
	Notes            sdk.Text                      `nexa:"string=Notes"`
}

// FleetVehicleStatusHistory records state audit transitions.
type FleetVehicleStatusHistory struct {
	sdk.Model `nexa:"model=fleet.vehicle.status.history"`

	VehicleID     sdk.Many2One[FleetVehicle] `nexa:"required,index,string=Vehicle"`
	ChangedAt     sdk.DateTime               `nexa:"required,string=Changed At"`
	ChangedBy     sdk.Many2One[CoreUser]     `nexa:"required,string=Changed By"`
	FromLifecycle sdk.String                 `nexa:"string=From Lifecycle"`
	ToLifecycle   sdk.String                 `nexa:"string=To Lifecycle"`
	FromCondition sdk.String                 `nexa:"string=From Condition"`
	ToCondition   sdk.String                 `nexa:"string=To Condition"`
	Reason        sdk.Text                   `nexa:"string=Reason"`
	Notes         sdk.Text                   `nexa:"string=Notes"`
}

// FleetService provides backward-compatible maintenance/service log entries.
type FleetService struct {
	sdk.Model `nexa:"model=fleet.service"`

	Name        sdk.String                 `nexa:"required,string=Description"`
	VehicleID   sdk.Many2One[FleetVehicle] `nexa:"required,string=Vehicle"`
	ServiceDate sdk.Date                   `nexa:"string=Service Date"`
	OdometerAt  sdk.Integer                `nexa:"string=Odometer"`
	Cost        sdk.Numeric                `nexa:"string=Cost,precision=18,scale=2"`
	Notes       sdk.Text                   `nexa:"string=Notes"`
}

func init() {
	sdk.MustRegister("fleet",
		&FleetVehicleBrand{},
		&FleetVehicleModel{},
		&FleetVehicleClass{},
		&FleetDepot{},
		&FleetVehicle{},
		&FleetOdometerReading{},
		&FleetVehicleStatusHistory{},
		&FleetService{},
	)
}
