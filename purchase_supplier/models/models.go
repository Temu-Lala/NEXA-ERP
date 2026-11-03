package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// Supplier governance layers procurement identity onto the shared partner
// record (core.partner) and never duplicates it. Ownership boundaries: this
// module owns commercial identity, onboarding, documents, banks, qualification
// and risk; Inventory owns physical goods and Accounting owns payables.

type (
	CorePartner = basemodels.CorePartner
	CoreUser    = basemodels.CoreUser
	CoreCompany = basemodels.CoreCompany
)

// ---------------------------------------------------------------------------
// Selection vocabularies
// ---------------------------------------------------------------------------

type SupplierState string

const (
	SuppDraft        SupplierState = "draft"
	SuppSubmitted    SupplierState = "submitted"
	SuppUnderReview  SupplierState = "under_review"
	SuppDueDiligence SupplierState = "due_diligence"
	SuppApproved     SupplierState = "approved"
	SuppActive       SupplierState = "active"
	SuppRejected     SupplierState = "rejected"
	SuppSuspended    SupplierState = "suspended"
	SuppBlocked      SupplierState = "blocked"
	SuppExpired      SupplierState = "expired"
	SuppArchived     SupplierState = "archived"
)

type SupplierRisk string

const (
	RiskUnclassified SupplierRisk = "unclassified"
	RiskLow          SupplierRisk = "low"
	RiskMedium       SupplierRisk = "medium"
	RiskHigh         SupplierRisk = "high"
	RiskCritical     SupplierRisk = "critical"
)

type SiteKind string

const (
	SiteDelivery   SiteKind = "delivery"
	SiteBilling    SiteKind = "billing"
	SiteRemittance SiteKind = "remittance"
)

type DocState string

const (
	DocPending  DocState = "pending"
	DocAccepted DocState = "accepted"
	DocRejected DocState = "rejected"
	DocExpiring DocState = "expiring"
	DocExpired  DocState = "expired"
)

type QualState string

const (
	QualDraft   QualState = "draft"
	QualActive  QualState = "active"
	QualExpired QualState = "expired"
	QualRevoked QualState = "revoked"
)

type ChangeState string

const (
	ChgDraft     ChangeState = "draft"
	ChgSubmitted ChangeState = "submitted"
	ChgVerified  ChangeState = "verified"
	ChgApplied   ChangeState = "applied"
	ChgRejected  ChangeState = "rejected"
)

type BankState string

const (
	BankActive     BankState = "active"
	BankSuperseded BankState = "superseded"
)

// ---------------------------------------------------------------------------
// Models
// ---------------------------------------------------------------------------

