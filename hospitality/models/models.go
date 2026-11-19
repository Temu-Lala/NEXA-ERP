package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// Hospitality: properties, room types, rooms, reservations and guest folio
// charges. ADR / RevPAR / occupancy are live computes on the property.

type CoreUser = basemodels.CoreUser

// HospitalityProperty is a hotel or serviced accommodation.
type HospitalityProperty struct {
	sdk.Model `nexa:"model=hospitality.property"`

	Name          sdk.String                        `nexa:"required,string=Property"`
	Ref           sdk.String                        `nexa:"string=Reference"`
	Address       sdk.Text                          `nexa:"string=Address"`
	City          sdk.String                        `nexa:"string=City"`
	Stars         sdk.Integer                       `nexa:"string=Stars,default=0"`
	ManagerID     sdk.Many2One[CoreUser]            `nexa:"string=Manager"`
	State         sdk.Selection[PropState]          `nexa:"string=Status,default=active,selection=active:Active,inactive:Inactive,closed:Closed"`
	RoomTypes     sdk.One2Many[HospitalityRoomType] `nexa:"string=Room Types"`
	OccupancyRate sdk.Numeric                       `nexa:"string=Occupancy %,precision=18,scale=2,default=0"`
	ADR           sdk.Numeric                       `nexa:"string=ADR,precision=18,scale=2,default=0"`
	RevPAR        sdk.Numeric                       `nexa:"string=RevPAR,precision=18,scale=2,default=0"`
}

type PropState string

const (
	HPropActive   PropState = "active"
	HPropInactive PropState = "inactive"
	HPropClosed   PropState = "closed"
)

// HospitalityRoomType defines a sellable category with a base rate.
type HospitalityRoomType struct {
	sdk.Model `nexa:"model=hospitality.room.type"`

	Name       sdk.String                        `nexa:"required,string=Room Type"`
	PropertyID sdk.Many2One[HospitalityProperty] `nexa:"required,index,string=Property"`
	Capacity   sdk.Integer                       `nexa:"string=Max Guests,default=2"`
	BaseRate   sdk.Numeric                       `nexa:"string=Base Rate/night,precision=18,scale=2,default=0"`
	Rooms      sdk.One2Many[HospitalityRoom]     `nexa:"string=Rooms"`
}

// HospitalityRoom is one physical room.
type HospitalityRoom struct {
	sdk.Model `nexa:"model=hospitality.room"`

	Name       sdk.String                        `nexa:"required,string=Room"`
	PropertyID sdk.Many2One[HospitalityProperty] `nexa:"required,index,string=Property"`
	RoomTypeID sdk.Many2One[HospitalityRoomType] `nexa:"required,index,string=Type"`
	Number     sdk.String                        `nexa:"string=Room Number"`
	Floor      sdk.String                        `nexa:"string=Floor"`
	State      sdk.Selection[RoomState]          `nexa:"string=Status,default=active,selection=active:Active,maintenance:Maintenance,out_of_order:Out of Order"`
}

type RoomState string

const (
	RoomActive      RoomState = "active"
	RoomMaintenance RoomState = "maintenance"
	RoomOutOfOrder  RoomState = "out_of_order"
)

// HospitalityReservation is a booking for a room over a date range.
type HospitalityReservation struct {
	sdk.Model `nexa:"model=hospitality.reservation"`

	Name         sdk.String                        `nexa:"required,string=Confirmation"`
	PropertyID   sdk.Many2One[HospitalityProperty] `nexa:"required,index,string=Property"`
	RoomID       sdk.Many2One[HospitalityRoom]     `nexa:"required,index,string=Room"`
	GuestID      sdk.Many2One[sdk.Any]             `nexa:"string=Guest,comodel=core.partner"`
	GuestName    sdk.String                        `nexa:"string=Guest Name"`
	CheckIn      sdk.Date                          `nexa:"string=Check-in"`
	CheckOut     sdk.Date                          `nexa:"string=Check-out"`
	RatePerNight sdk.Numeric                       `nexa:"string=Rate/night,precision=18,scale=2,default=0"`
	Nights       sdk.Integer                       `nexa:"string=Nights,default=1"`
	TotalAmount  sdk.Numeric                       `nexa:"string=Total,precision=18,scale=2,default=0"`
	State        sdk.Selection[ResState]           `nexa:"string=Status,default=tentative,selection=tentative:Tentative,confirmed:Confirmed,checked_in:Checked In,checked_out:Checked Out,cancelled:Cancelled,no_show:No Show"`
	Source       sdk.String                        `nexa:"string=Source"`
	Note         sdk.Text                          `nexa:"string=Note"`
	Charges      sdk.One2Many[HospitalityCharge]   `nexa:"string=Folio Charges"`
}

