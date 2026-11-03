package models

import (
	basemodels "nexa/addons/base/models"
	fleetmodels "nexa_addons/fleet/models"
	"nexa/core/sdk"
)

// Cross-module aliases
type CoreUser = basemodels.CoreUser
type FleetVehicle = fleetmodels.FleetVehicle

// TireType Selection: steer, drive, trailer, all_position
type TireType string

const (
	TireTypeSteer       TireType = "steer"
	TireTypeDrive       TireType = "drive"
	TireTypeTrailer     TireType = "trailer"
	TireTypeAllPosition TireType = "all_position"
)

// TireStatus Selection: in_stock, mounted, retreaded, scrapped
type TireStatus string

const (
	TireStatusInStock   TireStatus = "in_stock"
	TireStatusMounted   TireStatus = "mounted"
	TireStatusRetreaded TireStatus = "retreaded"
	TireStatusScrapped  TireStatus = "scrapped"
)

// WheelPosition Selection: steer_left, steer_right, drive1_left_outer, drive1_left_inner, drive1_right_inner, drive1_right_outer, trailer_left, trailer_right, spare
type WheelPosition string

const (
	WheelPosSteerLeft        WheelPosition = "steer_left"
	WheelPosSteerRight       WheelPosition = "steer_right"
	WheelPosDrive1LeftOuter  WheelPosition = "drive1_left_outer"
	WheelPosDrive1LeftInner  WheelPosition = "drive1_left_inner"
	WheelPosDrive1RightInner WheelPosition = "drive1_right_inner"
	WheelPosDrive1RightOuter WheelPosition = "drive1_right_outer"
	WheelPosTrailerLeft      WheelPosition = "trailer_left"
	WheelPosTrailerRight     WheelPosition = "trailer_right"
	WheelPosSpare            WheelPosition = "spare"
)

// RemovalReason Selection: none, normal_wear, puncture, retread, blowout, rotation
type RemovalReason string

const (
	RemovalReasonNone       RemovalReason = "none"
	RemovalReasonNormalWear RemovalReason = "normal_wear"
	RemovalReasonPuncture   RemovalReason = "puncture"
	RemovalReasonRetread    RemovalReason = "retread"
	RemovalReasonBlowout    RemovalReason = "blowout"
	RemovalReasonRotation   RemovalReason = "rotation"
)

// ActionRec Selection: continue, rotate, repair, replace
type ActionRec string

const (
	ActionRecContinue ActionRec = "continue"
	ActionRecRotate   ActionRec = "rotate"
	ActionRecRepair   ActionRec = "repair"
	ActionRecReplace  ActionRec = "replace"
)

// FleetTire (fleet.tire)
type FleetTire struct {
	sdk.Model `nexa:"model=fleet.tire"`

	Name                 sdk.String                `nexa:"required,index,string=Tire Code / Serial"`
	Brand                sdk.String                `nexa:"required,string=Brand"`
	ModelName            sdk.String                `nexa:"column=model,string=Model"`
	Size                 sdk.String                `nexa:"required,string=Size"`
	TireType             sdk.Selection[TireType]   `nexa:"string=Tire Type,default=all_position"`
	Status               sdk.Selection[TireStatus] `nexa:"string=Status,default=in_stock"`
	PurchaseDate         sdk.Date                  `nexa:"string=Purchase Date"`
	PurchaseCost         sdk.Numeric               `nexa:"string=Purchase Cost,precision=18,scale=2,default=0"`
	TreadDepthOriginalMm sdk.Numeric               `nexa:"string=Original Tread Depth (mm),precision=5,scale=2,default=16"`
	TreadDepthCurrentMm  sdk.Numeric               `nexa:"string=Current Tread Depth (mm),precision=5,scale=2,default=16"`
	WarrantyKm           sdk.Integer               `nexa:"string=Warranty (km),default=0"`
	Supplier             sdk.String                `nexa:"string=Supplier"`
	Notes                sdk.Text                  `nexa:"string=Notes"`
}

// FleetTireInstallation (fleet.tire.installation)
type FleetTireInstallation struct {
	sdk.Model `nexa:"model=fleet.tire.installation"`

	TireID          sdk.Many2One[FleetTire]      `nexa:"required,index,string=Tire"`
	VehicleID       sdk.Many2One[FleetVehicle]   `nexa:"required,index,string=Vehicle"`
	WheelPosition   sdk.Selection[WheelPosition] `nexa:"string=Wheel Position,default=steer_left"`
	AxleNumber      sdk.Integer                  `nexa:"string=Axle Number,default=1"`
	MountedDate     sdk.Date                     `nexa:"required,string=Mounted Date"`
	MountedOdometer sdk.Integer                  `nexa:"string=Mounted Odometer,default=0"`
	RemovedDate     sdk.Date                     `nexa:"string=Removed Date"`
	RemovedOdometer sdk.Integer                  `nexa:"string=Removed Odometer,default=0"`
	TotalKmRan      sdk.Integer                  `nexa:"string=Total Km Ran,default=0"`
	RemovalReason   sdk.Selection[RemovalReason] `nexa:"string=Removal Reason,default=none"`
	Notes           sdk.Text                     `nexa:"string=Notes"`
}

// FleetTireInspection (fleet.tire.inspection)
type FleetTireInspection struct {
	sdk.Model `nexa:"model=fleet.tire.inspection"`

	TireID               sdk.Many2One[FleetTire]    `nexa:"required,index,string=Tire"`
	VehicleID            sdk.Many2One[FleetVehicle] `nexa:"index,string=Vehicle"`
	InspectionDate       sdk.Date                   `nexa:"required,string=Inspection Date"`
	InspectorID          sdk.Many2One[CoreUser]     `nexa:"required,string=Inspector"`
	MeasuredTreadDepthMm sdk.Numeric                `nexa:"required,string=Measured Tread Depth (mm),precision=5,scale=2,default=0"`
	PressurePsi          sdk.Numeric                `nexa:"string=Pressure (PSI),precision=5,scale=2,default=0"`
	IrregularWear        sdk.String                 `nexa:"string=Irregular Wear,default=none"`
	DamageObserved       sdk.Text                   `nexa:"string=Damage Observed"`
	ActionRec            sdk.Selection[ActionRec]   `nexa:"string=Action Recommendation,default=continue"`
	Notes                sdk.Text                   `nexa:"string=Notes"`
}

func init() {
	sdk.MustRegister("fleet_tire",
		&FleetTire{},
		&FleetTireInstallation{},
		&FleetTireInspection{},
	)
}
