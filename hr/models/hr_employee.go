package models

import (
	"nexa/core/sdk"
)

// HrEmployee is the central employee record (HR & HCM SRS §2).
// Fields are grouped as: identity & personal info, work contacts, employment &
// organisation, lifecycle (§2.2), and manager-only sensitive data (§2.1).
type HrEmployee struct {
	sdk.Model `nexa:"model=hr.employee"`

	// --- Identity & personal info ---
	Name               sdk.String                     `nexa:"required,string=Employee Name"`
	IdentificationName sdk.String                     `nexa:"string=Legal Name,help=Name as it appears on official documents."`
	EmployeeID         sdk.String                     `nexa:"string=Employee ID,index"`
	Image              sdk.Image                      `nexa:"string=Photo"`
	Gender             sdk.Selection[EmployeeGender]  `nexa:"string=Gender,selection=male:Male,female:Female,other:Other,undisclosed:Undisclosed"`
	Birthday           sdk.Date                       `nexa:"string=Birthday"`
	Birthplace         sdk.String                     `nexa:"string=Birthplace"`
	MaritalStatus      sdk.Selection[EmployeeMarital] `nexa:"string=Marital Status,selection=single:Single,married:Married,cohabiting:Cohabiting,separated:Separated,divorced:Divorced,widowed:Widowed"`
	Nationality        sdk.String                     `nexa:"string=Nationality"`

	// --- Work & personal contacts ---
	WorkEmail             sdk.String `nexa:"string=Work Email"`
	WorkPhone             sdk.String `nexa:"string=Work Phone"`
	EmergencyContactName  sdk.String `nexa:"string=Emergency Contact"`
	EmergencyContactPhone sdk.String `nexa:"string=Emergency Phone"`
	WorkLocation          sdk.String `nexa:"string=Work Location,help=Office / branch or remote"`

	// --- Employment & organisation ---
	DepartmentID   sdk.Many2One[HrDepartment]  `nexa:"string=Department"`
	JobID          sdk.Many2One[HrJob]         `nexa:"string=Job Position"`
	ParentID       sdk.Many2One[HrEmployee]    `nexa:"string=Manager"`
	CoachID        sdk.Many2One[HrEmployee]    `nexa:"string=Coach"`
	EmploymentType sdk.Selection[EmployeeType] `nexa:"string=Employment Type,default=full_time,selection=full_time:Full Time,part_time:Part Time,fixed_term:Fixed Term,temporary:Temporary,intern:Intern,contractor:Contractor,freelance:Freelance,other:Other"`
	CompanyID      sdk.Many2One[CoreCompany]   `nexa:"string=Company"`
	UserID         sdk.Many2One[CoreUser]      `nexa:"string=Related User"`
	PartnerID      sdk.Many2One[CorePartner]   `nexa:"string=Related Contact"`

	// --- Lifecycle (§2.2) ---
	State             sdk.Selection[EmployeeState]       `nexa:"string=Status,index,default=active,selection=draft:Draft,pending:Pending Approval,active:Active,on_leave:On Leave,suspended:Suspended,notice:Notice Period,terminated:Terminated,retired:Retired"`
	HireDate          sdk.Date                           `nexa:"string=Hire Date"`
	ProbationEndDate  sdk.Date                           `nexa:"string=Probation End"`
	TerminationDate   sdk.Date                           `nexa:"string=Termination Date"`
	TerminationReason sdk.Selection[EmployeeTermination] `nexa:"string=Termination Reason,default=unspecified,selection=unspecified:Unspecified,resignation:Resignation,fired:Fired,retired:Retired,contract_end:Contract End,other:Other"`
	Active            sdk.Boolean                        `nexa:"string=Active,default=true"`

	// --- Manager-only sensitive data ---
	IDCertificateNumber sdk.String `nexa:"string=ID / Passport No.,column=certificate_number,groups=hr.group_hr_manager"`
	BankAccountNumber   sdk.String `nexa:"string=Bank Account No.,groups=hr.group_hr_manager"`

	// --- Skills & notes ---
	Skills         sdk.Text                         `nexa:"string=Skills"`
	Languages      sdk.Text                         `nexa:"string=Languages"`
	Certifications sdk.Text                         `nexa:"string=Certifications"`
	Color          sdk.Integer                      `nexa:"string=Color,default=0"`
	Notes          sdk.Text                         `nexa:"string=Notes"`
	DocumentIds    sdk.One2Many[HrEmployeeDocument] `nexa:"string=Documents,inverse=employee_id"`
}
