package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	hdmodels "nexa_addons/helpdesk/models"
)

// Cross-module reference aliases. The SLA companion links to the helpdesk anchor
// by Many2One and never replaces it, mirroring the crm_lifecycle / lending_* pattern.
type (
	HelpdeskTicket         = hdmodels.HelpdeskTicket
	HelpdeskTicketCategory = hdmodels.HelpdeskTicketCategory
	HelpdeskTeam           = hdmodels.HelpdeskTeam
	HelpdeskTicketMessage  = hdmodels.HelpdeskTicketMessage
	HelpdeskTicketHistory  = hdmodels.HelpdeskTicketHistory
	CoreUser               = basemodels.CoreUser
	CoreCompany            = basemodels.CoreCompany
)

type SLAType string

const (
	SLATypeFirstResponse SLAType = "first_response"
	SLATypeResolution    SLAType = "resolution"
)

type SLABreachState string

const (
	BreachStateOnTrack  SLABreachState = "on_track"
	BreachStateWarning  SLABreachState = "warning"
	BreachStateBreached SLABreachState = "breached"
	BreachStateMet      SLABreachState = "met"
	BreachStatePaused   SLABreachState = "paused"
)

type EscalationAction string

const (
	EscalationNotify         EscalationAction = "notify"
	EscalationReassign       EscalationAction = "reassign"
	EscalationNotifyEscalate EscalationAction = "notify_and_escalate"
)

// SLAPriority is the ticket-priority scope of a policy. "any" matches every
// priority, so a single catch-all policy can coexist with priority-specific ones.
type SLAPriority string

const (
	SLAPriorityAny    SLAPriority = "any"
	SLAPriorityLow    SLAPriority = "low"
	SLAPriorityNormal SLAPriority = "normal"
	SLAPriorityHigh   SLAPriority = "high"
	SLAPriorityUrgent SLAPriority = "urgent"
)

// SLAEventType classifies the append-only SLA audit trail entries.
type SLAEventType string

const (
	SLAEventCreated   SLAEventType = "created"
	SLAEventStarted   SLAEventType = "started"
	SLAEventPaused    SLAEventType = "paused"
	SLAEventResumed   SLAEventType = "resumed"
	SLAEventWarning   SLAEventType = "warning"
	SLAEventBreached  SLAEventType = "breached"
	SLAEventMet       SLAEventType = "met"
	SLAEventEscalated SLAEventType = "escalated"
)

// Weekday enumerates the days a business calendar can schedule working hours for.
type Weekday string

const (
	WeekdayMonday    Weekday = "monday"
	WeekdayTuesday   Weekday = "tuesday"
	WeekdayWednesday Weekday = "wednesday"
	WeekdayThursday  Weekday = "thursday"
	WeekdayFriday    Weekday = "friday"
	WeekdaySaturday  Weekday = "saturday"
	WeekdaySunday    Weekday = "sunday"
)

// HelpdeskSLAPolicy defines response/resolution targets and the scope in which the
// policy applies (priority + optional category), the working calendar that governs
// the clock, and the warning threshold that triggers escalation.
type HelpdeskSLAPolicy struct {
	sdk.Model `nexa:"model=helpdesk.sla.policy"`

	Name              sdk.String                             `nexa:"required,string=Policy Name"`
	Priority          sdk.Selection[SLAPriority]             `nexa:"string=Priority Scope,default=any"`
	CategoryID        sdk.Many2One[HelpdeskTicketCategory]   `nexa:"string=Category Scope"`
	FirstResponseMins sdk.Integer                            `nexa:"string=First Response (min),default=60"`
	ResolutionMins    sdk.Integer                            `nexa:"string=Resolution (min),default=480"`
	WarningPct        sdk.Integer                            `nexa:"string=Warning Threshold (%),default=75"`
	BusinessHoursOnly sdk.Boolean                            `nexa:"string=Business Hours Only,default=true"`
	CalendarID        sdk.Many2One[HelpdeskBusinessCalendar] `nexa:"string=Working Calendar"`
	Sequence          sdk.Integer                            `nexa:"string=Priority Order,default=10"`
	Active            sdk.Boolean                            `nexa:"string=Active,default=true"`
}

// HelpdeskSLATimer tracks one SLA clock instance for a ticket. The due date is a
// snapshot taken at creation, so later policy edits never silently rewrite an
// running clock (implicit policy versioning). Breach evaluation is driven by the
// frozen elapsed-vs-target comparison in the engine, not by rewriting due.
type HelpdeskSLATimer struct {
	sdk.Model `nexa:"model=helpdesk.sla.timer"`

	TicketID     sdk.Many2One[HelpdeskTicket]    `nexa:"required,string=Ticket,index"`
	PolicyID     sdk.Many2One[HelpdeskSLAPolicy] `nexa:"required,string=SLA Policy"`
	SLAType      sdk.Selection[SLAType]          `nexa:"string=SLA Type,default=first_response"`
	Start        sdk.DateTime                    `nexa:"string=Start"`
	Due          sdk.DateTime                    `nexa:"string=Due"`
	End          sdk.DateTime                    `nexa:"string=End (Actual)"`
	BreachState  sdk.Selection[SLABreachState]   `nexa:"string=Status,default=on_track"`
	TargetMins   sdk.Integer                     `nexa:"string=Target (min),default=0"`
	ElapsedMins  sdk.Integer                     `nexa:"string=Elapsed (min),default=0"`
	BreachMins   sdk.Integer                     `nexa:"string=Breach (min),default=0"`
	Paused       sdk.Boolean                     `nexa:"string=Paused,default=false"`
	PauseStarted sdk.DateTime                    `nexa:"string=Paused Since"`
	PausedMins   sdk.Integer                     `nexa:"string=Paused (min),default=0"`
	Escalated    sdk.Boolean                     `nexa:"string=Escalated,default=false"`
	CompanyID    sdk.Many2One[CoreCompany]       `nexa:"string=Company"`

	Events sdk.One2Many[HelpdeskSLAEvent] `nexa:"string=SLA Events"`
}

