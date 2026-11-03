package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

type (
	CoreUser    = basemodels.CoreUser
	CorePartner = basemodels.CorePartner
	CoreCompany = basemodels.CoreCompany
)

// ============================================================================
// Loan Product
// ============================================================================

type ProductType string

const (
	ProductTypePersonal       ProductType = "personal"
	ProductTypeSalaryBacked   ProductType = "salary_backed"
	ProductTypeMicrofinance   ProductType = "microfinance"
	ProductTypeSME            ProductType = "sme"
	ProductTypeCommercial     ProductType = "commercial"
	ProductTypeAssetFinancing ProductType = "asset_financing"
	ProductTypeSecured        ProductType = "secured"
)

type TenorUnit string

const (
	TenorUnitDay   TenorUnit = "day"
	TenorUnitWeek  TenorUnit = "week"
	TenorUnitMonth TenorUnit = "month"
)

type RateType string

const (
	RateTypeFixed    RateType = "fixed"
	RateTypeVariable RateType = "variable"
)

type InterestMethod string

const (
	InterestMethodDecliningBalance InterestMethod = "declining_balance"
	InterestMethodFlat             InterestMethod = "flat"
	InterestMethodInterestOnly     InterestMethod = "interest_only"
	InterestMethodBullet           InterestMethod = "bullet"
)

type DayCount string

const (
	DayCountActual365 DayCount = "actual365"
	DayCountActual360 DayCount = "actual360"
	DayCountThirty360 DayCount = "thirty360"
)

type RepaymentFrequency string

const (
	RepaymentFrequencyDaily   RepaymentFrequency = "daily"
	RepaymentFrequencyWeekly  RepaymentFrequency = "weekly"
	RepaymentFrequencyMonthly RepaymentFrequency = "monthly"
)

type ProductStatus string

const (
	ProductStatusActive   ProductStatus = "active"
	ProductStatusInactive ProductStatus = "inactive"
)

type LendingProduct struct {
	sdk.Model

	Name        sdk.String                   `nexa:"string=Name,required"`
	Code        sdk.String                   `nexa:"string=Code,index"`
	CompanyID   sdk.Many2One[CoreCompany]    `nexa:"string=Company"`
	ProductType sdk.Selection[ProductType]   `nexa:"string=Product Type,default=personal"`
	CurrencyID  sdk.Many2One[sdk.Any]        `nexa:"string=Currency,comodel=core.currency"`
	Status      sdk.Selection[ProductStatus] `nexa:"string=Status,default=active"`

	MinPrincipal sdk.Numeric              `nexa:"string=Min Principal,precision=18,scale=2"`
	MaxPrincipal sdk.Numeric              `nexa:"string=Max Principal,precision=18,scale=2"`
	MinTenor     sdk.Integer              `nexa:"string=Min Tenor"`
	MaxTenor     sdk.Integer              `nexa:"string=Max Tenor"`
	TenorUnit    sdk.Selection[TenorUnit] `nexa:"string=Tenor Unit,default=month"`

	InterestRate       sdk.Numeric                       `nexa:"string=Interest Rate,precision=18,scale=4"`
	RateType           sdk.Selection[RateType]           `nexa:"string=Rate Type,default=fixed"`
	InterestMethod     sdk.Selection[InterestMethod]     `nexa:"string=Interest Method,default=declining_balance"`
	DayCount           sdk.Selection[DayCount]           `nexa:"string=Day Count,default=actual365"`
	RepaymentFrequency sdk.Selection[RepaymentFrequency] `nexa:"string=Repayment Frequency,default=monthly"`

	OriginationFee     sdk.Numeric `nexa:"string=Origination Fee,precision=18,scale=2"`
	ServiceFee         sdk.Numeric `nexa:"string=Service Fee,precision=18,scale=2"`
	LateFee            sdk.Numeric `nexa:"string=Late Fee,precision=18,scale=2"`
	FeeCap             sdk.Numeric `nexa:"string=Fee Cap,precision=18,scale=2"`
	GracePeriod        sdk.Integer `nexa:"string=Grace Period (Days)"`
	MaxExposure        sdk.Numeric `nexa:"string=Max Exposure,precision=18,scale=2"`
	RequiredGuarantors sdk.Integer `nexa:"string=Required Guarantors"`
	RequiresCollateral sdk.Boolean `nexa:"string=Requires Collateral"`

	Active      sdk.Boolean `nexa:"string=Active,default=true"`
	Description sdk.Text    `nexa:"string=Description"`
	Note        sdk.Text    `nexa:"string=Internal Note"`

	Versions sdk.One2Many[LendingProductVersion] `nexa:"string=Versions"`

	ApplicationCount sdk.Integer `nexa:"string=Applications,compute=compute_product_application_count"`
	ActiveLoanCount  sdk.Integer `nexa:"string=Active Loans,compute=compute_product_active_loan_count"`
}

