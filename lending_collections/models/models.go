package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	lendingmodels "nexa_addons/lending/models"
)

type (
	CorePartner = basemodels.CorePartner
	CoreUser    = basemodels.CoreUser
	LoanAccount = lendingmodels.LoanAccount
)

// ============================================================================
// Delinquency Snapshot
// ============================================================================

type DelinquencyBucket string

const (
	DelinquencyBucketCurrent   DelinquencyBucket = "current"
	DelinquencyBucketDpd1_30   DelinquencyBucket = "dpd_1_30"
	DelinquencyBucketDpd31_60  DelinquencyBucket = "dpd_31_60"
	DelinquencyBucketDpd61_90  DelinquencyBucket = "dpd_61_90"
	DelinquencyBucketDpd90Plus DelinquencyBucket = "dpd_90_plus"
)

type LoanDelinquencySnapshot struct {
	sdk.Model

	AccountID            sdk.Many2One[LoanAccount]        `nexa:"string=Loan Account,required,index"`
	Date                 sdk.Date                         `nexa:"string=Date"`
	DaysPastDue          sdk.Integer                      `nexa:"string=Days Past Due"`
	OutstandingPrincipal sdk.Numeric                      `nexa:"string=Outstanding Principal,precision=18,scale=2"`
	OverduePrincipal     sdk.Numeric                      `nexa:"string=Overdue Principal,precision=18,scale=2"`
	OverdueInterest      sdk.Numeric                      `nexa:"string=Overdue Interest,precision=18,scale=2"`
	Bucket               sdk.Selection[DelinquencyBucket] `nexa:"string=Bucket,default=current"`
	Note                 sdk.Text                         `nexa:"string=Note"`

	// Non-stored total of the overdue components (recomputed on read).
	OverdueTotal sdk.Numeric `nexa:"string=Overdue Total,precision=18,scale=2,compute=compute_snapshot_overdue_total"`
}

// ============================================================================
// Collection Case
// ============================================================================

type CollectionPriority string

const (
	CollectionPriorityLow    CollectionPriority = "low"
	CollectionPriorityMedium CollectionPriority = "medium"
	CollectionPriorityHigh   CollectionPriority = "high"
)

type CollectionState string

const (
	CollectionStateOpen      CollectionState = "open"
	CollectionStateContacted CollectionState = "contacted"
	CollectionStatePromise   CollectionState = "promise"
	CollectionStateArranged  CollectionState = "arranged"
	CollectionStateResolved  CollectionState = "resolved"
	CollectionStateEscalated CollectionState = "escalated"
	CollectionStateClosed    CollectionState = "closed"
)

type LoanCollectionCase struct {
	sdk.Model

	Name         sdk.String                        `nexa:"string=Case,required"`
	AccountID    sdk.Many2One[LoanAccount]         `nexa:"string=Loan Account,required,index"`
	BorrowerID   sdk.Many2One[CorePartner]         `nexa:"string=Borrower,index"`
	CollectorID  sdk.Many2One[CoreUser]            `nexa:"string=Collector"`
	Priority     sdk.Selection[CollectionPriority] `nexa:"string=Priority,default=medium"`
	State        sdk.Selection[CollectionState]    `nexa:"string=Status,default=open"`
	OpenedDate   sdk.Date                          `nexa:"string=Opened Date"`
	CloseDate    sdk.Date                          `nexa:"string=Closed Date"`
	TotalOverdue sdk.Numeric                       `nexa:"string=Total Overdue,precision=18,scale=2"`
	HardshipFlag sdk.Boolean                       `nexa:"string=Hardship"`
	DisputeFlag  sdk.Boolean                       `nexa:"string=Dispute"`
	Note         sdk.Text                          `nexa:"string=Note"`

	Actions sdk.One2Many[LoanCollectionAction] `nexa:"string=Actions"`

	// Non-stored aggregates over the linked activity lines (recomputed on read).
	ActionCount  sdk.Integer `nexa:"string=Actions,compute=compute_case_action_count"`
	PromiseCount sdk.Integer `nexa:"string=Promises,compute=compute_case_promise_count"`
}

// ============================================================================
// Collection Action
// ============================================================================

type CollectionActionType string

const (
	CollectionActionTypeCall   CollectionActionType = "call"
	CollectionActionTypeSms    CollectionActionType = "sms"
	CollectionActionTypeEmail  CollectionActionType = "email"
	CollectionActionTypeVisit  CollectionActionType = "visit"
	CollectionActionTypeLetter CollectionActionType = "letter"
)

type LoanCollectionAction struct {
	sdk.Model

	CaseID      sdk.Many2One[LoanCollectionCase]    `nexa:"string=Case,required,index"`
	Date        sdk.DateTime                        `nexa:"string=Date"`
	ActionType  sdk.Selection[CollectionActionType] `nexa:"string=Action,default=call"`
	Outcome     sdk.String                          `nexa:"string=Outcome"`
	PerformedBy sdk.Many2One[CoreUser]              `nexa:"string=Performed By"`
	Notes       sdk.Text                            `nexa:"string=Notes"`
}

// ============================================================================
// Promise To Pay
// ============================================================================

type PromiseStatus string

const (
	PromiseStatusPending PromiseStatus = "pending"
	PromiseStatusKept    PromiseStatus = "kept"
	PromiseStatusBroken  PromiseStatus = "broken"
)

type LoanPromiseToPay struct {
	sdk.Model

	CaseID         sdk.Many2One[LoanCollectionCase] `nexa:"string=Case,required,index"`
	AccountID      sdk.Many2One[LoanAccount]        `nexa:"string=Loan Account"`
	PromisedDate   sdk.Date                         `nexa:"string=Promised Date"`
	PromisedAmount sdk.Numeric                      `nexa:"string=Promised Amount,precision=18,scale=2"`
	Status         sdk.Selection[PromiseStatus]     `nexa:"string=Status,default=pending"`
	Note           sdk.Text                         `nexa:"string=Note"`
}

func init() {
	sdk.MustRegister("lending_collections",
		&LoanDelinquencySnapshot{},
		&LoanCollectionCase{},
		&LoanCollectionAction{},
		&LoanPromiseToPay{},
	)
}
