package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	lendingmodels "nexa_addons/lending/models"
)

type (
	CoreUser       = basemodels.CoreUser
	LoanAccount    = lendingmodels.LoanAccount
	InterestMethod = lendingmodels.InterestMethod
)

// ============================================================================
// Schedule Version
// ============================================================================

type ScheduleStatus string

const (
	ScheduleStatusDraft      ScheduleStatus = "draft"
	ScheduleStatusApproved   ScheduleStatus = "approved"
	ScheduleStatusSuperseded ScheduleStatus = "superseded"
)

type LoanScheduleVersion struct {
	sdk.Model

	AccountID      sdk.Many2One[LoanAccount]     `nexa:"string=Loan Account,required,index"`
	Version        sdk.Integer                   `nexa:"string=Version,default=1"`
	InterestMethod sdk.Selection[InterestMethod] `nexa:"string=Interest Method,default=declining_balance"`
	GeneratedAt    sdk.DateTime                  `nexa:"string=Generated At"`
	Status         sdk.Selection[ScheduleStatus] `nexa:"string=Status,default=draft"`

	TotalPrincipal sdk.Numeric `nexa:"string=Total Principal,precision=18,scale=2"`
	TotalInterest  sdk.Numeric `nexa:"string=Total Interest,precision=18,scale=2"`
	TotalFees      sdk.Numeric `nexa:"string=Total Fees,precision=18,scale=2"`
	TotalAmount    sdk.Numeric `nexa:"string=Total Amount,precision=18,scale=2"`

	Reason sdk.Text `nexa:"string=Reason"`
	Note   sdk.Text `nexa:"string=Note"`

	Installments sdk.One2Many[LoanInstallment] `nexa:"string=Installments"`

	// Non-stored progress aggregates over the installment lines.
	InstallmentCount sdk.Integer `nexa:"string=Installments,compute=compute_schedule_installment_count"`
	PaidCount        sdk.Integer `nexa:"string=Paid,compute=compute_schedule_paid_count"`
}

// ============================================================================
// Installment
// ============================================================================

type InstallmentState string

const (
	InstallmentStateUpcoming InstallmentState = "upcoming"
	InstallmentStateDue      InstallmentState = "due"
	InstallmentStatePartial  InstallmentState = "partial"
	InstallmentStatePaid     InstallmentState = "paid"
	InstallmentStateOverdue  InstallmentState = "overdue"
)

type LoanInstallment struct {
	sdk.Model

	ScheduleID sdk.Many2One[LoanScheduleVersion] `nexa:"string=Schedule,required,index"`
	AccountID  sdk.Many2One[LoanAccount]         `nexa:"string=Loan Account,index"`
	Number     sdk.Integer                       `nexa:"string=Number"`
	DueDate    sdk.Date                          `nexa:"string=Due Date"`

	OpeningPrincipal sdk.Numeric `nexa:"string=Opening Principal,precision=18,scale=2"`
	PrincipalDue     sdk.Numeric `nexa:"string=Principal Due,precision=18,scale=2"`
	InterestDue      sdk.Numeric `nexa:"string=Interest Due,precision=18,scale=2"`
	FeesDue          sdk.Numeric `nexa:"string=Fees Due,precision=18,scale=2"`
	PenaltyDue       sdk.Numeric `nexa:"string=Penalty Due,precision=18,scale=2"`
	TotalDue         sdk.Numeric `nexa:"string=Total Due,precision=18,scale=2"`

	PaidPrincipal sdk.Numeric `nexa:"string=Paid Principal,precision=18,scale=2"`
	PaidInterest  sdk.Numeric `nexa:"string=Paid Interest,precision=18,scale=2"`
	PaidFees      sdk.Numeric `nexa:"string=Paid Fees,precision=18,scale=2"`
	PaidPenalty   sdk.Numeric `nexa:"string=Paid Penalty,precision=18,scale=2"`
	TotalPaid     sdk.Numeric `nexa:"string=Total Paid,precision=18,scale=2"`

	Balance     sdk.Numeric                     `nexa:"string=Outstanding Principal After,precision=18,scale=2"`
	DaysPastDue sdk.Integer                     `nexa:"string=Days Past Due"`
	State       sdk.Selection[InstallmentState] `nexa:"string=Status,default=upcoming"`
	Note        sdk.Text                        `nexa:"string=Note"`

	// Non-stored remaining amount (total due less total paid).
	AmountDue sdk.Numeric `nexa:"string=Amount Due,precision=18,scale=2,compute=compute_installment_amount_due"`
}

// ============================================================================
// Accrual
// ============================================================================

type LoanAccrual struct {
	sdk.Model

	AccountID            sdk.Many2One[LoanAccount] `nexa:"string=Loan Account,required,index"`
	Date                 sdk.Date                  `nexa:"string=Date"`
	Period               sdk.Integer               `nexa:"string=Period"`
	InterestAccrued      sdk.Numeric               `nexa:"string=Interest Accrued,precision=18,scale=2"`
	InterestReversed     sdk.Numeric               `nexa:"string=Interest Reversed,precision=18,scale=2"`
	PenaltyAccrued       sdk.Numeric               `nexa:"string=Penalty Accrued,precision=18,scale=2"`
	OutstandingPrincipal sdk.Numeric               `nexa:"string=Outstanding Principal,precision=18,scale=2"`
	Note                 sdk.Text                  `nexa:"string=Note"`
}

// ============================================================================
// Adjustment
// ============================================================================

type AdjustmentType string

const (
	AdjustmentTypeReschedule  AdjustmentType = "reschedule"
	AdjustmentTypeRestructure AdjustmentType = "restructure"
	AdjustmentTypeWaiver      AdjustmentType = "waiver"
	AdjustmentTypeCorrection  AdjustmentType = "correction"
)

type LoanAdjustment struct {
	sdk.Model

	AccountID  sdk.Many2One[LoanAccount]         `nexa:"string=Loan Account,required,index"`
	ScheduleID sdk.Many2One[LoanScheduleVersion] `nexa:"string=Schedule"`
	Type       sdk.Selection[AdjustmentType]     `nexa:"string=Type,default=correction"`
	Date       sdk.Date                          `nexa:"string=Date"`
	Amount     sdk.Numeric                       `nexa:"string=Amount,precision=18,scale=2"`
	Reason     sdk.Text                          `nexa:"string=Reason"`
	Approved   sdk.Boolean                       `nexa:"string=Approved"`
	ApprovedBy sdk.Many2One[CoreUser]            `nexa:"string=Approved By"`
	Note       sdk.Text                          `nexa:"string=Note"`
}

func init() {
	sdk.MustRegister("lending_servicing",
		&LoanScheduleVersion{},
		&LoanInstallment{},
		&LoanAccrual{},
		&LoanAdjustment{},
	)
}