// ============================================================================
// Loan Product Version
// ============================================================================

type VersionStatus string

const (
	VersionStatusDraft      VersionStatus = "draft"
	VersionStatusActive     VersionStatus = "active"
	VersionStatusSuperseded VersionStatus = "superseded"
)

type LendingProductVersion struct {
	sdk.Model

	ProductID     sdk.Many2One[LendingProduct] `nexa:"string=Product,required,index"`
	Version       sdk.Integer                  `nexa:"string=Version,default=1"`
	EffectiveDate sdk.Date                     `nexa:"string=Effective Date"`
	Status        sdk.Selection[VersionStatus] `nexa:"string=Status,default=draft"`

	InterestRate       sdk.Numeric                       `nexa:"string=Interest Rate,precision=18,scale=4"`
	RateType           sdk.Selection[RateType]           `nexa:"string=Rate Type,default=fixed"`
	InterestMethod     sdk.Selection[InterestMethod]     `nexa:"string=Interest Method,default=declining_balance"`
	DayCount           sdk.Selection[DayCount]           `nexa:"string=Day Count,default=actual365"`
	RepaymentFrequency sdk.Selection[RepaymentFrequency] `nexa:"string=Repayment Frequency,default=monthly"`
	MinPrincipal       sdk.Numeric                       `nexa:"string=Min Principal,precision=18,scale=2"`
	MaxPrincipal       sdk.Numeric                       `nexa:"string=Max Principal,precision=18,scale=2"`
	MinTenor           sdk.Integer                       `nexa:"string=Min Tenor"`
	MaxTenor           sdk.Integer                       `nexa:"string=Max Tenor"`
	FeeSchedule        sdk.Text                          `nexa:"string=Fee Schedule"`
	Note               sdk.Text                          `nexa:"string=Note"`
}

// ============================================================================
// Loan Application
// ============================================================================

type ApplicationState string

const (
	ApplicationStateDraft           ApplicationState = "draft"
	ApplicationStateSubmitted       ApplicationState = "submitted"
	ApplicationStateUnderReview     ApplicationState = "under_review"
	ApplicationStateAssessment      ApplicationState = "assessment"
	ApplicationStatePendingApproval ApplicationState = "pending_approval"
	ApplicationStateApproved        ApplicationState = "approved"
	ApplicationStateRejected        ApplicationState = "rejected"
	ApplicationStateWithdrawn       ApplicationState = "withdrawn"
	ApplicationStateExpired         ApplicationState = "expired"
	ApplicationStateCancelled       ApplicationState = "cancelled"
)