type ResState string

const (
	ResTentative  ResState = "tentative"
	ResConfirmed  ResState = "confirmed"
	ResCheckedIn  ResState = "checked_in"
	ResCheckedOut ResState = "checked_out"
	ResCancelled  ResState = "cancelled"
	ResNoShow     ResState = "no_show"
)

// HospitalityCharge is one folio line against a reservation (room, F&B,
// incidentals) — the guest bill.
type HospitalityCharge struct {
	sdk.Model `nexa:"model=hospitality.charge"`

	ReservationID sdk.Many2One[HospitalityReservation] `nexa:"required,index,ondelete=cascade,string=Reservation"`
	Department    sdk.Selection[ChargeDept]            `nexa:"string=Department,default=room,selection=room:Room,fb:Food & Beverage,incidentals:Incidentals,service:Service,other:Other"`
	Description   sdk.String                           `nexa:"required,string=Description"`
	Quantity      sdk.Numeric                          `nexa:"string=Qty,precision=18,scale=2,default=1"`
	UnitPrice     sdk.Numeric                          `nexa:"string=Unit Price,precision=18,scale=2,default=0"`
	Amount        sdk.Numeric                          `nexa:"string=Amount,precision=18,scale=2,default=0"`
	ChargeDate    sdk.Date                             `nexa:"string=Date"`
	Posted        sdk.Boolean                          `nexa:"string=Posted,default=false"`
}

type ChargeDept string

const (
	DeptRoom        ChargeDept = "room"
	DeptFB          ChargeDept = "fb"
	DeptIncidentals ChargeDept = "incidentals"
	DeptService     ChargeDept = "service"
	DeptOther       ChargeDept = "other"
)

// ---------------------------------------------------------------------------
// Housekeeping + channel manager (spec §8.2).
// ---------------------------------------------------------------------------

// HospitalityHousekeepingTask is a room servicing job.
type HospitalityHousekeepingTask struct {
	sdk.Model `nexa:"model=hospitality.housekeeping"`

	Name         sdk.String                    `nexa:"required,string=Task"`
	RoomID       sdk.Many2One[HospitalityRoom] `nexa:"required,index,string=Room"`
	Kind         sdk.Selection[HKKind]         `nexa:"string=Type,default=cleaning,selection=cleaning:Cleaning,inspection:Inspection,maintenance:Maintenance,turndown:Turndown,deep_clean:Deep Clean"`
	Priority     sdk.Selection[HKPri]          `nexa:"string=Priority,default=normal,selection=low:Low,normal:Normal,high:High,urgent:Urgent"`
	State        sdk.Selection[HKState]        `nexa:"string=Status,default=pending,selection=pending:Pending,assigned:Assigned,in_progress:In Progress,done:Done,cancelled:Cancelled"`
	AssignedToID sdk.Many2One[CoreUser]        `nexa:"string=Assigned To"`
	DueAt        sdk.DateTime                  `nexa:"string=Due At"`
	DoneAt       sdk.DateTime                  `nexa:"string=Done At"`
	Note         sdk.Text                      `nexa:"string=Note"`
}

type HKKind string

const (
	HKCleaning    HKKind = "cleaning"
	HKInspection  HKKind = "inspection"
	HKMaintenance HKKind = "maintenance"
	HKTurndown    HKKind = "turndown"
	HKDeepClean   HKKind = "deep_clean"
)

type HKPri string

const (
	HKLow    HKPri = "low"
	HKNormal HKPri = "normal"
	HKHigh   HKPri = "high"
	HKUrgent HKPri = "urgent"
)

type HKState string

const (
	HKPending    HKState = "pending"
	HKAssigned   HKState = "assigned"
	HKInProgress HKState = "in_progress"
	HKDone       HKState = "done"
	HKCancelled  HKState = "cancelled"
)

// HospitalityChannel is a booking source.
type HospitalityChannel struct {
	sdk.Model `nexa:"model=hospitality.channel"`

	Name          sdk.String  `nexa:"required,string=Channel"`
	Code          sdk.String  `nexa:"string=Code"`
	Kind          sdk.String  `nexa:"string=Channel Type"`
	CommissionPct sdk.Numeric `nexa:"string=Commission %,precision=18,scale=2,default=0"`
	Active        sdk.Boolean `nexa:"string=Active,default=true"`
}

func init() {
	sdk.MustRegister("hospitality",
		&HospitalityProperty{},
		&HospitalityRoomType{},
		&HospitalityRoom{},
		&HospitalityReservation{},
		&HospitalityCharge{},
		&HospitalityHousekeepingTask{},
		&HospitalityChannel{},
	)
}
