package models

import (
	"nexa/core/sdk"
	crmodels "nexa_addons/crm/models"
)

// Account/Customer 360 layer. Canonical person & organization identity lives in
// the shared contacts master (core.partner); these models add CRM-specific
// relationship, segmentation, health and retention data on top without
// duplicating the partner record. Financial figures are never copied here.

type (
	CorePartner = crmodels.CorePartner
	CoreUser    = crmodels.CoreUser
	CoreCompany = crmodels.CoreCompany
)

type AccountSize string

const (
	AcctSizeSmall      AccountSize = "small"
	AcctSizeMedium     AccountSize = "medium"
	AcctSizeLarge      AccountSize = "large"
	AcctSizeEnterprise AccountSize = "enterprise"
)

type AccountLifecycle string

const (
	LifecycleProspect AccountLifecycle = "prospect"
	LifecycleActive   AccountLifecycle = "active"
	LifecycleAtRisk   AccountLifecycle = "at_risk"
	LifecycleChurned  AccountLifecycle = "churned"
	LifecycleFormer   AccountLifecycle = "former"
)

type SegmentCriteria string

const (
	CriteriaIndustry  SegmentCriteria = "industry"
	CriteriaRegion    SegmentCriteria = "region"
	CriteriaSize      SegmentCriteria = "size"
	CriteriaLifecycle SegmentCriteria = "lifecycle"
	CriteriaHealth    SegmentCriteria = "health"
)

type RenewalStatus string

const (
	RenewalUpcoming  RenewalStatus = "upcoming"
	RenewalRefreshed RenewalStatus = "refreshed"
	RenewalExpanded  RenewalStatus = "expanded"
	RenewalRenewed   RenewalStatus = "renewed"
	RenewalLapsed    RenewalStatus = "lapsed"
)

// CrmAccountProfile is the CRM account view of a partner: segment, industry,
// region, owning account manager, lifecycle and an explainable health score.
type CrmAccountProfile struct {
	sdk.Model `nexa:"model=crm.account.profile"`

	Name             sdk.String                      `nexa:"required,size=120,string=Reference"`
	PartnerID        sdk.Many2One[CorePartner]       `nexa:"required,index,string=Account"`
	Code             sdk.String                      `nexa:"size=60,string=Account Code"`
	Segment          sdk.String                      `nexa:"size=120,string=Segment"`
	Industry         sdk.String                      `nexa:"size=120,string=Industry"`
	Size             sdk.Selection[AccountSize]      `nexa:"string=Company Size,default=medium"`
	Region           sdk.String                      `nexa:"size=120,string=Region"`
	Website          sdk.String                      `nexa:"size=200,string=Website"`
	AccountManagerID sdk.Many2One[CoreUser]          `nexa:"string=Account Manager"`
	Lifecycle        sdk.Selection[AccountLifecycle] `nexa:"string=Lifecycle,default=prospect"`
	HealthScore      sdk.Integer                     `nexa:"string=Health Score (0-100),default=0"`
	HealthNotes      sdk.Text                        `nexa:"string=Health Rationale"`
	TagLinkIDs       sdk.One2Many[CrmAccountTagLink] `nexa:"string=Tags"`
	CompanyID        sdk.Many2One[CoreCompany]       `nexa:"string=Company"`
	Notes            sdk.Text                        `nexa:"string=Notes"`
	Active           sdk.Boolean                     `nexa:"string=Active,default=true"`
}

// CrmAccountTag is a reusable classification label for accounts.
type CrmAccountTag struct {
	sdk.Model `nexa:"model=crm.account.tag"`

	Name   sdk.String  `nexa:"required,size=120,string=Name"`
	Color  sdk.Integer `nexa:"string=Color Index,default=0"`
	Active sdk.Boolean `nexa:"string=Active,default=true"`
}

// CrmAccountTagLink joins an account profile to a tag (many-to-many body).
type CrmAccountTagLink struct {
	sdk.Model `nexa:"model=crm.account.tag.link"`

	ProfileID sdk.Many2One[CrmAccountProfile] `nexa:"required,index,string=Account"`
	TagID     sdk.Many2One[CrmAccountTag]     `nexa:"required,string=Tag"`
	CompanyID sdk.Many2One[CoreCompany]       `nexa:"string=Company"`
}

// CrmAccountSegment is a saved, possibly dynamic, filter grouping accounts by
// a shared criterion (industry, region, size, lifecycle, health).
type CrmAccountSegment struct {
	sdk.Model `nexa:"model=crm.account.segment"`

	Name          sdk.String                     `nexa:"required,size=120,string=Name"`
	CriteriaType  sdk.Selection[SegmentCriteria] `nexa:"string=Criteria,default=industry"`
	CriteriaValue sdk.String                     `nexa:"size=200,string=Criteria Value"`
	MemberCount   sdk.Integer                    `nexa:"string=Members,default=0"`
	Dynamic       sdk.Boolean                    `nexa:"string=Dynamic,default=true"`
	OwnerID       sdk.Many2One[CoreUser]         `nexa:"string=Owned By"`
	CompanyID     sdk.Many2One[CoreCompany]      `nexa:"string=Company"`
	Active        sdk.Boolean                    `nexa:"string=Active,default=true"`
}

// CrmAccountRenewal tracks a renewal/retention opportunity on an account. The
// actual contract/subscription lifecycle is owned by Subscription/Sales; this
// records the CRM renewal pursuit only.
type CrmAccountRenewal struct {
	sdk.Model `nexa:"model=crm.account.renewal"`

	Name          sdk.String                      `nexa:"required,size=120,string=Reference"`
	ProfileID     sdk.Many2One[CrmAccountProfile] `nexa:"required,index,string=Account"`
	RenewalDate   sdk.Date                        `nexa:"string=Renewal Date"`
	ContractValue sdk.Numeric                     `nexa:"string=Contract Value,precision=18,scale=2,default=0"`
	Probability   sdk.Float                       `nexa:"string=Renewal Probability (%)"`
	Status        sdk.Selection[RenewalStatus]    `nexa:"string=Status,default=upcoming"`
	OwnerID       sdk.Many2One[CoreUser]          `nexa:"string=Responsible"`
	CompanyID     sdk.Many2One[CoreCompany]       `nexa:"string=Company"`
	Notes         sdk.Text                        `nexa:"string=Notes"`
	Active        sdk.Boolean                     `nexa:"string=Active,default=true"`
}

func init() {
	sdk.MustRegister("crm_account",
		&CrmAccountProfile{},
		&CrmAccountTag{},
		&CrmAccountTagLink{},
		&CrmAccountSegment{},
		&CrmAccountRenewal{},
	)
}
