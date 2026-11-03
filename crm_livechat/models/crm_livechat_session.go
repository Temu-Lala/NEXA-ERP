package models

import (
	"nexa/core/sdk"
)

type CrmLivechatSession struct {
	sdk.Model `nexa:"model=crm.livechat.session"`

	Name       sdk.String                  `nexa:"required,string=Session"`
	LeadID     sdk.Many2One[any]           `nexa:"comodel=crm.lead,string=Lead"`
	OperatorID sdk.Many2One[CoreUser]      `nexa:"string=Operator"`
	State      sdk.Selection[LivechatState] `nexa:"string=State,default=new"`
}
