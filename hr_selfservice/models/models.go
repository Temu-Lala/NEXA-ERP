package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	hrmodels "nexa_addons/hr/models"
)

// HR-F9 "Employee Self-Service & HR Service Desk". The HR modules built so far
// give an officer a back-office for every domain, but an ordinary employee has no
// single place to raise an HR request, ask for a letter / certificate, track that
// case, or route an approval to their manager - and a manager has no consolidated
// approval inbox. This companion adds exactly that on top of the existing anchor,
// reusing hr.employee / hr.department (never recreated) and following the proven
// helpdesk case pattern (queue + category + assignment + SLA window + message
// thread + append-only history) as a self-contained HR service desk. Ownership is
// isolated at the data layer with read-only record rules on a dedicated
// self-service group, so an employee sees only their own cases / requests while an
// HR Officer / Manager sees everything. Approving a self-service request only
// records a decision; it never posts a cost or mutates payroll / leave directly.

// Cross-module reference aliases.
type (
	CoreUser     = basemodels.CoreUser
	HrEmployee   = hrmodels.HrEmployee
	HrDepartment = hrmodels.HrDepartment
)

// ---------------------------------------------------------------------------
// Enumerations
// ---------------------------------------------------------------------------

type CaseState string

const (
	CaseNew        CaseState = "new"
	CaseAssigned   CaseState = "assigned"
	CaseInProgress CaseState = "in_progress"
	CasePending    CaseState = "pending"
	CaseResolved   CaseState = "resolved"
	CaseClosed     CaseState = "closed"
	CaseCancelled  CaseState = "cancelled"
)

type CasePriority string

const (
	PriorityLow    CasePriority = "low"
	PriorityNormal CasePriority = "normal"
	PriorityHigh   CasePriority = "high"
	PriorityUrgent CasePriority = "urgent"
)

type CaseCategoryKind string

const (
	KindLetter       CaseCategoryKind = "letter"
	KindCertificate  CaseCategoryKind = "certificate"
	KindPersonalInfo CaseCategoryKind = "personal_info"
	KindGeneral      CaseCategoryKind = "general"
	KindOther        CaseCategoryKind = "other"
)

type CaseHistoryAction string

const (
	HistCreated    CaseHistoryAction = "created"
	HistState      CaseHistoryAction = "state"
	HistAssignment CaseHistoryAction = "assignment"
	HistPriority   CaseHistoryAction = "priority"
	HistCategory   CaseHistoryAction = "category"
	HistResolve    CaseHistoryAction = "resolve"
	HistMessage    CaseHistoryAction = "message"
	HistNote       CaseHistoryAction = "note"
	HistSla        CaseHistoryAction = "sla"
)

type SlaState string

const (
	SlaOnTrack  SlaState = "on_track"
	SlaWarning  SlaState = "warning"
	SlaBreached SlaState = "breached"
	SlaMet      SlaState = "met"
)

type RequestState string

const (
	ReqDraft     RequestState = "draft"
	ReqSubmitted RequestState = "submitted"
	ReqApproved  RequestState = "approved"
	ReqRejected  RequestState = "rejected"
	ReqCancelled RequestState = "cancelled"
)

type RequestCategory string

const (
	ReqCatLeave      RequestCategory = "leave"
	ReqCatCorrection RequestCategory = "attendance_correction"
	ReqCatOvertime   RequestCategory = "overtime"
	ReqCatPersonal   RequestCategory = "personal_info"
	ReqCatLetter     RequestCategory = "letter"
	ReqCatOther      RequestCategory = "other"
)

// ---------------------------------------------------------------------------
// Service desk configuration
// ---------------------------------------------------------------------------