// PurchaseSupplier is the authoritative procurement supplier record.
type PurchaseSupplier struct {
	sdk.Model `nexa:"model=purchase.supplier"`

	Name             sdk.String                                  `nexa:"required,index,size=60,string=Reference"`
	PartnerID        sdk.Many2One[CorePartner]                   `nexa:"required,index,string=Partner"`
	CompanyID        sdk.Many2One[CoreCompany]                   `nexa:"string=Company"`
	SupplierCode     sdk.String                                  `nexa:"index,size=60,string=Supplier Code"`
	LegalName        sdk.String                                  `nexa:"required,size=255,string=Legal Business Name"`
	TradingName      sdk.String                                  `nexa:"size=255,string=Trading Name"`
	BusinessType     sdk.String                                  `nexa:"size=120,string=Business Type"`
	RegNo            sdk.String                                  `nexa:"size=120,string=Registration Number"`
	TaxId            sdk.String                                  `nexa:"size=120,string=Tax Identification"`
	TaxStatus        sdk.String                                  `nexa:"size=60,string=Tax Registration Status"`
	Country          sdk.String                                  `nexa:"size=120,string=Country"`
	Region           sdk.String                                  `nexa:"size=120,string=Region"`
	City             sdk.String                                  `nexa:"size=120,string=City"`
	Website          sdk.String                                  `nexa:"size=200,string=Website"`
	Email            sdk.String                                  `nexa:"size=120,string=Email"`
	Phone            sdk.String                                  `nexa:"size=60,string=Telephone"`
	EmergencyContact sdk.String                                  `nexa:"size=200,string=Emergency Contact"`
	Industry         sdk.String                                  `nexa:"size=120,string=Industry"`
	Category         sdk.String                                  `nexa:"size=120,string=Goods/Services Category"`
	State            sdk.Selection[SupplierState]                `nexa:"index,string=Status,default=draft"`
	Risk             sdk.Selection[SupplierRisk]                 `nexa:"string=Risk Level,default=unclassified"`
	RiskScore        sdk.Numeric                                 `nexa:"string=Risk Score,precision=6,scale=2,default=0"`
	DueDiligence     sdk.Boolean                                 `nexa:"string=Due Diligence Complete,default=false"`
	Sensitive        sdk.Boolean                                 `nexa:"string=High Value / Critical,default=false"`
	OnboardDate      sdk.Date                                    `nexa:"string=Onboard Date"`
	NextReviewDate   sdk.Date                                    `nexa:"string=Next Revalidation"`
	DuplicateFlag    sdk.Boolean                                 `nexa:"string=Possible Duplicate,default=false"`
	DuplicateNote    sdk.String                                  `nexa:"size=300,string=Duplicate Note"`
	LastExpiryScan   sdk.DateTime                                `nexa:"string=Last Expiry Scan"`
	Notes            sdk.Text                                    `nexa:"string=Notes"`
	Active           sdk.Boolean                                 `nexa:"string=Active,default=true"`
	Sites            sdk.One2Many[PurchaseSupplierSite]          `nexa:"string=Sites"`
	Contacts         sdk.One2Many[PurchaseSupplierContact]       `nexa:"string=Contacts"`
	Items            sdk.One2Many[PurchaseSupplierItem]          `nexa:"string=Supplier Items"`
	Documents        sdk.One2Many[PurchaseSupplierDocument]      `nexa:"string=Documents"`
	BankAccounts     sdk.One2Many[PurchaseSupplierBankAccount]   `nexa:"string=Bank Accounts"`
	Qualifications   sdk.One2Many[PurchaseSupplierQualification] `nexa:"string=Qualifications"`
}

// PurchaseSupplierSite is a delivery, billing or remittance location.
type PurchaseSupplierSite struct {
	sdk.Model `nexa:"model=purchase.supplier.site"`

	SupplierID sdk.Many2One[PurchaseSupplier] `nexa:"required,index,string=Supplier"`
	Name       sdk.String                     `nexa:"required,size=120,string=Site Name"`
	Kind       sdk.Selection[SiteKind]        `nexa:"string=Site Type,default=delivery"`
	Address    sdk.Text                       `nexa:"string=Address"`
	City       sdk.String                     `nexa:"size=120,string=City"`
	Country    sdk.String                     `nexa:"size=120,string=Country"`
	Contact    sdk.String                     `nexa:"size=120,string=Site Contact"`
	Phone      sdk.String                     `nexa:"size=60,string=Phone"`
	Active     sdk.Boolean                    `nexa:"string=Active,default=true"`
	Note       sdk.Text                       `nexa:"string=Note"`
}

// PurchaseSupplierContact is an individual and their authorized role.
type PurchaseSupplierContact struct {
	sdk.Model `nexa:"model=purchase.supplier.contact"`

	SupplierID sdk.Many2One[PurchaseSupplier] `nexa:"required,index,string=Supplier"`
	Name       sdk.String                     `nexa:"required,size=120,string=Name"`
	Role       sdk.String                     `nexa:"size=120,string=Authorized Role"`
	Email      sdk.String                     `nexa:"size=120,string=Email"`
	Phone      sdk.String                     `nexa:"size=60,string=Phone"`
	Primary    sdk.Boolean                    `nexa:"string=Primary Contact,default=false"`
	Active     sdk.Boolean                    `nexa:"string=Active,default=true"`
	Note       sdk.Text                       `nexa:"string=Note"`
}

