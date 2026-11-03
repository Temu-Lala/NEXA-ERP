package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// School Management — anchor (core academic + people) models.
// Student identity may link to the shared contacts master (core.partner) but
// the school keeps its own enrollment record. Financial authorities (invoices,
// payments, credit notes) live in Accounting; school-owned records below only
// structure the academic and enrollment data that fees/payments hang off.

type (
	CoreUser    = basemodels.CoreUser
	CorePartner = basemodels.CorePartner
	CoreCompany = basemodels.CoreCompany
)

type Gender string

const (
	GenderMale   Gender = "male"
	GenderFemale Gender = "female"
	GenderOther  Gender = "other"
)

type StudentStatus string

const (
	StuActive    StudentStatus = "active"
	StuInactive  StudentStatus = "inactive"
	StuRetained  StudentStatus = "retained"
	StuGraduated StudentStatus = "graduated"
	StuAlumni    StudentStatus = "alumni"
	StoSuspended StudentStatus = "suspended"
)

type FeeCategoryType string

const (
	FeeTuition   FeeCategoryType = "tuition"
	FeePenalty   FeeCategoryType = "penalty"
	FeeUniform   FeeCategoryType = "uniform"
	FeeBooks     FeeCategoryType = "books"
	FeeTutorial  FeeCategoryType = "tutorial"
	FeeBadge     FeeCategoryType = "badge"
	FeeTransport FeeCategoryType = "transport"
	FeeOther     FeeCategoryType = "other"
)

// SchoolAcademicYear defines an enrollment/fee accounting period.
type SchoolAcademicYear struct {
	sdk.Model `nexa:"model=school.academic.year"`

	Name      sdk.String                `nexa:"required,size=120,string=Name"`
	Code      sdk.String                `nexa:"size=20,string=Code"`
	DateStart sdk.Date                  `nexa:"required,string=Start Date"`
	DateEnd   sdk.Date                  `nexa:"required,string=End Date"`
	IsCurrent sdk.Boolean               `nexa:"string=Current Year,default=false"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Notes     sdk.Text                  `nexa:"string=Notes"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true"`
}

// SchoolClass is a grade level (Grade 1, Grade 2, ...).
type SchoolClass struct {
	sdk.Model `nexa:"model=school.class"`

	Name        sdk.String                `nexa:"required,size=120,string=Name"`
	Code        sdk.String                `nexa:"size=20,string=Code"`
	Level       sdk.Integer               `nexa:"string=Grade Level,default=1"`
	Description sdk.Text                  `nexa:"string=Description"`
	CompanyID   sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active      sdk.Boolean               `nexa:"string=Active,default=true"`
}

// SchoolSection is a sub-class/section within a class for an academic year.
type SchoolSection struct {
	sdk.Model `nexa:"model=school.section"`

	Name              sdk.String                       `nexa:"required,size=120,string=Name"`
	ClassID           sdk.Many2One[SchoolClass]        `nexa:"required,index,string=Class"`
	AcademicYearID    sdk.Many2One[SchoolAcademicYear] `nexa:"string=Academic Year"`
	Capacity          sdk.Integer                      `nexa:"string=Capacity,default=0"`
	HomeroomTeacherID sdk.Many2One[CoreUser]           `nexa:"string=Homeroom Teacher"`
	CompanyID         sdk.Many2One[CoreCompany]        `nexa:"string=Company"`
	Active            sdk.Boolean                      `nexa:"string=Active,default=true"`
}

// SchoolStudent is the school-side enrollment record for a learner.
type SchoolStudent struct {
	sdk.Model `nexa:"model=school.student"`

	Name              sdk.String                       `nexa:"required,size=120,string=Student Name"`
	Code              sdk.String                       `nexa:"required,index,size=40,string=Student ID"`
	PartnerID         sdk.Many2One[CorePartner]        `nexa:"string=Contact"`
	Gender            sdk.Selection[Gender]            `nexa:"string=Gender"`
	DateBirth         sdk.Date                         `nexa:"string=Date of Birth"`
	ClassID           sdk.Many2One[SchoolClass]        `nexa:"index,string=Class"`
	SectionID         sdk.Many2One[SchoolSection]      `nexa:"index,string=Section"`
	AcademicYearID    sdk.Many2One[SchoolAcademicYear] `nexa:"string=Academic Year"`
	RollNumber        sdk.String                       `nexa:"size=40,string=Roll Number"`
	GuardianName      sdk.String                       `nexa:"size=120,string=Parent/Guardian"`
	GuardianPhone     sdk.String                       `nexa:"size=40,string=Guardian Phone"`
	GuardianEmail     sdk.String                       `nexa:"size=120,string=Guardian Email"`
	GuardianPartnerID sdk.Many2One[CorePartner]        `nexa:"string=Guardian Contact"`
	AdmissionDate     sdk.Date                         `nexa:"string=Admission Date,default=today"`
	IsTeacherChild    sdk.Boolean                      `nexa:"string=Teacher's Child,default=false"`
	TeacherPartnerID  sdk.Many2One[CorePartner]        `nexa:"string=Teacher (Parent)"`
	Status            sdk.Selection[StudentStatus]     `nexa:"string=Status,default=active"`
	CompanyID         sdk.Many2One[CoreCompany]        `nexa:"string=Company"`
	Notes             sdk.Text                         `nexa:"string=Notes"`
	Active            sdk.Boolean                      `nexa:"string=Active,default=true"`
}

// SchoolFeeCategory classifies collectible income (tuition, uniform, books, ...)
// to power income-by-category reporting.
type SchoolFeeCategory struct {
	sdk.Model `nexa:"model=school.fee.category"`

	Name         sdk.String                     `nexa:"required,size=120,string=Name"`
	Code         sdk.String                     `nexa:"size=30,string=Code"`
	CategoryType sdk.Selection[FeeCategoryType] `nexa:"string=Category Type,default=other"`
	ProductID    sdk.Many2One[sdk.Any]          `nexa:"string=Product,comodel=product.product"`
	Sequence     sdk.Integer                    `nexa:"string=Sequence,default=10"`
	Active       sdk.Boolean                    `nexa:"string=Active,default=true"`
}

func init() {
	sdk.MustRegister("school",
		&SchoolAcademicYear{},
		&SchoolClass{},
		&SchoolSection{},
		&SchoolStudent{},
		&SchoolFeeCategory{},
	)
}
