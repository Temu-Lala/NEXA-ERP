package models

import (
	"nexa/core/sdk"
)

// HrEmployeeDocument is a structured entry in an employee's document register
// (HR & HCM SRS §2.3): the actual file is attached through the chatter on this
// record, while the metadata here drives categorisation and expiry tracking.
type HrEmployeeDocument struct {
	sdk.Model `nexa:"model=hr.employee.document"`

	Name           sdk.String                      `nexa:"required,string=Title"`
	EmployeeID     sdk.Many2One[HrEmployee]        `nexa:"required,string=Employee,index"`
	Category       sdk.Selection[DocumentCategory] `nexa:"string=Category,index,default=other,selection=identity:Identity,contract:Contract,certification:Certification,tax:Tax,diploma:Diploma,medical:Medical,other:Other"`
	DocumentNumber sdk.String                      `nexa:"string=Reference No."`
	IssueDate      sdk.Date                        `nexa:"string=Issue Date"`
	ExpiryDate     sdk.Date                        `nexa:"string=Expiry Date,index"`
	ReminderDays   sdk.Integer                     `nexa:"string=Remind Before (Days),default=30"`
	Status         sdk.Selection[DocumentStatus]   `nexa:"string=Status,index,compute=compute_document_status,store,selection=valid:Valid,expiring:Expiring Soon,expired:Expired"`
	Notes          sdk.Text                        `nexa:"string=Notes"`
	Active         sdk.Boolean                     `nexa:"string=Active,default=true"`
}
