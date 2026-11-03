package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// Budget control + approval engine (blueprint §4, §5, §6). These models keep
// procurement's own commitment ledger and configurable, versioned approval
// routing. Accounting still owns `account.budget` (read-only here) and the
// generic `approval.request` is used only when a single-level instance is
// enough; the matrix below covers amount/category routing, delegation,
// escalation and versioned evidence.

type (
	ReqCoreUser    = basemodels.CoreUser
	ReqCoreCompany = basemodels.CoreCompany
)

// ---------------------------------------------------------------------------
// Selection vocabularies
// ---------------------------------------------------------------------------

type CommitKind string

const (
	CommitReserve CommitKind = "reserve"
	CommitConsume CommitKind = "consume"
	CommitRelease CommitKind = "release"
)

type OverrideState string

const (
	OverrideProposed OverrideState = "proposed"
	OverrideApproved OverrideState = "approved"
	OverrideDeclined OverrideState = "declined"
)

type MatrixMode string

const (
	ModeSequential MatrixMode = "sequential"
	ModeParallel   MatrixMode = "parallel"
)

type InstanceState string

const (
	InstPending   InstanceState = "pending"
	InstApproved  InstanceState = "approved"
	InstRejected  InstanceState = "rejected"
	InstEscalated InstanceState = "escalated"
	InstCancelled InstanceState = "cancelled"
)

type StepState string

const (
	StepPending  StepState = "pending"
	StepApproved StepState = "approved"
	StepRejected StepState = "rejected"
	StepSkipped  StepState = "skipped"
)

type StepDecision string

const (
	DecNone     StepDecision = "none"
	DecApproved StepDecision = "approved"
	DecRejected StepDecision = "rejected"
)

// ---------------------------------------------------------------------------
// Commitment ledger (append-only)
// ---------------------------------------------------------------------------

// PurchaseBudgetCommitment records each budget movement. The open commitment
// against a budget is the sum of reserve rows minus consume and release rows;
// history is never overwritten. Invoice consumption (which turns a commitment
// into actual spend) is recorded here in T5 so nothing is double counted.
type PurchaseBudgetCommitment struct {
	sdk.Model `nexa:"model=purchase.budget.commitment"`

	Name          sdk.String                        `nexa:"required,index,size=60,string=Reference"`
	RequisitionID sdk.Many2One[PurchaseRequisition] `nexa:"index,string=Requisition"`
	BudgetID      sdk.Many2One[sdk.Any]             `nexa:"required,index,string=Budget,comodel=account.budget"`
	CompanyID     sdk.Many2One[ReqCoreCompany]      `nexa:"string=Company"`
	Kind          sdk.Selection[CommitKind]         `nexa:"required,index,string=Movement"`
	Amount        sdk.Numeric                       `nexa:"required,string=Amount,precision=18,scale=2,default=0"`
	OccuredOn     sdk.Date                          `nexa:"string=Date,default=today"`
	Reason        sdk.String                        `nexa:"size=300,string=Reason"`
	Note          sdk.Text                          `nexa:"string=Note"`
	Active        sdk.Boolean                       `nexa:"string=Active,default=true"`
}

// PurchaseBudgetOverride records an authorized decision to proceed despite a
// budget breach, capturing the governing policy version at the time.
type PurchaseBudgetOverride struct {
	sdk.Model `nexa:"model=purchase.budget.override"`

	Name          sdk.String                        `nexa:"required,index,size=60,string=Reference"`
	RequisitionID sdk.Many2One[PurchaseRequisition] `nexa:"required,index,string=Requisition"`
	BudgetID      sdk.Many2One[sdk.Any]             `nexa:"string=Budget,comodel=account.budget"`
	Policy        sdk.Selection[BudgetPolicy]       `nexa:"string=Budget Policy,default=conditional"`
	OverAmount    sdk.Numeric                       `nexa:"string=Amount Over Budget,precision=18,scale=2,default=0"`
	Reason        sdk.String                        `nexa:"required,size=300,string=Business Reason"`
	AuthorityID   sdk.Many2One[ReqCoreUser]         `nexa:"string=Authorized By"`
	State         sdk.Selection[OverrideState]      `nexa:"index,string=Status,default=proposed"`
	GrantedOn     sdk.DateTime                      `nexa:"string=Granted On"`
	Note          sdk.Text                          `nexa:"string=Note"`
	Active        sdk.Boolean                       `nexa:"string=Active,default=true"`
}

// ---------------------------------------------------------------------------
// Versioned approval matrix + runtime
// ---------------------------------------------------------------------------

