package wizard

import (
	"nexa/core/sdk"
	"nexa_addons/crm/models"
)

type CrmLead2OpportunityMass struct {
	sdk.Model `nexa:"model=crm.lead2opportunity.mass"`

	LeadIDs   sdk.String                     `nexa:"required,string=Lead IDs"`
	UserID    sdk.Many2One[models.CoreUser]  `nexa:"string=Salesperson"`
	TeamID    sdk.Many2One[models.CrmTeam]   `nexa:"string=Sales Team"`
	ForceAssign sdk.Boolean                  `nexa:"string=Force Assignment,default=false"`
}
