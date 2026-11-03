package models

import (
	basemodels "nexa/addons/base/models"
	fleetdrivermodels "nexa_addons/fleet_driver/models"
	fleetmodels "nexa_addons/fleet/models"
	"nexa/core/sdk"
)

// Cross-module aliases
type CoreUser = basemodels.CoreUser
type FleetVehicle = fleetmodels.FleetVehicle
type FleetDriver = fleetdrivermodels.FleetDriver

// DeviceProvider Selection: generic_gps, teltonika, geotab, samsara, calamp, obd2_adapter
type DeviceProvider string

const (
	DeviceProviderGenericGPS  DeviceProvider = "generic_gps"
	DeviceProviderTeltonika   DeviceProvider = "teltonika"
	DeviceProviderGeotab      DeviceProvider = "geotab"
	DeviceProviderSamsara     DeviceProvider = "samsara"
	DeviceProviderCalamp      DeviceProvider = "calamp"
	DeviceProviderOBD2Adapter DeviceProvider = "obd2_adapter"
)

// ConnectionStatus Selection: online, offline, no_signal
type ConnectionStatus string

const (
	ConnStatusOnline   ConnectionStatus = "online"
	ConnStatusOffline  ConnectionStatus = "offline"
	ConnStatusNoSignal ConnectionStatus = "no_signal"
)

// TelemetryEventType Selection: periodic, ignition_on, ignition_off, harsh_accel, harsh_brake, harsh_turn, overspeed, geofence_in, geofence_out, sos
type TelemetryEventType string

const (
	TelemetryTypePeriodic    TelemetryEventType = "periodic"
	TelemetryTypeIgnitionOn  TelemetryEventType = "ignition_on"
	TelemetryTypeIgnitionOff TelemetryEventType = "ignition_off"
	TelemetryTypeHarshAccel  TelemetryEventType = "harsh_accel"
	TelemetryTypeHarshBrake  TelemetryEventType = "harsh_brake"
	TelemetryTypeHarshTurn   TelemetryEventType = "harsh_turn"
	TelemetryTypeOverspeed   TelemetryEventType = "overspeed"
	TelemetryTypeGeofenceIn  TelemetryEventType = "geofence_in"
	TelemetryTypeGeofenceOut TelemetryEventType = "geofence_out"
	TelemetryTypeSOS         TelemetryEventType = "sos"
)

// ZoneType Selection: depot, branch, customer_site, restricted_area, border, fuel_station
type ZoneType string

const (
	ZoneTypeDepot          ZoneType = "depot"
	ZoneTypeBranch         ZoneType = "branch"
	ZoneTypeCustomerSite   ZoneType = "customer_site"
	ZoneTypeRestrictedArea ZoneType = "restricted_area"
	ZoneTypeBorder         ZoneType = "border"
	ZoneTypeFuelStation    ZoneType = "fuel_station"
)

// GeofenceShape Selection: circle, polygon
type GeofenceShape string

const (
	ShapeCircle  GeofenceShape = "circle"
	ShapePolygon GeofenceShape = "polygon"
)

// DrivingEventType Selection: overspeed, harsh_braking, harsh_acceleration, excessive_idling, after_hours_driving, cornering, seatbelt_unbuckled
type DrivingEventType string

const (
	DrivingEventOverspeed         DrivingEventType = "overspeed"
	DrivingEventHarshBraking      DrivingEventType = "harsh_braking"
	DrivingEventHarshAcceleration DrivingEventType = "harsh_acceleration"
	DrivingEventExcessiveIdling   DrivingEventType = "excessive_idling"
	DrivingEventAfterHours        DrivingEventType = "after_hours_driving"
	DrivingEventCornering         DrivingEventType = "cornering"
	DrivingEventSeatbelt          DrivingEventType = "seatbelt_unbuckled"
)

// EventSeverity Selection: info, warning, critical
type EventSeverity string

const (
	SeverityInfo     EventSeverity = "info"
	SeverityWarning  EventSeverity = "warning"
	SeverityCritical EventSeverity = "critical"
)

// FleetTelematicsDevice (fleet.telematics.device)
type FleetTelematicsDevice struct {
	sdk.Model `nexa:"model=fleet.telematics.device"`

	Name            sdk.String                      `nexa:"required,index,string=IMEI / Serial"`
	ModelName       sdk.String                      `nexa:"column=model,string=Model"`
	Provider        sdk.Selection[DeviceProvider]   `nexa:"string=Provider,default=generic_gps"`
	Status          sdk.Selection[ConnectionStatus] `nexa:"string=Connection Status,default=online"`
	VehicleID       sdk.Many2One[FleetVehicle]      `nexa:"index,string=Vehicle"`
	SIMCardNumber   sdk.String                      `nexa:"string=SIM Card Number"`
	FirmwareVersion sdk.String                      `nexa:"string=Firmware Version"`
	LastReportTime  sdk.DateTime                    `nexa:"string=Last Report Time"`
	Active          sdk.Boolean                     `nexa:"string=Active,default=true"`
	Notes           sdk.Text                        `nexa:"string=Notes"`
}

