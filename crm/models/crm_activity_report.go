package models

import (
	"nexa/core/sdk"
)

type CrmActivityReport struct {
	sdk.Model `nexa:"model=crm.activity.report"`

	ActivityID sdk.Many2One[MailActivity] `nexa:"string=Activity"`
	LeadID     sdk.Many2One[CrmLead]      `nexa:"string=Lead"`
	UserID     sdk.Many2One[CoreUser]       `nexa:"string=Assigned To"`
	TeamID     sdk.Many2One[CrmTeam]        `nexa:"string=Sales Team"`
	StageID    sdk.Many2One[CrmStage]       `nexa:"string=Stage"`
	Summary    sdk.String                   `nexa:"string=Summary"`
	DateDeadline sdk.Date                   `nexa:"string=Due Date"`
	State      sdk.String                   `nexa:"string=Status"`
}
