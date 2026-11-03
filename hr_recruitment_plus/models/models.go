package models

import (
	"nexa/core/sdk"
	hrmodels "nexa_addons/hr/models"
	onbmodels "nexa_addons/hr_onboarding/models"
	recmodels "nexa_addons/hr_recruitment/models"
)

// HR-F6 "Recruitment & Onboarding". The base hr_recruitment module already owns
// job openings, interviews and offers, and hr_onboarding owns plans + activities,
// but none of it enforces the production behaviours the blueprint requires:
// requisitions need budget/headcount approval before posting, candidates need
// privacy consent + duplicate detection, interviews need structured multi-criteria
// scorecards (not a single free rating), offers need maker-checker approval +
// letter templating, onboarding needs a reusable checklist template that provisions
// tasks, and hires need a probation period with a confirmation workflow.
//
// This companion adds exactly that WITHOUT migrating the existing tables: new
// models plus non-destructive inherit extensions bolting the extra columns onto
// the anchor opening / applicant / interview / offer / plan / employee records.

type (
	HrEmployee           = hrmodels.HrEmployee
	HrDepartment         = hrmodels.HrDepartment
	HrJob                = hrmodels.HrJob
	HrApplicant          = hrmodels.HrApplicant
	HrJobOpening         = recmodels.HrJobOpening
	HrRecInterview       = recmodels.HrRecruitmentInterview
	HrRecOffer           = recmodels.HrRecruitmentOffer
	HrOnboardingPlan     = onbmodels.HrOnboardingPlan
	HrOnboardingActivity = onbmodels.HrOnboardingActivity
	ResponsibleRole      = onbmodels.ResponsibleRole
)

// ---------------------------------------------------------------------------
// Enumerations
// ---------------------------------------------------------------------------

type RequisitionState string

const (
	ReqDraft    RequisitionState = "draft"
	ReqApproved RequisitionState = "approved"
	ReqRefused  RequisitionState = "refused"
)

type ConfirmationState string

const (
	ConfNone        ConfirmationState = "none"
	ConfInProgress  ConfirmationState = "in_progress"
	ConfConfirmed   ConfirmationState = "confirmed"
	ConfExtended    ConfirmationState = "extended"
	ConfNotRequired ConfirmationState = "not_required"
)

// ---------------------------------------------------------------------------
// Requisition approval (inherit hr.job.opening)
// ---------------------------------------------------------------------------

// HrJobOpeningRequisition turns a job opening into an approved requisition: a
// budget + headcount must be set and an HR Manager must approve (maker-checker)
// before the opening can be posted / filled.
type HrJobOpeningRequisition struct {
	sdk.Model `nexa:"inherit=hr.job.opening"`

	BudgetAnnual    sdk.Numeric `nexa:"string=Annual Budget,precision=18,scale=2,default=0"`
	Headcount       sdk.Integer `nexa:"string=Headcount Requested,default=1"`
	ReqApproved     sdk.Boolean `nexa:"string=Requisition Approved,default=false"`
	ApproverID      sdk.Integer `nexa:"string=Approver ID"`
	ApprovedDate    sdk.Date    `nexa:"string=Approved Date"`
	RejectionReason sdk.Text    `nexa:"string=Rejection Reason"`
}

// ---------------------------------------------------------------------------
// Candidate consent + duplicate detection (inherit hr.applicant)
// ---------------------------------------------------------------------------

// HrApplicantScreening adds GDPR-style consent capture and duplicate detection.
// A created/updated applicant is matched on email against earlier applicants; a
// duplicate is flagged (never silently merged) and linked for the recruiter.
type HrApplicantScreening struct {
	sdk.Model `nexa:"inherit=hr.applicant"`

	ConsentGiven      sdk.Boolean               `nexa:"string=Consent Given,default=false,index"`
	ConsentDate       sdk.Date                  `nexa:"string=Consent Date"`
	DuplicateOfID     sdk.Many2One[HrApplicant] `nexa:"string=Duplicate Of"`
	DuplicateDetected sdk.Boolean               `nexa:"string=Possible Duplicate,default=false,index"`
	ScreeningNote     sdk.Text                  `nexa:"string=Screening Note"`
}

// ---------------------------------------------------------------------------
// Interview scorecards
// ---------------------------------------------------------------------------

// HrInterviewScorecard is one weighted criterion line scored by an interviewer.
type HrInterviewScorecard struct {
	sdk.Model `nexa:"model=hr.recruitment.scorecard"`

	InterviewID sdk.Many2One[HrRecInterview] `nexa:"required,string=Interview,index"`
	Criterion   sdk.String                   `nexa:"required,string=Criterion"`
	MaxScore    sdk.Float64                  `nexa:"string=Max Score,default=5"`
	Score       sdk.Float64                  `nexa:"string=Score,default=0"`
	Weight      sdk.Float64                  `nexa:"string=Weight,default=1"`
	Feedback    sdk.Text                     `nexa:"string=Feedback"`
}

