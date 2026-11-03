package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	lendingmodels "nexa_addons/lending/models"
	servicemodels "nexa_addons/lending_servicing/models"
)

type (
	CoreUser        = basemodels.CoreUser
	LoanAccount     = lendingmodels.LoanAccount
	LoanApplication = lendingmodels.LoanApplication
	LoanInstallment = servicemodels.LoanInstallment
)

// ============================================================================
// Disbursement
// ============================================================================

type DisbursementMethod string

const (
	DisbursementMethodBankTransfer DisbursementMethod = "bank_transfer"
	DisbursementMethodMobileMoney  DisbursementMethod = "mobile_money"
	DisbursementMethodCash         DisbursementMethod = "cash"
	DisbursementMethodCheck        DisbursementMethod = "check"
)

type DisbursementStatus string

const (
	DisbursementStatusDraft     DisbursementStatus = "draft"
	DisbursementStatusPending   DisbursementStatus = "pending"
	DisbursementStatusConfirmed DisbursementStatus = "confirmed"
	DisbursementStatusFailed    DisbursementStatus = "failed"
	DisbursementStatusReversed  DisbursementStatus = "reversed"
)

type LoanDisbursement struct {
	sdk.Model

	Name           sdk.String                        `nexa:"string=Reference,required"`
	AccountID      sdk.Many2One[LoanAccount]         `nexa:"string=Loan Account,required,index"`
	ApplicationID  sdk.Many2One[LoanApplication]     `nexa:"string=Application"`
	GrossAmount    sdk.Numeric                       `nexa:"string=Gross Amount,precision=18,scale=2"`
	FeesWithheld   sdk.Numeric                       `nexa:"string=Fees Withheld,precision=18,scale=2"`
	NetAmount      sdk.Numeric                       `nexa:"string=Net Amount,precision=18,scale=2"`
	CurrencyID     sdk.Many2One[sdk.Any]             `nexa:"string=Currency,comodel=core.currency"`
	Method         sdk.Selection[DisbursementMethod] `nexa:"string=Method,default=bank_transfer"`
	ProviderRef    sdk.String                        `nexa:"string=Provider Reference"`
	IdempotencyKey sdk.String                        `nexa:"string=Idempotency Key,index"`
	Status         sdk.Selection[DisbursementStatus] `nexa:"string=Status,default=draft"`
	DisbursedAt    sdk.DateTime                      `nexa:"string=Disbursed At"`
	JournalEntryID sdk.Many2One[sdk.Any]             `nexa:"string=Journal Entry,comodel=account.move"`
	Note           sdk.Text                          `nexa:"string=Note"`
}

// ============================================================================
// Repayment
// ============================================================================

type RepaymentChannel string

const (
	RepaymentChannelBankTransfer RepaymentChannel = "bank_transfer"
	RepaymentChannelMobileMoney  RepaymentChannel = "mobile_money"
	RepaymentChannelCash         RepaymentChannel = "cash"
	RepaymentChannelDirectDebit  RepaymentChannel = "direct_debit"
	RepaymentChannelCheck        RepaymentChannel = "check"
)

type RepaymentStatus string

const (
	RepaymentStatusPending   RepaymentStatus = "pending"
	RepaymentStatusConfirmed RepaymentStatus = "confirmed"
	RepaymentStatusReversed  RepaymentStatus = "reversed"
)

type LoanRepayment struct {
	sdk.Model

	Name            sdk.String                      `nexa:"string=Reference,required"`
	AccountID       sdk.Many2One[LoanAccount]       `nexa:"string=Loan Account,required,index"`
	Date            sdk.Date                        `nexa:"string=Date"`
	AmountReceived  sdk.Numeric                     `nexa:"string=Amount Received,precision=18,scale=2"`
	AmountConfirmed sdk.Numeric                     `nexa:"string=Amount Confirmed,precision=18,scale=2"`
	Unallocated     sdk.Numeric                     `nexa:"string=Unallocated,precision=18,scale=2"`
	Channel         sdk.Selection[RepaymentChannel] `nexa:"string=Channel,default=bank_transfer"`
	ProviderRef     sdk.String                      `nexa:"string=Provider Reference"`
	IdempotencyKey  sdk.String                      `nexa:"string=Idempotency Key,index"`
	Status          sdk.Selection[RepaymentStatus]  `nexa:"string=Status,default=pending"`
	JournalEntryID  sdk.Many2One[sdk.Any]           `nexa:"string=Journal Entry,comodel=account.move"`
	Note            sdk.Text                        `nexa:"string=Note"`

	Allocations sdk.One2Many[LoanRepaymentAllocation] `nexa:"string=Allocations"`

	// Non-stored total allocated across installments.
	AllocatedTotal sdk.Numeric `nexa:"string=Allocated,precision=18,scale=2,compute=compute_repayment_allocated_total"`
}

// ============================================================================
// Repayment Allocation
// ============================================================================

type LoanRepaymentAllocation struct {
	sdk.Model

	RepaymentID     sdk.Many2One[LoanRepayment]   `nexa:"string=Repayment,required,index"`
	InstallmentID   sdk.Many2One[LoanInstallment] `nexa:"string=Installment,index"`
	AmountPrincipal sdk.Numeric                   `nexa:"string=Principal,precision=18,scale=2"`
	AmountInterest  sdk.Numeric                   `nexa:"string=Interest,precision=18,scale=2"`
	AmountFees      sdk.Numeric                   `nexa:"string=Fees,precision=18,scale=2"`
	AmountPenalty   sdk.Numeric                   `nexa:"string=Penalty,precision=18,scale=2"`
	TotalAllocated  sdk.Numeric                   `nexa:"string=Total Allocated,precision=18,scale=2"`
	Date            sdk.Date                      `nexa:"string=Date"`
}

// ============================================================================
// Payment Reversal
// ============================================================================

type ReversalStatus string

const (
	ReversalStatusPending ReversalStatus = "pending"
	ReversalStatusPosted  ReversalStatus = "posted"
)

type LoanPaymentReversal struct {
	sdk.Model

	Name           sdk.String                    `nexa:"string=Reference,required"`
	RepaymentID    sdk.Many2One[LoanRepayment]   `nexa:"string=Repayment,required,index"`
	AccountID      sdk.Many2One[LoanAccount]     `nexa:"string=Loan Account"`
	Date           sdk.Date                      `nexa:"string=Date"`
	Amount         sdk.Numeric                   `nexa:"string=Amount,precision=18,scale=2"`
	Reason         sdk.Text                      `nexa:"string=Reason"`
	ReversedBy     sdk.Many2One[CoreUser]        `nexa:"string=Reversed By"`
	Status         sdk.Selection[ReversalStatus] `nexa:"string=Status,default=pending"`
	JournalEntryID sdk.Many2One[sdk.Any]         `nexa:"string=Journal Entry,comodel=account.move"`
	Note           sdk.Text                      `nexa:"string=Note"`
}

func init() {
	sdk.MustRegister("lending_transactions",
		&LoanDisbursement{},
		&LoanRepayment{},
		&LoanRepaymentAllocation{},
		&LoanPaymentReversal{},
	)
}
