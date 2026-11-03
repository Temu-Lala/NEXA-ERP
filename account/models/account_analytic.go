package models

import (
	"nexa/core/sdk"
)

type AccountAnalyticPlan struct {
	sdk.Model `nexa:"model=account.analytic.plan"`

	Name   sdk.String  `nexa:"required,string=Plan"`
	Active sdk.Boolean `nexa:"string=Active,default=true"`
}

type AccountAnalyticAccount struct {
	sdk.Model `nexa:"model=account.analytic.account"`

	Name   sdk.String                       `nexa:"required,string=Analytic Account"`
	Code   sdk.String                       `nexa:"string=Reference"`
	PlanID sdk.Many2One[AccountAnalyticPlan] `nexa:"string=Plan"`
	Active sdk.Boolean                      `nexa:"string=Active,default=true"`
}
