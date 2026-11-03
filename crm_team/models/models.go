package models

import (
	"nexa/core/sdk"
	crmodels "nexa_addons/crm/models"
)

// Sales organization layer: territories, territory coverage for the existing
// crm.team, per-object territory assignment, and period sales targets. Targets
// are distinct from actual results - a target is a plan, achievement is
// recorded separately and attainment is derived.

type (
	CoreUser    = crmodels.CoreUser
	CoreCompany = crmodels.CoreCompany
	CrmTeam     = crmodels.CrmTeam
)

type TerritoryType string

const (
	TerrGeo      TerritoryType = "geo"
	TerrBranch   TerritoryType = "branch"
	TerrIndustry TerritoryType = "industry"
	TerrSegment  TerritoryType = "segment"
	TerrProduct  TerritoryType = "product"
	TerrAccount  TerritoryType = "account"
)

type AssignedObjectType string

const (
	ObjLead        AssignedObjectType = "lead"
	ObjOpportunity AssignedObjectType = "opportunity"
	ObjAccount     AssignedObjectType = "account"
)

type TargetOwnerType string

const (
	OwnerUser TargetOwnerType = "user"
	OwnerTeam TargetOwnerType = "team"
)

type TargetPeriod string

const (
	PeriodMonthly   TargetPeriod = "monthly"
	PeriodQuarterly TargetPeriod = "quarterly"
	PeriodYearly    TargetPeriod = "yearly"
)

type TargetMetric string

const (
	MetricLeads         TargetMetric = "leads"
	MetricQualified     TargetMetric = "qualified"
	MetricMeetings      TargetMetric = "meetings"
	MetricOpportunities TargetMetric = "opportunities"
	MetricQuotations    TargetMetric = "quotations"
	MetricWon           TargetMetric = "won"
	MetricRevenue       TargetMetric = "revenue"
	MetricPipelineCov   TargetMetric = "pipeline_coverage"
	MetricConversion    TargetMetric = "conversion_rate"
)

// CrmTerritory is a named commercial coverage area (geography, branch,
// industry, segment, product line, or named accounts) with optional parent.
type CrmTerritory struct {
	sdk.Model `nexa:"model=crm.territory"`

	Name          sdk.String                   `nexa:"required,size=120,string=Name"`
	Code          sdk.String                   `nexa:"size=60,string=Code"`
	TerritoryType sdk.Selection[TerritoryType] `nexa:"string=Type,default=geo"`
	ParentID      sdk.Many2One[CrmTerritory]   `nexa:"string=Parent Territory"`
	Description   sdk.Text                     `nexa:"string=Description"`
	CompanyID     sdk.Many2One[CoreCompany]    `nexa:"string=Company"`
	Active        sdk.Boolean                  `nexa:"string=Active,default=true"`
}

// CrmTerritoryAssignment records that a specific lead/opportunity/account is
// covered by a territory and an owner. It references crm.lead by comodel so no
// hard dependency is needed; the reference must point to an authoritative CRM
// record, never a copied one.
type CrmTerritoryAssignment struct {
	sdk.Model `nexa:"model=crm.territory.assignment"`

	TerritoryID sdk.Many2One[CrmTerritory]        `nexa:"required,index,string=Territory"`
	Name        sdk.String                        `nexa:"size=120,string=Reference"`
	ObjectType  sdk.Selection[AssignedObjectType] `nexa:"string=Object Type,default=lead"`
	ObjectRefID sdk.Many2One[sdk.Any]             `nexa:"string=Record,comodel=crm.lead"`
	UserID      sdk.Many2One[CoreUser]            `nexa:"required,string=Assigned To"`
	DateFrom    sdk.Date                          `nexa:"string=From"`
	DateTo      sdk.Date                          `nexa:"string=To"`
	CompanyID   sdk.Many2One[CoreCompany]         `nexa:"string=Company"`
	Active      sdk.Boolean                       `nexa:"string=Active,default=true"`
}

// CrmTeamTerritory links an existing crm.team to the territories it covers.
type CrmTeamTerritory struct {
	sdk.Model `nexa:"model=crm.team.territory"`

	CrmTeamID   sdk.Many2One[CrmTeam]      `nexa:"required,index,string=Sales Team"`
	TerritoryID sdk.Many2One[CrmTerritory] `nexa:"required,string=Territory"`
	CompanyID   sdk.Many2One[CoreCompany]  `nexa:"string=Company"`
}

// CrmSalesTarget sets a period target for a user or team against a chosen
// metric. Achieved value and attainment are recorded separately; a team target
// must never be treated as an individual target.
type CrmSalesTarget struct {
	sdk.Model `nexa:"model=crm.sales.target"`

	Name          sdk.String                     `nexa:"required,size=120,string=Name"`
	OwnerType     sdk.Selection[TargetOwnerType] `nexa:"string=Target For,default=user"`
	UserID        sdk.Many2One[CoreUser]         `nexa:"index,string=Salesperson"`
	TeamID        sdk.Many2One[CrmTeam]          `nexa:"index,string=Sales Team"`
	PeriodType    sdk.Selection[TargetPeriod]    `nexa:"string=Period,default=monthly"`
	PeriodDate    sdk.Date                       `nexa:"string=Period Start"`
	Metric        sdk.Selection[TargetMetric]    `nexa:"string=Metric,default=revenue"`
	TargetValue   sdk.Numeric                    `nexa:"string=Target,precision=18,scale=2,default=0"`
	AchievedValue sdk.Numeric                    `nexa:"string=Achieved,precision=18,scale=2,default=0"`
	Attainment    sdk.Float                      `nexa:"string=Attainment (%)"`
	CompanyID     sdk.Many2One[CoreCompany]      `nexa:"string=Company"`
	Active        sdk.Boolean                    `nexa:"string=Active,default=true"`
}

func init() {
	sdk.MustRegister("crm_team",
		&CrmTerritory{},
		&CrmTerritoryAssignment{},
		&CrmTeamTerritory{},
		&CrmSalesTarget{},
	)
}
