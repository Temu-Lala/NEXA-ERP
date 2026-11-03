package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	schoolmodels "nexa_addons/school/models"
)

// Read-models that aggregate the transactional school data into the reports the
// SRS calls for: per-student payment status (by class/section), income by fee
// category over a period, and a defaulters list. These are snapshot rows that a
// reporting job or an operator refresh regenerates from the authoritative bills
// and payments; they are meant to be read and exported, not hand-edited.

type (
	CoreCompany        = basemodels.CoreCompany
	SchoolStudent      = schoolmodels.SchoolStudent
	SchoolClass        = schoolmodels.SchoolClass
	SchoolSection      = schoolmodels.SchoolSection
	SchoolAcademicYear = schoolmodels.SchoolAcademicYear
	SchoolFeeCategory  = schoolmodels.SchoolFeeCategory
)

type ReportStatus string

const (
	RsPaid    ReportStatus = "paid"
	RsPartial ReportStatus = "partial"
	RsUnpaid  ReportStatus = "unpaid"
)

type IncomePeriod string

const (
	IpDaily     IncomePeriod = "daily"
	IpMonthly   IncomePeriod = "monthly"
	IpQuarterly IncomePeriod = "quarterly"
	IpYearly    IncomePeriod = "yearly"
	IpCustom    IncomePeriod = "custom"
)

// SchoolPaymentReport: one row per student for a reporting window, with the
// fee required, amount paid, remaining balance and status.
type SchoolPaymentReport struct {
	sdk.Model `nexa:"model=school.payment.report"`

	Name             sdk.String                       `nexa:"required,size=120,string=Reference"`
	StudentID        sdk.Many2One[SchoolStudent]      `nexa:"required,index,string=Student"`
	ClassID          sdk.Many2One[SchoolClass]        `nexa:"index,string=Class"`
	SectionID        sdk.Many2One[SchoolSection]      `nexa:"index,string=Section"`
	AcademicYearID   sdk.Many2One[SchoolAcademicYear] `nexa:"string=Academic Year"`
	PeriodStart      sdk.Date                         `nexa:"string=Period Start"`
	PeriodEnd        sdk.Date                         `nexa:"string=Period End"`
	TotalFeeRequired sdk.Numeric                      `nexa:"string=Total Fee Required,precision=18,scale=2,default=0"`
	TotalPaid        sdk.Numeric                      `nexa:"string=Total Paid,precision=18,scale=2,default=0"`
	RemainingBalance sdk.Numeric                      `nexa:"string=Remaining Balance,precision=18,scale=2,default=0"`
	LastPaymentDate  sdk.Date                         `nexa:"string=Last Payment Date"`
	Status           sdk.Selection[ReportStatus]      `nexa:"string=Status,default=unpaid"`
	CompanyID        sdk.Many2One[CoreCompany]        `nexa:"string=Company"`
	Notes            sdk.Text                         `nexa:"string=Notes"`
}

// SchoolIncomeReport: collected revenue grouped by fee category and period.
type SchoolIncomeReport struct {
	sdk.Model `nexa:"model=school.income.report"`

	Name             sdk.String                      `nexa:"required,size=120,string=Reference"`
	FeeCategoryID    sdk.Many2One[SchoolFeeCategory] `nexa:"index,string=Fee Category"`
	PeriodType       sdk.Selection[IncomePeriod]     `nexa:"string=Period,default=monthly"`
	DateStart        sdk.Date                        `nexa:"string=Date Start"`
	DateEnd          sdk.Date                        `nexa:"string=Date End"`
	TransactionCount sdk.Integer                     `nexa:"string=Transactions,default=0"`
	TotalCollected   sdk.Numeric                     `nexa:"string=Total Collected,precision=18,scale=2,default=0"`
	CompanyID        sdk.Many2One[CoreCompany]       `nexa:"string=Company"`
	Notes            sdk.Text                        `nexa:"string=Notes"`
}

// SchoolDefaulterReport: students with an outstanding balance as of a date,
// including which months are overdue and accumulated penalties.
type SchoolDefaulterReport struct {
	sdk.Model `nexa:"model=school.defaulter.report"`

	Name               sdk.String                       `nexa:"required,size=120,string=Reference"`
	StudentID          sdk.Many2One[SchoolStudent]      `nexa:"required,index,string=Student"`
	ClassID            sdk.Many2One[SchoolClass]        `nexa:"index,string=Class"`
	SectionID          sdk.Many2One[SchoolSection]      `nexa:"index,string=Section"`
	AcademicYearID     sdk.Many2One[SchoolAcademicYear] `nexa:"string=Academic Year"`
	RequiredFee        sdk.Numeric                      `nexa:"string=Required Fee,precision=18,scale=2,default=0"`
	AmountPaid         sdk.Numeric                      `nexa:"string=Amount Paid,precision=18,scale=2,default=0"`
	Outstanding        sdk.Numeric                      `nexa:"string=Outstanding,precision=18,scale=2,default=0"`
	DueDate            sdk.Date                         `nexa:"string=Due Date"`
	UnpaidMonthCount   sdk.Integer                      `nexa:"string=Unpaid Months,default=0"`
	UnpaidMonths       sdk.String                       `nexa:"size=255,string=Overdue Months"`
	PenaltyAccumulated sdk.Numeric                      `nexa:"string=Penalty Accumulated,precision=18,scale=2,default=0"`
	CompanyID          sdk.Many2One[CoreCompany]        `nexa:"string=Company"`
	Notes              sdk.Text                         `nexa:"string=Notes"`
}

func init() {
	sdk.MustRegister("school_reporting",
		&SchoolPaymentReport{},
		&SchoolIncomeReport{},
		&SchoolDefaulterReport{},
	)
}