// HrServiceTeam is an HR service-desk queue with a set of officer members and an
// optional department scope.
type HrServiceTeam struct {
	sdk.Model `nexa:"model=hr.service.team"`

	Name         sdk.String                 `nexa:"required,string=Team"`
	DepartmentID sdk.Many2One[HrDepartment] `nexa:"string=Department Scope"`
	MemberIDs    sdk.Many2Many[CoreUser]    `nexa:"string=Members,table=hr_service_team_member_rel,left=team_id,right=user_id"`
	Active       sdk.Boolean                `nexa:"string=Active,default=true"`
}

// HrServiceCategory classifies the kind of HR request and carries the SLA windows
// (in hours) plus the queue that handles it.
type HrServiceCategory struct {
	sdk.Model `nexa:"model=hr.service.category"`

	Name             sdk.String                      `nexa:"required,string=Category"`
	Kind             sdk.Selection[CaseCategoryKind] `nexa:"string=Kind,default=general,selection=letter:Letter,certificate:Certificate,personal_info:Personal Info,general:General,other:Other"`
	ResponseHours    sdk.Integer                     `nexa:"string=Response (hours),default=24"`
	ResolutionHours  sdk.Integer                     `nexa:"string=Resolution (hours),default=120"`
	DefaultTeamID    sdk.Many2One[HrServiceTeam]     `nexa:"string=Default Queue"`
	RequiresApproval sdk.Boolean                     `nexa:"string=Requires Approval,default=false"`
	Active           sdk.Boolean                     `nexa:"string=Active,default=true"`
}

// ---------------------------------------------------------------------------
// Service desk case
// ---------------------------------------------------------------------------

// HrServiceCase is one categorized HR case raised by (or on behalf of) an
// employee. The SLA deadlines are snapshotted at assignment so later category
// edits never silently rewrite a running clock, and the append-only history
// records every state / assignment / priority change.
type HrServiceCase struct {
	sdk.Model `nexa:"model=hr.service.case"`

	Name           sdk.String                         `nexa:"required,string=Subject"`
	Number         sdk.String                         `nexa:"string=Case Number"`
	EmployeeID     sdk.Many2One[HrEmployee]           `nexa:"required,string=Employee,index"`
	EmployeeUserID sdk.Integer                        `nexa:"string=Employee User,compute=compute_case_employee_user,store"`
	DepartmentID   sdk.Many2One[HrDepartment]         `nexa:"string=Department,index"`
	CategoryID     sdk.Many2One[HrServiceCategory]    `nexa:"string=Category,index"`
	TeamID         sdk.Many2One[HrServiceTeam]        `nexa:"string=Queue"`
	AssignedTo     sdk.Many2One[CoreUser]             `nexa:"string=Assigned To"`
	Priority       sdk.Selection[CasePriority]        `nexa:"string=Priority,default=normal,selection=low:Low,normal:Normal,high:High,urgent:Urgent"`
	State          sdk.Selection[CaseState]           `nexa:"string=Status,default=new,index,selection=new:New,assigned:Assigned,in_progress:In Progress,pending:Pending,resolved:Resolved,closed:Closed,cancelled:Cancelled"`
	Description    sdk.Text                           `nexa:"string=Description"`
	Resolution     sdk.Text                           `nexa:"string=Resolution"`
	ResponseDue    sdk.DateTime                       `nexa:"string=Response Due"`
	ResolutionDue  sdk.DateTime                       `nexa:"string=Resolution Due"`
	Breached       sdk.Boolean                        `nexa:"string=SLA Breached,default=false"`
	SlaState       sdk.Selection[SlaState]            `nexa:"string=SLA,compute=compute_case_sla_state,selection=on_track:On Track,warning:Warning,breached:Breached,met:Met"`
	ClosedDate     sdk.DateTime                       `nexa:"string=Closed Date"`
	Messages       sdk.One2Many[HrServiceCaseMessage] `nexa:"string=Messages,inverse=case_id"`
	History        sdk.One2Many[HrServiceCaseHistory] `nexa:"string=History,inverse=case_id"`
}

