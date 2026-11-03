package models

import (
	basemodels "nexa/addons/base/models"
	fleetmodels "nexa_addons/fleet/models"
	fleettripmodels "nexa_addons/fleet_trip/models"
	"nexa/core/sdk"
)

// Cross-module aliases
type CoreUser = basemodels.CoreUser
type FleetVehicle = fleetmodels.FleetVehicle
type FleetTrip = fleettripmodels.FleetTrip

// ExpenseCategory Selection: fuel, service_repair, tires, insurance, registration_permits, lease_rent, tolls_parking, cleaning, towing_roadside, accident_repair, depreciation, other
type ExpenseCategory string

const (
	ExpenseCatFuel                ExpenseCategory = "fuel"
	ExpenseCatServiceRepair       ExpenseCategory = "service_repair"
	ExpenseCatTires               ExpenseCategory = "tires"
	ExpenseCatInsurance           ExpenseCategory = "insurance"
	ExpenseCatRegistrationPermits ExpenseCategory = "registration_permits"
	ExpenseCatLeaseRent           ExpenseCategory = "lease_rent"
	ExpenseCatTollsParking        ExpenseCategory = "tolls_parking"
	ExpenseCatCleaning            ExpenseCategory = "cleaning"
	ExpenseCatTowingRoadside      ExpenseCategory = "towing_roadside"
	ExpenseCatAccidentRepair      ExpenseCategory = "accident_repair"
	ExpenseCatDepreciation        ExpenseCategory = "depreciation"
	ExpenseCatOther               ExpenseCategory = "other"
)

// ExpenseStatus Selection: draft, submitted, approved, reconciled
type ExpenseStatus string

const (
	ExpenseStatusDraft      ExpenseStatus = "draft"
	ExpenseStatusSubmitted  ExpenseStatus = "submitted"
	ExpenseStatusApproved   ExpenseStatus = "approved"
	ExpenseStatusReconciled ExpenseStatus = "reconciled"
)

// PeriodType Selection: monthly, quarterly, annual, lifetime
type PeriodType string

const (
	PeriodMonthly   PeriodType = "monthly"
	PeriodQuarterly PeriodType = "quarterly"
	PeriodAnnual    PeriodType = "annual"
	PeriodLifetime  PeriodType = "lifetime"
)

// FleetVehicleExpense (fleet.vehicle.expense)
type FleetVehicleExpense struct {
	sdk.Model `nexa:"model=fleet.vehicle.expense"`

	Name            sdk.String                    `nexa:"required,index,string=Expense #"`
	VehicleID       sdk.Many2One[FleetVehicle]    `nexa:"required,index,string=Vehicle"`
	TripID          sdk.Many2One[FleetTrip]       `nexa:"index,string=Trip"`
	ExpenseCategory sdk.Selection[ExpenseCategory]`nexa:"string=Category,default=other"`
	ExpenseStatus   sdk.Selection[ExpenseStatus]  `nexa:"string=Status,default=draft"`
	ExpenseDate     sdk.Date                      `nexa:"required,string=Expense Date"`
	Amount          sdk.Numeric                   `nexa:"required,string=Pre-tax Amount,precision=18,scale=2,default=0"`
	TaxAmount       sdk.Numeric                   `nexa:"string=Tax Amount,precision=18,scale=2,default=0"`
	TotalAmount     sdk.Numeric                   `nexa:"required,string=Total Amount,precision=18,scale=2,default=0"`
	Currency        sdk.String                    `nexa:"string=Currency,default=USD"`
	CostCenter      sdk.String                    `nexa:"string=Cost Center"`
	Department      sdk.String                    `nexa:"string=Department"`
	VendorName      sdk.String                    `nexa:"string=Vendor Name"`
	InvoiceNumber   sdk.String                    `nexa:"string=Invoice Number"`
	ApprovedBy      sdk.Many2One[CoreUser]        `nexa:"string=Approved By"`
	Notes           sdk.Text                      `nexa:"string=Notes"`
}

// FleetTCOSummary (fleet.tco.summary)
type FleetTCOSummary struct {
	sdk.Model `nexa:"model=fleet.tco.summary"`

	VehicleID               sdk.Many2One[FleetVehicle] `nexa:"required,index,string=Vehicle"`
	PeriodType              sdk.Selection[PeriodType]  `nexa:"string=Period Type,default=monthly"`
	PeriodStart             sdk.Date                   `nexa:"required,string=Period Start"`
	PeriodEnd               sdk.Date                   `nexa:"required,string=Period End"`
	TotalDistanceKm         sdk.Numeric                `nexa:"string=Total Distance (km),precision=18,scale=2,default=0"`
	AcquisitionDepreciation sdk.Numeric                `nexa:"string=Acquisition / Depreciation,precision=18,scale=2,default=0"`
	FuelAndEnergyCost       sdk.Numeric                `nexa:"string=Fuel & Energy Cost,precision=18,scale=2,default=0"`
	MaintenanceRepairCost   sdk.Numeric                `nexa:"string=Maintenance & Repair Cost,precision=18,scale=2,default=0"`
	TireCost                sdk.Numeric                `nexa:"string=Tire Cost,precision=18,scale=2,default=0"`
	InsurancePermitCost     sdk.Numeric                `nexa:"string=Insurance & Permit Cost,precision=18,scale=2,default=0"`
	TollParkingCost         sdk.Numeric                `nexa:"string=Tolls & Parking Cost,precision=18,scale=2,default=0"`
	OtherOperatingCost      sdk.Numeric                `nexa:"string=Other Operating Cost,precision=18,scale=2,default=0"`
	TotalTCO                sdk.Numeric                `nexa:"string=Total TCO,precision=18,scale=2,default=0"`
	CostPerKm               sdk.Numeric                `nexa:"string=Total Cost / km,precision=18,scale=2,default=0"`
	FuelCostPerKm           sdk.Numeric                `nexa:"string=Fuel Cost / km,precision=18,scale=2,default=0"`
	MaintenanceCostPerKm    sdk.Numeric                `nexa:"string=Maintenance Cost / km,precision=18,scale=2,default=0"`
	ComputedAt              sdk.DateTime               `nexa:"string=Computed At"`
	Notes                   sdk.Text                   `nexa:"string=Notes"`
}

func init() {
	sdk.MustRegister("fleet_cost",
		&FleetVehicleExpense{},
		&FleetTCOSummary{},
	)
}
