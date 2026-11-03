package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	hdmodels "nexa_addons/helpdesk/models"
)

// Cross-module reference aliases. The channel companion links to the helpdesk
// anchor by Many2One and never replaces it, mirroring the helpdesk_sla / _kb
// companion pattern.
type (
	HelpdeskTicket = hdmodels.HelpdeskTicket
	HelpdeskTeam   = hdmodels.HelpdeskTeam
	CoreUser       = basemodels.CoreUser
	CoreCompany    = basemodels.CoreCompany
)

// --- Selection types ---

// ChannelKind is the transport a connection or message uses.
type ChannelKind string

const (
	ChannelEmail      ChannelKind = "email"
	ChannelPortal     ChannelKind = "portal"
	ChannelChat       ChannelKind = "chat"
	ChannelWhatsApp   ChannelKind = "whatsapp"
	ChannelForm       ChannelKind = "form"
	ChannelAPI        ChannelKind = "api"
	ChannelPhone      ChannelKind = "phone"
	ChannelMonitoring ChannelKind = "monitoring"
)

// Direction marks a channel message as inbound (customer to desk) or outbound.
type Direction string

const (
	DirInbound  Direction = "inbound"
	DirOutbound Direction = "outbound"
)

// DeliveryStatus is the lifecycle of an outbound/inbound delivery attempt.
type DeliveryStatus string

const (
	DeliveryPending  DeliveryStatus = "pending"
	DeliveryReceived DeliveryStatus = "received"
	DeliverySent     DeliveryStatus = "sent"
	DeliveryFailed   DeliveryStatus = "failed"
	DeliverySkipped  DeliveryStatus = "skipped"
)

// CreationPolicy governs when an inbound channel message may mint a new ticket.
type CreationPolicy string

const (
	CreateStandard   CreationPolicy = "standard"
	CreateAll        CreationPolicy = "all"
	CreateRegistered CreationPolicy = "registered_only"
	CreateDuplicate  CreationPolicy = "link_duplicates"
)

// RoutingMethod decides how a matching rule assigns a ticket.
type RoutingMethod string

const (
	RouteTeam       RoutingMethod = "team"
	RouteUser       RoutingMethod = "user"
	RouteLeastLoad  RoutingMethod = "least_loaded"
	RouteRoundRobin RoutingMethod = "round_robin"
)

// MatchPriority / MatchChannel / MatchTicketType are routing-rule match scopes.
// Each carries an explicit "any" wildcard, following the helpdesk_sla
// SLAPriority precedent, so a rule can leave a dimension unconstrained without
// colliding with the anchor's closed enumerations.
type MatchPriority string

const (
	MPAny    MatchPriority = "any"
	MPLow    MatchPriority = "low"
	MPNormal MatchPriority = "normal"
	MPHigh   MatchPriority = "high"
	MPUrgent MatchPriority = "urgent"
)

type MatchChannel string

const (
	MCAny      MatchChannel = "any"
	MCEmail    MatchChannel = "email"
	MCPortal   MatchChannel = "portal"
	MCChat     MatchChannel = "chat"
	MCWhatsApp MatchChannel = "whatsapp"
	MCForm     MatchChannel = "form"
	MCAPI      MatchChannel = "api"
	MCPhone    MatchChannel = "phone"
	MCInternal MatchChannel = "internal"
)

type MatchTicketType string

const (
	MTAny            MatchTicketType = "any"
	MTIncident       MatchTicketType = "incident"
	MTServiceRequest MatchTicketType = "service_request"
	MTQuestion       MatchTicketType = "question"
	MTComplaint      MatchTicketType = "complaint"
	MTProblem        MatchTicketType = "problem"
	MTChange         MatchTicketType = "change_request"
	MTFeature        MatchTicketType = "feature_request"
	MTInternalIT     MatchTicketType = "internal_it"
)

// --- Models ---

// HelpdeskChannelConnection is a channel adapter: a named origin (mailbox, web
// form, chat widget, API key, monitoring feed) that normalises inbound events
// into the single helpdesk.ticket model rather than maintaining a per-channel
// ticketing silo. It owns the creation policy and duplicate window.
type HelpdeskChannelConnection struct {
	sdk.Model `nexa:"model=helpdesk.channel.connection"`

	Name            sdk.String                             `nexa:"required,string=Name"`
	Code            sdk.String                             `nexa:"size=60,string=Code"`
	Kind            sdk.Selection[ChannelKind]             `nexa:"string=Channel,default=email"`
	Active          sdk.Boolean                            `nexa:"string=Active,default=true"`
	TeamID          sdk.Many2One[HelpdeskTeam]             `nexa:"string=Default Team"`
	Priority        sdk.Selection[hdmodels.TicketPriority] `nexa:"string=Default Priority,default=normal"`
	CreationPolicy  sdk.Selection[CreationPolicy]          `nexa:"string=Creation Policy,default=standard"`
	DuplicateWindow sdk.Integer                            `nexa:"string=Duplicate Window (min),default=30"`
	MaxRetries      sdk.Integer                            `nexa:"string=Max Delivery Retries,default=3"`
	Notes           sdk.Text                               `nexa:"string=Notes"`
	CompanyID       sdk.Many2One[CoreCompany]              `nexa:"string=Company"`

	// Non-stored live aggregates.
	ChannelCount sdk.Integer `nexa:"string=# Messages,compute=compute_connection_channel_count"`
	TicketCount  sdk.Integer `nexa:"string=# Tickets,compute=compute_connection_ticket_count"`
}

