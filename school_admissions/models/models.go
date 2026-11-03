package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	schoolmodels "nexa_addons/school/models"
	setupmodels "nexa_addons/school_setup/models"
)

// School Admissions (blueprint section 2). An applicant and an enrolled student are
// deliberately different lifecycle objects: moving an application to "offered" or
// even "accepted" does NOT create a student, assign a class seat or take payment.
// Only the explicit, Manager-gated "Enroll" action materialises a school.student,
// and it is guarded by the target section's remaining capacity. Every state change
// is recorded on an append-only decision log so the admission trail is auditable.
// Application fees are captured as references only — actual invoicing/payments stay
// in the Payments domain (ownership rule), never reimplemented here.

type (
	CoreUser           = basemodels.CoreUser
	CoreCompany        = basemodels.CoreCompany
	SchoolClass        = schoolmodels.SchoolClass
	SchoolSection      = schoolmodels.SchoolSection
	SchoolAcademicYear = schoolmodels.SchoolAcademicYear
	SchoolStudent      = schoolmodels.SchoolStudent
	SchoolCampus       = setupmodels.SchoolCampus
)

// ApplicantState is the admission pipeline status. Values are referenced by the
// transition engine in engine.go.
type ApplicantState string

const (
	StateDraft      ApplicantState = "draft"
	StateSubmitted  ApplicantState = "submitted"
	StateReview     ApplicantState = "under_review"
	StateAssessment ApplicantState = "assessment"
	StateOffered    ApplicantState = "offered"
	StateWaitlisted ApplicantState = "waitlisted"
	StateAccepted   ApplicantState = "accepted"
	StateRejected   ApplicantState = "rejected"
	StateEnrolled   ApplicantState = "enrolled"
	StateCancelled  ApplicantState = "cancelled"
	StateExpired    ApplicantState = "expired"
)

// ApplicantSource records how the application arrived.
type ApplicantSource string

const (
	SrcOnline   ApplicantSource = "online"
	SrcWalkIn   ApplicantSource = "walkin"
	SrcReferral ApplicantSource = "referral"
	SrcAgent    ApplicantSource = "agent"
)

// Gender mirrors the anchor's student gender vocabulary.
type Gender string

const (
	GenderMale   Gender = "male"
	GenderFemale Gender = "female"
	GenderOther  Gender = "other"
)

