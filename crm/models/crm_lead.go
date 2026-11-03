package models

import (
	"nexa/core/sdk"
)

type CrmLead struct {
	sdk.Model `nexa:"model=crm.lead"`

	Name                   sdk.String                       `nexa:"required,string=Opportunity"`
	UserID                 sdk.Many2One[CoreUser]           `nexa:"string=Salesperson"`
	TeamID                 sdk.Many2One[CrmTeam]            `nexa:"string=Sales Team"`
	StageID                sdk.Many2One[CrmStage]           `nexa:"string=Stage"`
	PartnerID              sdk.Many2One[CorePartner]        `nexa:"string=Customer"`
	Type                   sdk.Selection[LeadType]          `nexa:"string=Type,default=lead"`
	Priority               sdk.Selection[LeadPriority]      `nexa:"string=Priority,default=1"`
	ExpectedRevenue        sdk.Numeric                      `nexa:"string=Expected Revenue,precision=18,scale=2"`
	Probability            sdk.Float                        `nexa:"string=Probability (%)"`
	AutomatedProbability   sdk.Float                        `nexa:"string=Automated Probability"`
	IsAutomatedProbability sdk.Boolean                    `nexa:"string=Auto Probability,default=true"`
	RecurringRevenue       sdk.Numeric                      `nexa:"string=Recurring Revenue,precision=18,scale=2"`
	RecurringPlan          sdk.Many2One[CrmRecurringPlan]   `nexa:"string=Recurring Plan"`
	ProratedRevenue        sdk.Numeric                      `nexa:"string=Prorated Revenue,precision=18,scale=2"`
	LostReasonID           sdk.Many2One[CrmLostReason]      `nexa:"string=Lost Reason"`
	LostFeedback           sdk.Text                         `nexa:"string=Closing Note"`
	WonStatus              sdk.Selection[WonStatus]         `nexa:"string=Won Status,default=pending"`
	DateDeadline           sdk.Date                         `nexa:"string=Expected Closing"`
	DateClosed             sdk.Date                         `nexa:"string=Closed Date"`
	DateOpen               sdk.Date                         `nexa:"string=Assignment Date"`
	DateLastStageUpdate    sdk.DateTime                     `nexa:"string=Last Stage Update"`
	Description            sdk.Text                         `nexa:"string=Notes"`
	Active                 sdk.Boolean                      `nexa:"string=Active,default=true"`
	ContactName            sdk.String                       `nexa:"string=Contact Name"`
	PartnerName            sdk.String                       `nexa:"string=Company Name"`
	EmailFrom              sdk.String                       `nexa:"string=Email"`
	Phone                  sdk.String                       `nexa:"string=Phone"`
	Website                sdk.String                       `nexa:"string=Website"`
	Street                 sdk.String                       `nexa:"string=Street"`
	Street2                sdk.String                       `nexa:"string=Street 2"`
	Zip                    sdk.String                       `nexa:"string=Zip"`
	City                   sdk.String                       `nexa:"string=City"`
	Color                  sdk.Integer                      `nexa:"string=Color Index,default=0"`
	TagIDs                 sdk.Many2Many[CrmTag]            `nexa:"string=Tags,table=crm_lead_tag_rel,left=lead_id,right=tag_id"`
	CampaignID             sdk.Many2One[UtmCampaign]        `nexa:"string=Campaign"`
	MediumID               sdk.Many2One[UtmMedium]          `nexa:"string=Medium"`
	SourceID               sdk.Many2One[UtmSource]          `nexa:"string=Source"`
	Referred               sdk.String                       `nexa:"string=Referred By"`
	CompanyID              sdk.Many2One[CoreCompany]        `nexa:"string=Company"`
	LangID                 sdk.Many2One[CoreLang]           `nexa:"string=Language"`
	StateID                sdk.Many2One[CoreCountryState]   `nexa:"string=State"`
	CountryID              sdk.Many2One[CoreCountry]        `nexa:"string=Country"`
	Function               sdk.String                       `nexa:"string=Job Position"`
	Mobile                 sdk.String                       `nexa:"string=Mobile"`
	RecurringRevenueMonthly sdk.Numeric                     `nexa:"string=Monthly Recurring Revenue,precision=18,scale=2"`
	CompanyCurrencyID      sdk.Many2One[CoreCurrency]       `nexa:"string=Company Currency"`
	DateConversion         sdk.Date                         `nexa:"string=Conversion Date"`
	DateAutomationLast     sdk.DateTime                     `nexa:"string=Last Automation"`
	DayOpen                sdk.Integer                      `nexa:"string=Days to Assign"`
	DayClose               sdk.Integer                      `nexa:"string=Days to Close"`
	DuplicateLeadCount     sdk.Integer                      `nexa:"string=Duplicate Count,default=0"`
	DuplicateLeadIDs       sdk.String                       `nexa:"string=Duplicate Lead IDs"`
	MeetingCount           sdk.Integer                      `nexa:"string=Meetings,default=0"`
	QuotationCount         sdk.Integer                      `nexa:"string=Quotations,default=0"`
	LeadProperties         sdk.Text                         `nexa:"string=Properties"`
}
