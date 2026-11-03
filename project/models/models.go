package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// CoreUser → core.user
type CoreUser = basemodels.CoreUser

type ProjectState string

const (
	ProjectStateDraft     ProjectState = "draft"
	ProjectStatePlanned   ProjectState = "planned"
	ProjectStateApproved  ProjectState = "approved"
	ProjectStateActive    ProjectState = "active"
	ProjectStateOnHold    ProjectState = "onhold"
	ProjectStateCompleted ProjectState = "completed"
	ProjectStateCancelled ProjectState = "cancelled"
	ProjectStateArchived  ProjectState = "archived"
)

type ProjectHealth string

const (
	HealthOnTrack     ProjectHealth = "ontrack"
	HealthAtRisk      ProjectHealth = "atrisk"
	HealthOffTrack    ProjectHealth = "offtrack"
	HealthNotAssessed ProjectHealth = "not_assessed"
)

type ProjectPriority string

const (
	PriorityLow    ProjectPriority = "low"
	PriorityNormal ProjectPriority = "normal"
	PriorityHigh   ProjectPriority = "high"
	PriorityUrgent ProjectPriority = "urgent"
)

type TaskState string

const (
	TaskStateBacklog    TaskState = "backlog"
	TaskStateReady      TaskState = "ready"
	TaskStateTodo       TaskState = "todo"
	TaskStateInProgress TaskState = "in_progress"
	TaskStateReview     TaskState = "in_review"
	TaskStateBlocked    TaskState = "blocked"
	TaskStateDone       TaskState = "done"
	TaskStateCancelled  TaskState = "cancelled"
)

type KanbanState string

const (
	KanbanStandard KanbanState = "standard"
	KanbanReady    KanbanState = "ready"
	KanbanBlocked  KanbanState = "blocked"
)

type MilestoneState string

const (
	MilestonePending MilestoneState = "pending"
	MilestoneReached MilestoneState = "reached"
	MilestoneDone    MilestoneState = "done"
)

// EntityKind selects which record a status-history row refers to (project or
// task). Declared in the core project module so lifecycle actions can write the
// audit timeline without importing companion modules.
type EntityKind string

const (
	EntityProject EntityKind = "project"
	EntityTask    EntityKind = "task"
)

// ProjectTag is a configurable label applied to issues for categorisation and
// quick filtering (Jira-style labels). Color drives the chip hue in the UI.
type ProjectTag struct {
	sdk.Model `nexa:"model=project.tag"`

	Name   sdk.String  `nexa:"required,size=60,index,string=Label"`
	Color  sdk.Integer `nexa:"string=Color Index,default=0"`
	Active sdk.Boolean `nexa:"string=Active,default=true"`
}

// ProjectType is a configurable project category driving behavior rules.
type ProjectType struct {
	sdk.Model `nexa:"model=project.type"`

	Name            sdk.String  `nexa:"required,index,string=Type"`
	AllowTimesheets sdk.Boolean `nexa:"string=Allow Timesheets,default=true"`
	RequireApproval sdk.Boolean `nexa:"string=Require Approval,default=false"`
	Active          sdk.Boolean `nexa:"string=Active,default=true"`
	Note            sdk.Text    `nexa:"string=Description"`
}

// ProjectPhase is a WBS phase grouping tasks and milestones within a project.
type ProjectPhase struct {
	sdk.Model `nexa:"model=project.phase"`

	Name         sdk.String                   `nexa:"required,string=Phase"`
	ProjectID    sdk.Many2One[ProjectProject] `nexa:"required,index,string=Project"`
	State        sdk.Selection[ProjectState]  `nexa:"string=Status,default=planned,selection=draft:Draft,planned:Planned,approved:Approved,active:Active,onhold:On Hold,completed:Completed,cancelled:Cancelled,archived:Archived"`
	PlannedStart sdk.Date                     `nexa:"string=Planned Start"`
	PlannedEnd   sdk.Date                     `nexa:"string=Planned End"`
	Sequence     sdk.Integer                  `nexa:"string=Sequence,default=10"`
	Color        sdk.Integer                  `nexa:"string=Color,default=0"`
}