// FleetTelemetryEvent (fleet.telemetry.event)
type FleetTelemetryEvent struct {
	sdk.Model `nexa:"model=fleet.telemetry.event"`

	EventType      sdk.Selection[TelemetryEventType]   `nexa:"string=Event Type,default=periodic"`
	DeviceID       sdk.Many2One[FleetTelematicsDevice] `nexa:"index,string=Device"`
	VehicleID      sdk.Many2One[FleetVehicle]          `nexa:"required,index,string=Vehicle"`
	Timestamp      sdk.DateTime                        `nexa:"required,index,string=Timestamp"`
	Latitude       sdk.Numeric                         `nexa:"string=Latitude,precision=10,scale=7,default=0"`
	Longitude      sdk.Numeric                         `nexa:"string=Longitude,precision=10,scale=7,default=0"`
	Altitude       sdk.Numeric                         `nexa:"string=Altitude (m),precision=8,scale=2,default=0"`
	SpeedKmh       sdk.Numeric                         `nexa:"string=Speed (km/h),precision=6,scale=2,default=0"`
	HeadingDegrees sdk.Integer                         `nexa:"string=Heading (Degrees),default=0"`
	IgnitionOn     sdk.Boolean                         `nexa:"string=Ignition On,default=true"`
	OdometerKm     sdk.Integer                         `nexa:"string=Odometer (km),default=0"`
	FuelLevelPct   sdk.Numeric                         `nexa:"string=Fuel Level (%),precision=5,scale=2,default=0"`
	BatteryVolts   sdk.Numeric                         `nexa:"string=Battery (V),precision=5,scale=2,default=0"`
	EngineRPM      sdk.Integer                         `nexa:"string=Engine RPM,default=0"`
}

// FleetGeofence (fleet.geofence)
type FleetGeofence struct {
	sdk.Model `nexa:"model=fleet.geofence"`

	Name            sdk.String                   `nexa:"required,index,string=Geofence Name"`
	ZoneType        sdk.Selection[ZoneType]      `nexa:"string=Zone Type,default=depot"`
	Shape           sdk.Selection[GeofenceShape] `nexa:"string=Shape,default=circle"`
	CenterLat       sdk.Numeric                  `nexa:"string=Center Latitude,precision=10,scale=7,default=0"`
	CenterLng       sdk.Numeric                  `nexa:"string=Center Longitude,precision=10,scale=7,default=0"`
	RadiusMeters    sdk.Numeric                  `nexa:"string=Radius (m),precision=10,scale=2,default=0"`
	CoordinatesJSON sdk.Text                     `nexa:"string=Coordinates JSON"`
	SpeedLimitKmh   sdk.Integer                  `nexa:"string=Speed Limit (km/h),default=0"`
	AlertOnEntry    sdk.Boolean                  `nexa:"string=Alert on Entry,default=true"`
	AlertOnExit     sdk.Boolean                  `nexa:"string=Alert on Exit,default=true"`
	Active          sdk.Boolean                  `nexa:"string=Active,default=true"`
	Notes           sdk.Text                     `nexa:"string=Notes"`
}

// FleetDrivingEvent (fleet.driving.event)
type FleetDrivingEvent struct {
	sdk.Model `nexa:"model=fleet.driving.event"`

	DrivingEventType sdk.Selection[DrivingEventType] `nexa:"string=Driving Event Type,default=overspeed"`
	Severity         sdk.Selection[EventSeverity]    `nexa:"string=Severity,default=info"`
	VehicleID        sdk.Many2One[FleetVehicle]      `nexa:"required,index,string=Vehicle"`
	DriverID         sdk.Many2One[FleetDriver]       `nexa:"index,string=Driver"`
	Timestamp        sdk.DateTime                    `nexa:"required,index,string=Timestamp"`
	Latitude         sdk.Numeric                     `nexa:"string=Latitude,precision=10,scale=7,default=0"`
	Longitude        sdk.Numeric                     `nexa:"string=Longitude,precision=10,scale=7,default=0"`
	SpeedKmh         sdk.Numeric                     `nexa:"string=Speed (km/h),precision=6,scale=2,default=0"`
	LimitKmh         sdk.Numeric                     `nexa:"string=Limit (km/h),precision=6,scale=2,default=0"`
	DurationSeconds  sdk.Integer                     `nexa:"string=Duration (s),default=0"`
	Acknowledged     sdk.Boolean                     `nexa:"string=Acknowledged,default=false"`
	Notes            sdk.Text                        `nexa:"string=Notes"`
}

func init() {
	sdk.MustRegister("fleet_telematics",
		&FleetTelematicsDevice{},
		&FleetTelemetryEvent{},
		&FleetGeofence{},
		&FleetDrivingEvent{},
	)
}
