package models

import (
	fleetmodels "nexa_addons/fleet/models"
	"nexa/core/sdk"
)

// Cross-module aliases
type FleetVehicle = fleetmodels.FleetVehicle

// ContractType Selection: operating_lease, finance_lease, rental, sale_and_leaseback
type ContractType string

const (
	ContractTypeOperatingLease   ContractType = "operating_lease"
	ContractTypeFinanceLease     ContractType = "finance_lease"
	ContractTypeRental           ContractType = "rental"
	ContractTypeSaleAndLeaseback ContractType = "sale_and_leaseback"
)

// ContractState Selection: draft, active, expiring_soon, returned, terminated
type ContractState string

const (
	ContractStateDraft        ContractState = "draft"
	ContractStateActive       ContractState = "active"
	ContractStateExpiringSoon ContractState = "expiring_soon"
	ContractStateReturned     ContractState = "returned"
	ContractStateTerminated   ContractState = "terminated"
)

// FleetLeaseContract (fleet.lease.contract)
type FleetLeaseContract struct {
	sdk.Model `nexa:"model=fleet.lease.contract"`

	Name                       sdk.String                   `nexa:"required,index,string=Contract Number"`
	LessorName                 sdk.String                   `nexa:"required,string=Lessor / Provider"`
	VehicleID                  sdk.Many2One[FleetVehicle]   `nexa:"required,index,string=Vehicle"`
	ContractType               sdk.Selection[ContractType]  `nexa:"string=Contract Type,default=operating_lease"`
	ContractState              sdk.Selection[ContractState] `nexa:"string=Status,default=draft"`
	StartDate                  sdk.Date                     `nexa:"required,string=Start Date"`
	EndDate                    sdk.Date                     `nexa:"required,index,string=End Date"`
	MonthlyRate                sdk.Numeric                  `nexa:"required,string=Monthly Rate,precision=18,scale=2,default=0"`
	Currency                   sdk.String                   `nexa:"string=Currency,default=USD"`
	MileageAllowanceTotalKm    sdk.Integer                  `nexa:"string=Total Mileage Allowance (km),default=0"`
	ExcessMileageRatePerKm     sdk.Numeric                  `nexa:"string=Excess Rate / km,precision=18,scale=2,default=0"`
	DepositAmount              sdk.Numeric                  `nexa:"string=Deposit Amount,precision=18,scale=2,default=0"`
	MaintenanceIncluded        sdk.Boolean                  `nexa:"string=Maintenance Included,default=false"`
	InsuranceIncluded          sdk.Boolean                  `nexa:"string=Insurance Included,default=false"`
	ReplacementVehicleEntitled sdk.Boolean                  `nexa:"string=Replacement Vehicle Entitled,default=false"`
	ReturnOdometer             sdk.Integer                  `nexa:"string=Return Odometer,default=0"`
	ExcessMileageCost          sdk.Numeric                  `nexa:"string=Excess Mileage Cost,precision=18,scale=2,default=0"`
	DamageDeductions           sdk.Numeric                  `nexa:"string=Damage Deductions,precision=18,scale=2,default=0"`
	ClosedDate                 sdk.Date                     `nexa:"string=Closed Date"`
	Notes                      sdk.Text                     `nexa:"string=Notes"`
}

func init() {
	sdk.MustRegister("fleet_lease",
		&FleetLeaseContract{},
	)
}
