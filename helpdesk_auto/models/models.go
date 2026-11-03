package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	hdmodels "nexa_addons/helpdesk/models"
)

// Cross-module reference aliases. The automation companion links to the helpdesk
// anchor (ticket, team) by Many2One and never replaces it, mirroring the
// helpdesk_sla / _channel / _catalog companion pattern.
type (
	HelpdeskTicket = hdmodels.HelpdeskTicket
	HelpdeskTeam   = hdmodels.HelpdeskTeam
	CoreUser       = basemodels.CoreUser
	CoreCompany    = basemodels.CoreCompany
)

// --- Selection types ---

// AutomationTrigger is the event that evaluates a rule.
type AutomationTrigger string

const (
	TriggerTicketCreated   AutomationTrigger = "ticket_created"
	TriggerTicketUpdated   AutomationTrigger = "ticket_updated"
	TriggerStateChanged    AutomationTrigger = "state_changed"
	TriggerPriorityChanged AutomationTrigger = "priority_changed"
	TriggerAssigneeChanged AutomationTrigger = "assignee_changed"
	TriggerScheduled       AutomationTrigger = "scheduled"
)

// MatchLogic controls how multiple conditions combine.
type MatchLogic string

const (
	MatchAll MatchLogic = "all"
	MatchAny MatchLogic = "any"
)

// ConditionField is the ticket field a condition inspects.
type ConditionField string

const (
	FieldState       ConditionField = "state"
	FieldPriority    ConditionField = "priority"
	FieldTicketType  ConditionField = "ticket_type"
	FieldChannel     ConditionField = "channel"
	FieldTeam        ConditionField = "team_id"
	FieldAssignee    ConditionField = "assigned_to"
	FieldCategory    ConditionField = "category_id"
	FieldPartner     ConditionField = "partner_id"
	FieldReopenCount ConditionField = "reopened_count"
	FieldName        ConditionField = "name"
	FieldDescription ConditionField = "description"
	FieldHasTag      ConditionField = "has_tag"
)

// ConditionOperator is the comparison applied by a condition.
type ConditionOperator string

const (
	OpEquals      ConditionOperator = "equals"
	OpNotEquals   ConditionOperator = "not_equals"
	OpContains    ConditionOperator = "contains"
	OpIsEmpty     ConditionOperator = "is_empty"
	OpIsNotEmpty  ConditionOperator = "is_not_empty"
	OpIn          ConditionOperator = "in"
	OpNotIn       ConditionOperator = "not_in"
	OpGreaterThan ConditionOperator = "greater_than"
	OpLessThan    ConditionOperator = "less_than"
)

// ActionType is a typed mutation the engine can apply to a ticket.
type ActionType string

const (
	ActionSetState    ActionType = "set_state"
	ActionSetPriority ActionType = "set_priority"
	ActionSetTeam     ActionType = "set_team"
	ActionSetAssignee ActionType = "set_assignee"
	ActionSetCategory ActionType = "set_category"
	ActionAddNote     ActionType = "add_note"
	ActionAddTag      ActionType = "add_tag"
	ActionEscalate    ActionType = "escalate"
)

// RunStatus is the lifecycle of a single rule evaluation against a ticket.
type RunStatus string

const (
	RunQueued   RunStatus = "queued"
	RunMatched  RunStatus = "matched"
	RunSkipped  RunStatus = "skipped"
	RunExecuted RunStatus = "executed"
	RunFailed   RunStatus = "failed"
	RunDryRun   RunStatus = "dry_run"
)

// --- Models ---

// HelpdeskAutomationRule is an event-driven rule: WHEN a trigger fires, IF the
// conditions match, THEN the ordered actions are applied to the ticket. Rules are
// versioned and can be dry-run or emergency-disabled.
type HelpdeskAutomationRule struct {
	sdk.Model `nexa:"model=helpdesk.automation.rule"`

	Name           sdk.String                                `nexa:"required,index,string=Name"`
	Code           sdk.String                                `nexa:"string=Code"`
	Description    sdk.Text                                  `nexa:"string=Description"`
	Active         sdk.Boolean                               `nexa:"string=Active,default=true"`
	Sequence       sdk.Integer                               `nexa:"string=Sequence,default=10"`
	TriggerType    sdk.Selection[AutomationTrigger]          `nexa:"required,string=Trigger,default=ticket_created"`
	MatchLogic     sdk.Selection[MatchLogic]                 `nexa:"string=Match Logic,default=all"`
	DryRun         sdk.Boolean                               `nexa:"string=Dry Run,default=false"`
	ApplyOnce      sdk.Boolean                               `nexa:"string=Apply Once,default=true"`
	CompanyID      sdk.Many2One[CoreCompany]                 `nexa:"string=Company"`
	Notes          sdk.Text                                  `nexa:"string=Notes"`
	Conditions     sdk.One2Many[HelpdeskAutomationCondition] `nexa:"string=Conditions"`
	Actions        sdk.One2Many[HelpdeskAutomationAction]    `nexa:"string=Actions"`
	Versions       sdk.One2Many[HelpdeskAutomationVersion]   `nexa:"string=Versions"`
	Runs           sdk.One2Many[HelpdeskAutomationRun]       `nexa:"string=Runs"`
	CurrentVersion sdk.Integer                               `nexa:"string=Current Version,default=0"`
	ConditionCount sdk.Integer                               `nexa:"string=# Conditions,compute=compute_rule_condition_count"`
	ActionCount    sdk.Integer                               `nexa:"string=# Actions,compute=compute_rule_action_count"`
	RunCount       sdk.Integer                               `nexa:"string=# Runs,default=0"`
	MatchCount     sdk.Integer                               `nexa:"string=# Matches,default=0"`
	ErrorCount     sdk.Integer                               `nexa:"string=# Errors,default=0"`
	LastRunAt      sdk.DateTime                              `nexa:"string=Last Run At"`
}

