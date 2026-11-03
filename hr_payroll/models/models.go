package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	acctmodels "nexa_addons/account/models"
	hrmodels "nexa_addons/hr/models"
)

// Cross-module reference aliases.
type (
	HrEmployee     = hrmodels.HrEmployee
	AccountAccount = acctmodels.AccountAccount
	AccountJournal = acctmodels.AccountJournal
	CoreCompany    = basemodels.CoreCompany
)

type CategoryKind string

const (
	CategoryKindEarning   CategoryKind = "earning"
	CategoryKindDeduction CategoryKind = "deduction"
)

type WageType string

const (
	WageTypeMonthly WageType = "monthly"
	WageTypeHourly  WageType = "hourly"
)

type RuleAmountType string

const (
	RuleAmountFixed           RuleAmountType = "fixed"
	RuleAmountPercentageBasic RuleAmountType = "percentage_basic"
	RuleAmountPercentageGross RuleAmountType = "percentage_gross"
)

type PayslipState string

const (
	PayslipStateDraft     PayslipState = "draft"
	PayslipStateConfirmed PayslipState = "confirmed"
	PayslipStatePaid      PayslipState = "paid"
)

type PayslipRunState string

const (
	PayslipRunStateDraft     PayslipRunState = "draft"
	PayslipRunStateConfirmed PayslipRunState = "confirmed"
	PayslipRunStateClosed    PayslipRunState = "closed"
)

type HrPayrollCategory struct {
	sdk.Model `nexa:"model=hr.payroll.category"`

	Name      sdk.String                   `nexa:"required,string=Name"`
	Code      sdk.String                   `nexa:"string=Code"`
	Kind      sdk.Selection[CategoryKind]  `nexa:"string=Category Type,default=earning"`
	AccountID sdk.Many2One[AccountAccount] `nexa:"string=Account"`
	Sequence  sdk.Integer                  `nexa:"string=Sequence,default=10"`
	Active    sdk.Boolean                  `nexa:"string=Active,default=true"`
}

type HrPayrollStructure struct {
	sdk.Model `nexa:"model=hr.payroll.structure"`

	Name      sdk.String                `nexa:"required,string=Name"`
	Code      sdk.String                `nexa:"string=Code"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	WageType  sdk.Selection[WageType]   `nexa:"string=Wage Type,default=monthly"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true"`
}

type HrSalaryRule struct {
	sdk.Model `nexa:"model=hr.salary.rule"`

	Name          sdk.String                       `nexa:"required,string=Name"`
	Code          sdk.String                       `nexa:"string=Code"`
	CategoryID    sdk.Many2One[HrPayrollCategory]  `nexa:"required,string=Category"`
	StructureID   sdk.Many2One[HrPayrollStructure] `nexa:"string=Structure"`
	Sequence      sdk.Integer                      `nexa:"string=Sequence,default=10"`
	AmountType    sdk.Selection[RuleAmountType]    `nexa:"string=Computation,default=fixed"`
	AmountPercent sdk.Numeric                      `nexa:"string=Percentage,precision=18,scale=2,default=0"`
	AmountFixed   sdk.Numeric                      `nexa:"string=Fixed Amount,precision=18,scale=2,default=0"`
	Active        sdk.Boolean                      `nexa:"string=Active,default=true"`
}

type HrPayslipRun struct {
	sdk.Model `nexa:"model=hr.payslip.run"`

	Name        sdk.String                       `nexa:"required,string=Name"`
	StructureID sdk.Many2One[HrPayrollStructure] `nexa:"string=Structure"`
	PeriodStart sdk.Date                         `nexa:"required,string=Period Start"`
	PeriodEnd   sdk.Date                         `nexa:"required,string=Period End"`
	JournalID   sdk.Many2One[AccountJournal]     `nexa:"string=Journal"`
	State       sdk.Selection[PayslipRunState]   `nexa:"string=Status,default=draft"`
}

type HrPayslip struct {
	sdk.Model `nexa:"model=hr.payslip"`

	Number         sdk.String                       `nexa:"string=Number"`
	EmployeeID     sdk.Many2One[HrEmployee]         `nexa:"required,string=Employee"`
	RunID          sdk.Many2One[HrPayslipRun]       `nexa:"string=Payslip Run"`
	StructureID    sdk.Many2One[HrPayrollStructure] `nexa:"string=Structure"`
	DateStart      sdk.Date                         `nexa:"string=Start Date"`
	DateEnd        sdk.Date                         `nexa:"string=End Date"`
	Basic          sdk.Numeric                      `nexa:"string=Basic Salary,precision=18,scale=2,default=0"`
	Gross          sdk.Numeric                      `nexa:"string=Gross Salary,precision=18,scale=2,default=0"`
	TotalDeduction sdk.Numeric                      `nexa:"string=Total Deduction,precision=18,scale=2,default=0"`
	Net            sdk.Numeric                      `nexa:"string=Net Salary,precision=18,scale=2,default=0"`
	State          sdk.Selection[PayslipState]      `nexa:"string=Status,default=draft"`
	Note           sdk.Text                         `nexa:"string=Note"`
}

type HrPayslipLine struct {
	sdk.Model `nexa:"model=hr.payslip.line"`

	PayslipID  sdk.Many2One[HrPayslip]         `nexa:"required,string=Payslip"`
	RuleID     sdk.Many2One[HrSalaryRule]      `nexa:"string=Salary Rule"`
	CategoryID sdk.Many2One[HrPayrollCategory] `nexa:"string=Category"`
	Name       sdk.String                      `nexa:"string=Name"`
	Sequence   sdk.Integer                     `nexa:"string=Sequence,default=10"`
	Amount     sdk.Numeric                     `nexa:"string=Amount,precision=18,scale=2,default=0"`
}

func init() {
	sdk.MustRegister("hr_payroll",
		&HrPayrollCategory{},
		&HrPayrollStructure{},
		&HrSalaryRule{},
		&HrPayslipRun{},
		&HrPayslip{},
		&HrPayslipLine{},
	)
}
