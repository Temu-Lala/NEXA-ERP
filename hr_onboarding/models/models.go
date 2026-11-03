package models

import (
	"nexa/core/sdk"
	hrmodels "nexa_addons/hr/models"
)

// Cross-module reference aliases.
type (
	HrEmployee   = hrmodels.HrEmployee
	HrDepartment = hrmodels.HrDepartment
)

type OnboardingState string

const (
	OnboardingStateDraft      OnboardingState = "draft"
	OnboardingStateInProgress OnboardingState = "in_progress"
	OnboardingStateDone       OnboardingState = "done"
	OnboardingStateCancelled  OnboardingState = "cancelled"
)

type ActivityState string

const (
	ActivityStatePending ActivityState = "pending"
	ActivityStateDone    ActivityState = "done"
	ActivityStateSkipped ActivityState = "skipped"
)

type ResponsibleRole string

const (
	ResponsibleHR       ResponsibleRole = "hr"
	ResponsibleManager  ResponsibleRole = "manager"
	ResponsibleIT       ResponsibleRole = "it"
	ResponsibleFinance  ResponsibleRole = "finance"
	ResponsibleAdmin    ResponsibleRole = "admin"
	ResponsibleEmployee ResponsibleRole = "employee"
)

// HrOnboardingPlan tracks the onboarding process for one employee.
type HrOnboardingPlan struct {
	sdk.Model `nexa:"model=hr.onboarding.plan"`

	Name         sdk.String                     `nexa:"required,string=Plan Name"`
	EmployeeID   sdk.Many2One[HrEmployee]       `nexa:"required,string=Employee"`
	DepartmentID sdk.Many2One[HrDepartment]     `nexa:"string=Department"`
	StartDate    sdk.Date                       `nexa:"string=Start Date"`
	TargetDate   sdk.Date                       `nexa:"string=Target Completion"`
	State        sdk.Selection[OnboardingState] `nexa:"string=Status,default=draft"`
	Notes        sdk.Text                       `nexa:"string=Notes"`
}

// HrOnboardingActivity is a checklist item within an onboarding plan.
type HrOnboardingActivity struct {
	sdk.Model `nexa:"model=hr.onboarding.activity"`

	PlanID        sdk.Many2One[HrOnboardingPlan] `nexa:"required,string=Plan"`
	Name          sdk.String                     `nexa:"required,string=Activity"`
	Responsible   sdk.Selection[ResponsibleRole] `nexa:"string=Responsible Party,default=hr"`
	EmployeeIDLvl sdk.Many2One[HrEmployee]       `nexa:"string=Assigned To"`
	DueDate       sdk.Date                       `nexa:"string=Due Date"`
	Sequence      sdk.Integer                    `nexa:"string=Sequence,default=10"`
	State         sdk.Selection[ActivityState]   `nexa:"string=Status,default=pending"`
	DoneDate      sdk.Date                       `nexa:"string=Completed Date"`
	Notes         sdk.Text                       `nexa:"string=Notes"`
}

func init() {
	sdk.MustRegister("hr_onboarding",
		&HrOnboardingPlan{},
		&HrOnboardingActivity{},
	)
}
