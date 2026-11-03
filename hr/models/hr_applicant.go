package models

import (
	"nexa/core/sdk"
)

// HrApplicant is a candidate moving through the recruitment pipeline, linked to
// an open Job Position and (once hired) to the created employee record.
type HrApplicant struct {
	sdk.Model `nexa:"model=hr.applicant"`

	Name         sdk.String                    `nexa:"required,string=Applicant"`
	JobID        sdk.Many2One[HrJob]           `nexa:"string=Job Position,index"`
	DepartmentID sdk.Many2One[HrDepartment]    `nexa:"string=Department"`
	Stage        sdk.Selection[ApplicantStage] `nexa:"string=Stage,index,default=new,selection=new:New Business,hold:Holding,interview:Interview,offer:Contract Proposal,hired:Hired,rejected:Rejected"`
	Email        sdk.String                    `nexa:"string=Email"`
	Phone        sdk.String                    `nexa:"string=Phone"`
	Source       sdk.String                    `nexa:"string=Source"`
	EmployeeID   sdk.Many2One[HrEmployee]      `nexa:"string=Hired Employee"`
	Notes        sdk.Text                      `nexa:"string=Notes"`
	Active       sdk.Boolean                   `nexa:"string=Active,default=true"`
}
