package wizard

import (
	"nexa/core/sdk"
	"nexa_addons/crm/models"
)

type CrmLeadLost struct {
	sdk.Model `nexa:"model=crm.lead.lost"`

	LeadID       sdk.Many2One[models.CrmLead]       `nexa:"required,string=Lead"`
	LostReasonID sdk.Many2One[models.CrmLostReason] `nexa:"string=Lost Reason"`
	LostFeedback sdk.Text                          `nexa:"string=Closing Note"`
}
