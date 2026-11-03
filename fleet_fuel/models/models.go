package models

import (
	basemodels "nexa/addons/base/models"
	fleetmodels "nexa_addons/fleet/models"
	fleetdrivermodels "nexa_addons/fleet_driver/models"
	fleettripmodels "nexa_addons/fleet_trip/models"
	"nexa/core/sdk"
)

// Cross-module aliases
type (
	CoreUser     = basemodels.CoreUser
	FleetVehicle = fleetmodels.FleetVehicle
	FleetDriver  = fleetdrivermodels.FleetDriver
	FleetTrip    = fleettripmodels.FleetTrip
)

// ---- Selection Types -------------------------------------------------------

type FuelCardStatus string

const (
	CardActive  FuelCardStatus = "active"
	CardBlocked FuelCardStatus = "blocked"
	CardExpired FuelCardStatus = "expired"
)

type FuelFillType string

const (
	FillFullTank FuelFillType = "full_tank"
	FillPartial  FuelFillType = "partial"
)

type FuelTxSource string

const (
	TxSourceManual     FuelTxSource = "manual"
	TxSourceCardImport FuelTxSource = "card_import"
	TxSourceTelematics FuelTxSource = "telematics"
)

type FuelTxStatus string

const (
	TxDraft      FuelTxStatus = "draft"
	TxVerified   FuelTxStatus = "verified"
	TxReconciled FuelTxStatus = "reconciled"
	TxFlagged    FuelTxStatus = "flagged"
)

type AnomalySeverity string

const (
	SeverityInfo     AnomalySeverity = "info"
	SeverityWarning  AnomalySeverity = "warning"
	SeverityCritical AnomalySeverity = "critical"
)

type AnomalyStatus string

const (
	AnomalyOpen            AnomalyStatus = "open"
	AnomalyUnderReview     AnomalyStatus = "under_review"
	AnomalyClearedFalsePos AnomalyStatus = "cleared_false_positive"
	AnomalyConfirmedFraud  AnomalyStatus = "confirmed_fraud"
)

// ---- Models ----------------------------------------------------------------

// FleetFuelCard tracks company fuel cards and assigned vehicles/drivers.
type FleetFuelCard struct {
	sdk.Model `nexa:"model=fleet.fuel.card"`

	CardNumber        sdk.String                    `nexa:"required,index,string=Card Number"`
	Provider          sdk.String                    `nexa:"required,string=Provider"`
	AssignedVehicleID sdk.Many2One[FleetVehicle]    `nexa:"string=Assigned Vehicle"`
	AssignedDriverID  sdk.Many2One[FleetDriver]     `nexa:"string=Assigned Driver"`
	ExpiryDate        sdk.Date                      `nexa:"string=Expiry Date"`
	DailyLimit        sdk.Numeric                   `nexa:"string=Daily Limit,precision=18,scale=2,default=0"`
	MonthlyLimit      sdk.Numeric                   `nexa:"string=Monthly Limit,precision=18,scale=2,default=0"`
	Status            sdk.Selection[FuelCardStatus] `nexa:"string=Status,default=active"`
	Notes             sdk.Text                      `nexa:"string=Notes"`
}

