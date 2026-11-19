package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// ITSM: incidents, problems, changes, releases — the ITIL layer above
// helpdesk tickets. Every record can link back to the originating ticket.

type CoreUser = basemodels.CoreUser

// ItsIncidents are service-disruption records; major incidents get a commander.
type ItsIncidents struct {
	sdk.Model `nexa:"model=itsm.incident"`

	Name        sdk.String                      `nexa:"required,string=Incident Reference"`
	TicketID    sdk.Many2One[sdk.Any]           `nexa:"index,string=Ticket,comodel=helpdesk.ticket"`
	Title       sdk.String                      `nexa:"required,string=Title"`
	Description sdk.Text                        `nexa:"string=Description"`
	Service     sdk.String                      `nexa:"string=Impacted Service"`
	Priority    sdk.Selection[IncPri]           `nexa:"string=Priority,default=medium,selection=low:Low,medium:Medium,high:High,critical:Critical"`
	Major       sdk.Boolean                     `nexa:"string=Major Incident,default=false"`
	CommanderID sdk.Many2One[CoreUser]          `nexa:"string=Incident Commander"`
	State       sdk.Selection[IncState]         `nexa:"string=Status,default=logged,selection=logged:Logged,triaged:Triaged,in_progress:In Progress,monitoring:Monitoring,resolved:Resolved,closed:Closed"`
	Workaround  sdk.Text                        `nexa:"string=Workaround"`
	RootCause   sdk.Text                        `nexa:"string=Root Cause"`
	DetectedAt  sdk.DateTime                    `nexa:"string=Detected At"`
	ResolvedAt  sdk.DateTime                    `nexa:"string=Resolved At"`
	ProblemID   sdk.Many2One[ItsProblems]       `nexa:"string=Linked Problem"`
	Timeline    sdk.One2Many[ItsmTimelineEntry] `nexa:"string=Timeline"`
}

type IncPri string

const (
	IncLow      IncPri = "low"
	IncMedium   IncPri = "medium"
	IncHigh     IncPri = "high"
	IncCritical IncPri = "critical"
)

type IncState string

const (
	IncLogged     IncState = "logged"
	IncTriaged    IncState = "triaged"
	IncInProgress IncState = "in_progress"
	IncMonitoring IncState = "monitoring"
	IncResolved   IncState = "resolved"
	IncClosed     IncState = "closed"
)

// ItsmTimelineEntry is an append-only incident timeline event.
type ItsmTimelineEntry struct {
	sdk.Model `nexa:"model=itsm.timeline"`

	IncidentID sdk.Many2One[ItsIncidents] `nexa:"required,index,ondelete=cascade,string=Incident"`
	At         sdk.DateTime               `nexa:"string=Timestamp"`
	Kind       sdk.String                 `nexa:"string=Event Type"`
	Note       sdk.Text                   `nexa:"string=Note"`
	UserID     sdk.Many2One[CoreUser]     `nexa:"string=Author"`
}

// ItsProblems are root-cause records linked to recurring incidents.
type ItsProblems struct {
	sdk.Model `nexa:"model=itsm.problem"`

	Name        sdk.String               `nexa:"required,string=Problem Reference"`
	Title       sdk.String               `nexa:"required,string=Title"`
	Description sdk.Text                 `nexa:"string=Description"`
	RootCause   sdk.Text                 `nexa:"string=Root Cause Analysis"`
	Workaround  sdk.Text                 `nexa:"string=Workaround"`
	Fix         sdk.Text                 `nexa:"string=Permanent Fix"`
	KnownError  sdk.Boolean              `nexa:"string=Known Error,default=false"`
	Priority    sdk.Selection[IncPri]    `nexa:"string=Priority,default=medium,selection=low:Low,medium:Medium,high:High,critical:Critical"`
	State       sdk.Selection[ProbState] `nexa:"string=Status,default=open,selection=open:Open,investigating:Investigating,known_error:Known Error,resolved:Resolved,closed:Closed"`
	OwnerID     sdk.Many2One[CoreUser]   `nexa:"string=Owner"`
}

type ProbState string

const (
	ProbOpen          ProbState = "open"
	ProbInvestigating ProbState = "investigating"
	ProbKnownError    ProbState = "known_error"
	ProbResolved      ProbState = "resolved"
	ProbClosed        ProbState = "closed"
)

