package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	projmodels "nexa_addons/project/models"
)

// Cross-module reference aliases.
type (
	ProjectProject = projmodels.ProjectProject
	CoreUser       = basemodels.CoreUser
)

// BaselineState is the lifecycle of an approved plan/budget snapshot.
type BaselineState string

const (
	BaselineDraft      BaselineState = "draft"
	BaselineApproved   BaselineState = "approved"
	BaselineSuperseded BaselineState = "superseded"
)

// SnapshotKind classifies a dated financial summary line.
type SnapshotKind string

const (
	SnapshotActual    SnapshotKind = "actual"
	SnapshotCommitted SnapshotKind = "committed"
	SnapshotForecast  SnapshotKind = "forecast"
	SnapshotRevenue   SnapshotKind = "revenue"
	SnapshotMargin    SnapshotKind = "margin"
)

// ProjectBaseline is an approved snapshot of a project's schedule and budget.
type ProjectBaseline struct {
	sdk.Model `nexa:"model=project.baseline"`

	ProjectID     sdk.Many2One[ProjectProject] `nexa:"required,index,string=Project"`
	Name          sdk.String                   `nexa:"required,string=Baseline"`
	State         sdk.Selection[BaselineState] `nexa:"string=Status,default=draft"`
	SnapshotDate  sdk.Date                     `nexa:"required,string=Snapshot Date"`
	BaselineStart sdk.Date                     `nexa:"string=Planned Start"`
	BaselineEnd   sdk.Date                     `nexa:"string=Planned End"`
	BudgetAmount  sdk.Numeric                  `nexa:"string=Budget,precision=18,scale=2,default=0"`
	ApprovedBy    sdk.Many2One[CoreUser]       `nexa:"string=Approved By"`
	Note          sdk.Text                     `nexa:"string=Note"`
}

// ProjectDecision records a key decision, its decision maker and rationale.
type ProjectDecision struct {
	sdk.Model `nexa:"model=project.decision"`

	ProjectID    sdk.Many2One[ProjectProject] `nexa:"required,index,string=Project"`
	Name         sdk.String                   `nexa:"required,string=Decision"`
	DecisionDate sdk.Date                     `nexa:"required,string=Date"`
	DecidedBy    sdk.Many2One[CoreUser]       `nexa:"string=Decided By"`
	OwnerID      sdk.Many2One[CoreUser]       `nexa:"string=Owner"`
	Status       sdk.String                   `nexa:"string=Status,default=open"`
	Rationale    sdk.Text                     `nexa:"string=Rationale"`
	Impact       sdk.Text                     `nexa:"string=Impact"`
}

// ProjectFinancialSnapshot is a dated financial summary line for trend reporting.
type ProjectFinancialSnapshot struct {
	sdk.Model `nexa:"model=project.financial.snapshot"`

	ProjectID   sdk.Many2One[ProjectProject] `nexa:"required,index,string=Project"`
	Date        sdk.Date                     `nexa:"required,string=Date"`
	Kind        sdk.Selection[SnapshotKind]  `nexa:"required,string=Measure,default=forecast"`
	Amount      sdk.Numeric                  `nexa:"string=Amount,precision=18,scale=2,default=0"`
	Description sdk.Text                     `nexa:"string=Description"`
}

func init() {
	sdk.MustRegister("project_governance",
		&ProjectBaseline{},
		&ProjectDecision{},
		&ProjectFinancialSnapshot{},
	)
}