// FleetFuelTransaction records fuel purchases, economy calculations, and verification.
type FleetFuelTransaction struct {
	sdk.Model `nexa:"model=fleet.fuel.transaction"`

	Name              sdk.String                  `nexa:"required,index,string=Transaction Number"`
	VehicleID         sdk.Many2One[FleetVehicle]  `nexa:"required,index,string=Vehicle"`
	DriverID          sdk.Many2One[FleetDriver]   `nexa:"index,string=Driver"`
	TripID            sdk.Many2One[FleetTrip]     `nexa:"index,string=Trip"`
	FuelCardID        sdk.Many2One[FleetFuelCard] `nexa:"index,string=Fuel Card"`
	StationName       sdk.String                  `nexa:"string=Station Name"`
	StationLocation   sdk.String                  `nexa:"string=Station Location"`
	Timestamp         sdk.DateTime                `nexa:"required,string=Timestamp"`
	Odometer          sdk.Integer                 `nexa:"string=Odometer,default=0"`
	FuelType          sdk.String                  `nexa:"string=Fuel Type,default=diesel"`
	QuantityLitres    sdk.Numeric                 `nexa:"required,string=Quantity (Litres),precision=18,scale=2,default=0"`
	UnitPrice         sdk.Numeric                 `nexa:"required,string=Unit Price,precision=18,scale=2,default=0"`
	TotalAmount       sdk.Numeric                 `nexa:"required,string=Total Amount,precision=18,scale=2,default=0"`
	Currency          sdk.String                  `nexa:"string=Currency,default=USD"`
	FillType          sdk.Selection[FuelFillType] `nexa:"string=Fill Type,default=full_tank"`
	PaymentMethod     sdk.String                  `nexa:"string=Payment Method,default=fuel_card"`
	ReceiptRef        sdk.String                  `nexa:"string=Receipt Reference"`
	Source            sdk.Selection[FuelTxSource] `nexa:"string=Source,default=manual"`
	FuelEconomyL100Km sdk.Numeric                 `nexa:"string=Fuel Economy (L/100km),precision=18,scale=2,default=0"`
	CostPerKm         sdk.Numeric                 `nexa:"string=Cost Per Km,precision=18,scale=2,default=0"`
	Status            sdk.Selection[FuelTxStatus] `nexa:"string=Status,default=verified"`
	FlagReason        sdk.String                  `nexa:"string=Flag Reason"`
	Notes             sdk.Text                    `nexa:"string=Notes"`
}

// FleetChargingSession tracks EV and hybrid charging events and energy costs.
type FleetChargingSession struct {
	sdk.Model `nexa:"model=fleet.charging.session"`

	Name               sdk.String                 `nexa:"required,index,string=Session Number"`
	VehicleID          sdk.Many2One[FleetVehicle] `nexa:"required,index,string=Vehicle"`
	DriverID           sdk.Many2One[FleetDriver]  `nexa:"index,string=Driver"`
	ChargingStation    sdk.String                 `nexa:"string=Charging Station"`
	Location           sdk.String                 `nexa:"string=Location"`
	StartTime          sdk.DateTime               `nexa:"required,string=Start Time"`
	EndTime            sdk.DateTime               `nexa:"string=End Time"`
	DurationMinutes    sdk.Integer                `nexa:"string=Duration (Minutes),default=0"`
	StartSOCPct        sdk.Numeric                `nexa:"string=Start SOC (%),precision=5,scale=2,default=0"`
	EndSOCPct          sdk.Numeric                `nexa:"string=End SOC (%),precision=5,scale=2,default=0"`
	EnergyDeliveredKWh sdk.Numeric                `nexa:"required,string=Energy Delivered (kWh),precision=18,scale=2,default=0"`
	UnitPriceKWh       sdk.Numeric                `nexa:"string=Unit Price (per kWh),precision=18,scale=2,default=0"`
	TotalCost          sdk.Numeric                `nexa:"string=Total Cost,precision=18,scale=2,default=0"`
	ConnectorType      sdk.String                 `nexa:"string=Connector Type"`
	Status             sdk.String                 `nexa:"string=Status,default=completed"`
	Notes              sdk.Text                   `nexa:"string=Notes"`
}

// FleetFuelAnomaly flags suspicious transactions (e.g. over capacity, fraud, odometer rollback).
type FleetFuelAnomaly struct {
	sdk.Model `nexa:"model=fleet.fuel.anomaly"`

	Name            sdk.String                         `nexa:"required,index,string=Anomaly ID"`
	VehicleID       sdk.Many2One[FleetVehicle]         `nexa:"required,index,string=Vehicle"`
	TransactionID   sdk.Many2One[FleetFuelTransaction] `nexa:"index,string=Fuel Transaction"`
	RuleTriggered   sdk.String                         `nexa:"required,string=Rule Triggered"`
	Severity        sdk.Selection[AnomalySeverity]     `nexa:"string=Severity,default=warning"`
	ConfidenceScore sdk.Numeric                        `nexa:"string=Confidence Score,precision=5,scale=2,default=0"`
	Status          sdk.Selection[AnomalyStatus]       `nexa:"string=Status,default=open"`
	ReviewerID      sdk.Many2One[CoreUser]             `nexa:"string=Reviewer"`
	ResolutionNotes sdk.Text                           `nexa:"string=Resolution Notes"`
}

func init() {
	sdk.MustRegister("fleet_fuel",
		&FleetFuelCard{},
		&FleetFuelTransaction{},
		&FleetChargingSession{},
		&FleetFuelAnomaly{},
	)
}
