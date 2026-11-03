package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	projmodels "nexa_addons/project/models"
)

// Cross-module reference aliases.
type (
	ProjectProject = projmodels.ProjectProject
	ProjectTask    = projmodels.ProjectTask
	CoreUser       = basemodels.CoreUser
)

// MeetingState is the lifecycle of a project meeting.
type MeetingState string

const (
	MeetingDraft     MeetingState = "draft"
	MeetingScheduled MeetingState = "scheduled"
	MeetingHeld      MeetingState = "held"
	MeetingCancelled MeetingState = "cancelled"
)

// ActionState is the lifecycle of a collaboration action item.
type ActionState string

const (
	ActionOpen       ActionState = "open"
	ActionInProgress ActionState = "in_progress"
	ActionDone       ActionState = "done"
	ActionCancelled  ActionState = "cancelled"
)

// ActionPriority ranks an action item's urgency.
type ActionPriority string

const (
	ActionLow    ActionPriority = "low"
	ActionNormal ActionPriority = "normal"
	ActionHigh   ActionPriority = "high"
)

// DocState is the review lifecycle of a project document.
type DocState string

const (
	DocDraft    DocState = "draft"
	DocInReview DocState = "in_review"
	DocApproved DocState = "approved"
	DocArchived DocState = "archived"
)

// ProjectMeeting is a scheduled working session for a project with an agenda,
// attendees and recorded minutes. Holding a meeting can spawn action items.
type ProjectMeeting struct {
	sdk.Model `nexa:"model=project.meeting"`

	Name      sdk.String                           `nexa:"required,string=Subject"`
	ProjectID sdk.Many2One[ProjectProject]         `nexa:"required,index,string=Project"`
	Date      sdk.Date                             `nexa:"required,index,string=Date"`
	Location  sdk.String                           `nexa:"string=Location"`
	Organizer sdk.Many2One[CoreUser]               `nexa:"string=Organizer"`
	State     sdk.Selection[MeetingState]          `nexa:"string=Status,default=draft"`
	Agenda    sdk.Text                             `nexa:"string=Agenda"`
	Minutes   sdk.Text                             `nexa:"string=Minutes"`
	Attendees sdk.One2Many[ProjectMeetingAttendee] `nexa:"string=Attendees"`
}

// ProjectMeetingAttendee is an invited person on a meeting and whether they came.
type ProjectMeetingAttendee struct {
	sdk.Model `nexa:"model=project.meeting.attendee"`

	MeetingID sdk.Many2One[ProjectMeeting] `nexa:"required,index,string=Meeting"`
	UserID    sdk.Many2One[CoreUser]       `nexa:"required,string=Attendee"`
	Required  sdk.Boolean                  `nexa:"string=Required,default=true"`
	Attended  sdk.Boolean                  `nexa:"string=Attended,default=false"`
}

// ProjectActionItem is a follow-up commitment raised in a meeting or on a task.
// It carries its own owner and due date; overdue status is derived by a cron.
type ProjectActionItem struct {
	sdk.Model `nexa:"model=project.action.item"`

	Name        sdk.String                    `nexa:"required,string=Action"`
	ProjectID   sdk.Many2One[ProjectProject]  `nexa:"required,index,string=Project"`
	MeetingID   sdk.Many2One[ProjectMeeting]  `nexa:"index,string=Origin Meeting"`
	TaskID      sdk.Many2One[ProjectTask]     `nexa:"string=Related Task"`
	AssignedTo  sdk.Many2One[CoreUser]        `nexa:"index,string=Owner"`
	DueDate     sdk.Date                      `nexa:"index,string=Due Date"`
	Priority    sdk.Selection[ActionPriority] `nexa:"string=Priority,default=normal"`
	State       sdk.Selection[ActionState]    `nexa:"string=Status,default=open"`
	Overdue     sdk.Boolean                   `nexa:"string=Overdue,default=false"`
	Description sdk.Text                      `nexa:"string=Details"`
}

// ProjectDocument registers a controlled deliverable or reference document for a
// project. Approval is separated from the author so a draft cannot self-approve.
type ProjectDocument struct {
	sdk.Model `nexa:"model=project.document"`

	Name        sdk.String                   `nexa:"required,string=Title"`
	ProjectID   sdk.Many2One[ProjectProject] `nexa:"required,index,string=Project"`
	Category    sdk.String                   `nexa:"string=Category"`
	Link        sdk.String                   `nexa:"string=Location / URL"`
	OwnerID     sdk.Many2One[CoreUser]       `nexa:"string=Owner"`
	Version     sdk.String                   `nexa:"string=Version"`
	State       sdk.Selection[DocState]      `nexa:"string=Status,default=draft"`
	ApprovedBy  sdk.Many2One[CoreUser]       `nexa:"string=Approved By"`
	Description sdk.Text                     `nexa:"string=Summary"`
}

func init() {
	sdk.MustRegister("project_collab",
		&ProjectMeeting{},
		&ProjectMeetingAttendee{},
		&ProjectActionItem{},
		&ProjectDocument{},
	)
}
