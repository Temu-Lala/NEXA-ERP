package models

import (
	"nexa/core/sdk"
	crmodels "nexa_addons/crm/models"
)

// Reporting read-models. Each record is a persisted summary snapshot for a
// period/dimension; the underlying detail stays in crm.lead and the connected
// modules. Pipeline value is not recognized revenue and weighted value is not
// a guarantee - labels keep those distinctions explicit.

type (
	CoreUser      = crmodels.CoreUser
	CoreCompany   = crmodels.CoreCompany
	CrmStage      = crmodels.CrmStage
	CrmLostReason = crmodels.CrmLostReason
)

// CrmFunnelReport summarizes lead volume and conversion by source and stage.
// The cohort denominator is explicit: leads created within the period.
type CrmFunnelReport struct {
	sdk.Model `nexa:"model=crm.funnel.report"`

	Name           sdk.String                `nexa:"required,size=120,string=Reference"`
	SourceID       sdk.Many2One[sdk.Any]     `nexa:"string=Source,comodel=utm.source"`
	StageID        sdk.Many2One[CrmStage]    `nexa:"string=Stage"`
	PeriodStart    sdk.Date                  `nexa:"string=Period Start"`
	PeriodEnd      sdk.Date                  `nexa:"string=Period End"`
	LeadCount      sdk.Integer               `nexa:"string=Leads,default=0"`
	ConvertedCount sdk.Integer               `nexa:"string=Converted,default=0"`
	ConversionRate sdk.Float                 `nexa:"string=Conversion Rate (%)"`
	CompanyID      sdk.Many2One[CoreCompany] `nexa:"string=Company"`
}

// CrmPipelineReport summarizes open pipeline by stage/owner for a period.
type CrmPipelineReport struct {
	sdk.Model `nexa:"model=crm.pipeline.report"`

	Name           sdk.String                `nexa:"required,size=120,string=Reference"`
	PipelineID     sdk.Many2One[sdk.Any]     `nexa:"string=Pipeline,comodel=crm.pipeline"`
	StageID        sdk.Many2One[CrmStage]    `nexa:"string=Stage"`
	OwnerID        sdk.Many2One[CoreUser]    `nexa:"string=Owner"`
	PeriodDate     sdk.Date                  `nexa:"string=As Of"`
	OpenCount      sdk.Integer               `nexa:"string=Open Deals,default=0"`
	OpenValue      sdk.Numeric               `nexa:"string=Pipeline Value,precision=18,scale=2,default=0"`
	WeightedValue  sdk.Numeric               `nexa:"string=Weighted Value,precision=18,scale=2,default=0"`
	AvgDaysInStage sdk.Integer               `nexa:"string=Avg Days In Stage,default=0"`
	CompanyID      sdk.Many2One[CoreCompany] `nexa:"string=Company"`
}

// CrmWinLossReport captures outcomes, win rate and the dominant lost reason.
type CrmWinLossReport struct {
	sdk.Model `nexa:"model=crm.winloss.report"`

	Name          sdk.String                  `nexa:"required,size=120,string=Reference"`
	PeriodStart   sdk.Date                    `nexa:"string=Period Start"`
	PeriodEnd     sdk.Date                    `nexa:"string=Period End"`
	WonCount      sdk.Integer                 `nexa:"string=Won,default=0"`
	LostCount     sdk.Integer                 `nexa:"string=Lost,default=0"`
	WonValue      sdk.Numeric                 `nexa:"string=Won Value,precision=18,scale=2,default=0"`
	LostValue     sdk.Numeric                 `nexa:"string=Lost Value,precision=18,scale=2,default=0"`
	WinRate       sdk.Float                   `nexa:"string=Win Rate (%)"`
	TopLostReason sdk.String                  `nexa:"size=200,string=Top Lost Reason"`
	LostReasonID  sdk.Many2One[CrmLostReason] `nexa:"string=Lost Reason"`
	CompanyID     sdk.Many2One[CoreCompany]   `nexa:"string=Company"`
}

// CrmAttributionReport measures leads/opportunities/outcomes per campaign and
// source. Attribution is a measurement model, not proof of causation.
type CrmAttributionReport struct {
	sdk.Model `nexa:"model=crm.attribution.report"`

	Name             sdk.String                `nexa:"required,size=120,string=Reference"`
	CampaignID       sdk.Many2One[sdk.Any]     `nexa:"string=Campaign,comodel=utm.campaign"`
	SourceID         sdk.Many2One[sdk.Any]     `nexa:"string=Source,comodel=utm.source"`
	LeadCount        sdk.Integer               `nexa:"string=Leads,default=0"`
	OpportunityCount sdk.Integer               `nexa:"string=Opportunities,default=0"`
	WonCount         sdk.Integer               `nexa:"string=Won,default=0"`
	PipelineValue    sdk.Numeric               `nexa:"string=Pipeline Value,precision=18,scale=2,default=0"`
	WonValue         sdk.Numeric               `nexa:"string=Won Value,precision=18,scale=2,default=0"`
	CompanyID        sdk.Many2One[CoreCompany] `nexa:"string=Company"`
}

func init() {
	sdk.MustRegister("crm_report",
		&CrmFunnelReport{},
		&CrmPipelineReport{},
		&CrmWinLossReport{},
		&CrmAttributionReport{},
	)
}
