package models

import (
	"nexa/core/sdk"
)

// HrLeaveType is a category of time off (Annual, Sick, Unpaid, ...) that
// requests and allocations reference.
type HrLeaveType struct {
	sdk.Model `nexa:"model=hr.leave.type"`

	Name               sdk.String  `nexa:"required,string=Leave Type"`
	RequiresAllocation sdk.Boolean `nexa:"string=Requires Allocation,default=true"`
	MaxDays            sdk.Integer `nexa:"string=Max Days Per Request,default=0"`
	Notes              sdk.Text    `nexa:"string=Notes"`
	Active             sdk.Boolean `nexa:"string=Active,default=true"`
}

// HrLeaveAllocation grants an employee a number of days of a leave type,
// itself subject to the same approval workflow as a request.
type HrLeaveAllocation struct {
	sdk.Model `nexa:"model=hr.leave.allocation"`

	EmployeeID   sdk.Many2One[HrEmployee]  `nexa:"required,string=Employee,index"`
	LeaveTypeID  sdk.Many2One[HrLeaveType] `nexa:"required,string=Leave Type,index"`
	NumberOfDays sdk.Integer               `nexa:"required,string=Days,default=0"`
	State        sdk.Selection[LeaveState] `nexa:"string=Status,index,default=draft,selection=draft:Draft,confirm:Waiting Approval,validate:Validated,refuse:Refused"`
	DateFrom     sdk.Date                  `nexa:"string=Valid From"`
	DateTo       sdk.Date                  `nexa:"string=Valid To"`
	Notes        sdk.Text                  `nexa:"string=Notes"`
	Active       sdk.Boolean               `nexa:"string=Active,default=true"`
}

// HrLeave is a time-off request. The day count is derived from the date range
// and the record moves through the draft -> confirm -> validate/refuse workflow.
type HrLeave struct {
	sdk.Model `nexa:"model=hr.leave"`

	EmployeeID   sdk.Many2One[HrEmployee]  `nexa:"required,string=Employee,index"`
	LeaveTypeID  sdk.Many2One[HrLeaveType] `nexa:"required,string=Leave Type,index"`
	DateFrom     sdk.Date                  `nexa:"required,string=Start Date,index"`
	DateTo       sdk.Date                  `nexa:"required,string=End Date,index"`
	NumberOfDays sdk.Float64               `nexa:"string=Days,compute=compute_leave_days"`
	State        sdk.Selection[LeaveState] `nexa:"string=Status,index,default=draft,selection=draft:Draft,confirm:Waiting Approval,validate:Approved,refuse:Refused"`
	Reason       sdk.Text                  `nexa:"string=Reason"`
	Active       sdk.Boolean               `nexa:"string=Active,default=true"`
}