// ProjectMilestone is a major delivery point on a project timeline.
type ProjectMilestone struct {
	sdk.Model `nexa:"model=project.milestone"`

	Name         sdk.String                    `nexa:"required,string=Milestone"`
	ProjectID    sdk.Many2One[ProjectProject]  `nexa:"required,index,string=Project"`
	PhaseID      sdk.Many2One[ProjectPhase]    `nexa:"string=Phase"`
	OwnerID      sdk.Many2One[CoreUser]        `nexa:"string=Owner"`
	DueDate      sdk.Date                      `nexa:"string=Target Date"`
	AchievedDate sdk.Date                      `nexa:"string=Achieved Date"`
	State        sdk.Selection[MilestoneState] `nexa:"string=Status,default=pending"`
	IsBillable   sdk.Boolean                   `nexa:"string=Billable Milestone,default=false"`
	Sequence     sdk.Integer                   `nexa:"string=Sequence,default=10"`
	Description  sdk.Text                      `nexa:"string=Description"`
}

// ProjectMember is a person assigned to a project with a role and allocation.
type ProjectMember struct {
	sdk.Model `nexa:"model=project.member"`

	ProjectID     sdk.Many2One[ProjectProject] `nexa:"required,index,string=Project"`
	UserID        sdk.Many2One[CoreUser]       `nexa:"required,string=User"`
	Role          sdk.String                   `nexa:"string=Role"`
	AllocationPct sdk.Numeric                  `nexa:"string=Allocation %,precision=18,scale=2,default=0"`
	Active        sdk.Boolean                  `nexa:"string=Active,default=true"`
}

// ProjectTaskComment is a threaded discussion entry on a task. The mail addon
// treats it as chatter on the host task: the commenter joins the thread, everyone
// already watching is told, and an @name in the body reaches and subscribes that
// person even if they were not watching.
type ProjectTaskComment struct {
	sdk.Model `nexa:"model=project.task.comment,mail_comment=task_id"`

	TaskID     sdk.Many2One[ProjectTask] `nexa:"required,index,string=Task"`
	AuthorID   sdk.Many2One[CoreUser]    `nexa:"string=Author"`
	Body       sdk.Text                  `nexa:"required,string=Comment"`
	IsInternal sdk.Boolean               `nexa:"string=Internal Note,default=false"`
}

