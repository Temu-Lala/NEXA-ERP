package models

import (
	"nexa/core/sdk"
	crmodels "nexa_addons/crm/models"
)

// Lead governance layer: append-only status history, assignment audit, reviewed
// duplicate candidates, typed assignment rules, and small reliable workflow
// rules. Duplicate detection surfaces candidates for an authorized user to
// review; it never auto-merges on name alone. crm.lead stays the authoritative
// record - these models only add governance around it.

type (
	CoreUser    = crmodels.CoreUser
	CoreCompany = crmodels.CoreCompany
	CrmLead     = crmodels.CrmLead
	CrmTeam     = crmodels.CrmTeam
)

type AssignMethod string

const (
	AssignManual     AssignMethod = "manual"
	AssignTeam       AssignMethod = "team"
	AssignTerritory  AssignMethod = "territory"
	AssignSource     AssignMethod = "source"
	AssignRoundRobin AssignMethod = "round_robin"
	AssignWorkload   AssignMethod = "workload"
)

type MatchSignal string

const (
	MatchEmail   MatchSignal = "email"
	MatchPhone   MatchSignal = "phone"
	MatchCompany MatchSignal = "company"
	MatchName    MatchSignal = "name"
)

type DuplicateStatus string

const (
	DupCandidate DuplicateStatus = "candidate"
	DupReviewed  DuplicateStatus = "reviewed"
	DupMerged    DuplicateStatus = "merged"
	DupIgnored   DuplicateStatus = "ignored"
)

type RuleApplyOn string

const (
	RuleLead        RuleApplyOn = "lead"
	RuleOpportunity RuleApplyOn = "opportunity"
)

type RuleType string

const (
	RoundTypeRoundRobin RuleType = "round_robin"
	RoundTypeTerritory  RuleType = "territory"
	RoundTypeSource     RuleType = "source"
	RoundTypeProduct    RuleType = "product"
	RoundTypeIndustry   RuleType = "industry"
	RoundTypeWorkload   RuleType = "workload"
)

type WorkflowTrigger string

const (
	TrigLeadCreated       WorkflowTrigger = "lead_created"
	TrigLeadAssigned      WorkflowTrigger = "lead_assigned"
	TrigOpportunityUpdate WorkflowTrigger = "opportunity_updated"
	TrigTaskOverdue       WorkflowTrigger = "task_overdue"
	TrigDealWon           WorkflowTrigger = "deal_won"
	TrigRenewalNear       WorkflowTrigger = "renewal_near"
)

type WorkflowAction string

const (
	ActCreateTask  WorkflowAction = "create_task"
	ActAssignOwner WorkflowAction = "assign_owner"
	ActNotify      WorkflowAction = "notify"
	ActRemind      WorkflowAction = "remind"
)

type WorkflowRecipient string

const (
	RecipOwner   WorkflowRecipient = "owner"
	RecipManager WorkflowRecipient = "manager"
	RecipTeam    WorkflowRecipient = "team"
)

// CrmLeadStatusHistory is an append-only log of lead/opportunity status moves.
type CrmLeadStatusHistory struct {
	sdk.Model `nexa:"model=crm.lead.status.history"`

	LeadID     sdk.Many2One[CrmLead]     `nexa:"required,index,string=Lead"`
	FromStatus sdk.String                `nexa:"size=60,string=From"`
	ToStatus   sdk.String                `nexa:"required,size=60,string=To"`
	Reason     sdk.String                `nexa:"size=200,string=Reason"`
	UserID     sdk.Many2One[CoreUser]    `nexa:"string=Changed By"`
	ChangedAt  sdk.DateTime              `nexa:"string=Changed At"`
	CompanyID  sdk.Many2One[CoreCompany] `nexa:"string=Company"`
}

