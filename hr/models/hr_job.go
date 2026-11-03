package models

import (
	"nexa/core/sdk"
)

// HrJob is a job position (title) that employees can hold within a department.
type HrJob struct {
	sdk.Model `nexa:"model=hr.job"`

	Name              sdk.String                 `nexa:"required,string=Job Position"`
	DepartmentID      sdk.Many2One[HrDepartment] `nexa:"string=Department"`
	EmployeeIds       sdk.One2Many[HrEmployee]   `nexa:"string=Employees,inverse=job_id"`
	NoOfEmployees     sdk.Integer                `nexa:"string=Employees Count,compute=compute_no_of_employees"`
	ExpectedEmployees sdk.Integer                `nexa:"string=Expected Employees,default=0"`
	Description       sdk.Text                   `nexa:"string=Job Description"`
	Active            sdk.Boolean                `nexa:"string=Active,default=true"`
}
