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
	// EntityKind is owned by the core project module; aliased here so the
	// status-history model and lifecycle actions share one definition.
	EntityKind = projmodels.EntityKind
)

// DependencyType is the kind of link between two tasks.
type DependencyType string

const (
	DepFinishToStart  DependencyType = "finish_to_start"
	DepStartToStart   DependencyType = "start_to_start"
	DepFinishToFinish DependencyType = "finish_to_finish"
	DepStartToFinish  DependencyType = "start_to_finish"
)

// DeliverableState is the acceptance lifecycle of a deliverable.
type DeliverableState string

const (
	DeliverableDraft     DeliverableState = "draft"
	DeliverableSubmitted DeliverableState = "submitted"
	DeliverableAccepted  DeliverableState = "accepted"
	DeliverableRejected  DeliverableState = "rejected"
)

// EntityKind is an alias to the core project definition (see the type block above).

// ProjectTaskDependency links a predecessor task to a successor task with a typed relation.
type ProjectTaskDependency struct {
	sdk.Model `nexa:"model=project.task.dependency"`

	ProjectID      sdk.Many2One[ProjectProject]  `nexa:"index,string=Project"`
	TaskID         sdk.Many2One[ProjectTask]     `nexa:"required,index,string=Task"`
	PredecessorID  sdk.Many2One[ProjectTask]     `nexa:"required,string=Predecessor"`
	DependencyType sdk.Selection[DependencyType] `nexa:"string=Type,default=finish_to_start"`
	LagDays        sdk.Integer                   `nexa:"string=Lag (days),default=0"`
	Note           sdk.Text                      `nexa:"string=Note"`
}

// ProjectDeliverable is a work output that must be produced and accepted.
type ProjectDeliverable struct {
	sdk.Model `nexa:"model=project.deliverable"`

	ProjectID    sdk.Many2One[ProjectProject]              `nexa:"required,index,string=Project"`
	MilestoneID  sdk.Many2One[projmodels.ProjectMilestone] `nexa:"string=Milestone"`
	TaskID       sdk.Many2One[ProjectTask]                 `nexa:"string=Source Task"`
	Name         sdk.String                                `nexa:"required,string=Deliverable"`
	State        sdk.Selection[DeliverableState]           `nexa:"string=Status,default=draft"`
	OwnerID      sdk.Many2One[CoreUser]                    `nexa:"string=Owner"`
	DueDate      sdk.Date                                  `nexa:"string=Due Date"`
	AcceptedDate sdk.Date                                  `nexa:"string=Accepted Date"`
	IsBillable   sdk.Boolean                               `nexa:"string=Billable,default=false"`
	Description  sdk.Text                                  `nexa:"string=Description"`
}

// ProjectStatusHistory records a status transition on a project or task (audit timeline).
type ProjectStatusHistory struct {
	sdk.Model `nexa:"model=project.status.history"`

	ProjectID  sdk.Many2One[ProjectProject] `nexa:"index,string=Project"`
	TaskID     sdk.Many2One[ProjectTask]    `nexa:"index,string=Task"`
	EntityType sdk.Selection[EntityKind]    `nexa:"required,string=Entity,default=project"`
	FromState  sdk.String                   `nexa:"string=From"`
	ToState    sdk.String                   `nexa:"string=To"`
	UserID     sdk.Many2One[CoreUser]       `nexa:"string=Changed By"`
	Note       sdk.Text                     `nexa:"string=Note"`
}

func init() {
	sdk.MustRegister("project_planning",
		&ProjectTaskDependency{},
		&ProjectDeliverable{},
		&ProjectStatusHistory{},
	)
}
