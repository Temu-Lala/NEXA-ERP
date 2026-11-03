package models

import (
	"nexa/core/sdk"
)

// ResConfigSettings extends platform settings with CRM options.
type ResConfigSettings struct {
	sdk.Model `nexa:"inherit=res.config.settings"`

	GroupUseLead               sdk.Boolean `nexa:"string=Leads"`
	GroupUseRecurringRevenues  sdk.Boolean `nexa:"string=Recurring Revenues"`
	IsMembershipMulti          sdk.Boolean `nexa:"string=Multi Teams"`
	CrmUseAutoAssignment       sdk.Boolean `nexa:"string=Rule-Based Assignment"`
	CrmAutoAssignmentAction    sdk.Selection[string] `nexa:"string=Assignment Mode,selection=manual:Manually,auto:Repeatedly,default=manual"`
	CrmAutoAssignmentIntervalNumber sdk.Integer `nexa:"string=Repeat Every,default=1"`
	CrmAutoAssignmentIntervalType   sdk.Selection[string] `nexa:"string=Interval Unit,selection=minutes:Minutes,hours:Hours,days:Days,weeks:Weeks,default=days"`
	CrmAutoAssignmentRunDatetime    sdk.DateTime `nexa:"string=Next Run"`
	LeadEnrichAuto             sdk.Selection[string] `nexa:"string=Lead Enrichment,selection=manual:On demand,auto:Automatic,default=auto"`
	LeadMiningInPipeline       sdk.Boolean `nexa:"string=Lead Mining in Pipeline"`
	ModuleCrmIapEnrich         sdk.Boolean `nexa:"string=Lead Enrichment Module"`
	ModuleCrmIapMine           sdk.Boolean `nexa:"string=Lead Mining Module"`
	ModuleWebsiteCrmReveal     sdk.Boolean `nexa:"string=Visits to Leads Module"`
	PredictiveLeadScoringStartDate sdk.Date `nexa:"string=PLS Start Date"`
	PredictiveLeadScoringFieldsStr sdk.String `nexa:"string=PLS Fields"`
	PredictiveLeadScoringFieldLabels sdk.String `nexa:"string=PLS Field Labels,readonly=1"`
}
