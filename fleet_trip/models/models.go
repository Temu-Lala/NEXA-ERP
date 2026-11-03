package models

import (
	basemodels "nexa/addons/base/models"
	fleetdrivermodels "nexa_addons/fleet_driver/models"
	fleetmodels "nexa_addons/fleet/models"
	"nexa/core/sdk"
)

// Cross-module imports aliased at top
type CoreUser = basemodels.CoreUser
type FleetVehicle = fleetmodels.FleetVehicle
type FleetDriver = fleetdrivermodels.FleetDriver

// ---- Selection Types -------------------------------------------------------

type TripState string

const (
	TripDraft      TripState = "draft"
	TripSubmitted  TripState = "submitted"
	TripApproved   TripState = "approved"
	TripScheduled  TripState = "scheduled"
	TripDispatched TripState = "dispatched"
	TripInProgress TripState = "in_progress"
	TripReturned   TripState = "returned"
	TripReconciled TripState = "reconciled"
	TripClosed     TripState = "closed"
	TripCancelled  TripState = "cancelled"
)

type TripPriority string

const (
	TripPriorityNormal    TripPriority = "normal"
	TripPriorityUrgent    TripPriority = "urgent"
	TripPriorityEmergency TripPriority = "emergency"
)

type TripPurpose string

const (
	PurposeCustomerVisit  TripPurpose = "customer_visit"
	PurposeDelivery       TripPurpose = "delivery"
	PurposeFieldService   TripPurpose = "field_service"
	PurposeStaffTransport TripPurpose = "staff_transport"
	PurposeInterBranch    TripPurpose = "inter_branch"
	PurposeTraining       TripPurpose = "training"
	PurposeOther          TripPurpose = "other"
)

type StopType string

const (
	StopPickup   StopType = "pickup"
	StopDelivery StopType = "delivery"
	StopWaypoint StopType = "waypoint"
	StopRest     StopType = "rest"
	StopService  StopType = "service"
)

type StopStatus string

const (
	StopPending   StopStatus = "pending"
	StopCompleted StopStatus = "completed"
	StopSkipped   StopStatus = "skipped"
	StopFailed    StopStatus = "failed"
)

// ---- Models ----------------------------------------------------------------

// FleetTrip represents an end-to-end vehicle excursion with authorization and reconciliation.
type FleetTrip struct {
	sdk.Model `nexa:"model=fleet.trip"`

	Name                sdk.String                  `nexa:"required,index,string=Trip Reference"`
	RequesterID         sdk.Many2One[CoreUser]      `nexa:"required,string=Requester"`
	Department          sdk.String                  `nexa:"string=Department"`
	Purpose             sdk.Selection[TripPurpose]  `nexa:"string=Purpose,default=field_service"`
	Priority            sdk.Selection[TripPriority] `nexa:"string=Priority,default=normal"`
	State               sdk.Selection[TripState]    `nexa:"required,index,string=Status,default=draft"`
	VehicleID           sdk.Many2One[FleetVehicle]  `nexa:"index,string=Vehicle"`
	DriverID            sdk.Many2One[FleetDriver]   `nexa:"index,string=Driver"`
	OriginAddress       sdk.String                  `nexa:"required,string=Origin Address"`
	DestinationAddress  sdk.String                  `nexa:"required,string=Destination Address"`
	PlannedDeparture    sdk.DateTime                `nexa:"required,string=Planned Departure"`
	PlannedReturn       sdk.DateTime                `nexa:"required,string=Planned Return"`
	ActualDeparture     sdk.DateTime                `nexa:"string=Actual Departure"`
	ActualReturn        sdk.DateTime                `nexa:"string=Actual Return"`
	EstimatedDistanceKm sdk.Numeric                 `nexa:"string=Estimated Distance (km),precision=18,scale=2,default=0"`
	ActualDistanceKm    sdk.Numeric                 `nexa:"string=Actual Distance (km),precision=18,scale=2,default=0"`
	StartOdometer       sdk.Integer                 `nexa:"string=Start Odometer,default=0"`
	EndOdometer         sdk.Integer                 `nexa:"string=End Odometer,default=0"`
	PassengerCount      sdk.Integer                 `nexa:"string=Passenger Count,default=1"`
	CargoDescription    sdk.Text                    `nexa:"string=Cargo Description"`
	CargoWeightKg       sdk.Numeric                 `nexa:"string=Cargo Weight (kg),precision=18,scale=2,default=0"`
	ApprovedBy          sdk.Many2One[CoreUser]      `nexa:"string=Approved By"`
	DispatchedBy        sdk.Many2One[CoreUser]      `nexa:"string=Dispatched By"`
	ReconciledBy        sdk.Many2One[CoreUser]      `nexa:"string=Reconciled By"`
	FuelCost            sdk.Numeric                 `nexa:"string=Fuel Cost,precision=18,scale=2,default=0"`
	TollCost            sdk.Numeric                 `nexa:"string=Toll Cost,precision=18,scale=2,default=0"`
	ParkingCost         sdk.Numeric                 `nexa:"string=Parking Cost,precision=18,scale=2,default=0"`
	OtherExpenses       sdk.Numeric                 `nexa:"string=Other Expenses,precision=18,scale=2,default=0"`
	TotalTripCost       sdk.Numeric                 `nexa:"string=Total Trip Cost,precision=18,scale=2,default=0"`
	DriverRating        sdk.Integer                 `nexa:"string=Driver Rating,default=5"`
	Notes               sdk.Text                    `nexa:"string=Notes"`
}

// FleetTripStop defines intermediate waypoints or delivery/service stops.
type FleetTripStop struct {
	sdk.Model `nexa:"model=fleet.trip.stop"`

	TripID            sdk.Many2One[FleetTrip]  `nexa:"required,index,string=Trip"`
	Sequence          sdk.Integer              `nexa:"string=Sequence,default=10"`
	StopType          sdk.Selection[StopType]  `nexa:"string=Stop Type,default=delivery"`
	LocationName      sdk.String               `nexa:"required,string=Location Name"`
	Address           sdk.String               `nexa:"string=Address"`
	ScheduledArrival  sdk.DateTime             `nexa:"string=Scheduled Arrival"`
	ActualArrival     sdk.DateTime             `nexa:"string=Actual Arrival"`
	ActualDeparture   sdk.DateTime             `nexa:"string=Actual Departure"`
	Status            sdk.Selection[StopStatus]`nexa:"string=Status,default=pending"`
	ContactPerson     sdk.String               `nexa:"string=Contact Person"`
	ContactPhone      sdk.String               `nexa:"string=Contact Phone"`
	Notes             sdk.Text                 `nexa:"string=Notes"`
}

func init() {
	sdk.MustRegister("fleet_trip",
		&FleetTrip{},
		&FleetTripStop{},
	)
}
