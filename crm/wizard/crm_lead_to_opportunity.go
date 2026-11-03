package wizard

import (
	"nexa/core/sdk"
	"nexa_addons/crm/models"
)

type CrmLead2Opportunity struct {
	sdk.Model `nexa:"model=crm.lead2opportunity"`

	LeadID    sdk.Many2One[models.CrmLead]   `nexa:"required,string=Lead"`
	PartnerID sdk.Many2One[models.CorePartner] `nexa:"string=Customer"`
	Name      sdk.String                     `nexa:"string=Opportunity Name"`
	UserID    sdk.Many2One[models.CoreUser]  `nexa:"string=Salesperson"`
	TeamID    sdk.Many2One[models.CrmTeam]   `nexa:"string=Sales Team"`
}
