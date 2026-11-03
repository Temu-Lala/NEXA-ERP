package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	hrmodels "nexa_addons/hr/models"
	paymodels "nexa_addons/hr_payroll/models"
)

// HR-F4 "Payroll hardening". The base hr_payroll module already stores salary
// categories/structures/rules, payslip runs (draft -> confirmed -> closed),
// payslips and payslip lines. What it does NOT have is the production guardrails
// the blueprint demands: rules that are versioned/effective-dated, statutory
// (tax / pension) configuration by jurisdiction and effective date, a
// maker-checker approval that FREEZES an approved run so payslips cannot be
// silently edited, idempotent payment (no double disbursement), traceable
// adjustment / reversal entries instead of in-place edits, payslip delivery to
// the employee, a bank payment file and a reconciliation with audit.
//
// This companion adds exactly that WITHOUT touching hr_payroll's tables: new
// models plus non-destructive inherit extensions that bolt the extra columns onto
// the anchor payslip / run / rule records. The engine keeps all money math pure
// (see engine.go) so gross-to-net, rule and statutory computations are unit
// tested. Nothing here is asserted legally correct for a given jurisdiction:
// statutory rates are configurable but must be verified before production use
// (blueprint guardrail).

type (
	HrEmployee         = hrmodels.HrEmployee
	HrPayrollCategory  = paymodels.HrPayrollCategory
	HrPayrollStructure = paymodels.HrPayrollStructure
	HrSalaryRule       = paymodels.HrSalaryRule
	HrPayslipRun       = paymodels.HrPayslipRun
	HrPayslip          = paymodels.HrPayslip
	HrPayslipLine      = paymodels.HrPayslipLine
	CoreCompany        = basemodels.CoreCompany
)

// ---------------------------------------------------------------------------
// Enumerations
// ---------------------------------------------------------------------------

type StatutoryKind string

const (
	StatTax     StatutoryKind = "tax"
	StatPension StatutoryKind = "pension"
	StatSocial  StatutoryKind = "social"
	StatOther   StatutoryKind = "other"
)

type StatutoryBase string

const (
	BaseGross StatutoryBase = "gross"
	BaseBasic StatutoryBase = "basic"
)

type AmountBasis string

const (
	BasisPercentage AmountBasis = "percentage"
	BasisFixed      AmountBasis = "fixed"
)

type PayState string

const (
	PayDraft PayState = "draft"
	PayPaid  PayState = "paid"
	PayVoid  PayState = "void"
)

type AdjState string

const (
	AdjDraft    AdjState = "draft"
	AdjValidate AdjState = "validate"
	AdjRefuse   AdjState = "refuse"
)

type AdjDirection string

const (
	AdjIncrease AdjDirection = "increase"
	AdjDecrease AdjDirection = "decrease"
)

type ReconState string

const (
	ReconDraft     ReconState = "draft"
	ReconBalanced  ReconState = "reconciled"
	ReconException ReconState = "exception"
)

type FileState string

const (
	FileDraft     FileState = "draft"
	FileFinalized FileState = "finalized"
)

// ---------------------------------------------------------------------------
// Effective-dating bolt-ons (inherit)
// ---------------------------------------------------------------------------

// HrSalaryRuleVersion turns a salary rule into an effective-dated, versioned
// definition so a change of rate never rewrites history: a new row with a later
// ValidFrom supersedes the previous one for periods on/after that date.
type HrSalaryRuleVersion struct {
	sdk.Model `nexa:"inherit=hr.salary.rule"`

	ValidFrom sdk.Date    `nexa:"string=Valid From,index"`
	ValidTo   sdk.Date    `nexa:"string=Valid To"`
	Version   sdk.Integer `nexa:"string=Version,default=1"`
	Remark    sdk.Text    `nexa:"string=Remark"`
}

// HrPayslipRunApproval adds maker-checker approval + freeze to a payslip run.
// Approved/frozen are written only through guarded object actions.
type HrPayslipRunApproval struct {
	sdk.Model `nexa:"inherit=hr.payslip.run"`

	Approved    sdk.Boolean  `nexa:"string=Approved,default=false"`
	Frozen      sdk.Boolean  `nexa:"string=Frozen,default=false"`
	ApproverID  sdk.Integer  `nexa:"string=Approver ID"`
	ApproveNote sdk.Text     `nexa:"string=Approval Note"`
	ApprovedOn  sdk.DateTime `nexa:"string=Approved On"`
}