type ProjectProject struct {
	// Chatter opt-in: the manager and the sponsor are subscribed when the project
	// names them, and a change of state or priority is announced to whoever is
	// watching. Health and progress stay out of the thread — the daily recompute
	// writes them whether or not anything meaningful happened.
	sdk.Model `nexa:"model=project.project,mail_thread=manager_id;sponsor_id,mail_notify=state;priority"`

	Name          sdk.String                     `nexa:"required,string=Project Name"`
	Image         sdk.Image                      `nexa:"string=Logo"`
	Code          sdk.String                     `nexa:"index,string=Reference"`
	TypeID        sdk.Many2One[ProjectType]      `nexa:"string=Project Type"`
	ManagerID     sdk.Many2One[CoreUser]         `nexa:"string=Project Manager"`
	SponsorID     sdk.Many2One[CoreUser]         `nexa:"string=Sponsor"`
	CustomerId    sdk.Many2One[sdk.Any]          `nexa:"comodel=core.partner,string=Customer"`
	State         sdk.Selection[ProjectState]    `nexa:"string=Status,default=active,selection=draft:Draft,planned:Planned,approved:Approved,active:Active,onhold:On Hold,completed:Completed,cancelled:Cancelled,archived:Archived"`
	Health        sdk.Selection[ProjectHealth]   `nexa:"string=Health,default=not_assessed,selection=ontrack:On Track,atrisk:At Risk,offtrack:Off Track,not_assessed:Not Assessed"`
	Priority      sdk.Selection[ProjectPriority] `nexa:"string=Priority,default=normal"`
	PlannedStart  sdk.Date                       `nexa:"string=Planned Start"`
	PlannedEnd    sdk.Date                       `nexa:"string=Planned End"`
	TargetEnd     sdk.Date                       `nexa:"string=Target End"`
	ActualStart   sdk.Date                       `nexa:"string=Actual Start"`
	ActualEnd     sdk.Date                       `nexa:"string=Actual End"`
	Progress      sdk.Numeric                    `nexa:"string=Progress %,precision=18,scale=2,default=0"`
	HealthNote    sdk.Text                       `nexa:"string=Health Rationale"`
	Budget        sdk.Numeric                    `nexa:"string=Budget,precision=18,scale=2,default=0"`
	ActualCost    sdk.Numeric                    `nexa:"string=Actual Cost,precision=18,scale=2,default=0"`
	CommittedCost sdk.Numeric                    `nexa:"string=Committed Cost,precision=18,scale=2,default=0"`
	ContractValue sdk.Numeric                    `nexa:"string=Contract Value,precision=18,scale=2,default=0"`
	Color         sdk.Integer                    `nexa:"string=Color,default=0"`
	Phases        sdk.One2Many[ProjectPhase]     `nexa:"string=Phases"`
	Milestones    sdk.One2Many[ProjectMilestone] `nexa:"string=Milestones"`
	Members       sdk.One2Many[ProjectMember]    `nexa:"string=Team"`
	Sprints       sdk.One2Many[ProjectSprint]    `nexa:"string=Sprints"`
	Epics         sdk.One2Many[ProjectEpic]      `nexa:"string=Epics"`
	Description   sdk.Text                       `nexa:"string=Description"`
	Note          sdk.Text                       `nexa:"string=Notes"`
	Active        sdk.Boolean                    `nexa:"string=Active,default=true"`

	// Space shell counts: the live roll-ups that drive the per-project section
	// navigator on the form. Each is computed from real rows the moment the record
	// is read (see space.go), so a section badge is never a hand-typed number. They
	// span companion modules by model string — risks and documents report 0 when
	// those modules are not installed rather than fabricating anything.
	IssueCount     sdk.Integer `nexa:"string=Issues,compute=computeProjectIssueCount"`
	OpenIssueCount sdk.Integer `nexa:"string=Open,compute=computeProjectOpenIssueCount"`
	BacklogCount   sdk.Integer `nexa:"string=Backlog,compute=computeProjectBacklogCount"`
	BlockedCount   sdk.Integer `nexa:"string=Blocked,compute=computeProjectBlockedCount"`
	SprintCount    sdk.Integer `nexa:"string=Sprints,compute=computeProjectSprintCount"`
	EpicCount      sdk.Integer `nexa:"string=Epics,compute=computeProjectEpicCount"`
	MilestoneCount sdk.Integer `nexa:"string=Milestones,compute=computeProjectMilestoneCount"`
	MemberCount    sdk.Integer `nexa:"string=Team,compute=computeProjectMemberCount"`
	RiskCount      sdk.Integer `nexa:"string=Risks,compute=computeProjectRiskCount"`
	DocumentCount  sdk.Integer `nexa:"string=Documents,compute=computeProjectDocumentCount"`
}