// PurchaseSupplierItem is a supplier-specific product code and terms.
type PurchaseSupplierItem struct {
	sdk.Model `nexa:"model=purchase.supplier.item"`

	SupplierID   sdk.Many2One[PurchaseSupplier] `nexa:"required,index,string=Supplier"`
	ProductID    sdk.Many2One[sdk.Any]          `nexa:"index,string=Product,comodel=product.product"`
	SupplierProd sdk.String                     `nexa:"size=60,string=Supplier Product Code"`
	Description  sdk.String                     `nexa:"size=255,string=Description"`
	Price        sdk.Numeric                    `nexa:"string=Negotiated Price,precision=18,scale=2,default=0"`
	Currency     sdk.String                     `nexa:"size=3,string=Currency,default=USD"`
	LeadTimeDays sdk.Integer                    `nexa:"string=Lead Time (days),default=0"`
	MinOrderQty  sdk.Numeric                    `nexa:"string=Min Order Qty,precision=18,scale=3,default=0"`
	OrderMult    sdk.Numeric                    `nexa:"string=Order Multiple,precision=18,scale=3,default=0"`
	Active       sdk.Boolean                    `nexa:"string=Active,default=true"`
	Note         sdk.Text                       `nexa:"string=Note"`
}

// PurchaseSupplierDocument is a compliance certificate with review + expiry.
type PurchaseSupplierDocument struct {
	sdk.Model `nexa:"model=purchase.supplier.document"`

	SupplierID  sdk.Many2One[PurchaseSupplier] `nexa:"required,index,string=Supplier"`
	Name        sdk.String                     `nexa:"required,size=120,string=Document"`
	DocType     sdk.String                     `nexa:"size=60,string=Document Type"`
	CatScope    sdk.String                     `nexa:"size=120,string=Category Scope"`
	ReferenceNo sdk.String                     `nexa:"size=120,string=Reference No."`
	IssueDate   sdk.Date                       `nexa:"string=Issue Date"`
	ExpiryDate  sdk.Date                       `nexa:"index,string=Expiry Date"`
	ReviewState sdk.Selection[DocState]        `nexa:"index,string=Review State,default=pending"`
	Mandatory   sdk.Boolean                    `nexa:"string=Mandatory,default=false"`
	Filename    sdk.String                     `nexa:"size=255,string=File"`
	ReviewedBy  sdk.Many2One[CoreUser]         `nexa:"string=Reviewed By"`
	RejectWhy   sdk.String                     `nexa:"size=300,string=Rejection Reason"`
	Note        sdk.Text                       `nexa:"string=Note"`
	Active      sdk.Boolean                    `nexa:"string=Active,default=true"`
}

// PurchaseSupplierBankAccount is a restricted, change-controlled payment
// instruction. Changing these must not silently update payment details.
type PurchaseSupplierBankAccount struct {
	sdk.Model `nexa:"model=purchase.supplier.bank.account"`

	SupplierID sdk.Many2One[PurchaseSupplier] `nexa:"required,index,string=Supplier"`
	Label      sdk.String                     `nexa:"size=120,string=Label"`
	BankName   sdk.String                     `nexa:"size=200,string=Bank Name"`
	AccountRef sdk.String                     `nexa:"size=120,string=Account Reference"`
	SortCode   sdk.String                     `nexa:"size=60,string=Branch / Sort Code"`
	Currency   sdk.String                     `nexa:"size=3,string=Currency,default=USD"`
	Primary    sdk.Boolean                    `nexa:"string=Primary,default=false"`
	State      sdk.Selection[BankState]       `nexa:"string=Status,default=active"`
	Verified   sdk.Boolean                    `nexa:"string=Independently Verified,default=false"`
	VerifiedBy sdk.Many2One[CoreUser]         `nexa:"string=Verified By"`
	Note       sdk.Text                       `nexa:"string=Note"`
	Active     sdk.Boolean                    `nexa:"string=Active,default=true"`
}

// PurchaseSupplierQualification approves a supplier for a category/site.
type PurchaseSupplierQualification struct {
	sdk.Model `nexa:"model=purchase.supplier.qualification"`

	SupplierID   sdk.Many2One[PurchaseSupplier]     `nexa:"required,index,string=Supplier"`
	Category     sdk.String                         `nexa:"required,index,size=120,string=Approved Category"`
	SiteID       sdk.Many2One[PurchaseSupplierSite] `nexa:"string=Site"`
	State        sdk.Selection[QualState]           `nexa:"index,string=Status,default=draft"`
	ApprovedDate sdk.Date                           `nexa:"string=Approved Date"`
	ExpiryDate   sdk.Date                           `nexa:"string=Valid Until"`
	ApprovedBy   sdk.Many2One[CoreUser]             `nexa:"string=Approved By"`
	Evidence     sdk.Text                           `nexa:"string=Evidence"`
	Active       sdk.Boolean                        `nexa:"string=Active,default=true"`
}