// HelpdeskSLAEvent is the append-only audit trail of SLA clock activity: creation,
// pause/resume, warning, breach, escalation and final evaluation of a timer.
type HelpdeskSLAEvent struct {
	sdk.Model `nexa:"model=helpdesk.sla.event"`

	TicketID sdk.Many2One[HelpdeskTicket]   `nexa:"required,index,string=Ticket"`
	TimerID  sdk.Many2One[HelpdeskSLATimer] `nexa:"string=Timer,index"`
	Event    sdk.Selection[SLAEventType]    `nexa:"string=Event,default=created"`
	Message  sdk.String                     `nexa:"size=300,string=Message"`
	AuthorID sdk.Many2One[CoreUser]         `nexa:"string=Source"`
	Date     sdk.DateTime                   `nexa:"string=Date"`
}

// HelpdeskEscalationRule fires when an SLA threshold is crossed for its policy.
type HelpdeskEscalationRule struct {
	sdk.Model `nexa:"model=helpdesk.escalation.rule"`

	PolicyID     sdk.Many2One[HelpdeskSLAPolicy] `nexa:"required,string=SLA Policy"`
	TriggerPct   sdk.Integer                     `nexa:"string=Trigger at (%),default=75"`
	Action       sdk.Selection[EscalationAction] `nexa:"string=Action,default=notify"`
	NotifyTeamID sdk.Many2One[HelpdeskTeam]      `nexa:"string=Notify Team"`
	NotifyUserID sdk.Many2One[CoreUser]          `nexa:"string=Notify / Reassign To"`
	Sequence     sdk.Integer                     `nexa:"string=Sequence,default=10"`
	Active       sdk.Boolean                     `nexa:"string=Active,default=true"`
}

// HelpdeskBusinessCalendar is a named working-time schedule (weekly hours + closed
// days) with an IANA timezone. SLA policies reference it to compute business-hour
// deadlines; policies without a calendar fall back to wall-clock time.
type HelpdeskBusinessCalendar struct {
	sdk.Model `nexa:"model=helpdesk.business.calendar"`

	Name      sdk.String                                    `nexa:"required,string=Calendar Name"`
	Timezone  sdk.String                                    `nexa:"size=60,string=Timezone,default=UTC"`
	CompanyID sdk.Many2One[CoreCompany]                     `nexa:"string=Company"`
	Active    sdk.Boolean                                   `nexa:"string=Active,default=true"`
	Lines     sdk.One2Many[HelpdeskBusinessCalendarLine]    `nexa:"string=Working Hours"`
	Holidays  sdk.One2Many[HelpdeskBusinessCalendarHoliday] `nexa:"string=Holidays"`
	Notes     sdk.Text                                      `nexa:"string=Notes"`
}

// HelpdeskBusinessCalendarLine defines the working window(s) for a weekday. A day
// with no line is a non-working day; multiple lines model split shifts / lunches.
type HelpdeskBusinessCalendarLine struct {
	sdk.Model `nexa:"model=helpdesk.business.calendar.line"`

	CalendarID sdk.Many2One[HelpdeskBusinessCalendar] `nexa:"required,index,string=Calendar"`
	DayOfWeek  sdk.Selection[Weekday]                 `nexa:"required,string=Day"`
	HourFrom   sdk.Integer                            `nexa:"string=From Hour,default=9"`
	HourTo     sdk.Integer                            `nexa:"string=To Hour,default=17"`
}

// HelpdeskBusinessCalendarHoliday closes a calendar range regardless of weekday.
type HelpdeskBusinessCalendarHoliday struct {
	sdk.Model `nexa:"model=helpdesk.business.calendar.holiday"`

	CalendarID sdk.Many2One[HelpdeskBusinessCalendar] `nexa:"required,index,string=Calendar"`
	Name       sdk.String                             `nexa:"required,string=Name"`
	Date       sdk.Date                               `nexa:"required,string=Date"`
	DateTo     sdk.Date                               `nexa:"string=End Date"`
}

func init() {
	sdk.MustRegister("helpdesk_sla",
		&HelpdeskBusinessCalendar{},
		&HelpdeskBusinessCalendarLine{},
		&HelpdeskBusinessCalendarHoliday{},
		&HelpdeskSLAPolicy{},
		&HelpdeskSLATimer{},
		&HelpdeskSLAEvent{},
		&HelpdeskEscalationRule{},
	)
}
