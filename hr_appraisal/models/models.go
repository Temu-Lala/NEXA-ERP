package models

import (
	"nexa/core/sdk"
	hrmodels "nexa_addons/hr/models"
	skillmodels "nexa_addons/hr_skill/models"
)

// Cross-module reference aliases.
type (
	HrEmployee   = hrmodels.HrEmployee
	HrDepartment = hrmodels.HrDepartment
	HrSkill      = skillmodels.HrSkill
)

type AppraisalState string

const (
	AppraisalStateDraft      AppraisalState = "draft"
	AppraisalStateInProgress AppraisalState = "in_progress"
	AppraisalStateDone       AppraisalState = "done"
	AppraisalStateApproved   AppraisalState = "approved"
	AppraisalStateCancelled  AppraisalState = "cancelled"
)

type GoalState string

const (
	GoalStateDraft      GoalState = "draft"
	GoalStateInProgress GoalState = "in_progress"
	GoalStateReached    GoalState = "reached"
	GoalStateCancelled  GoalState = "cancelled"
)

type CycleState string

const (
	CycleStateDraft  CycleState = "draft"
	CycleStateOpen   CycleState = "open"
	CycleStateDone   CycleState = "done"
	CycleStateLocked CycleState = "locked"
)

// HrAppraisalCycle defines a review period (e.g. Annual 2026, Q1 Review).
type HrAppraisalCycle struct {
	sdk.Model `nexa:"model=hr.appraisal.cycle"`

	Name         sdk.String                 `nexa:"required,string=Name"`
	DateStart    sdk.Date                   `nexa:"required,string=Start Date"`
	DateEnd      sdk.Date                   `nexa:"required,string=End Date"`
	DepartmentID sdk.Many2One[HrDepartment] `nexa:"string=Department"`
	State        sdk.Selection[CycleState]  `nexa:"string=Status,default=draft"`
}

// HrAppraisal is an individual employee appraisal within a cycle.
type HrAppraisal struct {
	sdk.Model `nexa:"model=hr.appraisal"`

	CycleID         sdk.Many2One[HrAppraisalCycle] `nexa:"required,string=Cycle"`
	EmployeeID      sdk.Many2One[HrEmployee]       `nexa:"required,string=Employee"`
	ManagerID       sdk.Many2One[HrEmployee]       `nexa:"string=Appraiser"`
	OverallRating   sdk.Numeric                    `nexa:"string=Overall Rating,precision=18,scale=2,default=0"`
	SelfReview      sdk.Text                       `nexa:"string=Self Review"`
	ManagerFeedback sdk.Text                       `nexa:"string=Manager Feedback"`
	State           sdk.Selection[AppraisalState]  `nexa:"string=Status,default=draft"`
}

// HrAppraisalGoal represents a measurable objective.
type HrAppraisalGoal struct {
	sdk.Model `nexa:"model=hr.appraisal.goal"`

	Name         sdk.String                 `nexa:"required,string=Goal"`
	EmployeeID   sdk.Many2One[HrEmployee]   `nexa:"required,string=Employee"`
	DepartmentID sdk.Many2One[HrDepartment] `nexa:"string=Department"`
	Description  sdk.Text                   `nexa:"string=Description"`
	TargetDate   sdk.Date                   `nexa:"string=Target Date"`
	Progress     sdk.Numeric                `nexa:"string=Progress (%),precision=18,scale=2,default=0"`
	Weight       sdk.Numeric                `nexa:"string=Weight (%),precision=18,scale=2,default=100"`
	State        sdk.Selection[GoalState]   `nexa:"string=Status,default=draft"`
}

// HrAppraisalLine is a scored evaluation item within an appraisal.
type HrAppraisalLine struct {
	sdk.Model `nexa:"model=hr.appraisal.line"`

	AppraisalID sdk.Many2One[HrAppraisal] `nexa:"required,string=Appraisal"`
	SkillID     sdk.Many2One[HrSkill]     `nexa:"string=Competency"`
	Name        sdk.String                `nexa:"string=Criteria"`
	Rating      sdk.Numeric               `nexa:"string=Rating,precision=18,scale=2,default=0"`
	Feedback    sdk.Text                  `nexa:"string=Feedback"`
	Sequence    sdk.Integer               `nexa:"string=Sequence,default=10"`
}

func init() {
	sdk.MustRegister("hr_appraisal",
		&HrAppraisalCycle{},
		&HrAppraisal{},
		&HrAppraisalGoal{},
		&HrAppraisalLine{},
	)
}
