package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	hdmodels "nexa_addons/helpdesk/models"
)

// Cross-module reference aliases. The catalog companion links to the helpdesk
// anchor (team, category, ticket) by Many2One and never replaces it, mirroring
// the helpdesk_sla / _channel companion pattern.
type (
	HelpdeskTicket = hdmodels.HelpdeskTicket
	HelpdeskTeam   = hdmodels.HelpdeskTeam
	CoreUser       = basemodels.CoreUser
	CorePartner    = basemodels.CorePartner
	CoreCompany    = basemodels.CoreCompany
)

// --- Selection types ---

// ItemState is the publication lifecycle of a catalog item.
type ItemState string

const (
	ItemDraft     ItemState = "draft"
	ItemInReview  ItemState = "in_review"
	ItemPublished ItemState = "published"
	ItemArchived  ItemState = "archived"
)

// FieldType is the data type of a request-form field.
type FieldType string

const (
	FieldText     FieldType = "text"
	FieldTextarea FieldType = "textarea"
	FieldInteger  FieldType = "integer"
	FieldFloat    FieldType = "float"
	FieldBoolean  FieldType = "boolean"
	FieldDate     FieldType = "date"
	FieldDatetime FieldType = "datetime"
	FieldSelect   FieldType = "select"
	FieldPartner  FieldType = "partner"
)

// RequestState is the governed fulfilment lifecycle of a service request.
type RequestState string

const (
	ReqDraft      RequestState = "draft"
	ReqSubmitted  RequestState = "submitted"
	ReqInApproval RequestState = "in_approval"
	ReqFulfilling RequestState = "fulfilling"
	ReqCompleted  RequestState = "completed"
	ReqRejected   RequestState = "rejected"
	ReqCancelled  RequestState = "cancelled"
)

// TaskState is the lifecycle of a fulfilment task.
type TaskState string

const (
	TaskPending    TaskState = "pending"
	TaskInProgress TaskState = "in_progress"
	TaskDone       TaskState = "done"
	TaskSkipped    TaskState = "skipped"
)

// ApprovalState is the lifecycle of a request approval step.
type ApprovalState string

const (
	ApprovalPending   ApprovalState = "pending"
	ApprovalApproved  ApprovalState = "approved"
	ApprovalRejected  ApprovalState = "rejected"
	ApprovalCancelled ApprovalState = "cancelled"
)

// --- Models ---

// HelpdeskCatalogCategory groups service items (IT, HR, Finance, Facilities…).
type HelpdeskCatalogCategory struct {
	sdk.Model `nexa:"model=helpdesk.catalog.category"`

	Name     sdk.String                            `nexa:"required,string=Name"`
	Code     sdk.String                            `nexa:"size=60,string=Code"`
	ParentID sdk.Many2One[HelpdeskCatalogCategory] `nexa:"string=Parent"`
	Icon     sdk.String                            `nexa:"size=60,string=Icon"`
	Sequence sdk.Integer                           `nexa:"string=Sequence,default=10"`
	Active   sdk.Boolean                           `nexa:"string=Active,default=true"`
}

// HelpdeskCatalogItem is a requestable service with a form, an owning team, an
// expected turnaround, optional charge and an approval gate. It is the difference
// between a blank ticket box and a governed self-service catalog.
type HelpdeskCatalogItem struct {
	sdk.Model `nexa:"model=helpdesk.catalog.item"`

	Name             sdk.String                             `nexa:"required,string=Name"`
	Code             sdk.String                             `nexa:"size=60,string=Code"`
	CategoryID       sdk.Many2One[HelpdeskCatalogCategory]  `nexa:"string=Category"`
	Description      sdk.Text                               `nexa:"string=Description"`
	Instructions     sdk.Text                               `nexa:"string=How to Use"`
	TeamID           sdk.Many2One[HelpdeskTeam]             `nexa:"string=Fulfilment Team"`
	DefaultPriority  sdk.Selection[hdmodels.TicketPriority] `nexa:"string=Default Priority,default=normal"`
	ExpectedMins     sdk.Integer                            `nexa:"string=Expected Turnaround (min),default=480"`
	ApprovalRequired sdk.Boolean                            `nexa:"string=Approval Required,default=false"`
	ApproverUserID   sdk.Many2One[CoreUser]                 `nexa:"string=Approver"`
	Chargeable       sdk.Boolean                            `nexa:"string=Chargeable,default=false"`
	FeeCents         sdk.Integer                            `nexa:"string=Fee (cents),default=0"`
	State            sdk.Selection[ItemState]               `nexa:"string=Status,default=draft"`
	InternalOnly     sdk.Boolean                            `nexa:"string=Internal Only,default=true"`
	Sequence         sdk.Integer                            `nexa:"string=Sequence,default=10"`
	Active           sdk.Boolean                            `nexa:"string=Active,default=true"`
	CompanyID        sdk.Many2One[CoreCompany]              `nexa:"string=Company"`

	Fields sdk.One2Many[HelpdeskCatalogField] `nexa:"string=Form Fields"`

	FieldCount sdk.Integer `nexa:"string=# Fields,compute=compute_item_field_count"`
}

// HelpdeskCatalogField defines one field on a request form. Values captured per
// request are stored in helpdesk.service.request.value.
type HelpdeskCatalogField struct {
	sdk.Model `nexa:"model=helpdesk.catalog.field"`

	ItemID       sdk.Many2One[HelpdeskCatalogItem] `nexa:"required,index,string=Item"`
	Label        sdk.String                        `nexa:"required,string=Label"`
	FieldType    sdk.Selection[FieldType]          `nexa:"string=Type,default=text"`
	Required     sdk.Boolean                       `nexa:"string=Required,default=false"`
	Options      sdk.String                        `nexa:"size=300,string=Options (comma-separated)"`
	HelpText     sdk.String                        `nexa:"size=200,string=Help Text"`
	DefaultValue sdk.String                        `nexa:"size=200,string=Default Value"`
	Sequence     sdk.Integer                       `nexa:"string=Sequence,default=10"`
}