// HrPayslipHardening adds delivery + payment tracking onto a payslip. Paid is the
// idempotency latch: once true (or a Paid payment row exists) the payslip cannot
// be disbursed again.
type HrPayslipHardening struct {
	sdk.Model `nexa:"inherit=hr.payslip"`

	ReleasedToEmployee sdk.Boolean  `nexa:"string=Released To Employee,default=false"`
	ReleasedOn         sdk.DateTime `nexa:"string=Released On"`
	Paid               sdk.Boolean  `nexa:"string=Paid,default=false,index"`
	PaidOn             sdk.DateTime `nexa:"string=Paid On"`
	PaymentRef         sdk.String   `nexa:"string=Payment Ref,index"`
}

// ---------------------------------------------------------------------------
// Statutory configuration (effective-dated by jurisdiction)
// ---------------------------------------------------------------------------

// HrPayrollStatutory is a configurable statutory contribution (income tax,
// pension, social security ...). It applies to a period bounded by ValidFrom /
// ValidTo, is scoped by Jurisdiction and posts against a payroll category.
type HrPayrollStatutory struct {
	sdk.Model `nexa:"model=hr.payroll.statutory"`

	Name         sdk.String                      `nexa:"required,string=Name"`
	Jurisdiction sdk.String                      `nexa:"string=Jurisdiction,index,default=general"`
	Kind         sdk.Selection[StatutoryKind]    `nexa:"string=Kind,default=tax"`
	AppliesTo    sdk.Selection[StatutoryBase]    `nexa:"string=Applies To,default=gross"`
	AmountType   sdk.Selection[AmountBasis]      `nexa:"string=Computation,default=percentage"`
	Percent      sdk.Numeric                     `nexa:"string=Percentage,precision=18,scale=2,default=0"`
	Fixed        sdk.Numeric                     `nexa:"string=Fixed Amount,precision=18,scale=2,default=0"`
	Cap          sdk.Numeric                     `nexa:"string=Cap (0 = none),precision=18,scale=2,default=0"`
	CategoryID   sdk.Many2One[HrPayrollCategory] `nexa:"string=Category"`
	ValidFrom    sdk.Date                        `nexa:"string=Valid From,index"`
	ValidTo      sdk.Date                        `nexa:"string=Valid To"`
	Sequence     sdk.Integer                     `nexa:"string=Sequence,default=100"`
	Active       sdk.Boolean                     `nexa:"string=Active,default=true"`
	CompanyID    sdk.Many2One[CoreCompany]       `nexa:"string=Company"`
}

// ---------------------------------------------------------------------------
// Idempotent payment record
// ---------------------------------------------------------------------------

// HrPayslipPayment is the disbursement ledger for a single payslip. Exactly one
// non-void payment per payslip is enforced by the engine, giving duplicate-payment
// protection (idempotency) and an audit trail of reference + date.
type HrPayslipPayment struct {
	sdk.Model `nexa:"model=hr.payslip.payment"`

	PayslipID  sdk.Many2One[HrPayslip]    `nexa:"required,string=Payslip,index"`
	EmployeeID sdk.Many2One[HrEmployee]   `nexa:"string=Employee,index"`
	RunID      sdk.Many2One[HrPayslipRun] `nexa:"string=Run,index"`
	Amount     sdk.Numeric                `nexa:"string=Amount,precision=18,scale=2,default=0"`
	PaymentRef sdk.String                 `nexa:"string=Payment Ref,index"`
	Date       sdk.Date                   `nexa:"string=Payment Date"`
	State      sdk.Selection[PayState]    `nexa:"string=Status,default=draft"`
	CompanyID  sdk.Many2One[CoreCompany]  `nexa:"string=Company"`
}

// ---------------------------------------------------------------------------
// Traceable adjustment / reversal
// ---------------------------------------------------------------------------

