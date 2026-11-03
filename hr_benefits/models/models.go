package models

import (
	"nexa/core/sdk"
	hrmodels "nexa_addons/hr/models"
	peoplemodels "nexa_addons/hr_people/models"
)

// HR-F5 "Benefits & Compensation". The anchor keeps only a flat contract and the
// payroll module only computes payslips; neither models benefit plans, enrolments,
// allowances, salary-band-governed pay changes or compensation review cycles. This
// companion adds exactly that WITHOUT migrating any existing table. It reuses the
// F1 hr.job.grade salary bands (never recreated) and, in line with the blueprint
// guardrail, treats money as advisory: no benefit, allowance or change here ever
// silently writes into payroll — every value-changing transition is a guarded,
// approval-controlled (maker-checker) step that only records an outcome.

type (
	HrEmployee   = hrmodels.HrEmployee
	HrDepartment = hrmodels.HrDepartment
	HrJobGrade   = peoplemodels.HrJobGrade
)

// ---------------------------------------------------------------------------
// Enumerations
// ---------------------------------------------------------------------------

type BenefitKind string

const (
	BenefitInsurance  BenefitKind = "insurance"
	BenefitHealth     BenefitKind = "health"
	BenefitRetirement BenefitKind = "retirement"
	BenefitOther      BenefitKind = "other"
)

type ContributionType string

const (
	ContributionFixed ContributionType = "fixed"
	ContributionPct   ContributionType = "percentage_salary"
)

type EnrollmentState string

const (
	EnrDraft     EnrollmentState = "draft"
	EnrPending   EnrollmentState = "pending"
	EnrActive    EnrollmentState = "active"
	EnrCancelled EnrollmentState = "cancelled"
)

type DependentRelation string

const (
	DepSpouse DependentRelation = "spouse"
	DepChild  DependentRelation = "child"
	DepParent DependentRelation = "parent"
	DepOther  DependentRelation = "other"
)

type AllowanceKind string

const (
	AllowanceAllowance     AllowanceKind = "allowance"
	AllowanceReimbursement AllowanceKind = "reimbursement"
)

type AllowanceFrequency string

const (
	FreqOnce    AllowanceFrequency = "once"
	FreqMonthly AllowanceFrequency = "monthly"
	FreqAnnual  AllowanceFrequency = "annual"
)

type AllowanceState string

const (
	AlwDraft     AllowanceState = "draft"
	AlwSubmitted AllowanceState = "submitted"
	AlwApproved  AllowanceState = "approved"
	AlwRefused   AllowanceState = "refused"
)

type CompChangeType string

const (
	ChangePromotion  CompChangeType = "promotion"
	ChangeRaise      CompChangeType = "raise"
	ChangeMarketAdj  CompChangeType = "market_adjustment"
	ChangeCorrection CompChangeType = "correction"
)

type CompChangeState string

const (
	ChgDraft     CompChangeState = "draft"
	ChgSubmitted CompChangeState = "submitted"
	ChgApplied   CompChangeState = "applied"
	ChgRefused   CompChangeState = "refused"
)

type ReviewCycleState string

const (
	CycPlanning ReviewCycleState = "planning"
	CycActive   ReviewCycleState = "active"
	CycReview   ReviewCycleState = "review"
	CycLocked   ReviewCycleState = "locked"
	CycClosed   ReviewCycleState = "closed"
)

type ReviewLineState string

const (
	RevDraft       ReviewLineState = "draft"
	RevRecommended ReviewLineState = "recommended"
	RevApproved    ReviewLineState = "approved"
	RevRejected    ReviewLineState = "rejected"
)

// ---------------------------------------------------------------------------
// Benefit plans + eligibility rules + employer contributions
// ---------------------------------------------------------------------------

// HrBenefitPlan defines a benefit and its eligibility gate. Eligibility is a
// minimum tenure plus an optional department restriction; employer contribution
// is a pure function of the plan type/rate/cap against a reference salary.
type HrBenefitPlan struct {
	sdk.Model `nexa:"model=hr.benefit.plan"`

	Name             sdk.String                      `nexa:"required,string=Plan Name"`
	Kind             sdk.Selection[BenefitKind]      `nexa:"string=Kind,index,default=insurance,selection=insurance:Insurance,health:Health,retirement:Retirement,other:Other"`
	Description      sdk.Text                        `nexa:"string=Description"`
	MinTenureDays    sdk.Integer                     `nexa:"string=Min Tenure (days),default=0"`
	EligibleDeptID   sdk.Many2One[HrDepartment]      `nexa:"string=Eligible Department,help=Leave empty to make the plan open to all departments."`
	ContributionType sdk.Selection[ContributionType] `nexa:"string=Employer Contribution Type,default=fixed,selection=fixed:Fixed Amount,percentage_salary:% of Salary"`
	ContributionRate sdk.Float64                     `nexa:"string=Employer Contribution (amount or %),default=0"`
	ContributionCap  sdk.Float64                     `nexa:"string=Employer Contribution Cap,help=0 means no cap."`
	EmployeeRate     sdk.Float64                     `nexa:"string=Employee Contribution %,default=0,help=Percentage of the reference salary withheld from the employee."`
	MaxDependents    sdk.Integer                     `nexa:"string=Max Dependents,default=0,help=0 means unlimited."`
	ValidFrom        sdk.Date                        `nexa:"string=Valid From"`
	ValidTo          sdk.Date                        `nexa:"string=Valid To"`
	Active           sdk.Boolean                     `nexa:"string=Active,default=true"`
}