// SchoolApplicant is a single admission application moving through the pipeline.
// The code carries the human application number; the exam/interview scores feed an
// optional ranking; decision fields record who decided and when; StudentID links the
// created learner only after the Enroll action runs.
type SchoolApplicant struct {
	sdk.Model `nexa:"model=school.applicant"`

	ApplicationNo sdk.String                     `nexa:"required,index,size=40,string=Application No"`
	FirstName     sdk.String                     `nexa:"required,size=120,string=First Name"`
	LastName      sdk.String                     `nexa:"required,size=120,string=Last Name"`
	Gender        sdk.Selection[Gender]          `nexa:"string=Gender,selection=male:Male,female:Female,other:Other"`
	DateBirth     sdk.Date                       `nexa:"string=Date of Birth"`
	Source        sdk.Selection[ApplicantSource] `nexa:"string=Source,default=walkin,selection=online:Online,walkin:Walk-in,referral:Referral,agent:Agent"`

	CampusID       sdk.Many2One[SchoolCampus]       `nexa:"index,string=Campus"`
	AcademicYearID sdk.Many2One[SchoolAcademicYear] `nexa:"index,string=Academic Year"`
	ClassID        sdk.Many2One[SchoolClass]        `nexa:"index,string=Class Applying For"`
	SectionID      sdk.Many2One[SchoolSection]      `nexa:"index,string=Section"`

	GuardianName  sdk.String `nexa:"size=120,string=Guardian Name"`
	GuardianPhone sdk.String `nexa:"size=40,string=Guardian Phone"`
	GuardianEmail sdk.String `nexa:"size=120,string=Guardian Email"`

	PreviousSchool sdk.String  `nexa:"size=200,string=Previous School"`
	ExamScore      sdk.Float64 `nexa:"string=Entrance Exam Score,default=0"`
	InterviewScore sdk.Float64 `nexa:"string=Interview Score,default=0"`

	ApplicationFee sdk.Numeric `nexa:"string=Application Fee,precision=18,scale=2,default=0"`
	FeePaid        sdk.Boolean `nexa:"string=Application Fee Paid,default=false"`

	State        sdk.Selection[ApplicantState] `nexa:"string=Status,default=draft,index,selection=draft:Draft,submitted:Submitted,under_review:Under Review,assessment:Assessment,offered:Offered,waitlisted:Waitlisted,accepted:Accepted,rejected:Rejected,enrolled:Enrolled,cancelled:Cancelled,expired:Expired"`
	WaitlistRank sdk.Integer                   `nexa:"string=Waitlist Rank,default=0"`

	DecisionByID sdk.Many2One[CoreUser]      `nexa:"string=Decided By"`
	DecisionDate sdk.Date                    `nexa:"string=Decision Date"`
	OfferExpiry  sdk.Date                    `nexa:"string=Offer Expiry Date"`
	StudentID    sdk.Many2One[SchoolStudent] `nexa:"string=Enrolled Student"`
	AppliedDate  sdk.Date                    `nexa:"string=Applied Date,default=today"`
	CompanyID    sdk.Many2One[CoreCompany]   `nexa:"string=Company"`
	Notes        sdk.Text                    `nexa:"string=Notes"`
	Active       sdk.Boolean                 `nexa:"string=Active,default=true"`

	// Inline one-to-many lines (defined after the child models in this package).
	Documents sdk.One2Many[SchoolApplicationDocument] `nexa:"string=Documents,inverse=applicant_id"`
	Decisions sdk.One2Many[SchoolAdmissionDecision]   `nexa:"string=Decision Trail,inverse=applicant_id"`

	// Computed (non-stored) read-only signals for the UI.
	DocsComplete     sdk.Boolean `nexa:"string=Documents Complete,compute=compute_applicant_docs"`
	DecisionCount    sdk.Integer `nexa:"string=Decision Trail,compute=compute_applicant_decisions"`
	DuplicateSuspect sdk.Boolean `nexa:"string=Duplicate Suspect,compute=compute_applicant_duplicate"`
	SeatAvailable    sdk.Boolean `nexa:"string=Seat Available,compute=compute_applicant_seat"`
}

// SchoolApplicationDocument is one item on an applicant's required-document
// checklist. Required+Received drive the "Documents Complete" gate before an offer.
type SchoolApplicationDocument struct {
	sdk.Model `nexa:"model=school.application.document"`

	ApplicantID sdk.Many2One[SchoolApplicant] `nexa:"required,index,string=Application"`
	Name        sdk.String                    `nexa:"required,size=200,string=Document"`
	Required    sdk.Boolean                   `nexa:"string=Required,default=true"`
	Received    sdk.Boolean                   `nexa:"string=Received,default=false"`
	Notes       sdk.Text                      `nexa:"string=Notes"`
}

// SchoolAdmissionDecision is the append-only audit trail of every state change.
// Rows are created by the transition engine and are never edited or deleted by users.
type SchoolAdmissionDecision struct {
	sdk.Model `nexa:"model=school.admission.decision"`

	ApplicantID sdk.Many2One[SchoolApplicant] `nexa:"required,index,string=Application"`
	FromState   sdk.String                    `nexa:"size=30,string=From"`
	ToState     sdk.String                    `nexa:"size=30,string=To,index"`
	Action      sdk.String                    `nexa:"size=60,string=Action"`
	Reason      sdk.Text                      `nexa:"string=Reason"`
	OperatorID  sdk.Integer                   `nexa:"string=Operator"`
	Timestamp   sdk.DateTime                  `nexa:"string=Timestamp"`
}

func init() {
	sdk.MustRegister("school_admissions",
		&SchoolApplicant{},
		&SchoolApplicationDocument{},
		&SchoolAdmissionDecision{},
	)
}
