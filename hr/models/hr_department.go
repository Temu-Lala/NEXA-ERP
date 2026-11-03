package models

import (
	"nexa/core/sdk"
)

// HrDepartment is an organisational unit in the company hierarchy.
type HrDepartment struct {
	sdk.Model `nexa:"model=hr.department"`

	Name           sdk.String                 `nexa:"required,string=Department"`
	ParentID       sdk.Many2One[HrDepartment] `nexa:"string=Parent Department"`
	ManagerID      sdk.Many2One[HrEmployee]   `nexa:"string=Manager"`
	EmployeeIds    sdk.One2Many[HrEmployee]   `nexa:"string=Employees,inverse=department_id"`
	TotalEmployees sdk.Integer                `nexa:"string=Total Employees,compute=compute_total_employees"`
	Color          sdk.Integer                `nexa:"string=Color,default=0"`
	Active         sdk.Boolean                `nexa:"string=Active,default=true"`
}