// ---------------------------------------------------------------------------
// Enrolments (with dependents)
// ---------------------------------------------------------------------------

// HrBenefitEnrollment enrols an employee into a plan. Contributions are computed
// from the plan against an enrolment-captured reference salary; the confirm step
// re-validates eligibility and is maker-checker guarded.
type HrBenefitEnrollment struct {
	sdk.Model `nexa:"model=hr.benefit.enrollment"`

	EmployeeID           sdk.Many2One[HrEmployee]         `nexa:"required,string=Employee,index"`
	PlanID               sdk.Many2One[HrBenefitPlan]      `nexa:"required,string=Plan,index"`
	State                sdk.Selection[EnrollmentState]   `nexa:"string=Status,index,default=draft,selection=draft:Draft,pending:Pending,active:Active,cancelled:Cancelled"`
	EnrollmentDate       sdk.Date                         `nexa:"string=Enrollment Date"`
	StartDate            sdk.Date                         `nexa:"string=Coverage Start"`
	EndDate              sdk.Date                         `nexa:"string=Coverage End"`
	ReferenceSalary      sdk.Float64                      `nexa:"string=Reference Salary,default=0,help=Base salary the contributions are calculated against."`
	Dependents           sdk.One2Many[HrBenefitDependent] `nexa:"string=Dependents,inverse=enrollment_id"`
	EmployerContribution sdk.Float64                      `nexa:"string=Employer Contribution,compute=compute_employer_contribution,store"`
	EmployeeContribution sdk.Float64                      `nexa:"string=Employee Contribution,compute=compute_employee_contribution,store"`
	ApproverID           sdk.Integer                      `nexa:"string=Approver ID"`
	ApprovedDate         sdk.Date                         `nexa:"string=Approved Date"`
	Notes                sdk.Text                         `nexa:"string=Notes"`
}

// HrBenefitDependent is a covered person on an enrolment.
type HrBenefitDependent struct {
	sdk.Model `nexa:"model=hr.benefit.dependent"`

	EnrollmentID sdk.Many2One[HrBenefitEnrollment] `nexa:"required,string=Enrollment,index"`
	Name         sdk.String                        `nexa:"required,string=Name"`
	Relation     sdk.Selection[DependentRelation]  `nexa:"string=Relationship,default=other,selection=spouse:Spouse,child:Child,parent:Parent,other:Other"`
	BirthDate    sdk.Date                          `nexa:"string=Birth Date"`
	Covered      sdk.Boolean                       `nexa:"string=Covered,default=true"`
}

// ---------------------------------------------------------------------------
// Allowances / reimbursements (effective-dated, approval-controlled)
// ---------------------------------------------------------------------------

// HrAllowanceRequest is an effective-dated allowance or reimbursement that must
// be approved (maker-checker) before it is considered granted. It records an
// outcome only; it never posts into payroll.
type HrAllowanceRequest struct {
	sdk.Model `nexa:"model=hr.allowance"`

	EmployeeID  sdk.Many2One[HrEmployee]          `nexa:"required,string=Employee,index"`
	Kind        sdk.Selection[AllowanceKind]      `nexa:"string=Kind,index,default=allowance,selection=allowance:Allowance,reimbursement:Reimbursement"`
	Category    sdk.String                        `nexa:"string=Category"`
	Description sdk.Text                          `nexa:"string=Description"`
	Amount      sdk.Numeric                       `nexa:"required,string=Amount,precision=12,scale=2,default=0"`
	Frequency   sdk.Selection[AllowanceFrequency] `nexa:"string=Frequency,default=once,selection=once:One Time,monthly:Monthly,annual:Annual"`
	ValidFrom   sdk.Date                          `nexa:"string=Valid From"`
	ValidTo     sdk.Date                          `nexa:"string=Valid To"`
	State       sdk.Selection[AllowanceState]     `nexa:"string=Status,index,default=draft,selection=draft:Draft,submitted:Submitted,approved:Approved,refused:Refused"`
	RequestedBy sdk.Integer                       `nexa:"string=Requested By"`
	ApproverID  sdk.Integer                       `nexa:"string=Approver ID"`
	ApproveDate sdk.Date                          `nexa:"string=Approved Date"`
}

// ---------------------------------------------------------------------------
// Promotion / salary-change approvals (effective-dated, band-checked)
// ---------------------------------------------------------------------------

