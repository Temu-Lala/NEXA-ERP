package models

import (
	"nexa/core/sdk"
)

type CrmLeadScoringFrequency struct {
	sdk.Model `nexa:"model=crm.lead.scoring.frequency"`

	Variable sdk.String  `nexa:"required,string=Variable"`
	WonCount sdk.Integer `nexa:"string=Won Count,default=0"`
	LostCount sdk.Integer `nexa:"string=Lost Count,default=0"`
}

type CrmLeadScoringFrequencyField struct {
	sdk.Model `nexa:"model=crm.lead.scoring.frequency.field"`

	FrequencyID sdk.Many2One[CrmLeadScoringFrequency] `nexa:"required,index,string=Frequency"`
	Field       sdk.String                            `nexa:"required,string=Field"`
	Value       sdk.String                            `nexa:"required,string=Value"`
	WonCount    sdk.Integer                           `nexa:"string=Won Count,default=0"`
	LostCount   sdk.Integer                           `nexa:"string=Lost Count,default=0"`
}