// HelpdeskChannelMessage is the canonical, channel-normalised message record. It
// preserves the external message id, thread correlation key and delivery result
// so a conversation can be unified onto one ticket while retaining its origin.
type HelpdeskChannelMessage struct {
	sdk.Model `nexa:"model=helpdesk.channel.message"`

	ConnectionID   sdk.Many2One[HelpdeskChannelConnection] `nexa:"required,index,string=Connection"`
	TicketID       sdk.Many2One[HelpdeskTicket]            `nexa:"string=Ticket"`
	Direction      sdk.Selection[Direction]                `nexa:"string=Direction,default=inbound"`
	Kind           sdk.Selection[ChannelKind]              `nexa:"string=Channel,default=email"`
	ExternalID     sdk.String                              `nexa:"size=200,string=External Message ID,index"`
	CorrelationKey sdk.String                              `nexa:"size=200,string=Correlation Key,index"`
	FromAddr       sdk.String                              `nexa:"size=200,string=From"`
	ToAddr         sdk.String                              `nexa:"size=200,string=To"`
	Subject        sdk.String                              `nexa:"string=Subject"`
	Body           sdk.Text                                `nexa:"string=Body"`
	DeliveryStatus sdk.Selection[DeliveryStatus]           `nexa:"string=Delivery,default=pending"`
	RetryCount     sdk.Integer                             `nexa:"string=Retries,default=0"`
	IsDuplicate    sdk.Boolean                             `nexa:"string=Duplicate,default=false"`
	ReceivedAt     sdk.DateTime                            `nexa:"string=Received"`
	SentAt         sdk.DateTime                            `nexa:"string=Sent"`
	LastError      sdk.String                              `nexa:"size=300,string=Last Error"`
}

// HelpdeskSavedView is a personal or shared saved filter over a model (tickets by
// default). The filter is stored as a domain expression; usage counters are live.
type HelpdeskSavedView struct {
	sdk.Model `nexa:"model=helpdesk.saved.view"`

	Name        sdk.String                 `nexa:"required,string=Name"`
	Code        sdk.String                 `nexa:"size=60,string=Code"`
	TargetModel sdk.String                 `nexa:"size=80,string=Model,default=helpdesk.ticket"`
	UserID      sdk.Many2One[CoreUser]     `nexa:"string=Owner"`
	TeamID      sdk.Many2One[HelpdeskTeam] `nexa:"string=Team"`
	FilterJSON  sdk.Text                   `nexa:"string=Filter Domain"`
	GroupBy     sdk.String                 `nexa:"size=80,string=Group By"`
	Shared      sdk.Boolean                `nexa:"string=Shared,default=false"`
	IsDefault   sdk.Boolean                `nexa:"string=Default View,default=false"`
	Sequence    sdk.Integer                `nexa:"string=Sequence,default=10"`
	Active      sdk.Boolean                `nexa:"string=Active,default=true"`
	UsageCount  sdk.Integer                `nexa:"string=# Uses,default=0"`
}

// HelpdeskRoutingRule is an ordered routing rule. Blank match fields act as
// wildcards; the first active rule (lowest sequence) that matches a ticket
// decides the destination team/agent and may raise priority.
type HelpdeskRoutingRule struct {
	sdk.Model `nexa:"model=helpdesk.routing.rule"`

	Name            sdk.String                                    `nexa:"required,string=Name"`
	Active          sdk.Boolean                                   `nexa:"string=Active,default=true"`
	Sequence        sdk.Integer                                   `nexa:"string=Priority Order,default=10"`
	MatchPriority   sdk.Selection[MatchPriority]                  `nexa:"string=Match Priority,default=any"`
	MatchChannel    sdk.Selection[MatchChannel]                   `nexa:"string=Match Channel,default=any"`
	MatchTicketType sdk.Selection[MatchTicketType]                `nexa:"string=Match Type,default=any"`
	CategoryID      sdk.Many2One[hdmodels.HelpdeskTicketCategory] `nexa:"string=Match Category"`
	Keyword         sdk.String                                    `nexa:"size=120,string=Keyword in Subject"`
	Method          sdk.Selection[RoutingMethod]                  `nexa:"string=Assignment,default=team"`
	TeamID          sdk.Many2One[HelpdeskTeam]                    `nexa:"string=Assign Team"`
	UserID          sdk.Many2One[CoreUser]                        `nexa:"string=Assign Agent"`
	RaisePriority   sdk.Boolean                                   `nexa:"string=Raise Priority,default=false"`
	NewPriority     sdk.Selection[hdmodels.TicketPriority]        `nexa:"string=New Priority,default=high"`
	Notes           sdk.Text                                      `nexa:"string=Notes"`
	HitsCount       sdk.Integer                                   `nexa:"string=# Matches,default=0"`
}

// HelpdeskAgentLoad records an agent's capacity so routing can prefer the
// least-loaded available agent. Current load is computed live from open tickets.
type HelpdeskAgentLoad struct {
	sdk.Model `nexa:"model=helpdesk.agent.load"`

	UserID    sdk.Many2One[CoreUser]     `nexa:"required,string=Agent,index"`
	TeamID    sdk.Many2One[HelpdeskTeam] `nexa:"string=Team"`
	MaxOpen   sdk.Integer                `nexa:"string=Max Open Tickets,default=25"`
	Available sdk.Boolean                `nexa:"string=Available,default=true"`
	Notes     sdk.Text                   `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany]  `nexa:"string=Company"`

	CurrentOpen sdk.Integer `nexa:"string=Current Open,compute=compute_agent_current_open"`
}

func init() {
	sdk.MustRegister("helpdesk_channel",
		&HelpdeskChannelConnection{},
		&HelpdeskChannelMessage{},
		&HelpdeskSavedView{},
		&HelpdeskRoutingRule{},
		&HelpdeskAgentLoad{},
	)
}