// HrCompensationChange is a proposed pay move. When a target grade is set the
// new salary is band-checked against that grade's F1 salary band; applying is
// manager + maker-checker guarded and blocked when the salary falls outside the
// band. Applying records the decision only — payroll remains a separate run.
type HrCompensationChange struct {
	sdk.Model `nexa:"model=hr.compensation.change"`

	EmployeeID    sdk.Many2One[HrEmployee]       `nexa:"required,string=Employee,index"`
	ChangeType    sdk.Selection[CompChangeType]  `nexa:"string=Change Type,index,default=raise,selection=promotion:Promotion,raise:Salary Raise,market_adjustment:Market Adjustment,correction:Correction"`
	Reason        sdk.String                     `nexa:"string=Reason"`
	OldGradeID    sdk.Many2One[HrJobGrade]       `nexa:"string=Current Grade"`
	NewGradeID    sdk.Many2One[HrJobGrade]       `nexa:"string=New Grade"`
	OldSalary     sdk.Numeric                    `nexa:"string=Current Salary,precision=12,scale=2,default=0"`
	NewSalary     sdk.Numeric                    `nexa:"string=New Salary,precision=12,scale=2,default=0"`
	EffectiveDate sdk.Date                       `nexa:"required,string=Effective Date,index"`
	State         sdk.Selection[CompChangeState] `nexa:"string=Status,index,default=draft,selection=draft:Draft,submitted:Submitted,applied:Applied,refused:Refused"`
	WithinBand    sdk.Boolean                    `nexa:"string=Within Grade Band,compute=compute_within_band,store"`
	RequestedBy   sdk.Integer                    `nexa:"string=Requested By"`
	ApproverID    sdk.Integer                    `nexa:"string=Approver ID"`
	ApproveDate   sdk.Date                       `nexa:"string=Approved Date"`
	Notes         sdk.Text                       `nexa:"string=Notes"`
}

// ---------------------------------------------------------------------------
// Compensation review cycles + lines
// ---------------------------------------------------------------------------

// HrCompReviewCycle scopes a compensation review to a period and an optional
// department, with a budget cap on individual increases.
type HrCompReviewCycle struct {
	sdk.Model `nexa:"model=hr.comp.review.cycle"`

	Name              sdk.String                      `nexa:"required,string=Cycle Name"`
	CycleYear         sdk.Integer                     `nexa:"string=Year"`
	PeriodStart       sdk.Date                        `nexa:"string=Period Start"`
	PeriodEnd         sdk.Date                        `nexa:"string=Period End"`
	DepartmentID      sdk.Many2One[HrDepartment]      `nexa:"string=Department,help=Leave empty for a company-wide cycle."`
	BudgetIncreasePct sdk.Float64                     `nexa:"string=Budget Increase Cap %,default=0,help=Maximum approved percentage increase per line; 0 means no cap."`
	State             sdk.Selection[ReviewCycleState] `nexa:"string=Status,index,default=planning,selection=planning:Planning,active:Active,review:In Review,locked:Locked,closed:Closed"`
	Lines             sdk.One2Many[HrCompReviewLine]  `nexa:"string=Review Lines,inverse=cycle_id"`
	Notes             sdk.Text                        `nexa:"string=Notes"`
}

// HrCompReviewLine is one employee's proposed change inside a cycle. Approval is
// manager + maker-checker guarded and respects the cycle's budget cap.
type HrCompReviewLine struct {
	sdk.Model `nexa:"model=hr.comp.review.line"`

	CycleID             sdk.Many2One[HrCompReviewCycle] `nexa:"required,string=Cycle,index"`
	EmployeeID          sdk.Many2One[HrEmployee]        `nexa:"required,string=Employee,index"`
	CurrentSalary       sdk.Numeric                     `nexa:"string=Current Salary,precision=12,scale=2,default=0"`
	ProposedSalary      sdk.Numeric                     `nexa:"string=Proposed Salary,precision=12,scale=2,default=0"`
	Rating              sdk.Float64                     `nexa:"string=Performance Rating,default=0"`
	ProposedIncreasePct sdk.Float64                     `nexa:"string=Increase %,compute=compute_review_increase_pct,store"`
	WithinBudget        sdk.Boolean                     `nexa:"string=Within Budget,compute=compute_review_within_budget,store"`
	ReviewerID          sdk.Many2One[HrEmployee]        `nexa:"string=Reviewer"`
	State               sdk.Selection[ReviewLineState]  `nexa:"string=Status,index,default=draft,selection=draft:Draft,recommended:Recommended,approved:Approved,rejected:Rejected"`
	Notes               sdk.Text                        `nexa:"string=Notes"`
}

func init() {
	sdk.MustRegister("hr_benefits",
		&HrBenefitPlan{},
		&HrBenefitEnrollment{},
		&HrBenefitDependent{},
		&HrAllowanceRequest{},
		&HrCompensationChange{},
		&HrCompReviewCycle{},
		&HrCompReviewLine{},
	)
}
