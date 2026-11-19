package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// Construction management on top of `project`: a construction project is a
// project.project; this addon adds the site-specific documents — BOQ
// estimating, RFIs, variation orders and daily site diaries.

type CoreUser = basemodels.CoreUser

// BoqHeader groups BOQ lines for one project/contract revision.
type BoqHeader struct {
	sdk.Model `nexa:"model=construction.boq"`

	Name        sdk.String              `nexa:"required,string=BOQ Reference"`
	ProjectID   sdk.Many2One[sdk.Any]   `nexa:"required,index,string=Project,comodel=project.project"`
	Revision    sdk.String              `nexa:"string=Revision"`
	State       sdk.Selection[BoqState] `nexa:"string=Status,default=draft,selection=draft:Draft,approved:Approved,archived:Archived"`
	Description sdk.Text                `nexa:"string=Description"`
	TotalAmount sdk.Numeric             `nexa:"string=Total,precision=18,scale=2,default=0"`
	Lines       sdk.One2Many[BoqLine]   `nexa:"string=Items"`
}

type BoqState string

const (
	BoqDraft    BoqState = "draft"
	BoqApproved BoqState = "approved"
	BoqArchived BoqState = "archived"
)

// BoqLine is one bill-of-quantities item: measured work × rate.
type BoqLine struct {
	sdk.Model `nexa:"model=construction.boq.line"`

	BoqID    sdk.Many2One[BoqHeader] `nexa:"required,index,ondelete=cascade,string=BOQ"`
	Code     sdk.String              `nexa:"string=Item Code"`
	Name     sdk.String              `nexa:"required,string=Description"`
	Uom      sdk.String              `nexa:"string=Unit"`
	Quantity sdk.Numeric             `nexa:"string=Quantity,precision=18,scale=3,default=0"`
	UnitRate sdk.Numeric             `nexa:"string=Unit Rate,precision=18,scale=2,default=0"`
	Amount   sdk.Numeric             `nexa:"string=Amount,precision=18,scale=2,default=0"`
	Section  sdk.String              `nexa:"string=Section"`
	Sequence sdk.Integer             `nexa:"string=Sequence,default=10"`
}

// ConstructionRfi is a request for information on site.
type ConstructionRfi struct {
	sdk.Model `nexa:"model=construction.rfi"`

	Name         sdk.String                 `nexa:"required,string=RFI Reference"`
	ProjectID    sdk.Many2One[sdk.Any]      `nexa:"required,index,string=Project,comodel=project.project"`
	Subject      sdk.String                 `nexa:"required,string=Subject"`
	Question     sdk.Text                   `nexa:"string=Question"`
	Answer       sdk.Text                   `nexa:"string=Answer"`
	State        sdk.Selection[RfiState]    `nexa:"string=Status,default=open,selection=open:Open,answered:Answered,closed:Closed,cancelled:Cancelled"`
	RaisedByID   sdk.Many2One[CoreUser]     `nexa:"string=Raised By"`
	AnsweredByID sdk.Many2One[CoreUser]     `nexa:"string=Answered By"`
	DateRaised   sdk.Date                   `nexa:"string=Raised Date"`
	DateAnswered sdk.Date                   `nexa:"string=Answered Date"`
	Priority     sdk.Selection[RfiPriority] `nexa:"string=Priority,default=normal,selection=low:Low,normal:Normal,high:High,urgent:Urgent"`
}

type RfiState string

const (
	RfiOpen      RfiState = "open"
	RfiAnswered  RfiState = "answered"
	RfiClosed    RfiState = "closed"
	RfiCancelled RfiState = "cancelled"
)

type RfiPriority string

const (
	RfiLow    RfiPriority = "low"
	RfiNormal RfiPriority = "normal"
	RfiHigh   RfiPriority = "high"
	RfiUrgent RfiPriority = "urgent"
)

// ConstructionVariation is a change/variation order against the contract.
type ConstructionVariation struct {
	sdk.Model `nexa:"model=construction.variation"`

	Name          sdk.String              `nexa:"required,string=Variation Reference"`
	ProjectID     sdk.Many2One[sdk.Any]   `nexa:"required,index,string=Project,comodel=project.project"`
	Subject       sdk.String              `nexa:"required,string=Subject"`
	Description   sdk.Text                `nexa:"string=Description"`
	Amount        sdk.Numeric             `nexa:"string=Amount,precision=18,scale=2,default=0"`
	State         sdk.Selection[VarState] `nexa:"string=Status,default=draft,selection=draft:Draft,submitted:Submitted,approved:Approved,rejected:Rejected,cancelled:Cancelled"`
	SubmittedBy   sdk.Many2One[CoreUser]  `nexa:"string=Submitted By"`
	ApprovedBy    sdk.Many2One[CoreUser]  `nexa:"string=Approved By"`
	DateSubmitted sdk.Date                `nexa:"string=Submitted Date"`
	DateDecided   sdk.Date                `nexa:"string=Decision Date"`
}

type VarState string

