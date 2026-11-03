package models

import (
	"nexa/core/sdk"
)

type CrmOpportunityToQuotation struct {
	sdk.Model `nexa:"model=crm.opportunity.to.quotation"`

	LeadID    sdk.Many2One[CrmLead]       `nexa:"required,string=Opportunity"`
	PartnerID sdk.Many2One[CorePartner]   `nexa:"string=Customer"`
	ProductID sdk.Many2One[ProductProduct] `nexa:"string=Product"`
	Quantity  sdk.Numeric                 `nexa:"string=Quantity,precision=18,scale=2,default=1"`
	MarkWon   sdk.Boolean                 `nexa:"string=Mark Won,default=false"`
}