// PurchaseSupplierChangeRequest records a controlled profile/bank change that
// needs independent verification before it is applied.
type PurchaseSupplierChangeRequest struct {
	sdk.Model `nexa:"model=purchase.supplier.change.request"`

	Name          sdk.String                     `nexa:"required,index,size=60,string=Reference"`
	SupplierID    sdk.Many2One[PurchaseSupplier] `nexa:"required,index,string=Supplier"`
	FieldKind     sdk.String                     `nexa:"required,size=40,string=Change Type"`
	TargetSummary sdk.String                     `nexa:"size=255,string=What Changes"`
	ProposedValue sdk.Text                       `nexa:"string=Proposed Value"`
	Reason        sdk.String                     `nexa:"size=300,string=Reason"`
	State         sdk.Selection[ChangeState]     `nexa:"index,string=Status,default=draft"`
	RequestedBy   sdk.Many2One[CoreUser]         `nexa:"string=Requested By"`
	VerifiedBy    sdk.Many2One[CoreUser]         `nexa:"string=Verified By"`
	AppliedOn     sdk.DateTime                   `nexa:"string=Applied On"`
	Note          sdk.Text                       `nexa:"string=Note"`
	Active        sdk.Boolean                    `nexa:"string=Active,default=true"`
}

// PurchaseSupplierRiskAssessment is an explainable multi-dimension risk score.
type PurchaseSupplierRiskAssessment struct {
	sdk.Model `nexa:"model=purchase.supplier.risk.assessment"`

	Name           sdk.String                     `nexa:"required,index,size=60,string=Reference"`
	SupplierID     sdk.Many2One[PurchaseSupplier] `nexa:"required,index,string=Supplier"`
	ScoreDate      sdk.Date                       `nexa:"string=Assessment Date,default=today"`
	DocumentRisk   sdk.Integer                    `nexa:"string=Document Validity,default=0"`
	FinancialRisk  sdk.Integer                    `nexa:"string=Financial Stability,default=0"`
	DeliveryRisk   sdk.Integer                    `nexa:"string=Delivery Reliability,default=0"`
	QualityRisk    sdk.Integer                    `nexa:"string=Quality,default=0"`
	GeographicRisk sdk.Integer                    `nexa:"string=Geographic / Logistics,default=0"`
	ConcentrRisk   sdk.Integer                    `nexa:"string=Concentration / Dependency,default=0"`
	SecurityRisk   sdk.Integer                    `nexa:"string=Cyber / Data Protection,default=0"`
	ContinuityRisk sdk.Integer                    `nexa:"string=Business Continuity,default=0"`
	EthicsRisk     sdk.Integer                    `nexa:"string=Ethical Sourcing,default=0"`
	RegulatoryRisk sdk.Integer                    `nexa:"string=Regulatory / Sanctions,default=0"`
	OverallScore   sdk.Numeric                    `nexa:"string=Overall Score,precision=6,scale=2,default=0"`
	Level          sdk.Selection[SupplierRisk]    `nexa:"string=Risk Level,default=unclassified"`
	Factors        sdk.Text                       `nexa:"string=Explainable Factors"`
	NextReviewDate sdk.Date                       `nexa:"string=Next Reassessment"`
	AssessedBy     sdk.Many2One[CoreUser]         `nexa:"string=Assessed By"`
	Note           sdk.Text                       `nexa:"string=Note"`
	Active         sdk.Boolean                    `nexa:"string=Active,default=true"`
}

func init() {
	sdk.MustRegister("purchase_supplier",
		&PurchaseSupplier{},
		&PurchaseSupplierSite{},
		&PurchaseSupplierContact{},
		&PurchaseSupplierItem{},
		&PurchaseSupplierDocument{},
		&PurchaseSupplierBankAccount{},
		&PurchaseSupplierQualification{},
		&PurchaseSupplierChangeRequest{},
		&PurchaseSupplierRiskAssessment{},
	)
}
