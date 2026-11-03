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

type LoanState string

const (
	LoanStateDraft     LoanState = "draft"
	LoanStateConfirmed LoanState = "confirmed"
	LoanStateDisbursed LoanState = "disbursed"
	LoanStateActive    LoanState = "active"
	LoanStateClosed    LoanState = "closed"
	LoanStateRejected  LoanState = "rejected"
)

type RepaymentState string

const (
	RepaymentStatePending RepaymentState = "pending"
	RepaymentStatePaid    RepaymentState = "paid"
	RepaymentStateSkipped RepaymentState = "skipped"
)

// HrLoanType defines a loan product with eligibility rules.
type HrLoanType struct {
	sdk.Model `nexa:"model=hr.loan.type"`

	Name            sdk.String  `nexa:"required,string=Name"`
	MaxAmount       sdk.Numeric `nexa:"string=Max Amount,precision=18,scale=2,default=0"`
	MinAmount       sdk.Numeric `nexa:"string=Min Amount,precision=18,scale=2,default=0"`
	MaxInstallments sdk.Integer `nexa:"string=Max Installments,default=12"`
	InterestRate    sdk.Numeric `nexa:"string=Interest Rate (%),precision=18,scale=2,default=0"`
	Active          sdk.Boolean `nexa:"string=Active,default=true"`
}

// HrLoan is an individual employee loan or salary advance.
type HrLoan struct {
	sdk.Model `nexa:"model=hr.loan"`

	EmployeeID       sdk.Many2One[HrEmployee]     `nexa:"required,string=Employee"`
	LoanTypeID       sdk.Many2One[HrLoanType]     `nexa:"required,string=Loan Type"`
	Amount           sdk.Numeric                  `nexa:"required,string=Amount,precision=18,scale=2,default=0"`
	InstallmentCount sdk.Integer                  `nexa:"string=Installments,default=12"`
	MonthlyDeduction sdk.Numeric                  `nexa:"string=Monthly Deduction,precision=18,scale=2,default=0"`
	RemainingBalance sdk.Numeric                  `nexa:"string=Remaining Balance,precision=18,scale=2,default=0"`
	StartDate        sdk.Date                     `nexa:"string=Start Date"`
	EndDate          sdk.Date                     `nexa:"string=End Date"`
	JournalID        sdk.Many2One[AccountJournal] `nexa:"string=Disbursement Journal"`
	AccountID        sdk.Many2One[AccountAccount] `nexa:"string=Loan Account"`
	CompanyID        sdk.Many2One[CoreCompany]    `nexa:"string=Company"`
	State            sdk.Selection[LoanState]     `nexa:"string=Status,default=draft"`
	Notes            sdk.Text                     `nexa:"string=Notes"`
}

// HrLoanLine represents one scheduled repayment installment.
type HrLoanLine struct {
	sdk.Model `nexa:"model=hr.loan.line"`

	LoanID   sdk.Many2One[HrLoan]          `nexa:"required,string=Loan"`
	Number   sdk.Integer                   `nexa:"string=Installment #"`
	DueDate  sdk.Date                      `nexa:"string=Due Date"`
	Amount   sdk.Numeric                   `nexa:"string=Amount,precision=18,scale=2,default=0"`
	State    sdk.Selection[RepaymentState] `nexa:"string=Status,default=pending"`
	PaidDate sdk.Date                      `nexa:"string=Paid Date"`
}

func init() {
	sdk.MustRegister("hr_loan",
		&HrLoanType{},
		&HrLoan{},
		&HrLoanLine{},
	)
}