// HelpdeskServiceRequest is a governed business process instance, not merely a
// labelled ticket. It captures form values, approval steps and fulfilment tasks.
type HelpdeskServiceRequest struct {
	sdk.Model `nexa:"model=helpdesk.service.request"`

	Number        sdk.String                             `nexa:"string=Reference"`
	ItemID        sdk.Many2One[HelpdeskCatalogItem]      `nexa:"required,index,string=Service Item"`
	RequestorID   sdk.Many2One[CorePartner]              `nexa:"string=Requested For"`
	Department    sdk.String                             `nexa:"size=120,string=Department"`
	UserID        sdk.Many2One[CoreUser]                 `nexa:"string=Raised By"`
	State         sdk.Selection[RequestState]            `nexa:"string=Status,default=draft"`
	Priority      sdk.Selection[hdmodels.TicketPriority] `nexa:"string=Priority,default=normal"`
	TicketID      sdk.Many2One[HelpdeskTicket]           `nexa:"string=Linked Ticket"`
	Justification sdk.Text                               `nexa:"string=Business Justification"`
	RequiredBy    sdk.Date                               `nexa:"string=Required By"`
	SubmittedAt   sdk.DateTime                           `nexa:"string=Submitted"`
	CompletedAt   sdk.DateTime                           `nexa:"string=Completed"`
	CompanyID     sdk.Many2One[CoreCompany]              `nexa:"string=Company"`

	Values    sdk.One2Many[HelpdeskServiceRequestValue] `nexa:"string=Form Values"`
	Tasks     sdk.One2Many[HelpdeskFulfillmentTask]     `nexa:"string=Fulfilment Tasks"`
	Approvals sdk.One2Many[HelpdeskRequestApproval]     `nexa:"string=Approvals"`

	DaysOpen        sdk.Integer `nexa:"string=Days Open,compute=compute_request_days_open"`
	TaskProgressPct sdk.Integer `nexa:"string=Task Progress %,compute=compute_request_task_progress"`
	CanComplete     sdk.Boolean `nexa:"string=Can Complete,compute=compute_request_can_complete"`
}

// HelpdeskServiceRequestValue records a captured form field value for a request,
// preserving the label so historical requests remain readable after form edits.
type HelpdeskServiceRequestValue struct {
	sdk.Model `nexa:"model=helpdesk.service.request.value"`

	RequestID sdk.Many2One[HelpdeskServiceRequest] `nexa:"required,index,string=Request"`
	FieldID   sdk.Many2One[HelpdeskCatalogField]   `nexa:"string=Field"`
	Label     sdk.String                           `nexa:"size=120,string=Label"`
	Value     sdk.Text                             `nexa:"string=Value"`
	Sequence  sdk.Integer                          `nexa:"string=Sequence,default=10"`
}

// HelpdeskFulfillmentTask is one unit of work within a request (e.g. HR, IT,
// Facilities each get a task). Tasks may depend on another task completing first.
type HelpdeskFulfillmentTask struct {
	sdk.Model `nexa:"model=helpdesk.fulfillment.task"`

	RequestID   sdk.Many2One[HelpdeskServiceRequest]  `nexa:"required,index,string=Request"`
	Name        sdk.String                            `nexa:"required,string=Task"`
	TeamID      sdk.Many2One[HelpdeskTeam]            `nexa:"string=Responsible Team"`
	AssignedTo  sdk.Many2One[CoreUser]                `nexa:"string=Assigned To"`
	State       sdk.Selection[TaskState]              `nexa:"string=Status,default=pending"`
	Required    sdk.Boolean                           `nexa:"string=Required,default=true"`
	Sequence    sdk.Integer                           `nexa:"string=Sequence,default=10"`
	DependsOnID sdk.Many2One[HelpdeskFulfillmentTask] `nexa:"string=Depends On"`
	DueDate     sdk.DateTime                          `nexa:"string=Due"`
	CompletedAt sdk.DateTime                          `nexa:"string=Completed"`
	Notes       sdk.Text                              `nexa:"string=Notes"`

	IsReady sdk.Boolean `nexa:"string=Ready,compute=compute_task_is_ready"`
}

// HelpdeskRequestApproval is a single approval step on a request. The append-only
// decision trail lives here; separation of duties is enforced in the engine.
type HelpdeskRequestApproval struct {
	sdk.Model `nexa:"model=helpdesk.request.approval"`

	RequestID  sdk.Many2One[HelpdeskServiceRequest] `nexa:"required,index,string=Request"`
	Level      sdk.Integer                          `nexa:"string=Level,default=1"`
	State      sdk.Selection[ApprovalState]         `nexa:"string=Status,default=pending"`
	ApproverID sdk.Many2One[CoreUser]               `nexa:"string=Approver"`
	DecidedBy  sdk.Many2One[CoreUser]               `nexa:"string=Decided By"`
	DecidedAt  sdk.DateTime                         `nexa:"string=Decided At"`
	Note       sdk.Text                             `nexa:"string=Note"`
	Sequence   sdk.Integer                          `nexa:"string=Sequence,default=10"`
}

func init() {
	sdk.MustRegister("helpdesk_catalog",
		&HelpdeskCatalogCategory{},
		&HelpdeskCatalogItem{},
		&HelpdeskCatalogField{},
		&HelpdeskServiceRequest{},
		&HelpdeskServiceRequestValue{},
		&HelpdeskFulfillmentTask{},
		&HelpdeskRequestApproval{},
	)
}
