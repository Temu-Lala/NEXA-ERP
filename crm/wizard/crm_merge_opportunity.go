package wizard

import (
	"nexa/core/sdk"
)

type CrmMergeOpportunity struct {
	sdk.Model `nexa:"model=crm.merge.opportunity"`

	LeadIds sdk.String `nexa:"string=Lead IDs"`
	Name    sdk.String `nexa:"string=Merged Name"`
}
