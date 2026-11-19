package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// Real estate: property portfolio, units, leases and rent payments.

type CoreUser = basemodels.CoreUser

// RealEstateProperty is a building or land parcel.
type RealEstateProperty struct {
	sdk.Model `nexa:"model=realestate.property"`

	Name          sdk.String                   `nexa:"required,string=Property"`
	Ref           sdk.String                   `nexa:"string=Reference"`
	Kind          sdk.Selection[PropKind]      `nexa:"string=Type,default=residential,selection=residential:Residential,commercial:Commercial,industrial:Industrial,land:Land,mixed:Mixed Use"`
	Address       sdk.Text                     `nexa:"string=Address"`
	City          sdk.String                   `nexa:"string=City"`
	OwnerID       sdk.Many2One[sdk.Any]        `nexa:"string=Owner,comodel=core.partner"`
	ManagerID     sdk.Many2One[CoreUser]       `nexa:"string=Manager"`
	State         sdk.Selection[PropState]     `nexa:"string=Status,default=active,selection=active:Active,inactive:Inactive,sold:Sold,demolished:Demolished"`
	PurchasePrice sdk.Numeric                  `nexa:"string=Purchase Price,precision=18,scale=2,default=0"`
	Units         sdk.One2Many[RealEstateUnit] `nexa:"string=Units"`
}

type PropKind string

const (
	PropResidential PropKind = "residential"
	PropCommercial  PropKind = "commercial"
	PropIndustrial  PropKind = "industrial"
	PropLand        PropKind = "land"
	PropMixed       PropKind = "mixed"
)

type PropState string

const (
	PropActive     PropState = "active"
	PropInactive   PropState = "inactive"
	PropSold       PropState = "sold"
	PropDemolished PropState = "demolished"
)

// RealEstateUnit is one rentable space inside a property.
type RealEstateUnit struct {
	sdk.Model `nexa:"model=realestate.unit"`

	Name       sdk.String                       `nexa:"required,string=Unit"`
	PropertyID sdk.Many2One[RealEstateProperty] `nexa:"required,index,string=Property"`
	UnitType   sdk.String                       `nexa:"string=Unit Type"`
	Beds       sdk.Integer                      `nexa:"string=Bedrooms,default=0"`
	Baths      sdk.Integer                      `nexa:"string=Bathrooms,default=0"`
	AreaM2     sdk.Numeric                      `nexa:"string=Area (m²),precision=18,scale=2,default=0"`
	RentAmount sdk.Numeric                      `nexa:"string=Monthly Rent,precision=18,scale=2,default=0"`
	State      sdk.Selection[UnitState]         `nexa:"string=Status,default=vacant,selection=vacant:Vacant,occupied:Occupied,reserved:Reserved,maintenance:Maintenance"`
	Floor      sdk.String                       `nexa:"string=Floor"`
}

type UnitState string

const (
	UnitVacant      UnitState = "vacant"
	UnitOccupied    UnitState = "occupied"
	UnitReserved    UnitState = "reserved"
	UnitMaintenance UnitState = "maintenance"
)

// RealEstateLease is a rental agreement on a unit.
type RealEstateLease struct {
	sdk.Model `nexa:"model=realestate.lease"`

	Name          sdk.String                          `nexa:"required,string=Lease Reference"`
	UnitID        sdk.Many2One[RealEstateUnit]        `nexa:"required,index,string=Unit"`
	TenantID      sdk.Many2One[sdk.Any]               `nexa:"required,string=Tenant,comodel=core.partner"`
	State         sdk.Selection[LeaseState]           `nexa:"string=Status,default=draft,selection=draft:Draft,active:Active,expired:Expired,terminated:Terminated"`
	StartDate     sdk.Date                            `nexa:"string=Start Date"`
	EndDate       sdk.Date                            `nexa:"string=End Date"`
	RentAmount    sdk.Numeric                         `nexa:"string=Monthly Rent,precision=18,scale=2,default=0"`
	Deposit       sdk.Numeric                         `nexa:"string=Deposit,precision=18,scale=2,default=0"`
	RentDueDay    sdk.Integer                         `nexa:"string=Rent Due Day,default=1"`
	EscalationPct sdk.Numeric                         `nexa:"string=Annual Escalation %,precision=18,scale=2,default=0"`
	Payments      sdk.One2Many[RealEstateRentPayment] `nexa:"string=Payments"`
}

type LeaseState string

const (
	LeaseDraft      LeaseState = "draft"
	LeaseActive     LeaseState = "active"
	LeaseExpired    LeaseState = "expired"
	LeaseTerminated LeaseState = "terminated"
)

// RealEstateRentPayment is one rent collection against a lease period.
type RealEstateRentPayment struct {
	sdk.Model `nexa:"model=realestate.rent.payment"`

	LeaseID   sdk.Many2One[RealEstateLease] `nexa:"required,index,ondelete=cascade,string=Lease"`
	Period    sdk.String                    `nexa:"string=Period,help='YYYY-MM'"`
	DueDate   sdk.Date                      `nexa:"string=Due Date"`
	Amount    sdk.Numeric                   `nexa:"string=Amount,precision=18,scale=2,default=0"`
	PaidDate  sdk.Date                      `nexa:"string=Paid Date"`
	State     sdk.Selection[RentPayState]   `nexa:"string=Status,default=pending,selection=pending:Pending,paid:Paid,partial:Partial,overdue:Overdue,waived:Waived"`
	Reference sdk.String                    `nexa:"string=Payment Reference"`
	Note      sdk.String                    `nexa:"string=Note"`
}

type RentPayState string

const (
	RentPending RentPayState = "pending"
	RentPaid    RentPayState = "paid"
	RentPartial RentPayState = "partial"
	RentOverdue RentPayState = "overdue"
	RentWaived  RentPayState = "waived"
)

func init() {
	sdk.MustRegister("real_estate",
		&RealEstateProperty{},
		&RealEstateUnit{},
		&RealEstateLease{},
		&RealEstateRentPayment{},
	)
}