// CrmLeadAssignment records each ownership change: previous owner, new owner,
// who assigned it, method and timestamp. Reassignment never drops history.
type CrmLeadAssignment struct {
	sdk.Model `nexa:"model=crm.lead.assignment"`

	LeadID          sdk.Many2One[CrmLead]       `nexa:"required,index,string=Lead"`
	PreviousOwnerID sdk.Many2One[CoreUser]      `nexa:"string=Previous Owner"`
	NewOwnerID      sdk.Many2One[CoreUser]      `nexa:"required,string=New Owner"`
	AssignedByID    sdk.Many2One[CoreUser]      `nexa:"string=Assigned By"`
	AssignedAt      sdk.DateTime                `nexa:"string=Assigned At"`
	Method          sdk.Selection[AssignMethod] `nexa:"string=Method,default=manual"`
	CompanyID       sdk.Many2One[CoreCompany]   `nexa:"string=Company"`
}

// CrmLeadDuplicate is a possible-duplicate pair awaiting human review. It
// stores the match signal and confidence but never merges automatically.
type CrmLeadDuplicate struct {
	sdk.Model `nexa:"model=crm.lead.duplicate"`

	LeadID          sdk.Many2One[CrmLead]          `nexa:"required,index,string=Lead"`
	CandidateLeadID sdk.Many2One[CrmLead]          `nexa:"required,string=Possible Duplicate"`
	MatchSignal     sdk.Selection[MatchSignal]     `nexa:"string=Match Signal,default=email"`
	Confidence      sdk.Float                      `nexa:"string=Confidence (%)"`
	Status          sdk.Selection[DuplicateStatus] `nexa:"string=Status,default=candidate"`
	ReviewedBy      sdk.Many2One[CoreUser]         `nexa:"string=Reviewed By"`
	CompanyID       sdk.Many2One[CoreCompany]      `nexa:"string=Company"`
}

// CrmAssignmentRule is a typed, ordered routing rule for assigning new work.
type CrmAssignmentRule struct {
	sdk.Model `nexa:"model=crm.assignment.rule"`

	Name      sdk.String                 `nexa:"required,size=120,string=Name"`
	ApplyOn   sdk.Selection[RuleApplyOn] `nexa:"string=Apply On,default=lead"`
	RuleType  sdk.Selection[RuleType]    `nexa:"string=Rule Type,default=round_robin"`
	Sequence  sdk.Integer                `nexa:"string=Sequence,default=10"`
	Enabled   sdk.Boolean                `nexa:"string=Enabled,default=true"`
	TeamID    sdk.Many2One[CrmTeam]      `nexa:"string=Sales Team"`
	CompanyID sdk.Many2One[CoreCompany]  `nexa:"string=Company"`
	Active    sdk.Boolean                `nexa:"string=Active,default=true"`
}

// CrmWorkflowRule is a small typed automation rule (trigger + action) with an
// execution counter. Intervals and recipients are configurable, not hardcoded.
type CrmWorkflowRule struct {
	sdk.Model `nexa:"model=crm.workflow.rule"`

	Name         sdk.String                       `nexa:"required,size=120,string=Name"`
	Trigger      sdk.Selection[WorkflowTrigger]   `nexa:"required,string=Trigger,default=lead_created"`
	Conditions   sdk.Text                         `nexa:"string=Conditions"`
	Action       sdk.Selection[WorkflowAction]    `nexa:"required,string=Action,default=create_task"`
	IntervalDays sdk.Integer                      `nexa:"string=Interval Days,default=3"`
	Recipient    sdk.Selection[WorkflowRecipient] `nexa:"string=Notify,default=owner"`
	Enabled      sdk.Boolean                      `nexa:"string=Enabled,default=true"`
	ExecCount    sdk.Integer                      `nexa:"string=Executions,default=0"`
	CompanyID    sdk.Many2One[CoreCompany]        `nexa:"string=Company"`
	Active       sdk.Boolean                      `nexa:"string=Active,default=true"`
}

func init() {
	sdk.MustRegister("crm_lifecycle",
		&CrmLeadStatusHistory{},
		&CrmLeadAssignment{},
		&CrmLeadDuplicate{},
		&CrmAssignmentRule{},
		&CrmWorkflowRule{},
	)
}
