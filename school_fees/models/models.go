package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	schoolmodels "nexa_addons/school/models"
)

// Fee structure and dynamic discount/penalty configuration. All thresholds are
// admin-configurable records (never hardcoded): family multi-sibling annual
// discounts, teacher's-child experience-to-discount matrix, and daily penalty
// rules. Percentages are stored as whole numbers (e.g. 45 => 45%).

type (
	CoreCompany        = basemodels.CoreCompany
	SchoolClass        = schoolmodels.SchoolClass
	SchoolSection      = schoolmodels.SchoolSection
	SchoolAcademicYear = schoolmodels.SchoolAcademicYear
	SchoolFeeCategory  = schoolmodels.SchoolFeeCategory
)

type FeeFrequency string

const (
	FreqMonthly   FeeFrequency = "monthly"
	FreqQuarterly FeeFrequency = "quarterly"
	FreqYearly    FeeFrequency = "yearly"
	FreqTerm      FeeFrequency = "term"
)

type PenaltyCalc string

const (
	PenDaily   PenaltyCalc = "daily"
	PenFixed   PenaltyCalc = "fixed"
	PenPercent PenaltyCalc = "percent"
)

// SchoolFeeStructure defines how much a class/section must pay for a fee
// category, at what frequency, and by which day of the period.
type SchoolFeeStructure struct {
	sdk.Model `nexa:"model=school.fee.structure"`

	Name           sdk.String                       `nexa:"required,size=120,string=Name"`
	ClassID        sdk.Many2One[SchoolClass]        `nexa:"index,string=Class"`
	SectionID      sdk.Many2One[SchoolSection]      `nexa:"index,string=Section"`
	FeeCategoryID  sdk.Many2One[SchoolFeeCategory]  `nexa:"required,string=Fee Category"`
	AcademicYearID sdk.Many2One[SchoolAcademicYear] `nexa:"string=Academic Year"`
	Amount         sdk.Numeric                      `nexa:"required,string=Amount,precision=18,scale=2,default=0"`
	Frequency      sdk.Selection[FeeFrequency]      `nexa:"string=Frequency,default=monthly"`
	DueDay         sdk.Integer                      `nexa:"string=Due Day of Month,default=30"`
	Currency       sdk.String                       `nexa:"size=3,string=Currency,default=USD"`
	CompanyID      sdk.Many2One[CoreCompany]        `nexa:"string=Company"`
	Notes          sdk.Text                         `nexa:"string=Notes"`
	Active         sdk.Boolean                      `nexa:"string=Active,default=true"`
}

// SchoolPenaltyRule configures late-payment penalties. When Calculation is
// daily, Rate is charged per day past the due date (after GraceDays).
type SchoolPenaltyRule struct {
	sdk.Model `nexa:"model=school.penalty.rule"`

	Name          sdk.String                      `nexa:"required,size=120,string=Name"`
	FeeCategoryID sdk.Many2One[SchoolFeeCategory] `nexa:"string=Applies To Category"`
	Calculation   sdk.Selection[PenaltyCalc]      `nexa:"string=Calculation,default=daily"`
	Rate          sdk.Numeric                     `nexa:"required,string=Rate,precision=18,scale=2,default=0"`
	GraceDays     sdk.Integer                     `nexa:"string=Grace Days,default=0"`
	MaxPenalty    sdk.Numeric                     `nexa:"string=Max Penalty (0=uncapped),precision=18,scale=2,default=0"`
	CompanyID     sdk.Many2One[CoreCompany]       `nexa:"string=Company"`
	Active        sdk.Boolean                     `nexa:"string=Active,default=true"`
}

// SchoolFamilyDiscountRule grants an annual discount during enrollment when a
// family registers at least MinSiblings students (e.g. >3 => 45%, >5 => 50%).
type SchoolFamilyDiscountRule struct {
	sdk.Model `nexa:"model=school.discount.family"`

	Name           sdk.String                       `nexa:"required,size=120,string=Name"`
	MinSiblings    sdk.Integer                      `nexa:"required,string=Min Siblings,default=0"`
	DiscountPct    sdk.Numeric                      `nexa:"required,string=Discount %,precision=5,scale=2,default=0"`
	AcademicYearID sdk.Many2One[SchoolAcademicYear] `nexa:"string=Academic Year"`
	Priority       sdk.Integer                      `nexa:"string=Priority,default=10"`
	CompanyID      sdk.Many2One[CoreCompany]        `nexa:"string=Company"`
	Active         sdk.Boolean                      `nexa:"string=Active,default=true"`
}

// SchoolTeacherDiscountRule maps a teacher's years of service to a monthly
// discount for their children (Experience-to-Discount Matrix).
type SchoolTeacherDiscountRule struct {
	sdk.Model `nexa:"model=school.discount.teacher"`

	Name            sdk.String                `nexa:"required,size=120,string=Name"`
	MinYearsService sdk.Integer               `nexa:"required,string=Min Years of Service,default=0"`
	DiscountPct     sdk.Numeric               `nexa:"required,string=Monthly Discount %,precision=5,scale=2,default=0"`
	Priority        sdk.Integer               `nexa:"string=Priority,default=10"`
	CompanyID       sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active          sdk.Boolean               `nexa:"string=Active,default=true"`
}

func init() {
	sdk.MustRegister("school_fees",
		&SchoolFeeStructure{},
		&SchoolPenaltyRule{},
		&SchoolFamilyDiscountRule{},
		&SchoolTeacherDiscountRule{},
	)
}
