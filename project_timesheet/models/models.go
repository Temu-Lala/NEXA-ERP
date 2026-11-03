package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	projmodels "nexa_addons/project/models"
)

// Cross-module reference aliases.
type (
	ProjectProject = projmodels.ProjectProject
	ProjectTask    = projmodels.ProjectTask
	CoreUser       = basemodels.CoreUser
)

// TimesheetState is the review lifecycle of a weekly timesheet.
type TimesheetState string

const (
	TimesheetDraft     TimesheetState = "draft"
	TimesheetSubmitted TimesheetState = "submitted"
	TimesheetApproved  TimesheetState = "approved"
	TimesheetRejected  TimesheetState = "rejected"
)

// ProjectTimesheet is one employee's effort for a week, reviewed as a unit.
type ProjectTimesheet struct {
	sdk.Model `nexa:"model=project.timesheet"`

	UserID        sdk.Many2One[CoreUser]             `nexa:"required,index,string=Employee"`
	WeekStart     sdk.Date                           `nexa:"required,index,string=Week Start"`
	ProjectID     sdk.Many2One[ProjectProject]       `nexa:"string=Primary Project"`
	State         sdk.Selection[TimesheetState]      `nexa:"string=Status,default=draft"`
	ManagerID     sdk.Many2One[CoreUser]             `nexa:"string=Reviewer"`
	Lines         sdk.One2Many[ProjectTimesheetLine] `nexa:"string=Entries"`
	TotalHours    sdk.Numeric                        `nexa:"string=Total Hours,precision=18,scale=2,default=0"`
	BillableHours sdk.Numeric                        `nexa:"string=Billable Hours,precision=18,scale=2,default=0"`
	ApprovedBy    sdk.Many2One[CoreUser]             `nexa:"string=Approved By"`
	ApprovedDate  sdk.Date                           `nexa:"string=Approved Date"`
	Note          sdk.Text                           `nexa:"string=Note"`
}

// ProjectTimesheetLine is a single effort entry against a task on a timesheet.
type ProjectTimesheetLine struct {
	sdk.Model `nexa:"model=project.timesheet.line"`

	TimesheetID sdk.Many2One[ProjectTimesheet] `nexa:"required,index,string=Timesheet"`
	ProjectID   sdk.Many2One[ProjectProject]   `nexa:"string=Project"`
	TaskID      sdk.Many2One[ProjectTask]      `nexa:"required,index,string=Task"`
	Date        sdk.Date                       `nexa:"required,string=Date"`
	Hours       sdk.Numeric                    `nexa:"required,string=Hours,precision=18,scale=2,default=0"`
	Billable    sdk.Boolean                    `nexa:"string=Billable,default=true"`
	Description sdk.Text                       `nexa:"string=Description"`
}

func init() {
	sdk.MustRegister("project_timesheet",
		&ProjectTimesheet{},
		&ProjectTimesheetLine{},
	)
}
