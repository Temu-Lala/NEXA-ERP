package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// CorePartner → core.partner
type CorePartner = basemodels.CorePartner

type EventState string

const (
	EventStateDraft     EventState = "draft"
	EventStateConfirmed EventState = "confirmed"
	EventStateDone      EventState = "done"
	EventStateCancelled EventState = "cancelled"
)

type RegistrationState string

const (
	RegistrationStatePending   RegistrationState = "pending"
	RegistrationStateConfirmed RegistrationState = "confirmed"
	RegistrationStateAttended  RegistrationState = "attended"
	RegistrationStateCancelled RegistrationState = "cancelled"
)

type EventEvent struct {
	sdk.Model `nexa:"model=event.event"`

	Name        sdk.String                `nexa:"required,string=Event Name"`
	OrganizerID sdk.Many2One[CorePartner] `nexa:"string=Organizer"`
	DateStart   sdk.DateTime              `nexa:"string=Start"`
	DateEnd     sdk.DateTime              `nexa:"string=End"`
	Venue       sdk.String                `nexa:"string=Venue"`
	Capacity    sdk.Integer               `nexa:"string=Capacity,default=0"`
	State       sdk.Selection[EventState] `nexa:"string=Status,default=draft"`
	Description sdk.Text                  `nexa:"string=Description"`
}

type EventRegistration struct {
	sdk.Model `nexa:"model=event.registration"`

	Name      sdk.String                       `nexa:"required,string=Attendee"`
	EventID   sdk.Many2One[EventEvent]         `nexa:"required,string=Event"`
	PartnerID sdk.Many2One[CorePartner]        `nexa:"string=Contact"`
	Email     sdk.Email                        `nexa:"string=Email"`
	State     sdk.Selection[RegistrationState] `nexa:"string=Status,default=pending"`
}

func init() {
	sdk.MustRegister("event",
		&EventEvent{},
		&EventRegistration{},
	)
}
