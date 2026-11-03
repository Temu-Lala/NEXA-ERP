package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// CorePartner → core.partner
type CorePartner = basemodels.CorePartner

type SubscriptionState string

const (
	SubscriptionStateDraft     SubscriptionState = "draft"
	SubscriptionStateActive    SubscriptionState = "active"
	SubscriptionStateRenewed   SubscriptionState = "renewed"
	SubscriptionStateClosed    SubscriptionState = "closed"
	SubscriptionStateCancelled SubscriptionState = "cancelled"
)

type IntervalUnit string

const (
	IntervalUnitDay   IntervalUnit = "day"
	IntervalUnitWeek  IntervalUnit = "week"
	IntervalUnitMonth IntervalUnit = "month"
	IntervalUnitYear  IntervalUnit = "year"
)

type SubscriptionPlan struct {
	sdk.Model `nexa:"model=subscription.plan"`

	Name         sdk.String                  `nexa:"required,string=Plan Name"`
	Amount       sdk.Numeric                 `nexa:"string=Price,precision=18,scale=2"`
	Interval     sdk.Integer                 `nexa:"string=Interval,default=1"`
	IntervalUnit sdk.Selection[IntervalUnit] `nexa:"string=Unit,default=month"`
	Description  sdk.Text                    `nexa:"string=Description"`
	Active       sdk.Boolean                 `nexa:"string=Active,default=true"`
}

type SubscriptionSubscription struct {
	sdk.Model `nexa:"model=subscription.subscription"`

	Name      sdk.String                       `nexa:"required,string=Reference"`
	PartnerID sdk.Many2One[CorePartner]        `nexa:"required,string=Customer"`
	PlanID    sdk.Many2One[SubscriptionPlan]   `nexa:"required,string=Plan"`
	StartDate sdk.Date                         `nexa:"string=Start Date"`
	Amount    sdk.Numeric                      `nexa:"string=Recurring Amount,precision=18,scale=2"`
	State     sdk.Selection[SubscriptionState] `nexa:"string=Status,default=draft"`
}

func init() {
	sdk.MustRegister("subscription",
		&SubscriptionPlan{},
		&SubscriptionSubscription{},
	)
}
