package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	projmodels "nexa_addons/project/models"
)

// Cross-module reference aliases.
type (
	ProjectProject = projmodels.ProjectProject
	CoreUser       = basemodels.CoreUser
)

type Level string

const (
	LevelLow      Level = "low"
	LevelMedium   Level = "medium"
	LevelHigh     Level = "high"
	LevelCritical Level = "critical"
)

type RiskStatus string

const (
	RiskOpen       RiskStatus = "open"
	RiskMitigating RiskStatus = "mitigating"
	RiskClosed     RiskStatus = "closed"
)

type IssueStatus string

const (
	IssueNew        IssueStatus = "new"
	IssueInProgress IssueStatus = "in_progress"
	IssueResolved   IssueStatus = "resolved"
	IssueClosed     IssueStatus = "closed"
)

type ChangeType string

const (
	ChangeScope       ChangeType = "scope"
	ChangeBudget      ChangeType = "budget"
	ChangeDates       ChangeType = "dates"
	ChangeDeliverable ChangeType = "deliverable"
	ChangeOther       ChangeType = "other"
)

type ChangeState string

const (
	ChangeDraft     ChangeState = "draft"
	ChangeSubmitted ChangeState = "submitted"
	ChangeApproved  ChangeState = "approved"
	ChangeRejected  ChangeState = "rejected"
)

// ProjectRisk is a risk register entry with probability, impact and mitigation.
type ProjectRisk struct {
	sdk.Model `nexa:"model=project.risk"`

	ProjectID   sdk.Many2One[ProjectProject] `nexa:"required,index,string=Project"`
	Name        sdk.String                   `nexa:"required,string=Risk"`
	Probability sdk.Selection[Level]         `nexa:"string=Probability,default=medium"`
	Impact      sdk.Selection[Level]         `nexa:"string=Impact,default=medium"`
	Severity    sdk.Selection[Level]         `nexa:"string=Severity,default=medium"`
	Status      sdk.Selection[RiskStatus]    `nexa:"string=Status,default=open"`
	OwnerID     sdk.Many2One[CoreUser]       `nexa:"string=Owner"`
	ReviewDate  sdk.Date                     `nexa:"string=Next Review"`
	Mitigation  sdk.Text                     `nexa:"string=Mitigation"`
	Description sdk.Text                     `nexa:"string=Description"`
}

// ProjectIssue is a delivery issue or blocker raised on a project.
type ProjectIssue struct {
	sdk.Model `nexa:"model=project.issue"`

	ProjectID      sdk.Many2One[ProjectProject] `nexa:"required,index,string=Project"`
	Name           sdk.String                   `nexa:"required,string=Issue"`
	Priority       sdk.Selection[Level]         `nexa:"string=Priority,default=medium"`
	Status         sdk.Selection[IssueStatus]   `nexa:"string=Status,default=new"`
	OwnerID        sdk.Many2One[CoreUser]       `nexa:"string=Owner"`
	ResolutionDate sdk.Date                     `nexa:"string=Resolution Date"`
	Resolution     sdk.Text                     `nexa:"string=Resolution"`
	Description    sdk.Text                     `nexa:"string=Description"`
}

// ProjectChangeRequest records a proposed change and its approved outcome.
type ProjectChangeRequest struct {
	sdk.Model `nexa:"model=project.change.request"`

	ProjectID          sdk.Many2One[ProjectProject] `nexa:"required,index,string=Project"`
	Name               sdk.String                   `nexa:"required,string=Change Title"`
	ChangeType         sdk.Selection[ChangeType]    `nexa:"required,string=Change Type,default=scope"`
	State              sdk.Selection[ChangeState]   `nexa:"string=Status,default=draft"`
	RequestedBy        sdk.Many2One[CoreUser]       `nexa:"string=Requested By"`
	ApprovedBy         sdk.Many2One[CoreUser]       `nexa:"string=Approved By"`
	DecisionDate       sdk.Date                     `nexa:"string=Decision Date"`
	BudgetImpact       sdk.Numeric                  `nexa:"string=Budget Impact,precision=18,scale=2,default=0"`
	ScheduleImpactDays sdk.Integer                  `nexa:"string=Schedule Impact (days),default=0"`
	ProposedChange     sdk.Text                     `nexa:"string=Proposed Change"`
	ImpactNotes        sdk.Text                     `nexa:"string=Impact Analysis"`
}

func init() {
	sdk.MustRegister("project_risk",
		&ProjectRisk{},
		&ProjectIssue{},
		&ProjectChangeRequest{},
	)
}
