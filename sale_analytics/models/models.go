package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// sale_analytics provides a period-based read-model for sales performance:
// targets, attainment, and forecasts per team or individual rep. Actual
// revenue figures are batch-refreshed from confirmed orders by a scheduled
// job (owned operationally); this module defines the structure, not the ETL.

type (
	CoreUser    = basemodels.CoreUser
	CoreCompany = basemodels.CoreCompany
)

type PeriodType string

const (
	PeriodMonthly   PeriodType = "monthly"
	PeriodQuarterly PeriodType = "quarterly"
	PeriodYearly    PeriodType = "yearly"
)

type TargetState string

const (
	TargetDraft  TargetState = "draft"
	TargetActive TargetState = "active"
	TargetClosed TargetState = "closed"
)

// SaleTargetPeriod captures a defined measurement period target for a team or
// individual salesperson, along with achieved and forecast figures.
type SaleTargetPeriod struct {
	sdk.Model `nexa:"model=sale.target.period"`

	Name              sdk.String                 `nexa:"required,size=120,string=Reference"`
	TeamID            sdk.Many2One[sdk.Any]      `nexa:"string=Sales Team,comodel=sale.team"`
	UserID            sdk.Many2One[CoreUser]     `nexa:"string=Salesperson"`
	CompanyID         sdk.Many2One[CoreCompany]  `nexa:"string=Company"`
	PeriodType        sdk.Selection[PeriodType]  `nexa:"string=Period Type,default=monthly"`
	DateStart         sdk.Date                   `nexa:"required,string=Start Date"`
	DateEnd           sdk.Date                   `nexa:"required,string=End Date"`
	TargetRevenue     sdk.Numeric                `nexa:"string=Revenue Target,precision=18,scale=2,default=0"`
	TargetVolume      sdk.Numeric                `nexa:"string=Volume Target,precision=18,scale=2,default=0"`
	TargetMarginPct   sdk.Numeric                `nexa:"string=Margin Target %,precision=5,scale=2,default=0"`
	AchievedRevenue   sdk.Numeric                `nexa:"string=Achieved Revenue,precision=18,scale=2,default=0"`
	AchievedVolume    sdk.Numeric                `nexa:"string=Achieved Volume,precision=18,scale=2,default=0"`
	AchievedMarginPct sdk.Numeric                `nexa:"string=Achieved Margin %,precision=5,scale=2,default=0"`
	AttainmentPct     sdk.Numeric                `nexa:"string=Attainment %,precision=5,scale=2,default=0"`
	ForecastRevenue   sdk.Numeric                `nexa:"string=Forecast Revenue,precision=18,scale=2,default=0"`
	State             sdk.Selection[TargetState] `nexa:"string=Status,default=draft"`
	Currency          sdk.String                 `nexa:"size=3,string=Currency,default=USD"`
	Notes             sdk.Text                   `nexa:"string=Notes"`
	Active            sdk.Boolean                `nexa:"string=Active,default=true"`
}

func init() {
	sdk.MustRegister("sale_analytics",
		&SaleTargetPeriod{},
	)
}
