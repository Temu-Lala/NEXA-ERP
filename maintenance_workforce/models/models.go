package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	maintmodels "nexa_addons/maintenance/models"
)

// Cross-module reference aliases.
type (
	CoreUser             = basemodels.CoreUser
	MaintenanceWorkOrder = maintmodels.MaintenanceWorkOrder
)

// ---- Selection types -------------------------------------------------------

type TechnicianStatus string

const (
	TechnicianStatusActive     TechnicianStatus = "active"
	TechnicianStatusOnLeave    TechnicianStatus = "on_leave"
	TechnicianStatusInactive   TechnicianStatus = "inactive"
	TechnicianStatusContractor TechnicianStatus = "contractor"
)

type CompetencyLevel string

const (
	CompetencyLevelAwareness  CompetencyLevel = "awareness"
	CompetencyLevelCompetent  CompetencyLevel = "competent"
	CompetencyLevelSkilled    CompetencyLevel = "skilled"
	CompetencyLevelExpert     CompetencyLevel = "expert"
	CompetencyLevelTrainer    CompetencyLevel = "trainer"
)

type SkillCategory string

const (
	SkillCategoryElectrical      SkillCategory = "electrical"
	SkillCategoryMechanical      SkillCategory = "mechanical"
	SkillCategoryInstrumentation SkillCategory = "instrumentation"
	SkillCategoryCivil           SkillCategory = "civil"
	SkillCategoryHVAC            SkillCategory = "hvac"
	SkillCategoryPlumbing        SkillCategory = "plumbing"
	SkillCategoryWelding         SkillCategory = "welding"
	SkillCategoryIT              SkillCategory = "it"
	SkillCategoryGeneral         SkillCategory = "general"
)

type CertStatus string

const (
	CertStatusValid          CertStatus = "valid"
	CertStatusExpiringSoon   CertStatus = "expiring_soon"
	CertStatusExpired        CertStatus = "expired"
	CertStatusPendingRenewal CertStatus = "pending_renewal"
)

type ShiftType string

const (
	ShiftTypeDay       ShiftType = "day"
	ShiftTypeAfternoon ShiftType = "afternoon"
	ShiftTypeNight     ShiftType = "night"
	ShiftTypeRotating  ShiftType = "rotating"
	ShiftTypeOnCall    ShiftType = "on_call"
)

type LaborEntryState string

const (
	LaborEntryStateDraft     LaborEntryState = "draft"
	LaborEntryStateSubmitted LaborEntryState = "submitted"
	LaborEntryStateApproved  LaborEntryState = "approved"
	LaborEntryStateRejected  LaborEntryState = "rejected"
)

// ---- Models ----------------------------------------------------------------

// MaintSkillCategory classifies maintenance skills by trade discipline.
type MaintSkillCategory struct {
	sdk.Model `nexa:"model=maintenance.skill.category"`

	Name        sdk.String  `nexa:"required,index,string=Category"`
	Code        sdk.String  `nexa:"string=Code"`
	Description sdk.Text    `nexa:"string=Description"`
	Active      sdk.Boolean `nexa:"string=Active,default=true"`
}

// MaintSkill defines an individual skill within a skill category.
type MaintSkill struct {
	sdk.Model `nexa:"model=maintenance.skill"`

	Name        sdk.String                   `nexa:"required,index,string=Skill"`
	CategoryID  sdk.Many2One[MaintSkillCategory] `nexa:"required,string=Category"`
	Description sdk.Text                     `nexa:"string=Description"`
	Active      sdk.Boolean                  `nexa:"string=Active,default=true"`
}

// MaintTechnician holds the maintenance technician profile.
type MaintTechnician struct {
	sdk.Model `nexa:"model=maintenance.technician"`

	Name               sdk.String                             `nexa:"required,index,string=Name"`
	UserID             sdk.Many2One[CoreUser]                 `nexa:"required,index,unique,string=User"`
	EmployeeCode       sdk.String                             `nexa:"index,string=Employee Code"`
	Status             sdk.Selection[TechnicianStatus]        `nexa:"required,string=Status,default=active"`
	Trade              sdk.String                             `nexa:"string=Trade"`
	Specialty          sdk.String                             `nexa:"string=Specialty"`
	TeamName           sdk.String                             `nexa:"string=Team / Cost Centre"`
	LaborRatePerHour   sdk.Numeric                            `nexa:"string=Labor Rate / Hour,precision=18,scale=2,default=0"`
	OvertimeMultiplier sdk.Numeric                            `nexa:"string=Overtime Multiplier,precision=18,scale=2,default=1.5"`
	IsContractor       sdk.Boolean                            `nexa:"string=Is Contractor,default=false"`
	ContractorCompany  sdk.String                             `nexa:"string=Contractor Company"`
	ShiftType          sdk.Selection[ShiftType]               `nexa:"string=Shift Type,default=day"`
	HireDate           sdk.Date                               `nexa:"string=Hire Date"`
	Phone              sdk.String                             `nexa:"string=Phone"`
	EmergencyContact   sdk.String                             `nexa:"string=Emergency Contact"`
	Notes              sdk.Text                               `nexa:"string=Notes"`
	Active             sdk.Boolean                            `nexa:"string=Active,default=true"`
	Skills             sdk.One2Many[MaintTechnicianSkill]     `nexa:"string=Skills"`
	Certificates       sdk.One2Many[MaintTechnicianCertificate] `nexa:"string=Certificates"`
}