type ProjectTask struct {
	// Chatter opt-in. The assignee is subscribed automatically, and the listed
	// fields are announced to everyone watching the issue. The lifecycle STATE is
	// tracked rather than the board column: a drag writes the stage and the
	// reconciler then writes the state, so tracking both would tell the thread
	// about one action twice.
	sdk.Model `nexa:"model=project.task,mail_thread=assigned_to,mail_notify=assigned_to;state;priority;deadline"`

	Name           sdk.String                       `nexa:"required,string=Task Title"`
	Image          sdk.Image                        `nexa:"string=Preview"`
	ProjectID      sdk.Many2One[ProjectProject]     `nexa:"index,string=Project"`
	ParentID       sdk.Many2One[ProjectTask]        `nexa:"string=Parent Task"`
	PhaseID        sdk.Many2One[ProjectPhase]       `nexa:"string=Phase"`
	MilestoneID    sdk.Many2One[ProjectMilestone]   `nexa:"string=Milestone"`
	AssignedTo     sdk.Many2One[CoreUser]           `nexa:"string=Assigned To"`
	Priority       sdk.Selection[TaskPriority]      `nexa:"string=Priority,default=normal"`
	State          sdk.Selection[TaskState]         `nexa:"string=Status,default=todo"`
	KanbanState    sdk.Selection[KanbanState]       `nexa:"string=Kanban State,default=standard"`
	Deadline       sdk.Date                         `nexa:"string=Deadline"`
	PlannedStart   sdk.Date                         `nexa:"string=Planned Start"`
	EstHours       sdk.Numeric                      `nexa:"string=Estimated Hours,precision=18,scale=2,default=0"`
	RemainingHours sdk.Numeric                      `nexa:"string=Remaining Hours,compute=computeTaskRemainingHours"`
	ActualHours    sdk.Numeric                      `nexa:"string=Spent Hours,compute=computeTaskSpentHours"`
	Worklogs       sdk.One2Many[ProjectWorklog]     `nexa:"string=Worklogs"`
	Sequence       sdk.Integer                      `nexa:"string=Sequence,default=10"`
	Comments       sdk.One2Many[ProjectTaskComment] `nexa:"string=Comments"`
	Description    sdk.Text                         `nexa:"string=Description"`
	TagIDs         sdk.Many2Many[ProjectTag]        `nexa:"string=Labels,table=project_task_tag_rel,left=task_id,right=tag_id"`
	FollowerCount  sdk.Integer                      `nexa:"string=Watchers,compute=computeTaskFollowers"`
	IsFollowed     sdk.Boolean                      `nexa:"string=Following,compute=computeTaskFollowing"`

	// Agile board layer (nullable, additive — see agile.go). Stage drives the state
	// machine; Key is a stable PROJECT-n identifier allocated on create.
	StageID     sdk.Many2One[ProjectBoardStage] `nexa:"index,string=Stage"`
	SprintID    sdk.Many2One[ProjectSprint]     `nexa:"index,string=Sprint"`
	EpicID      sdk.Many2One[ProjectEpic]       `nexa:"index,string=Epic"`
	IssueTypeID sdk.Many2One[ProjectIssueType]  `nexa:"index,string=Issue Type"`
	StoryPoints sdk.Integer                     `nexa:"string=Story Points,default=0"`
	Key         sdk.String                      `nexa:"index,size=30,string=Key"`
	Resolution  sdk.Selection[Resolution]       `nexa:"string=Resolution,default=none,selection=none:Not Resolved,done:Done,wont_fix:Won't Fix,duplicate:Duplicate,cannot_reproduce:Cannot Reproduce,cancelled:Cancelled"`
}

type TaskPriority string

const (
	TaskPriorityLow    TaskPriority = "low"
	TaskPriorityNormal TaskPriority = "normal"
	TaskPriorityHigh   TaskPriority = "high"
	TaskPriorityUrgent TaskPriority = "urgent"
)

func init() {
	sdk.MustRegister("project",
		&ProjectType{},
		&ProjectTag{},
		&ProjectPhase{},
		&ProjectMilestone{},
		&ProjectMember{},
		&ProjectTaskComment{},
		&ProjectProject{},
		&ProjectTask{},
		&ProjectBoardStage{},
		&ProjectIssueType{},
		&ProjectSprint{},
		&ProjectSprintSnapshot{},
		&ProjectEpic{},
		&ProjectWorklog{},
	)
}