// HrServiceCaseMessage is a note / reply on a case; internal notes are hidden from
// the requesting employee.
type HrServiceCaseMessage struct {
	sdk.Model `nexa:"model=hr.service.case.message"`

	CaseID     sdk.Many2One[HrServiceCase] `nexa:"required,string=Case,index"`
	AuthorID   sdk.Many2One[CoreUser]      `nexa:"string=Author"`
	Body       sdk.Text                    `nexa:"required,string=Message"`
	IsInternal sdk.Boolean                 `nexa:"string=Internal Note,default=false"`
	Date       sdk.DateTime                `nexa:"string=Date"`
}

// HrServiceCaseHistory is the append-only audit of every meaningful case change.
type HrServiceCaseHistory struct {
	sdk.Model `nexa:"model=hr.service.case.history"`

	CaseID      sdk.Many2One[HrServiceCase]      `nexa:"required,string=Case,index"`
	Action      sdk.Selection[CaseHistoryAction] `nexa:"string=Action,default=note,selection=created:Created,state:State,assignment:Assignment,priority:Priority,category:Category,resolve:Resolved,message:Message,note:Note,sla:SLA"`
	Field       sdk.String                       `nexa:"size=60,string=Field"`
	OldValue    sdk.String                       `nexa:"size=200,string=From"`
	NewValue    sdk.String                       `nexa:"size=200,string=To"`
	PerformedBy sdk.Many2One[CoreUser]           `nexa:"string=By"`
	PerformedAt sdk.DateTime                     `nexa:"string=Date"`
}

// ---------------------------------------------------------------------------
// Employee self-service requests (approval inbox)
// ---------------------------------------------------------------------------

// HrSelfServiceRequest is the unified surface an employee uses to raise a
// leave / attendance-correction / overtime / personal-info / letter request and
// the inbox a manager approves. The approver is derived from the employee's own
// manager on submit; approving only records a decision.
type HrSelfServiceRequest struct {
	sdk.Model `nexa:"model=hr.selfservice.request"`

	Name           sdk.String                     `nexa:"required,string=Summary"`
	Category       sdk.Selection[RequestCategory] `nexa:"string=Type,default=personal_info,index,selection=leave:Leave,attendance_correction:Attendance Correction,overtime:Overtime,personal_info:Personal Info,letter:Letter / Certificate,other:Other"`
	EmployeeID     sdk.Many2One[HrEmployee]       `nexa:"required,string=Employee,index"`
	EmployeeUserID sdk.Integer                    `nexa:"string=Employee User,compute=compute_request_employee_user,store"`
	DepartmentID   sdk.Many2One[HrDepartment]     `nexa:"string=Department,index"`
	ApproverID     sdk.Many2One[HrEmployee]       `nexa:"string=Approver"`
	RequestedBy    sdk.Integer                    `nexa:"string=Requested By"`
	Details        sdk.Text                       `nexa:"string=Details"`
	EffectiveDate  sdk.Date                       `nexa:"string=Effective Date"`
	Amount         sdk.Numeric                    `nexa:"string=Amount,precision=12,scale=2,default=0"`
	RefModel       sdk.String                     `nexa:"size=60,string=Linked Model"`
	RefID          sdk.Integer                    `nexa:"string=Linked Record"`
	State          sdk.Selection[RequestState]    `nexa:"string=Status,default=draft,index,selection=draft:Draft,submitted:Submitted,approved:Approved,rejected:Rejected,cancelled:Cancelled"`
	DecideDate     sdk.Date                       `nexa:"string=Decision Date"`
	DecisionNote   sdk.Text                       `nexa:"string=Decision Note"`
}

func init() {
	sdk.MustRegister("hr_selfservice",
		&HrServiceTeam{},
		&HrServiceCategory{},
		&HrServiceCase{},
		&HrServiceCaseMessage{},
		&HrServiceCaseHistory{},
		&HrSelfServiceRequest{},
	)
}
