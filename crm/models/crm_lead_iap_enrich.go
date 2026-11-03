package models

import (
	"nexa/core/sdk"
)

// CrmLeadIapEnrich extends crm.lead with IAP enrichment tracking.
type CrmLeadIapEnrich struct {
	sdk.Model `nexa:"inherit=crm.lead"`

	IapEnrichDone sdk.Boolean `nexa:"string=IAP Enrichment Done,default=false"`
}
