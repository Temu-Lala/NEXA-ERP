package models

import (
	"nexa/core/sdk"
)

// HrContract is an employment agreement for an employee. Its status is derived
// live from the end date (open-ended, running, expiring or expired).
type HrContract struct {
	sdk.Model `nexa:"model=hr.contract"`

	Name         sdk.String                   `nexa:"required,string=Contract Reference"`
	EmployeeID   sdk.Many2One[HrEmployee]     `nexa:"required,string=Employee,index"`
	Type         sdk.Selection[ContractType]  `nexa:"string=Contract Type,index,default=permanent,selection=permanent:Permanent,fixed_term:Fixed Term,internship:Internship,contractor:Contractor,temporary:Temporary"`
	DateStart    sdk.Date                     `nexa:"required,string=Start Date,index"`
	DateEnd      sdk.Date                     `nexa:"string=End Date,index"`
	TrialEndDate sdk.Date                     `nexa:"string=Trial Period End"`
	State        sdk.Selection[ContractState] `nexa:"string=Status,index,compute=compute_contract_state,store,selection=open:Open-Ended,running:Running,expiring:Expiring Soon,expired:Expired"`
	Notes        sdk.Text                     `nexa:"string=Notes"`
	Active       sdk.Boolean                  `nexa:"string=Active,default=true"`
}
