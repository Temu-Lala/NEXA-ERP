package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// CoreUser → core.user
type CoreUser = basemodels.CoreUser

// CorePartner → core.partner
type CorePartner = basemodels.CorePartner

// CoreCompany → res.company
type CoreCompany = basemodels.CoreCompany

// --- Selection types ---

type TicketState string

const (
	TicketStateNew        TicketState = "new"
	TicketStateOpen       TicketState = "open"
	TicketStateInProgress TicketState = "in_progress"
	TicketStatePending    TicketState = "pending"
	TicketStateResolved   TicketState = "resolved"
	TicketStateClosed     TicketState = "closed"
	TicketStateCancelled  TicketState = "cancelled"
)

type TicketPriority string

const (
	TicketPriorityLow    TicketPriority = "low"
	TicketPriorityNormal TicketPriority = "normal"
	TicketPriorityHigh   TicketPriority = "high"
	TicketPriorityUrgent TicketPriority = "urgent"
)

type TicketType string

const (
	TicketTypeIncident       TicketType = "incident"
	TicketTypeServiceRequest TicketType = "service_request"
	TicketTypeQuestion       TicketType = "question"
	TicketTypeComplaint      TicketType = "complaint"
	TicketTypeProblem        TicketType = "problem"
	TicketTypeChangeRequest  TicketType = "change_request"
	TicketTypeFeatureRequest TicketType = "feature_request"
	TicketTypeInternalIT     TicketType = "internal_it"
)

type TicketChannel string

const (
	ChannelEmail    TicketChannel = "email"
	ChannelPortal   TicketChannel = "portal"
	ChannelPhone    TicketChannel = "phone"
	ChannelChat     TicketChannel = "chat"
	ChannelWhatsApp TicketChannel = "whatsapp"
	ChannelAPI      TicketChannel = "api"
	ChannelInternal TicketChannel = "internal"
)

type MessageType string

const (
	MessageTypeComment MessageType = "comment"
	MessageTypeEmail   MessageType = "email"
	MessageTypeNote    MessageType = "note"
)

type HistoryAction string

const (
	HistoryCreated    HistoryAction = "created"
	HistoryState      HistoryAction = "state"
	HistoryPriority   HistoryAction = "priority"
	HistoryAssignment HistoryAction = "assignment"
	HistoryCategorize HistoryAction = "category"
	HistoryLink       HistoryAction = "link"
	HistoryNote       HistoryAction = "note"
)

type LinkRelation string

const (
	LinkParent    LinkRelation = "parent"
	LinkChild     LinkRelation = "child"
	LinkDuplicate LinkRelation = "duplicate"
	LinkRelated   LinkRelation = "related"
)

// --- Models ---

type HelpdeskTeam struct {
	sdk.Model `nexa:"model=helpdesk.team"`

	Name         sdk.String              `nexa:"required,string=Team Name"`
	Email        sdk.Email               `nexa:"string=Email"`
	MemberIDs    sdk.Many2Many[CoreUser] `nexa:"string=Members,table=helpdesk_team_member_rel,left=team_id,right=user_id"`
	AllowInbound sdk.Boolean             `nexa:"string=Allow Inbound,default=false"`
	Notes        sdk.Text                `nexa:"string=Notes"`
	Active       sdk.Boolean             `nexa:"string=Active,default=true"`
}

type HelpdeskTicketTag struct {
	sdk.Model `nexa:"model=helpdesk.ticket.tag"`

	Name   sdk.String  `nexa:"required,string=Name"`
	Color  sdk.Integer `nexa:"string=Color,default=0"`
	Active sdk.Boolean `nexa:"string=Active,default=true"`
}

type HelpdeskTicketCategory struct {
	sdk.Model `nexa:"model=helpdesk.ticket.category"`

	Name     sdk.String                           `nexa:"required,string=Name"`
	ParentID sdk.Many2One[HelpdeskTicketCategory] `nexa:"string=Parent Category"`
	TeamID   sdk.Many2One[HelpdeskTeam]           `nexa:"string=Default Team"`
	Color    sdk.Integer                          `nexa:"string=Color,default=0"`
	Sequence sdk.Integer                          `nexa:"string=Sequence,default=10"`
	Active   sdk.Boolean                          `nexa:"string=Active,default=true"`
}

