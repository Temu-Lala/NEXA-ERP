package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	lendingmodels "nexa_addons/lending/models"
)

type (
	CorePartner        = basemodels.CorePartner
	CoreUser           = basemodels.CoreUser
	LoanApplication    = lendingmodels.LoanApplication
	LoanAccount        = lendingmodels.LoanAccount
	RepaymentFrequency = lendingmodels.RepaymentFrequency
)

// ============================================================================
// Credit Assessment
// ============================================================================

type AffordabilityResult string

const (
	AffordabilityResultPass  AffordabilityResult = "pass"
	AffordabilityResultFail  AffordabilityResult = "fail"
	AffordabilityResultRefer AffordabilityResult = "refer"
)

type Recommendation string

const (
	RecommendationApprove Recommendation = "approve"
	RecommendationReject  Recommendation = "reject"
	RecommendationRefer   Recommendation = "refer"
)

type LoanCreditAssessment struct {
	sdk.Model

	ApplicationID       sdk.Many2One[LoanApplication]      `nexa:"string=Application,required,index"`
	BorrowerID          sdk.Many2One[CorePartner]          `nexa:"string=Borrower,index"`
	ScoreValue          sdk.Integer                        `nexa:"string=Score Value"`
	ScoreProvider       sdk.String                         `nexa:"string=Score Provider"`
	ModelVersion        sdk.String                         `nexa:"string=Model Version"`
	ScoreDate           sdk.Date                           `nexa:"string=Score Date"`
	VerifiedIncome      sdk.Numeric                        `nexa:"string=Verified Income,precision=18,scale=2"`
	ExistingObligations sdk.Numeric                        `nexa:"string=Existing Obligations,precision=18,scale=2"`
	DebtServiceRatio    sdk.Float                          `nexa:"string=Debt Service Ratio (%)"`
	AffordabilityResult sdk.Selection[AffordabilityResult] `nexa:"string=Affordability,default=pass"`
	Recommendation      sdk.Selection[Recommendation]      `nexa:"string=Recommendation,default=refer"`
	AssessedBy          sdk.Many2One[CoreUser]             `nexa:"string=Assessed By"`
	AssessedAt          sdk.DateTime                       `nexa:"string=Assessed At"`
	Note                sdk.Text                           `nexa:"string=Note"`

	Factors sdk.One2Many[LoanCreditFactor] `nexa:"string=Factors"`

	// Non-stored aggregates over the factor lines (recomputed on read).
	FactorCount sdk.Integer `nexa:"string=Factors,compute=compute_assessment_factor_count"`
	TotalWeight sdk.Float   `nexa:"string=Total Weight,compute=compute_assessment_total_weight"`
}

// ============================================================================
// Credit Assessment Factor
// ============================================================================

type FactorType string

const (
	FactorTypeIncome     FactorType = "income"
	FactorTypeObligation FactorType = "obligation"
	FactorTypeScore      FactorType = "score"
	FactorTypeHistory    FactorType = "history"
	FactorTypeCollateral FactorType = "collateral"
	FactorTypeOther      FactorType = "other"
)

type FactorImpact string

const (
	FactorImpactPositive FactorImpact = "positive"
	FactorImpactNegative FactorImpact = "negative"
	FactorImpactNeutral  FactorImpact = "neutral"
)

type LoanCreditFactor struct {
	sdk.Model

	AssessmentID sdk.Many2One[LoanCreditAssessment] `nexa:"string=Assessment,required,index"`
	Name         sdk.String                         `nexa:"string=Name"`
	FactorType   sdk.Selection[FactorType]          `nexa:"string=Factor Type,default=other"`
	Value        sdk.Numeric                        `nexa:"string=Value,precision=18,scale=2"`
	Weight       sdk.Float                          `nexa:"string=Weight"`
	Impact       sdk.Selection[FactorImpact]        `nexa:"string=Impact,default=neutral"`
	Note         sdk.Text                           `nexa:"string=Note"`
}

// ============================================================================
// Loan Approval (maker-checker)
// ============================================================================

type ApprovalType string

const (
	ApprovalTypeCredit       ApprovalType = "credit"
	ApprovalTypeDisbursement ApprovalType = "disbursement"
)

type ApprovalDecision string

const (
	ApprovalDecisionPending   ApprovalDecision = "pending"
	ApprovalDecisionApproved  ApprovalDecision = "approved"
	ApprovalDecisionRejected  ApprovalDecision = "rejected"
	ApprovalDecisionEscalated ApprovalDecision = "escalated"
)

type LoanApproval struct {
	sdk.Model

	ApplicationID   sdk.Many2One[LoanApplication]   `nexa:"string=Application,required,index"`
	ApprovalType    sdk.Selection[ApprovalType]     `nexa:"string=Approval Type,default=credit"`
	RequestedAmount sdk.Numeric                     `nexa:"string=Requested Amount,precision=18,scale=2"`
	Level           sdk.Integer                     `nexa:"string=Level"`
	MakerID         sdk.Many2One[CoreUser]          `nexa:"string=Maker"`
	CheckerID       sdk.Many2One[CoreUser]          `nexa:"string=Checker"`
	ApproverID      sdk.Many2One[CoreUser]          `nexa:"string=Approver"`
	Decision        sdk.Selection[ApprovalDecision] `nexa:"string=Decision,default=pending"`
	DecidedAt       sdk.DateTime                    `nexa:"string=Decided At"`
	Rationale       sdk.Text                        `nexa:"string=Rationale"`
	Conditions      sdk.Text                        `nexa:"string=Conditions"`
	Note            sdk.Text                        `nexa:"string=Note"`
}

// ============================================================================
// Loan Agreement
// ============================================================================

type LoanAgreement struct {
	sdk.Model

	ApplicationID          sdk.Many2One[LoanApplication]     `nexa:"string=Application,index"`
	AccountID              sdk.Many2One[LoanAccount]         `nexa:"string=Loan Account"`
	BorrowerID             sdk.Many2One[CorePartner]         `nexa:"string=Borrower,index"`
	Principal              sdk.Numeric                       `nexa:"string=Principal,precision=18,scale=2"`
	InterestRate           sdk.Numeric                       `nexa:"string=Interest Rate,precision=18,scale=4"`
	Tenor                  sdk.Integer                       `nexa:"string=Tenor"`
	RepaymentFrequency     sdk.Selection[RepaymentFrequency] `nexa:"string=Repayment Frequency,default=monthly"`
	DueDatePolicy          sdk.String                        `nexa:"string=Due Date Policy"`
	Accepted               sdk.Boolean                       `nexa:"string=Accepted"`
	AgreementVersion       sdk.Integer                       `nexa:"string=Agreement Version"`
	DocumentRef            sdk.String                        `nexa:"string=Document Reference"`
	SignatureRef           sdk.String                        `nexa:"string=Signature Reference"`
	EffectiveDate          sdk.Date                          `nexa:"string=Effective Date"`
	DisbursementConditions sdk.Text                          `nexa:"string=Disbursement Conditions"`
	Note                   sdk.Text                          `nexa:"string=Note"`

	// Non-stored view of the linked loan account's current outstanding balance.
	AccountOutstanding sdk.Numeric `nexa:"string=Account Outstanding,precision=18,scale=2,compute=compute_agreement_account_outstanding"`
}

func init() {
	sdk.MustRegister("lending_credit",
		&LoanCreditAssessment{},
		&LoanCreditFactor{},
		&LoanApproval{},
		&LoanAgreement{},
	)
}
