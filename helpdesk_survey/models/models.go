package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	hdmodels "nexa_addons/helpdesk/models"
)

// CSAT companion for the Help Desk. It is deliberately separate from the generic
// `survey` addon (which has no response/scoring engine and is not ticket aware):
// this module models a ticket-resolution satisfaction programme — delivery on
// resolve, 1..N ratings with free-text feedback, per agent/team/category history,
// and low-rating alerts posted back onto the ticket's own message thread rather
// than into a duplicated messaging system.

type (
	HelpdeskTicket         = hdmodels.HelpdeskTicket
	HelpdeskTicketCategory = hdmodels.HelpdeskTicketCategory
	HelpdeskTeam           = hdmodels.HelpdeskTeam
	HelpdeskTicketMessage  = hdmodels.HelpdeskTicketMessage
	CoreUser               = basemodels.CoreUser
	CorePartner            = basemodels.CorePartner
	CoreCompany            = basemodels.CoreCompany
)

type ResponseState string

const (
	ResponseStatePending  ResponseState = "pending"
	ResponseStateSent     ResponseState = "sent"
	ResponseStateAnswered ResponseState = "answered"
	ResponseStateExpired  ResponseState = "expired"
)

// HelpdeskSurvey is a CSAT questionnaire template. Scale is the maximum rating
// (5 by default); LowRatingAt marks responses at or below it (and above zero) as
// unhappy so they can trigger an alert.
type HelpdeskSurvey struct {
	sdk.Model `nexa:"model=helpdesk.survey"`

	Name        sdk.String  `nexa:"required,string=Survey Name"`
	Question    sdk.String  `nexa:"string=Question,default=How would you rate the support you received?"`
	Scale       sdk.Integer `nexa:"string=Rating Scale,default=5"`
	LowRatingAt sdk.Integer `nexa:"string=Low Rating At,default=2"`
	Active      sdk.Boolean `nexa:"string=Active,default=true"`

	Responses sdk.One2Many[HelpdeskSurveyResponse] `nexa:"string=Responses"`

	AvgRating     sdk.Numeric `nexa:"string=Avg Rating,precision=4,scale=2,compute=compute_survey_avg_rating"`
	ResponseCount sdk.Integer `nexa:"string=Responses,compute=compute_survey_response_count"`
	Satisfaction  sdk.Numeric `nexa:"string=CSAT %,precision=4,scale=1,compute=compute_survey_satisfaction"`
}

// HelpdeskSurveyResponse is one invitation + (optional) reply, tied to a resolved
// ticket. Team / agent / category are snapshotted at delivery so satisfaction can
// be attributed even if the ticket is later reassigned or reclassified.
type HelpdeskSurveyResponse struct {
	sdk.Model `nexa:"model=helpdesk.survey.response"`

	SurveyID   sdk.Many2One[HelpdeskSurvey]         `nexa:"string=Survey,index"`
	TicketID   sdk.Many2One[HelpdeskTicket]         `nexa:"required,index,string=Ticket"`
	PartnerID  sdk.Many2One[CorePartner]            `nexa:"string=Respondent"`
	AgentID    sdk.Many2One[CoreUser]               `nexa:"string=Agent"`
	TeamID     sdk.Many2One[HelpdeskTeam]           `nexa:"string=Team"`
	CategoryID sdk.Many2One[HelpdeskTicketCategory] `nexa:"string=Category"`

	Rating     sdk.Integer                  `nexa:"string=Rating,default=0"`
	Comment    sdk.Text                     `nexa:"string=Feedback"`
	State      sdk.Selection[ResponseState] `nexa:"string=Status,default=pending"`
	SentAt     sdk.DateTime                 `nexa:"string=Sent At"`
	AnsweredAt sdk.DateTime                 `nexa:"string=Answered At"`
	Alerted    sdk.Boolean                  `nexa:"string=Alerted,default=false"`
	CompanyID  sdk.Many2One[CoreCompany]    `nexa:"string=Company"`

	LowRating sdk.Boolean `nexa:"string=Low Rating,compute=compute_response_low_rating"`
}

func init() {
	sdk.MustRegister("helpdesk_survey",
		&HelpdeskSurvey{},
		&HelpdeskSurveyResponse{},
	)
}
