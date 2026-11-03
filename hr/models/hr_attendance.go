package models

import (
	"nexa/core/sdk"
)

// HrAttendance records an employee's check-in / check-out pair. The worked time
// is derived from the two timestamps so it stays consistent with the raw data.
type HrAttendance struct {
	sdk.Model `nexa:"model=hr.attendance"`

	EmployeeID  sdk.Many2One[HrEmployee] `nexa:"required,string=Employee,index"`
	CheckIn     sdk.DateTime             `nexa:"required,string=Check In,index"`
	CheckOut    sdk.DateTime             `nexa:"string=Check Out"`
	WorkedHours sdk.Float64              `nexa:"string=Worked Hours,compute=compute_worked_hours"`
	Notes       sdk.Text                 `nexa:"string=Notes"`
}