// HrPayslipAdjustment is a post-approval correction. Rather than editing a frozen
// payslip, a validated adjustment posts a new payslip line (increase or decrease)
// so the original detail is preserved and every change is attributable. A row may
// reference the adjustment it reverses.
type HrPayslipAdjustment struct {
	sdk.Model `nexa:"model=hr.payslip.adjustment"`

	PayslipID  sdk.Many2One[HrPayslip]           `nexa:"required,string=Payslip,index"`
	EmployeeID sdk.Many2One[HrEmployee]          `nexa:"string=Employee,index"`
	RunID      sdk.Many2One[HrPayslipRun]        `nexa:"string=Run"`
	CategoryID sdk.Many2One[HrPayrollCategory]   `nexa:"string=Category"`
	Direction  sdk.Selection[AdjDirection]       `nexa:"string=Direction,default=decrease"`
	Amount     sdk.Numeric                       `nexa:"required,string=Amount,precision=18,scale=2,default=0"`
	Reason     sdk.Text                          `nexa:"required,string=Reason"`
	ReversesID sdk.Many2One[HrPayslipAdjustment] `nexa:"string=Reverses"`
	State      sdk.Selection[AdjState]           `nexa:"string=Status,index,default=draft,selection=draft:Draft,validate:Applied,refuse:Refused"`
	Applied    sdk.Boolean                       `nexa:"string=Applied,default=false"`
	CompanyID  sdk.Many2One[CoreCompany]         `nexa:"string=Company"`
}

// ---------------------------------------------------------------------------
// Bank payment file
// ---------------------------------------------------------------------------

// HrPayrollPaymentFile is a payroll transfer batch generated from an approved,
// frozen run. Generation is idempotent: payslips already paid are skipped.
type HrPayrollPaymentFile struct {
	sdk.Model `nexa:"model=hr.payroll.payment.file"`

	Name        sdk.String                 `nexa:"required,string=Name"`
	RunID       sdk.Many2One[HrPayslipRun] `nexa:"required,string=Run,index"`
	GeneratedOn sdk.DateTime               `nexa:"string=Generated On"`
	TotalAmount sdk.Numeric                `nexa:"string=Total Amount,precision=18,scale=2,default=0"`
	LineCount   sdk.Integer                `nexa:"string=Line Count,default=0"`
	State       sdk.Selection[FileState]   `nexa:"string=Status,default=draft"`
	CompanyID   sdk.Many2One[CoreCompany]  `nexa:"string=Company"`
}

// ---------------------------------------------------------------------------
// Reconciliation (computed from payslips vs payments)
// ---------------------------------------------------------------------------

// HrPayrollReconciliation compares what a run should pay (sum of its payslips'
// net) against what was actually disbursed (sum of its payments), surfacing the
// difference and a balanced / exception verdict. The three money columns are
// stored computes refreshed on demand by an object action.
type HrPayrollReconciliation struct {
	sdk.Model `nexa:"model=hr.payroll.reconciliation"`

	Name        sdk.String                 `nexa:"string=Name"`
	RunID       sdk.Many2One[HrPayslipRun] `nexa:"required,string=Run,index"`
	Date        sdk.Date                   `nexa:"string=Date"`
	ExpectedNet sdk.Numeric                `nexa:"string=Expected Net,precision=18,scale=2,default=0,store,compute=compute_recon_expected"`
	PaidNet     sdk.Numeric                `nexa:"string=Paid Net,precision=18,scale=2,default=0,store,compute=compute_recon_paid"`
	Difference  sdk.Numeric                `nexa:"string=Difference,precision=18,scale=2,default=0,store,compute=compute_recon_diff"`
	State       sdk.Selection[ReconState]  `nexa:"string=Status,default=draft,store,compute=compute_recon_state"`
	Note        sdk.Text                   `nexa:"string=Note"`
	CompanyID   sdk.Many2One[CoreCompany]  `nexa:"string=Company"`
}

func init() {
	sdk.MustRegister("hr_payroll_hardening",
		&HrSalaryRuleVersion{},
		&HrPayslipRunApproval{},
		&HrPayslipHardening{},
		&HrPayrollStatutory{},
		&HrPayslipPayment{},
		&HrPayslipAdjustment{},
		&HrPayrollPaymentFile{},
		&HrPayrollReconciliation{},
	)
}
