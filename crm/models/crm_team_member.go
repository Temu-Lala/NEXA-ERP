package models

import (
	"nexa/core/sdk"
)

type CrmTeamMember struct {
	sdk.Model `nexa:"model=crm.team.member"`

	TeamID           sdk.Many2One[CrmTeam]  `nexa:"required,string=Sales Team"`
	UserID           sdk.Many2One[CoreUser] `nexa:"required,string=Member"`
	Active           sdk.Boolean            `nexa:"string=Active,default=true"`
	AssignmentOptout sdk.Boolean            `nexa:"string=Skip Auto Assignment"`
	AssignmentMax    sdk.Integer            `nexa:"string=Max Leads,default=30"`
	LeadMonthCount   sdk.Integer            `nexa:"string=Leads This Month,default=0"`
}