const (
	VarDraft     VarState = "draft"
	VarSubmitted VarState = "submitted"
	VarApproved  VarState = "approved"
	VarRejected  VarState = "rejected"
	VarCancelled VarState = "cancelled"
)

// ConstructionDiary is a daily site report.
type ConstructionDiary struct {
	sdk.Model `nexa:"model=construction.diary"`

	Name      sdk.String             `nexa:"required,string=Diary Entry"`
	ProjectID sdk.Many2One[sdk.Any]  `nexa:"required,index,string=Project,comodel=project.project"`
	Date      sdk.Date               `nexa:"required,string=Date"`
	Weather   sdk.String             `nexa:"string=Weather"`
	Manpower  sdk.Integer            `nexa:"string=Manpower"`
	WorkDone  sdk.Text               `nexa:"string=Work Done"`
	Issues    sdk.Text               `nexa:"string=Issues / Delays"`
	AuthorID  sdk.Many2One[CoreUser] `nexa:"string=Author"`
}

func init() {
	sdk.MustRegister("construction",
		&BoqHeader{},
		&BoqLine{},
		&ConstructionRfi{},
		&ConstructionVariation{},
		&ConstructionDiary{},
		&ConstructionSubcontract{},
		&ConstructionCertificate{},
	)
}

// ---------------------------------------------------------------------------
// Subcontracts + payment certificates (spec §7.3).
// ---------------------------------------------------------------------------

// ConstructionSubcontract is a subcontractor agreement on a project.
type ConstructionSubcontract struct {
	sdk.Model `nexa:"model=construction.subcontract"`

	Name          sdk.String                            `nexa:"required,string=Subcontract"`
	ProjectID     sdk.Many2One[sdk.Any]                 `nexa:"required,index,string=Project,comodel=project.project"`
	PartnerID     sdk.Many2One[sdk.Any]                 `nexa:"required,string=Subcontractor,comodel=core.partner"`
	Scope         sdk.Text                              `nexa:"string=Scope of Works"`
	ContractValue sdk.Numeric                           `nexa:"string=Contract Value,precision=18,scale=2,default=0"`
	RetentionPct  sdk.Numeric                           `nexa:"string=Retention %,precision=18,scale=2,default=0"`
	AdvanceAmount sdk.Numeric                           `nexa:"string=Advance Payment,precision=18,scale=2,default=0"`
	AdvancePct    sdk.Numeric                           `nexa:"string=Advance Recovery %,precision=18,scale=2,default=0"`
	State         sdk.Selection[SubState]               `nexa:"string=Status,default=draft,selection=draft:Draft,active:Active,suspended:Suspended,completed:Completed,terminated:Terminated"`
	StartDate     sdk.Date                              `nexa:"string=Start Date"`
	EndDate       sdk.Date                              `nexa:"string=End Date"`
	Certificates  sdk.One2Many[ConstructionCertificate] `nexa:"string=Payment Certificates"`
}

type SubState string

const (
	SubDraft      SubState = "draft"
	SubActive     SubState = "active"
	SubSuspended  SubState = "suspended"
	SubCompleted  SubState = "completed"
	SubTerminated SubState = "terminated"
)

// ConstructionCertificate is an interim payment certificate.
type ConstructionCertificate struct {
	sdk.Model `nexa:"model=construction.certificate"`

	Name             sdk.String                            `nexa:"required,string=Certificate"`
	SubcontractID    sdk.Many2One[ConstructionSubcontract] `nexa:"required,index,string=Subcontract"`
	Period           sdk.String                            `nexa:"string=Period (e.g. 2025-01)"`
	WorkDonePct      sdk.Numeric                           `nexa:"string=Work Done %,precision=18,scale=2,default=0"`
	AmountThisPeriod sdk.Numeric                           `nexa:"string=Amount This Period,precision=18,scale=2,default=0"`
	RetentionHeld    sdk.Numeric                           `nexa:"string=Retention Held,precision=18,scale=2,default=0"`
	AdvanceRecovery  sdk.Numeric                           `nexa:"string=Advance Recovery,precision=18,scale=2,default=0"`
	NetPayable       sdk.Numeric                           `nexa:"string=Net Payable,precision=18,scale=2,default=0"`
	CumulativePaid   sdk.Numeric                           `nexa:"string=Cumulative Paid,precision=18,scale=2,default=0"`
	State            sdk.Selection[CertState]              `nexa:"string=Status,default=draft,selection=draft:Draft,submitted:Submitted,certified:Certified,paid:Paid,rejected:Rejected"`
	CertifiedByID    sdk.Many2One[CoreUser]                `nexa:"string=Certified By"`
	PaidAt           sdk.DateTime                          `nexa:"string=Paid At"`
	Note             sdk.Text                              `nexa:"string=Note"`
}

type CertState string

const (
	CertDraft     CertState = "draft"
	CertSubmitted CertState = "submitted"
	CertCertified CertState = "certified"
	CertPaid      CertState = "paid"
	CertRejected  CertState = "rejected"
)
