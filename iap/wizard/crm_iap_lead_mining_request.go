package wizard

import (
	"nexa/core/sdk"
)

type CrmIapLeadMiningRequest struct {
	sdk.Model `nexa:"model=crm.iap.lead.mining.request"`

	CountryID sdk.Many2One[sdk.Any] `nexa:"comodel=core.country,string=Country"`
	Industry  sdk.String            `nexa:"string=Industry"`
	Size      sdk.Selection[string] `nexa:"string=Company Size,selection=1-10:1-10,11-50:11-50,51-200:51-200,200+:200+,default=11-50"`
}