// HelpdeskAutomationCondition is a single IF clause of a rule.
type HelpdeskAutomationCondition struct {
	sdk.Model `nexa:"model=helpdesk.automation.condition"`

	RuleID   sdk.Many2One[HelpdeskAutomationRule] `nexa:"required,index,string=Rule"`
	Sequence sdk.Integer                          `nexa:"string=Sequence,default=10"`
	Field    sdk.Selection[ConditionField]        `nexa:"required,string=Field,default=state"`
	Operator sdk.Selection[ConditionOperator]     `nexa:"required,string=Operator,default=equals"`
	Value    sdk.String                           `nexa:"string=Value"`
}

// HelpdeskAutomationAction is a single THEN step of a rule, applied in sequence order.
type HelpdeskAutomationAction struct {
	sdk.Model `nexa:"model=helpdesk.automation.action"`

	RuleID     sdk.Many2One[HelpdeskAutomationRule] `nexa:"required,index,string=Rule"`
	Sequence   sdk.Integer                          `nexa:"string=Sequence,default=10"`
	ActionType sdk.Selection[ActionType]            `nexa:"required,string=Action,default=set_priority"`
	Value      sdk.String                           `nexa:"string=Value"`
	Notes      sdk.Text                             `nexa:"string=Notes"`
}

// HelpdeskAutomationVersion is an immutable snapshot of a rule, taken when it is
// published. It provides the explicit version-and-publish safeguard.
type HelpdeskAutomationVersion struct {
	sdk.Model `nexa:"model=helpdesk.automation.version"`

	RuleID        sdk.Many2One[HelpdeskAutomationRule] `nexa:"required,index,string=Rule"`
	VersionNumber sdk.Integer                          `nexa:"required,index,string=Version,default=1"`
	Snapshot      sdk.Text                             `nexa:"string=Snapshot"`
	ChangeNote    sdk.Text                             `nexa:"string=Change Note"`
	PublishedBy   sdk.Many2One[CoreUser]               `nexa:"string=Published By"`
	PublishedAt   sdk.DateTime                         `nexa:"string=Published At"`
}

// HelpdeskAutomationRun is the execution log of a rule evaluated against a ticket,
// recording match, planned/applied action counts and any failure reason.
type HelpdeskAutomationRun struct {
	sdk.Model `nexa:"model=helpdesk.automation.run"`

	RuleID         sdk.Many2One[HelpdeskAutomationRule] `nexa:"required,index,string=Rule"`
	TicketID       sdk.Many2One[HelpdeskTicket]         `nexa:"index,string=Ticket"`
	TriggerType    sdk.Selection[AutomationTrigger]     `nexa:"string=Trigger,default=ticket_created"`
	Status         sdk.Selection[RunStatus]             `nexa:"required,index,string=Status,default=queued"`
	Matched        sdk.Boolean                          `nexa:"string=Matched,default=false"`
	DryRun         sdk.Boolean                          `nexa:"string=Dry Run,default=false"`
	ActionsPlanned sdk.Integer                          `nexa:"string=Actions Planned,default=0"`
	ActionsApplied sdk.Integer                          `nexa:"string=Actions Applied,default=0"`
	Detail         sdk.Text                             `nexa:"string=Detail"`
	ErrorReason    sdk.Text                             `nexa:"string=Error Reason"`
	StartedAt      sdk.DateTime                         `nexa:"string=Started At"`
	FinishedAt     sdk.DateTime                         `nexa:"string=Finished At"`
}

func init() {
	sdk.MustRegister("helpdesk_auto",
		&HelpdeskAutomationRule{},
		&HelpdeskAutomationCondition{},
		&HelpdeskAutomationAction{},
		&HelpdeskAutomationVersion{},
		&HelpdeskAutomationRun{},
	)
}