// ItsChanges are change requests with risk assessment and approval.
type ItsChanges struct {
	sdk.Model `nexa:"model=itsm.change"`

	Name          sdk.String              `nexa:"required,string=Change Reference"`
	TicketID      sdk.Many2One[sdk.Any]   `nexa:"index,string=Ticket,comodel=helpdesk.ticket"`
	Title         sdk.String              `nexa:"required,string=Title"`
	Description   sdk.Text                `nexa:"string=Description"`
	Kind          sdk.Selection[ChgKind]  `nexa:"string=Type,default=normal,selection=standard:Standard,normal:Normal,emergency:Emergency"`
	Risk          sdk.Selection[ChgRisk]  `nexa:"string=Risk,default=low,selection=low:Low,medium:Medium,high:High"`
	Impact        sdk.String              `nexa:"string=Impact Assessment"`
	Plan          sdk.Text                `nexa:"string=Implementation Plan"`
	TestPlan      sdk.Text                `nexa:"string=Test Plan"`
	RollbackPlan  sdk.Text                `nexa:"string=Rollback Plan"`
	WindowStart   sdk.DateTime            `nexa:"string=Maintenance Window Start"`
	WindowEnd     sdk.DateTime            `nexa:"string=Maintenance Window End"`
	State         sdk.Selection[ChgState] `nexa:"string=Status,default=draft,selection=draft:Draft,assessment:Assessment,approval:Approval,scheduled:Scheduled,implementing:Implementing,review:Review,closed:Closed,cancelled:Cancelled"`
	RequestedByID sdk.Many2One[CoreUser]  `nexa:"string=Requested By"`
	ApprovedByID  sdk.Many2One[CoreUser]  `nexa:"string=Approved By"`
}

type ChgKind string

const (
	ChgStandard  ChgKind = "standard"
	ChgNormal    ChgKind = "normal"
	ChgEmergency ChgKind = "emergency"
)

type ChgRisk string

const (
	ChgRiskLow    ChgRisk = "low"
	ChgRiskMedium ChgRisk = "medium"
	ChgRiskHigh   ChgRisk = "high"
)

type ChgState string

const (
	ChgDraft        ChgState = "draft"
	ChgAssessment   ChgState = "assessment"
	ChgApproval     ChgState = "approval"
	ChgScheduled    ChgState = "scheduled"
	ChgImplementing ChgState = "implementing"
	ChgReview       ChgState = "review"
	ChgClosed       ChgState = "closed"
	ChgCancelled    ChgState = "cancelled"
)

// ItsReleases are release plans with staged deployment.
type ItsReleases struct {
	sdk.Model `nexa:"model=itsm.release"`

	Name         sdk.String                       `nexa:"required,string=Release"`
	Version      sdk.String                       `nexa:"required,string=Version"`
	Description  sdk.Text                         `nexa:"string=Description"`
	ReleaseNotes sdk.Text                         `nexa:"string=Release Notes"`
	Stage        sdk.Selection[RelStage]          `nexa:"string=Stage,default=planning,selection=planning:Planning,build:Build,test:Test,approval:Approval,deploying:Deploying,done:Done,cancelled:Cancelled"`
	PlannedDate  sdk.Date                         `nexa:"string=Planned Date"`
	DeployedAt   sdk.DateTime                     `nexa:"string=Deployed At"`
	RollbackPlan sdk.Text                         `nexa:"string=Rollback Plan"`
	TestEvidence sdk.Text                         `nexa:"string=Test Evidence"`
	OwnerID      sdk.Many2One[CoreUser]           `nexa:"string=Release Manager"`
	Packages     sdk.One2Many[ItsmReleasePackage] `nexa:"string=Packages"`
}

type RelStage string

const (
	RelPlanning  RelStage = "planning"
	RelBuild     RelStage = "build"
	RelTest      RelStage = "test"
	RelApproval  RelStage = "approval"
	RelDeploying RelStage = "deploying"
	RelDone      RelStage = "done"
	RelCancelled RelStage = "cancelled"
)

// ItsmReleasePackage is one deployable unit inside a release.
type ItsmReleasePackage struct {
	sdk.Model `nexa:"model=itsm.release.package"`

	ReleaseID sdk.Many2One[ItsReleases] `nexa:"required,index,ondelete=cascade,string=Release"`
	Name      sdk.String                `nexa:"required,string=Package"`
	Version   sdk.String                `nexa:"string=Version"`
	DependsOn sdk.String                `nexa:"string=Depends On"`
	Deployed  sdk.Boolean               `nexa:"string=Deployed,default=false"`
	Sequence  sdk.Integer               `nexa:"string=Deploy Order,default=10"`
}

func init() {
	sdk.MustRegister("itsm",
		&ItsIncidents{},
		&ItsmTimelineEntry{},
		&ItsProblems{},
		&ItsChanges{},
		&ItsReleases{},
		&ItsmReleasePackage{},
	)
}