type HelpdeskTicket struct {
	sdk.Model `nexa:"model=helpdesk.ticket"`

	Name            sdk.String                           `nexa:"required,string=Subject"`
	Number          sdk.String                           `nexa:"string=Ticket Number"`
	PartnerID       sdk.Many2One[CorePartner]            `nexa:"string=Reported By"`
	TeamID          sdk.Many2One[HelpdeskTeam]           `nexa:"string=Team"`
	AssignedTo      sdk.Many2One[CoreUser]               `nexa:"string=Assigned To"`
	CategoryID      sdk.Many2One[HelpdeskTicketCategory] `nexa:"string=Category"`
	Priority        sdk.Selection[TicketPriority]        `nexa:"string=Priority,default=normal"`
	State           sdk.Selection[TicketState]           `nexa:"string=Status,default=new"`
	TicketType      sdk.Selection[TicketType]            `nexa:"string=Type,default=incident"`
	Channel         sdk.Selection[TicketChannel]         `nexa:"string=Channel,default=email"`
	TagIDs          sdk.Many2Many[HelpdeskTicketTag]     `nexa:"string=Tags,table=helpdesk_ticket_tag_rel,left=ticket_id,right=tag_id"`
	Description     sdk.Text                             `nexa:"string=Description"`
	Resolution      sdk.Text                             `nexa:"string=Resolution"`
	FirstResponseAt sdk.DateTime                         `nexa:"string=First Response At"`
	ReopenedCount   sdk.Integer                          `nexa:"string=Reopen Count,default=0"`
	ClosedDate      sdk.DateTime                         `nexa:"string=Closed Date"`
	CompanyID       sdk.Many2One[CoreCompany]            `nexa:"string=Company"`
	Messages        sdk.One2Many[HelpdeskTicketMessage]  `nexa:"string=Messages"`
	History         sdk.One2Many[HelpdeskTicketHistory]  `nexa:"string=History"`
	Links           sdk.One2Many[HelpdeskTicketLink]     `nexa:"string=Related Tickets"`
	MessageCount    sdk.Integer                          `nexa:"string=# Messages,compute=compute_ticket_message_count"`
	HistoryCount    sdk.Integer                          `nexa:"string=# History,compute=compute_ticket_history_count"`
}

type HelpdeskTicketHistory struct {
	sdk.Model `nexa:"model=helpdesk.ticket.history"`

	TicketID  sdk.Many2One[HelpdeskTicket] `nexa:"required,index,string=Ticket"`
	Action    sdk.Selection[HistoryAction] `nexa:"string=Action,default=note"`
	Field     sdk.String                   `nexa:"size=60,string=Field"`
	OldValue  sdk.String                   `nexa:"size=200,string=From"`
	NewValue  sdk.String                   `nexa:"size=200,string=To"`
	Summary   sdk.Text                     `nexa:"string=Summary"`
	AuthorID  sdk.Many2One[CoreUser]       `nexa:"string=Changed By"`
	Date      sdk.DateTime                 `nexa:"string=Date"`
	CompanyID sdk.Many2One[CoreCompany]    `nexa:"string=Company"`
}

type HelpdeskTicketLink struct {
	sdk.Model `nexa:"model=helpdesk.ticket.link"`

	TicketID        sdk.Many2One[HelpdeskTicket] `nexa:"required,index,string=Ticket"`
	RelatedTicketID sdk.Many2One[HelpdeskTicket] `nexa:"required,string=Related Ticket"`
	Relation        sdk.Selection[LinkRelation]  `nexa:"string=Relation,default=related"`
	Note            sdk.String                   `nexa:"size=200,string=Note"`
}

type HelpdeskTicketMessage struct {
	sdk.Model `nexa:"model=helpdesk.ticket.message"`

	TicketID    sdk.Many2One[HelpdeskTicket] `nexa:"required,string=Ticket"`
	AuthorID    sdk.Many2One[CoreUser]       `nexa:"string=Author"`
	Body        sdk.Text                     `nexa:"required,string=Message"`
	MessageType sdk.Selection[MessageType]   `nexa:"string=Type,default=comment"`
	IsInternal  sdk.Boolean                  `nexa:"string=Internal Note,default=false"`
}

type HelpdeskCannedResponse struct {
	sdk.Model `nexa:"model=helpdesk.canned.response"`

	Name   sdk.String                 `nexa:"required,string=Shortcut"`
	Body   sdk.Text                   `nexa:"required,string=Response"`
	TeamID sdk.Many2One[HelpdeskTeam] `nexa:"string=Team"`
	Active sdk.Boolean                `nexa:"string=Active,default=true"`
}

func init() {
	sdk.MustRegister("helpdesk",
		&HelpdeskTeam{},
		&HelpdeskTicketTag{},
		&HelpdeskTicketCategory{},
		&HelpdeskTicket{},
		&HelpdeskTicketMessage{},
		&HelpdeskTicketHistory{},
		&HelpdeskTicketLink{},
		&HelpdeskCannedResponse{},
	)
}