// MaintTechnicianSkill links a skill and competency level to a technician.
type MaintTechnicianSkill struct {
	sdk.Model `nexa:"model=maintenance.technician.skill"`

	TechnicianID   sdk.Many2One[MaintTechnician]    `nexa:"required,index,string=Technician"`
	SkillID        sdk.Many2One[MaintSkill]          `nexa:"required,string=Skill"`
	CompetencyLevel sdk.Selection[CompetencyLevel]   `nexa:"required,string=Competency Level,default=competent"`
	Certified      sdk.Boolean                       `nexa:"string=Certified,default=false"`
	CertExpiry     sdk.Date                          `nexa:"string=Certificate Expiry"`
	Notes          sdk.Text                          `nexa:"string=Notes"`
}

// MaintTechnicianCertificate tracks certificates and their expiry dates.
type MaintTechnicianCertificate struct {
	sdk.Model `nexa:"model=maintenance.technician.certificate"`

	TechnicianID    sdk.Many2One[MaintTechnician] `nexa:"required,index,string=Technician"`
	CertificateName sdk.String                    `nexa:"required,string=Certificate Name"`
	IssuingAuthority sdk.String                   `nexa:"string=Issuing Authority"`
	CertificateNumber sdk.String                  `nexa:"string=Certificate Number"`
	IssuedDate      sdk.Date                      `nexa:"string=Issued Date"`
	ExpiryDate      sdk.Date                      `nexa:"string=Expiry Date"`
	Status          sdk.Selection[CertStatus]     `nexa:"required,string=Status,default=valid"`
	Notes           sdk.Text                      `nexa:"string=Notes"`
}

// MaintShiftSchedule manages shift schedules and working hours for technicians.
type MaintShiftSchedule struct {
	sdk.Model `nexa:"model=maintenance.shift.schedule"`

	Name           sdk.String                   `nexa:"required,index,string=Schedule Name"`
	TechnicianID   sdk.Many2One[MaintTechnician] `nexa:"required,index,string=Technician"`
	ShiftType      sdk.Selection[ShiftType]      `nexa:"required,string=Shift Type,default=day"`
	StartDate      sdk.Date                      `nexa:"required,string=Start Date"`
	EndDate        sdk.Date                      `nexa:"string=End Date"`
	MondayHours    sdk.Integer                   `nexa:"string=Monday Hours,default=8"`
	TuesdayHours   sdk.Integer                   `nexa:"string=Tuesday Hours,default=8"`
	WednesdayHours sdk.Integer                   `nexa:"string=Wednesday Hours,default=8"`
	ThursdayHours  sdk.Integer                   `nexa:"string=Thursday Hours,default=8"`
	FridayHours    sdk.Integer                   `nexa:"string=Friday Hours,default=8"`
	SaturdayHours  sdk.Integer                   `nexa:"string=Saturday Hours,default=0"`
	SundayHours    sdk.Integer                   `nexa:"string=Sunday Hours,default=0"`
	Notes          sdk.Text                      `nexa:"string=Notes"`
	Active         sdk.Boolean                   `nexa:"string=Active,default=true"`
}

// MaintLaborEntry records labor time entries against work orders.
type MaintLaborEntry struct {
	sdk.Model `nexa:"model=maintenance.labor.entry"`

	WorkOrderID  sdk.Many2One[MaintenanceWorkOrder] `nexa:"required,index,string=Work Order"`
	TechnicianID sdk.Many2One[MaintTechnician]      `nexa:"index,string=Technician"`
	UserID       sdk.Many2One[CoreUser]             `nexa:"required,index,string=User"`
	Date         sdk.Date                           `nexa:"required,string=Date"`
	StartTime    sdk.DateTime                       `nexa:"string=Start Time"`
	EndTime      sdk.DateTime                       `nexa:"string=End Time"`
	Hours        sdk.Numeric                        `nexa:"required,string=Hours,precision=18,scale=2,default=0"`
	IsOvertime   sdk.Boolean                        `nexa:"string=Overtime,default=false"`
	Description  sdk.Text                           `nexa:"string=Description"`
	State        sdk.Selection[LaborEntryState]     `nexa:"required,string=State,default=draft"`
	ApprovedBy   sdk.Many2One[CoreUser]             `nexa:"string=Approved By"`
	ApprovedAt   sdk.DateTime                       `nexa:"string=Approved At"`
	LaborRate    sdk.Numeric                        `nexa:"string=Labor Rate,precision=18,scale=2,default=0"`
	TotalCost    sdk.Numeric                        `nexa:"string=Total Cost,precision=18,scale=2,default=0"`
	Notes        sdk.Text                           `nexa:"string=Notes"`
}

func init() {
	sdk.MustRegister("maintenance_workforce",
		&MaintSkillCategory{},
		&MaintSkill{},
		&MaintTechnician{},
		&MaintTechnicianSkill{},
		&MaintTechnicianCertificate{},
		&MaintShiftSchedule{},
		&MaintLaborEntry{},
	)
}
