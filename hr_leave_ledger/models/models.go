package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	hrmodels "nexa_addons/hr/models"
)

// HR-F3 "Leave Ledger". The anchor hr module tracks allocations and requests but
// has no running balance — nothing derives entitlement from what has been granted
// versus taken. This companion turns leave into a double-entry-style, append-only
// ledger:
//
//   - hr.leave.transaction records every movement (accrual, usage, adjustment,
//     reversal, carry-over) against an employee/leave-type with an effective date
//     and a link back to the source record. Entries are never edited or deleted;
//     corrections are new, opposite-signed reversals.
//   - hr.leave.balance is a live, computed view (accrued / taken / balance) that
//     aggregates the posted ledger so a balance can never drift from its entries.
//   - The engine posts/reverses ledger rows from the anchor's existing approval
//     workflow (allocation.validate -> accrual; leave.validate -> usage; refuse or
//     reset of a posted record -> reversal), idempotently, so the manually-edited
//     "balance field" pattern is fully replaced.
//   - Periodic accrual rules (amount, frequency, carry-over cap, expiry) are
//     applied by a scheduled job, also idempotent per period.
//
// Half-day / hourly leave is expressed as a fraction of a day so all balances stay
// in one unit and never mix days and hours.

type (
	HrEmployee        = hrmodels.HrEmployee
	HrLeaveType       = hrmodels.HrLeaveType
	HrLeave           = hrmodels.HrLeave
	HrLeaveAllocation = hrmodels.HrLeaveAllocation
	CoreCompany       = basemodels.CoreCompany
)

// ---------------------------------------------------------------------------
// Enumerations
// ---------------------------------------------------------------------------

// TxnCategory classifies a ledger movement. Sign convention: accrual / carry-over /
// adjustment carry their own sign; usage is always negative (consumes balance);
// reversal flips a prior entry.
type TxnCategory string

const (
	TxnAccrual   TxnCategory = "accrual"
	TxnUsage     TxnCategory = "usage"
	TxnAdjust    TxnCategory = "adjustment"
	TxnReversal  TxnCategory = "reversal"
	TxnCarryOver TxnCategory = "carryover"
)

// AccrualFrequency drives the periodic accrual job.
type AccrualFrequency string

const (
	AccrueMonthly AccrualFrequency = "monthly"
	AccrueYearly  AccrualFrequency = "yearly"
)

// ---------------------------------------------------------------------------
// Ledger
// ---------------------------------------------------------------------------

// HrLeaveTransaction is one immutable movement on an employee's leave balance,
// expressed in days (fractional for half-day/hourly). SourceModel/SourceID make
// posting idempotent and trace the entry to the record that produced it.
type HrLeaveTransaction struct {
	sdk.Model `nexa:"model=hr.leave.transaction"`

	EmployeeID  sdk.Many2One[HrEmployee]   `nexa:"required,string=Employee,index"`
	LeaveTypeID sdk.Many2One[HrLeaveType]  `nexa:"required,string=Leave Type,index"`
	Category    sdk.Selection[TxnCategory] `nexa:"string=Category,index,default=accrual,selection=accrual:Accrual,usage:Usage,adjustment:Adjustment,reversal:Reversal,carryover:Carry Over"`
	Amount      sdk.Float64                `nexa:"required,string=Days (+/-)"`
	Date        sdk.Date                   `nexa:"required,string=Effective Date,index"`
	SourceModel sdk.String                 `nexa:"string=Source Model"`
	SourceID    sdk.Integer                `nexa:"string=Source ID"`
	Note        sdk.Text                   `nexa:"string=Note"`
	CompanyID   sdk.Many2One[CoreCompany]  `nexa:"string=Company"`
}

// HrLeaveBalance is a computed, queryable projection of the ledger for one
// employee/leave-type. It stores no amounts: accrued, taken and balance are
// derived live from the posted transactions.
type HrLeaveBalance struct {
	sdk.Model `nexa:"model=hr.leave.balance"`

	EmployeeID  sdk.Many2One[HrEmployee]  `nexa:"required,string=Employee,index"`
	LeaveTypeID sdk.Many2One[HrLeaveType] `nexa:"required,string=Leave Type,index"`
	CompanyID   sdk.Many2One[CoreCompany] `nexa:"string=Company"`

	Accrued sdk.Float64 `nexa:"string=Accrued,compute=compute_leave_accrued"`
	Taken   sdk.Float64 `nexa:"string=Taken,compute=compute_leave_taken"`
	Balance sdk.Float64 `nexa:"string=Balance,compute=compute_leave_balance"`
}

// ---------------------------------------------------------------------------
// Accrual rules
// ---------------------------------------------------------------------------

// HrLeaveAccrualRule configures automatic accrual for a leave type: how many days
// accrue per period, how often, and the carry-over cap / expiry the job enforces
// when it opens a new period.
type HrLeaveAccrualRule struct {
	sdk.Model `nexa:"model=hr.leave.accrual.rule"`

	Name          sdk.String                      `nexa:"required,string=Rule"`
	LeaveTypeID   sdk.Many2One[HrLeaveType]       `nexa:"required,string=Leave Type,index"`
	DaysPerPeriod sdk.Float64                     `nexa:"required,string=Days Per Period"`
	Frequency     sdk.Selection[AccrualFrequency] `nexa:"string=Frequency,default=monthly,selection=monthly:Monthly,yearly:Yearly"`
	CarryOverMax  sdk.Float64                     `nexa:"string=Carry-Over Cap"`
	ExpiryDays    sdk.Integer                     `nexa:"string=Expiry (Days After Period),default=0"`
	Active        sdk.Boolean                     `nexa:"string=Active,default=true"`
	CompanyID     sdk.Many2One[CoreCompany]       `nexa:"string=Company"`
}

// ---------------------------------------------------------------------------
// Anchor extensions (non-destructive field additions via inherit)
// ---------------------------------------------------------------------------

// HrLeaveHalfDay adds half-day / hourly granularity to the anchor's leave request
// without editing hr. The engine converts hours to a day fraction when posting.
type HrLeaveHalfDay struct {
	sdk.Model `nexa:"inherit=hr.leave"`

	HalfDay sdk.Boolean `nexa:"string=Half Day,default=false"`
	Hours   sdk.Float64 `nexa:"string=Hours (0 = whole day)"`
}

func init() {
	sdk.MustRegister("hr_leave_ledger",
		&HrLeaveTransaction{},
		&HrLeaveBalance{},
		&HrLeaveAccrualRule{},
		&HrLeaveHalfDay{},
	)
}
