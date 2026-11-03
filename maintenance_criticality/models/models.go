package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	maintmodels "nexa_addons/maintenance/models"
)

// Cross-module reference aliases.
type (
	MaintenanceAsset = maintmodels.MaintenanceAsset
	CoreUser         = basemodels.CoreUser
)

// ImpactCategory groups a criticality factor by the type of consequence it measures.
type ImpactCategory string

const (
	ImpactSafety      ImpactCategory = "safety"
	ImpactEnvironment ImpactCategory = "environment"
	ImpactProduction  ImpactCategory = "production"
	ImpactFinancial   ImpactCategory = "financial"
	ImpactFrequency   ImpactCategory = "frequency"
	ImpactLeadTime    ImpactCategory = "lead_time"
	ImpactBackup      ImpactCategory = "backup"
	ImpactRegulatory  ImpactCategory = "regulatory"
)

// AssessmentStatus is the workflow state of a criticality assessment.
type AssessmentStatus string

const (
	AssessDraft     AssessmentStatus = "draft"
	AssessConfirmed AssessmentStatus = "confirmed"
	AssessArchived  AssessmentStatus = "archived"
)

// CriticalityLevel is the resulting classification band.
type CriticalityLevel string

const (
	LevelLow      CriticalityLevel = "low"
	LevelNormal   CriticalityLevel = "normal"
	LevelHigh     CriticalityLevel = "high"
	LevelCritical CriticalityLevel = "critical"
)

// MaintenanceCriticalityFactor is a single weighted risk dimension (e.g. safety impact).
type MaintenanceCriticalityFactor struct {
	sdk.Model `nexa:"model=maintenance.criticality.factor"`

	Code           sdk.String                    `nexa:"required,index,string=Code"`
	Name           sdk.String                    `nexa:"required,string=Name"`
	ImpactCategory sdk.Selection[ImpactCategory] `nexa:"string=Impact Category,default=production"`
	DefaultWeight  sdk.Numeric                   `nexa:"string=Default Weight,precision=18,scale=4,default=1"`
	Description    sdk.Text                      `nexa:"string=Description"`
	Active         sdk.Boolean                   `nexa:"string=Active,default=true"`
}

// MaintenanceCriticalityModel is a configurable scoring template of weighted factors.
type MaintenanceCriticalityModel struct {
	sdk.Model `nexa:"model=maintenance.criticality.model"`

	Name        sdk.String                                      `nexa:"required,index,string=Name"`
	Code        sdk.String                                      `nexa:"required,index,string=Code"`
	Description sdk.Text                                        `nexa:"string=Description"`
	Active      sdk.Boolean                                     `nexa:"string=Active,default=true"`
	Factors     sdk.One2Many[MaintenanceCriticalityModelFactor] `nexa:"string=Weighting"`
}

// MaintenanceCriticalityModelFactor binds a factor to a model with a specific weight and scale.
type MaintenanceCriticalityModelFactor struct {
	sdk.Model `nexa:"model=maintenance.criticality.model.factor"`

	ModelID  sdk.Many2One[MaintenanceCriticalityModel]  `nexa:"required,index,string=Scoring Model"`
	FactorID sdk.Many2One[MaintenanceCriticalityFactor] `nexa:"required,index,string=Factor"`
	Weight   sdk.Numeric                                `nexa:"string=Weight,precision=18,scale=4,default=1"`
	ScaleMax sdk.Numeric                                `nexa:"string=Max Rating,precision=18,scale=4,default=10"`
	Active   sdk.Boolean                                `nexa:"string=Active,default=true"`
}

// MaintenanceCriticalityAssessment scores one asset against a model, keeping history.
type MaintenanceCriticalityAssessment struct {
	sdk.Model `nexa:"model=maintenance.criticality.assessment"`

	Name             sdk.String                                         `nexa:"required,index,string=Reference"`
	AssetID          sdk.Many2One[MaintenanceAsset]                     `nexa:"required,index,string=Asset"`
	ModelID          sdk.Many2One[MaintenanceCriticalityModel]          `nexa:"required,index,string=Scoring Model"`
	AssessmentDate   sdk.Date                                           `nexa:"string=Assessment Date"`
	Status           sdk.Selection[AssessmentStatus]                    `nexa:"string=Status,default=draft"`
	ResponsibleID    sdk.Many2One[CoreUser]                             `nexa:"string=Responsible"`
	TotalScore       sdk.Numeric                                        `nexa:"string=Total Score,precision=18,scale=4,compute=compute_criticality_total_score"`
	ScoreIndex       sdk.Numeric                                        `nexa:"string=Criticality Index (%),precision=18,scale=2,compute=compute_criticality_score_index"`
	CriticalityLevel sdk.Selection[CriticalityLevel]                    `nexa:"string=Criticality Level,compute=compute_criticality_level,store,selection=low:Low,normal:Normal,high:High,critical:Critical"`
	Notes            sdk.Text                                           `nexa:"string=Notes"`
	Lines            sdk.One2Many[MaintenanceCriticalityAssessmentLine] `nexa:"string=Scores"`
}

// MaintenanceCriticalityAssessmentLine is a per-factor rating and its weighted contribution.
type MaintenanceCriticalityAssessmentLine struct {
	sdk.Model `nexa:"model=maintenance.criticality.assessment.line"`

	AssessmentID  sdk.Many2One[MaintenanceCriticalityAssessment] `nexa:"required,index,string=Assessment"`
	FactorID      sdk.Many2One[MaintenanceCriticalityFactor]     `nexa:"required,index,string=Factor"`
	Weight        sdk.Numeric                                    `nexa:"string=Weight,precision=18,scale=4,default=1"`
	ScaleMax      sdk.Numeric                                    `nexa:"string=Max Rating,precision=18,scale=4,default=10"`
	Rating        sdk.Numeric                                    `nexa:"string=Rating,precision=18,scale=4,default=0"`
	WeightedScore sdk.Numeric                                    `nexa:"string=Weighted Score,precision=18,scale=4,compute=compute_criticality_weighted_score"`
	Note          sdk.Text                                       `nexa:"string=Note"`
}

func init() {
	sdk.MustRegister("maintenance_criticality",
		&MaintenanceCriticalityFactor{},
		&MaintenanceCriticalityModel{},
		&MaintenanceCriticalityModelFactor{},
		&MaintenanceCriticalityAssessment{},
		&MaintenanceCriticalityAssessmentLine{},
	)
}
