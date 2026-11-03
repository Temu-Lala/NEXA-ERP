package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	hrmodels "nexa_addons/hr/models"
	projmodels "nexa_addons/project/models"
)

// Cross-module reference aliases.
type (
	HrEmployee     = hrmodels.HrEmployee
	ProjectProject = projmodels.ProjectProject
	ProjectTask    = projmodels.ProjectTask
	CoreCompany    = basemodels.CoreCompany
)

type TimesheetState string

const (
	TimesheetStateDraft     TimesheetState = "draft"
	TimesheetStateValidated TimesheetState = "validated"
	TimesheetStateDone      TimesheetState = "done"
)

type HrTimesheet struct {
	sdk.Model `nexa:"model=hr.timesheet"`

	Name        sdk.String                    `nexa:"string=Summary"`
	EmployeeID  sdk.Many2One[HrEmployee]      `nexa:"required,string=Employee"`
	ProjectID   sdk.Many2One[ProjectProject]  `nexa:"string=Project"`
	TaskID      sdk.Many2One[ProjectTask]     `nexa:"string=Task"`
	Date        sdk.Date                      `nexa:"required,string=Date"`
	Hours       sdk.Numeric                   `nexa:"string=Duration (Hours),precision=18,scale=2,default=0"`
	IsBillable  sdk.Boolean                   `nexa:"string=Billable,default=true"`
	Description sdk.Text                      `nexa:"string=Description"`
	State       sdk.Selection[TimesheetState] `nexa:"string=Status,default=draft"`
	CompanyID   sdk.Many2One[CoreCompany]     `nexa:"string=Company"`
}

func init() {
	sdk.MustRegister("hr_timesheet",
		&HrTimesheet{},
	)
}
