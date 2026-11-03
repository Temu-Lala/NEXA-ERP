package models

import (
	"nexa/core/sdk"
	crmodels "nexa_addons/crm/models"
)

// Pipeline machinery layered over the base CRM. crm.lead remains the
// authoritative deal record; this module adds named pipelines that order the
// existing crm.stage records, an append-only stage-transition history, forecast
// categories, and per-deal forecast snapshots. Historical stage events are
// never overwritten.

type (
	CoreUser    = crmodels.CoreUser
	CoreCompany = crmodels.CoreCompany
	CrmTeam     = crmodels.CrmTeam
	CrmStage    = crmodels.CrmStage
	CrmLead     = crmodels.CrmLead
)

// CrmPipeline is a named pipeline that gives an ordered sequence to otherwise
// global crm.stage records and may be scoped to a sales team.
type CrmPipeline struct {
	sdk.Model `nexa:"model=crm.pipeline"`

	Name      sdk.String                        `nexa:"required,size=120,string=Name"`
	Code      sdk.String                        `nexa:"size=60,string=Code"`
	TeamID    sdk.Many2One[CrmTeam]             `nexa:"string=Sales Team"`
	Sequence  sdk.Integer                       `nexa:"string=Sequence,default=10"`
	IsDefault sdk.Boolean                       `nexa:"string=Default Pipeline,default=false"`
	StageRels sdk.One2Many[CrmPipelineStageRel] `nexa:"string=Stages"`
	CompanyID sdk.Many2One[CoreCompany]         `nexa:"string=Company"`
	Active    sdk.Boolean                       `nexa:"string=Active,default=true"`
}

// CrmPipelineStageRel orders an existing crm.stage within a pipeline.
type CrmPipelineStageRel struct {
	sdk.Model `nexa:"model=crm.pipeline.stage.rel"`

	PipelineID sdk.Many2One[CrmPipeline] `nexa:"required,index,string=Pipeline"`
	StageID    sdk.Many2One[CrmStage]    `nexa:"required,string=Stage"`
	Sequence   sdk.Integer               `nexa:"string=Order,default=1"`
	CompanyID  sdk.Many2One[CoreCompany] `nexa:"string=Company"`
}

// CrmStageHistory is an append-only log of opportunity stage transitions,
// kept even though the current stage also lives on crm.lead for fast queries.
type CrmStageHistory struct {
	sdk.Model `nexa:"model=crm.stage.history"`

	LeadID         sdk.Many2One[CrmLead]     `nexa:"required,index,string=Opportunity"`
	FromStageID    sdk.Many2One[CrmStage]    `nexa:"string=From Stage"`
	ToStageID      sdk.Many2One[CrmStage]    `nexa:"required,string=To Stage"`
	UserID         sdk.Many2One[CoreUser]    `nexa:"string=Moved By"`
	TransitionDate sdk.DateTime              `nexa:"string=Transition Date"`
	DaysInStage    sdk.Integer               `nexa:"string=Days In Previous Stage,default=0"`
	CompanyID      sdk.Many2One[CoreCompany] `nexa:"string=Company"`
}

// CrmForecastCategory classifies how committed a forecast line is (pipeline,
// best case, committed) with a confidence weight.
type CrmForecastCategory struct {
	sdk.Model `nexa:"model=crm.forecast.category"`

	Name       sdk.String                `nexa:"required,size=120,string=Name"`
	Code       sdk.String                `nexa:"size=60,string=Code"`
	Confidence sdk.Float                 `nexa:"string=Confidence (%)"`
	IsCommit   sdk.Boolean               `nexa:"string=Committed,default=false"`
	Sequence   sdk.Integer               `nexa:"string=Sequence,default=10"`
	CompanyID  sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active     sdk.Boolean               `nexa:"string=Active,default=true"`
}

// CrmLeadForecast is a forecast snapshot for an opportunity. Pipeline and
// weighted values are estimates for planning, not recognized revenue.
type CrmLeadForecast struct {
	sdk.Model `nexa:"model=crm.lead.forecast"`

	LeadID             sdk.Many2One[CrmLead]             `nexa:"required,index,string=Opportunity"`
	ForecastCategoryID sdk.Many2One[CrmForecastCategory] `nexa:"required,string=Forecast Category"`
	ExpectedClose      sdk.Date                          `nexa:"string=Expected Close"`
	PipelineValue      sdk.Numeric                       `nexa:"string=Pipeline Value,precision=18,scale=2,default=0"`
	WeightedValue      sdk.Numeric                       `nexa:"string=Weighted Value,precision=18,scale=2,default=0"`
	CompanyID          sdk.Many2One[CoreCompany]         `nexa:"string=Company"`
}

func init() {
	sdk.MustRegister("crm_pipeline",
		&CrmPipeline{},
		&CrmPipelineStageRel{},
		&CrmStageHistory{},
		&CrmForecastCategory{},
		&CrmLeadForecast{},
	)
}