type LoanApplication struct {
	sdk.Model

	Name               sdk.String                          `nexa:"string=Application,required"`
	Number             sdk.String                          `nexa:"string=Number,index"`
	BorrowerID         sdk.Many2One[CorePartner]           `nexa:"string=Borrower,required,index"`
	ProductID          sdk.Many2One[LendingProduct]        `nexa:"string=Product,required,index"`
	VersionID          sdk.Many2One[LendingProductVersion] `nexa:"string=Product Version"`
	RequestedAmount    sdk.Numeric                         `nexa:"string=Requested Amount,precision=18,scale=2"`
	CurrencyID         sdk.Many2One[sdk.Any]               `nexa:"string=Currency,comodel=core.currency"`
	Tenor              sdk.Integer                         `nexa:"string=Tenor"`
	Purpose            sdk.String                          `nexa:"string=Purpose"`
	CompanyID          sdk.Many2One[CoreCompany]           `nexa:"string=Company"`
	OfficerID          sdk.Many2One[CoreUser]              `nexa:"string=Loan Officer"`
	AppliedDate        sdk.Date                            `nexa:"string=Applied Date,default=today"`
	GuarantorCount     sdk.Integer                         `nexa:"string=Guarantor Count"`
	CollateralRequired sdk.Boolean                         `nexa:"string=Collateral Required"`
	State              sdk.Selection[ApplicationState]     `nexa:"string=Status,default=draft"`
	Decision           sdk.Text                            `nexa:"string=Decision Rationale"`
	Note               sdk.Text                            `nexa:"string=Internal Note"`
}

// ============================================================================
// Loan Account
// ============================================================================

type AccountStatus string

const (
	AccountStatusPending      AccountStatus = "pending"
	AccountStatusActive       AccountStatus = "active"
	AccountStatusDelinquent   AccountStatus = "delinquent"
	AccountStatusRestructured AccountStatus = "restructured"
	AccountStatusClosed       AccountStatus = "closed"
	AccountStatusWrittenOff   AccountStatus = "written_off"
)

type LoanAccount struct {
	sdk.Model

	Name          sdk.String                          `nexa:"string=Loan,required"`
	Number        sdk.String                          `nexa:"string=Number,index"`
	ApplicationID sdk.Many2One[LoanApplication]       `nexa:"string=Application"`
	BorrowerID    sdk.Many2One[CorePartner]           `nexa:"string=Borrower,required,index"`
	ProductID     sdk.Many2One[LendingProduct]        `nexa:"string=Product"`
	VersionID     sdk.Many2One[LendingProductVersion] `nexa:"string=Product Version"`

	Principal       sdk.Numeric                   `nexa:"string=Principal,precision=18,scale=2"`
	DisbursedAmount sdk.Numeric                   `nexa:"string=Disbursed Amount,precision=18,scale=2"`
	CurrencyID      sdk.Many2One[sdk.Any]         `nexa:"string=Currency,comodel=core.currency"`
	InterestRate    sdk.Numeric                   `nexa:"string=Interest Rate,precision=18,scale=4"`
	InterestMethod  sdk.Selection[InterestMethod] `nexa:"string=Interest Method,default=declining_balance"`
	DayCount        sdk.Selection[DayCount]       `nexa:"string=Day Count,default=actual365"`
	Tenor           sdk.Integer                   `nexa:"string=Tenor"`
	StartDate       sdk.Date                      `nexa:"string=Start Date"`
	MaturityDate    sdk.Date                      `nexa:"string=Maturity Date"`
	OfficerID       sdk.Many2One[CoreUser]        `nexa:"string=Loan Officer"`
	CompanyID       sdk.Many2One[CoreCompany]     `nexa:"string=Company"`
	Status          sdk.Selection[AccountStatus]  `nexa:"string=Status,default=pending"`

	OutstandingPrincipal sdk.Numeric `nexa:"string=Outstanding Principal,precision=18,scale=2"`
	AccruedInterest      sdk.Numeric `nexa:"string=Accrued Interest,precision=18,scale=2"`
	PaidTotal            sdk.Numeric `nexa:"string=Paid Total,precision=18,scale=2"`
	NextDueDate          sdk.Date    `nexa:"string=Next Due Date"`
	DaysPastDue          sdk.Integer `nexa:"string=Days Past Due"`

	Note sdk.Text `nexa:"string=Internal Note"`
}

func init() {
	sdk.MustRegister("lending",
		&LendingProduct{},
		&LendingProductVersion{},
		&LoanApplication{},
		&LoanAccount{},
	)
}
