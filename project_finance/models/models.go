package models

import (
	"nexa/core/sdk"
	projmodels "nexa_addons/project/models"
)

// ProjectProject -> project.project
type ProjectProject = projmodels.ProjectProject

type BudgetState string

const (
	BudgetStateDraft     BudgetState = "draft"
	BudgetStateConfirmed BudgetState = "confirmed"
	BudgetStateApproved  BudgetState = "approved"
)

type CostCategory string

const (
	CostLabor       CostCategory = "labor"
	CostContractor  CostCategory = "contractor"
	CostMaterials   CostCategory = "materials"
	CostProcurement CostCategory = "procurement"
	CostTravel      CostCategory = "travel"
	CostSoftware    CostCategory = "software"
	CostOverhead    CostCategory = "overhead"
	CostOther       CostCategory = "other"
)

// ProjectBudget is an approved financial plan for a project.
type ProjectBudget struct {
	sdk.Model `nexa:"model=project.budget"`

	ProjectID   sdk.Many2One[ProjectProject]    `nexa:"required,index,string=Project"`
	Name        sdk.String                      `nexa:"required,string=Budget Name"`
	State       sdk.Selection[BudgetState]      `nexa:"string=Status,default=draft"`
	TotalAmount sdk.Numeric                     `nexa:"string=Total,precision=18,scale=2,default=0"`
	Lines       sdk.One2Many[ProjectBudgetLine] `nexa:"string=Lines"`
	Note        sdk.Text                        `nexa:"string=Note"`
}

// ProjectBudgetLine is a budgeted amount for one cost category.
type ProjectBudgetLine struct {
	sdk.Model `nexa:"model=project.budget.line"`

	BudgetID    sdk.Many2One[ProjectBudget] `nexa:"required,index,string=Budget"`
	Category    sdk.Selection[CostCategory] `nexa:"required,string=Cost Category"`
	Amount      sdk.Numeric                 `nexa:"string=Planned Amount,precision=18,scale=2,default=0"`
	Description sdk.Text                    `nexa:"string=Description"`
	Sequence    sdk.Integer                 `nexa:"string=Sequence,default=10"`
}

// ProjectCost is a source-linked cost snapshot charged against a project.
type ProjectCost struct {
	sdk.Model `nexa:"model=project.cost"`

	ProjectID   sdk.Many2One[ProjectProject] `nexa:"required,index,string=Project"`
	Date        sdk.Date                     `nexa:"required,string=Date"`
	Category    sdk.Selection[CostCategory]  `nexa:"required,string=Cost Category"`
	Amount      sdk.Numeric                  `nexa:"required,string=Amount,precision=18,scale=2,default=0"`
	Billable    sdk.Boolean                  `nexa:"string=Billable,default=false"`
	SourceType  sdk.String                   `nexa:"string=Source Reference"`
	Description sdk.Text                     `nexa:"string=Description"`
}

func init() {
	sdk.MustRegister("project_finance",
		&ProjectBudget{},
		&ProjectBudgetLine{},
		&ProjectCost{},
	)
}