// HrInterviewAssessment adds the weighted-average verdict back onto the interview.
type HrInterviewAssessment struct {
	sdk.Model `nexa:"inherit=hr.recruitment.interview"`

	ScoreLines sdk.One2Many[HrInterviewScorecard] `nexa:"string=Scorecards,inverse=interview_id"`
	AvgScore   sdk.Float64                        `nexa:"string=Average Score,default=0,store"`
	ScorePct   sdk.Float64                        `nexa:"string=Score %,default=0,store"`
	Recommend  sdk.Boolean                        `nexa:"string=Recommend Hire,default=false"`
}

// ---------------------------------------------------------------------------
// Offer approval + letter templating (inherit hr.recruitment.offer) + template
// ---------------------------------------------------------------------------

// HrOfferApproval gates an offer behind maker-checker approval and renders a
// letter from a template before it is sent.
type HrOfferApproval struct {
	sdk.Model `nexa:"inherit=hr.recruitment.offer"`

	Approved    sdk.Boolean                   `nexa:"string=Approved,default=false"`
	ApproverID  sdk.Integer                   `nexa:"string=Approver ID"`
	ApproveDate sdk.Date                      `nexa:"string=Approved Date"`
	TemplateID  sdk.Many2One[HrOfferTemplate] `nexa:"string=Letter Template"`
	Letter      sdk.Text                      `nexa:"string=Offer Letter"`
}

// HrOfferTemplate is a reusable offer letter with {{placeholder}} tokens.
type HrOfferTemplate struct {
	sdk.Model `nexa:"model=hr.recruitment.offer.template"`

	Name    sdk.String  `nexa:"required,string=Name"`
	Subject sdk.String  `nexa:"string=Subject"`
	Body    sdk.Text    `nexa:"required,string=Body"`
	Active  sdk.Boolean `nexa:"string=Active,default=true"`
}

// ---------------------------------------------------------------------------
// Onboarding templates + provisioning (inherit hr.onboarding.plan)
// ---------------------------------------------------------------------------

// HrOnboardingTemplate is a reusable checklist definition.
type HrOnboardingTemplate struct {
	sdk.Model `nexa:"model=hr.onboarding.template"`

	Name        sdk.String  `nexa:"required,string=Name"`
	Active      sdk.Boolean `nexa:"string=Active,default=true"`
	Description sdk.Text    `nexa:"string=Description"`
}

// HrOnboardingTemplateLine is one default activity in a template; DueOffsetDays is
// measured from the plan start date.
type HrOnboardingTemplateLine struct {
	sdk.Model `nexa:"model=hr.onboarding.template.line"`

	TemplateID    sdk.Many2One[HrOnboardingTemplate] `nexa:"required,string=Template,index"`
	Name          sdk.String                         `nexa:"required,string=Activity"`
	Responsible   sdk.Selection[ResponsibleRole]     `nexa:"string=Responsible Party,default=hr"`
	DueOffsetDays sdk.Integer                        `nexa:"string=Due Offset (days),default=0"`
	Sequence      sdk.Integer                        `nexa:"string=Sequence,default=10"`
	Active        sdk.Boolean                        `nexa:"string=Active,default=true"`
}

// HrOnboardingPlanExtension lets a plan be seeded from a template and track
// whether provisioning has run (so instantiate is idempotent).
type HrOnboardingPlanExtension struct {
	sdk.Model `nexa:"inherit=hr.onboarding.plan"`

	TemplateID  sdk.Many2One[HrOnboardingTemplate] `nexa:"string=Checklist Template"`
	Provisioned sdk.Boolean                        `nexa:"string=Provisioned,default=false"`
}

// ---------------------------------------------------------------------------
// Probation + confirmation (inherit hr.employee)
// ---------------------------------------------------------------------------

// HrEmployeeProbation records the probation window and confirmation outcome of a
// hire. Confirmation is a guarded, auditable transition (not automatic).
type HrEmployeeProbation struct {
	sdk.Model `nexa:"inherit=hr.employee"`

	ProbationStart    sdk.Date                         `nexa:"string=Probation Start"`
	ProbationEnd      sdk.Date                         `nexa:"string=Probation End,index"`
	ConfirmationState sdk.Selection[ConfirmationState] `nexa:"string=Confirmation,default=not_required,selection=none:Not Set,in_progress:In Probation,confirmed:Confirmed,extended:Extended,not_required:Not Required"`
	ConfirmedDate     sdk.Date                         `nexa:"string=Confirmed Date"`
	ConfirmationNote  sdk.Text                         `nexa:"string=Confirmation Note"`
}

func init() {
	sdk.MustRegister("hr_recruitment_plus",
		&HrJobOpeningRequisition{},
		&HrApplicantScreening{},
		&HrInterviewScorecard{},
		&HrInterviewAssessment{},
		&HrOfferApproval{},
		&HrOfferTemplate{},
		&HrOnboardingTemplate{},
		&HrOnboardingTemplateLine{},
		&HrOnboardingPlanExtension{},
		&HrEmployeeProbation{},
	)
}
