package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// Facilities: buildings, rooms/spaces, bookings and service requests.

type CoreUser = basemodels.CoreUser

// FacilitiesBuilding is a building or site.
type FacilitiesBuilding struct {
	sdk.Model `nexa:"model=facilities.building"`

	Name      sdk.String                   `nexa:"required,string=Building"`
	Address   sdk.Text                     `nexa:"string=Address"`
	Floors    sdk.Integer                  `nexa:"string=Floors,default=1"`
	ManagerID sdk.Many2One[CoreUser]       `nexa:"string=Manager"`
	State     sdk.Selection[BldState]      `nexa:"string=Status,default=active,selection=active:Active,inactive:Inactive"`
	Rooms     sdk.One2Many[FacilitiesRoom] `nexa:"string=Rooms"`
}

type BldState string

const (
	BldActive   BldState = "active"
	BldInactive BldState = "inactive"
)

// FacilitiesRoom is a bookable space (meeting room, desk, lab).
type FacilitiesRoom struct {
	sdk.Model `nexa:"model=facilities.room"`

	Name       sdk.String                       `nexa:"required,string=Room/Space"`
	BuildingID sdk.Many2One[FacilitiesBuilding] `nexa:"required,index,string=Building"`
	Kind       sdk.String                       `nexa:"string=Room Type"`
	Floor      sdk.String                       `nexa:"string=Floor"`
	Capacity   sdk.Integer                      `nexa:"string=Capacity,default=0"`
	Equipment  sdk.String                       `nexa:"string=Equipment"`
	State      sdk.Selection[RoomState]         `nexa:"string=Status,default=active,selection=active:Active,maintenance:Maintenance,out_of_service:Out of Service"`
	Bookings   sdk.One2Many[FacilitiesBooking]  `nexa:"string=Bookings"`
}

type RoomState string

const (
	RoomActive       RoomState = "active"
	RoomMaintenance  RoomState = "maintenance"
	RoomOutOfService RoomState = "out_of_service"
)

// FacilitiesBooking reserves a room for a time window.
type FacilitiesBooking struct {
	sdk.Model `nexa:"model=facilities.booking"`

	Name      sdk.String                   `nexa:"required,string=Booking"`
	RoomID    sdk.Many2One[FacilitiesRoom] `nexa:"required,index,string=Room"`
	UserID    sdk.Many2One[CoreUser]       `nexa:"required,string=Booked By"`
	Purpose   sdk.String                   `nexa:"string=Purpose"`
	StartAt   sdk.DateTime                 `nexa:"required,string=Start"`
	EndAt     sdk.DateTime                 `nexa:"required,string=End"`
	Attendees sdk.Integer                  `nexa:"string=Attendees,default=1"`
	State     sdk.Selection[BkgState]      `nexa:"string=Status,default=tentative,selection=tentative:Tentative,confirmed:Confirmed,cancelled:Cancelled,completed:Completed"`
}

type BkgState string

const (
	BkgTentative BkgState = "tentative"
	BkgConfirmed BkgState = "confirmed"
	BkgCancelled BkgState = "cancelled"
	BkgCompleted BkgState = "completed"
)

// FacilitiesRequest is a service request (cleaning, security, repair...).
type FacilitiesRequest struct {
	sdk.Model `nexa:"model=facilities.request"`

	Name          sdk.String                       `nexa:"required,string=Request"`
	BuildingID    sdk.Many2One[FacilitiesBuilding] `nexa:"required,index,string=Building"`
	RoomID        sdk.Many2One[FacilitiesRoom]     `nexa:"string=Room/Space"`
	Kind          sdk.Selection[ReqKind]           `nexa:"required,string=Type,selection=cleaning:Cleaning,security:Security,repair:Repair,maintenance:Maintenance,other:Other"`
	Description   sdk.Text                         `nexa:"string=Description"`
	Priority      sdk.Selection[ReqPri]            `nexa:"string=Priority,default=normal,selection=low:Low,normal:Normal,high:High,urgent:Urgent"`
	State         sdk.Selection[ReqState]          `nexa:"string=Status,default=open,selection=open:Open,assigned:Assigned,in_progress:In Progress,resolved:Resolved,cancelled:Cancelled"`
	RequestedByID sdk.Many2One[CoreUser]           `nexa:"string=Requested By"`
	AssignedToID  sdk.Many2One[CoreUser]           `nexa:"string=Assigned To"`
	ResolvedAt    sdk.DateTime                     `nexa:"string=Resolved At"`
}

type ReqKind string

const (
	ReqCleaning    ReqKind = "cleaning"
	ReqSecurity    ReqKind = "security"
	ReqRepair      ReqKind = "repair"
	ReqMaintenance ReqKind = "maintenance"
	ReqOther       ReqKind = "other"
)

type ReqPri string

const (
	ReqLow    ReqPri = "low"
	ReqNormal ReqPri = "normal"
	ReqHigh   ReqPri = "high"
	ReqUrgent ReqPri = "urgent"
)

type ReqState string

const (
	ReqOpen       ReqState = "open"
	ReqAssigned   ReqState = "assigned"
	ReqInProgress ReqState = "in_progress"
	ReqResolved   ReqState = "resolved"
	ReqCancelled  ReqState = "cancelled"
)

func init() {
	sdk.MustRegister("facilities",
		&FacilitiesBuilding{},
		&FacilitiesRoom{},
		&FacilitiesBooking{},
		&FacilitiesRequest{},
	)
}
