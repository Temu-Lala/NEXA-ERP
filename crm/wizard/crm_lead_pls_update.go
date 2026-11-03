package wizard

import (
	"nexa/core/sdk"
)

type CrmLeadPlsUpdate struct {
	sdk.Model `nexa:"model=crm.lead.pls.update"`

	StartDate sdk.Date   `nexa:"string=Start Date"`
	FieldList sdk.String `nexa:"string=Fields"`
}