// PurchaseApprovalMatrix is a versioned routing rule: for an amount band and a
// set of matching dimensions it selects an approver role/user at a level.
// Bands within a version must be mutually exclusive per dimension set.
type PurchaseApprovalMatrix struct {
	sdk.Model `nexa:"model=purchase.approval.matrix"`

	Name          sdk.String                   `nexa:"required,index,size=60,string=Reference"`
	Version       sdk.Integer                  `nexa:"index,string=Version,default=1"`
	Active        sdk.Boolean                  `nexa:"string=Active,default=true"`
	Priority      sdk.Integer                  `nexa:"string=Priority,default=10"`
	CompanyID     sdk.Many2One[ReqCoreCompany] `nexa:"string=Company"`
	ReqType       sdk.String                   `nexa:"size=40,string=Requisition Type"`
	Department    sdk.String                   `nexa:"size=120,string=Department"`
	Branch        sdk.String                   `nexa:"size=120,string=Entity / Branch"`
	Category      sdk.String                   `nexa:"size=120,string=Category"`
	EmergencyOnly sdk.Boolean                  `nexa:"string=Emergency Only,default=false"`
	CapexOnly     sdk.Boolean                  `nexa:"string=Capex Only,default=false"`
	SingleSource  sdk.Boolean                  `nexa:"string=Single Source Only,default=false"`
	AmountFrom    sdk.Numeric                  `nexa:"string=Amount From,precision=18,scale=2,default=0"`
	AmountTo      sdk.Numeric                  `nexa:"string=Amount To,precision=18,scale=2"`
	Level         sdk.Integer                  `nexa:"required,index,string=Level,default=1"`
	ApproverRole  sdk.String                   `nexa:"size=120,string=Approver Role"`
	ApproverID    sdk.Many2One[ReqCoreUser]    `nexa:"string=Approver User"`
	Mode          sdk.Selection[MatrixMode]    `nexa:"string=Mode,default=sequential"`
	MinApprovers  sdk.Integer                  `nexa:"string=Min Approvers,default=1"`
	DelegateStart sdk.Date                     `nexa:"string=Delegation Start"`
	DelegateEnd   sdk.Date                     `nexa:"string=Delegation End"`
	EscalateHours sdk.Integer                  `nexa:"string=Escalation (hours),default=0"`
	ExpiryDays    sdk.Integer                  `nexa:"string=Expiry (days),default=0"`
	Note          sdk.Text                     `nexa:"string=Note"`
}

// PurchaseApprovalInstance is a requisition's resolved approval run against a
// matrix version, with a snapshot of the reviewed content.
type PurchaseApprovalInstance struct {
	sdk.Model `nexa:"model=purchase.approval.instance"`

	Name            sdk.String                          `nexa:"required,index,size=60,string=Reference"`
	RequisitionID   sdk.Many2One[PurchaseRequisition]   `nexa:"required,index,string=Requisition"`
	MatrixVersion   sdk.Integer                         `nexa:"string=Matrix Version,default=1"`
	RequestedAmount sdk.Numeric                         `nexa:"string=Amount,precision=18,scale=2,default=0"`
	Currency        sdk.String                          `nexa:"size=3,string=Currency,default=USD"`
	State           sdk.Selection[InstanceState]        `nexa:"index,string=Status,default=pending"`
	RequesterID     sdk.Many2One[ReqCoreUser]           `nexa:"string=Requested By"`
	TotalLevels     sdk.Integer                         `nexa:"string=Total Levels,default=0"`
	Snapshot        sdk.Text                            `nexa:"string=Reviewed Version Snapshot"`
	StartedOn       sdk.DateTime                        `nexa:"string=Started On"`
	CompletedOn     sdk.DateTime                        `nexa:"string=Completed On"`
	Active          sdk.Boolean                         `nexa:"string=Active,default=true"`
	Steps           sdk.One2Many[PurchaseApprovalStep]  `nexa:"string=Steps"`
	Events          sdk.One2Many[PurchaseApprovalEvent] `nexa:"string=Events"`
}

// PurchaseApprovalStep is one level of an instance (sequential or parallel).
type PurchaseApprovalStep struct {
	sdk.Model `nexa:"model=purchase.approval.step"`

	InstanceID   sdk.Many2One[PurchaseApprovalInstance] `nexa:"required,index,string=Instance"`
	Level        sdk.Integer                            `nexa:"required,index,string=Level,default=1"`
	ApproverRole sdk.String                             `nexa:"size=120,string=Approver Role"`
	ApproverID   sdk.Many2One[ReqCoreUser]              `nexa:"string=Approver User"`
	Mode         sdk.Selection[MatrixMode]              `nexa:"string=Mode,default=sequential"`
	MinApprovers sdk.Integer                            `nexa:"string=Min Approvers,default=1"`
	State        sdk.Selection[StepState]               `nexa:"index,string=Status,default=pending"`
	Decision     sdk.Selection[StepDecision]            `nexa:"string=Decision,default=none"`
	DecidedBy    sdk.Many2One[ReqCoreUser]              `nexa:"string=Decided By"`
	Comment      sdk.Text                               `nexa:"string=Comment"`
	DecidedOn    sdk.DateTime                           `nexa:"string=Decided On"`
	Sequence     sdk.Integer                            `nexa:"string=Sequence,default=10"`
	Active       sdk.Boolean                            `nexa:"string=Active,default=true"`
}

// PurchaseApprovalEvent is append-only audit of each approver's reviewed version
// and every routing decision, preserving evidence across re-approvals.
type PurchaseApprovalEvent struct {
	sdk.Model `nexa:"model=purchase.approval.event"`

	InstanceID      sdk.Many2One[PurchaseApprovalInstance] `nexa:"required,index,string=Instance"`
	StepID          sdk.Many2One[PurchaseApprovalStep]     `nexa:"string=Step"`
	Timestamp       sdk.DateTime                           `nexa:"string=At"`
	ActorID         sdk.Many2One[ReqCoreUser]              `nexa:"string=Actor"`
	Action          sdk.String                             `nexa:"required,size=40,string=Action"`
	VersionSnapshot sdk.Text                               `nexa:"string=Reviewed Version Snapshot"`
	Detail          sdk.String                             `nexa:"size=300,string=Detail"`
}

func init() {
	sdk.MustRegister("purchase_requisition",
		&PurchaseBudgetCommitment{},
		&PurchaseBudgetOverride{},
		&PurchaseApprovalMatrix{},
		&PurchaseApprovalInstance{},
		&PurchaseApprovalStep{},
		&PurchaseApprovalEvent{},
	)
}
