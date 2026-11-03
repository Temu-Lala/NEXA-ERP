package models

import (
	"nexa/core/sdk"
)

type CrmTeam struct {
	sdk.Model `nexa:"model=crm.team"`

	Name              sdk.String                `nexa:"required,string=Sales Team"`
	Active            sdk.Boolean               `nexa:"string=Active,default=true"`
	Sequence          sdk.Integer               `nexa:"string=Sequence,default=10"`
	UserID            sdk.Many2One[CoreUser]    `nexa:"string=Team Leader"`
	CompanyID         sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	UseLeads          sdk.Boolean               `nexa:"string=Leads,default=true"`
	UseOpportunities  sdk.Boolean               `nexa:"string=Pipeline,default=true"`
	LeadIDs           sdk.One2Many[CrmLead]     `nexa:"string=Leads"`
	MemberIDs         sdk.One2Many[CrmTeamMember] `nexa:"string=Members"`
	AssignmentEnabled sdk.Boolean               `nexa:"string=Lead Assignment,default=false"`
	AssignmentMax     sdk.Integer               `nexa:"string=Max Leads / Member,default=30"`
	AssignmentDomain           sdk.String  `nexa:"string=Assignment Domain"`
	LeadPropertiesDefinition   sdk.Text    `nexa:"string=Lead Properties Definition"`
	InvoicingTarget            sdk.Numeric `nexa:"string=Invoicing Target,precision=18,scale=2"`
	QuotationTarget            sdk.Numeric `nexa:"string=Quotation Target,precision=18,scale=2"`
}
