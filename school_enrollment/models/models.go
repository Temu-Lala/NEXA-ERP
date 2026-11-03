package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	schoolmodels "nexa_addons/school/models"
	setupmodels "nexa_addons/school_setup/models"
)

// School Enrollment (blueprint section 6). An enrollment is a student's placement for
// ONE academic year in a class/section/campus and is modelled separately from the
// permanent student profile: the same learner has a chain of enrollments across years.
// Enrollment is also distinct from admission — the applicant pipeline (school.
// admissions) materialises the student, but placing them in a year/section, moving
// them between sections (transfer), pulling them out (withdrawal) and finishing them
// (graduation -> alumni) all live here. Transfers and graduation never rewrite history:
// a transfer closes the current enrollment and opens a new one, and every state move
// is captured on an append-only enrollment-history log so academic records stay
// traceable. Duplicate protection blocks two active enrollments for the same learner
// in the same academic year.

type (
	CoreUser           = basemodels.CoreUser
	CoreCompany        = basemodels.CoreCompany
	SchoolStudent      = schoolmodels.SchoolStudent
	SchoolClass        = schoolmodels.SchoolClass
	SchoolSection      = schoolmodels.SchoolSection
	SchoolAcademicYear = schoolmodels.SchoolAcademicYear
	SchoolCampus       = setupmodels.SchoolCampus
)

// EnrollmentState is the placement lifecycle of a single enrollment record.
type EnrollmentState string

const (
	EnrDraft       EnrollmentState = "draft"
	EnrActive      EnrollmentState = "active"
	EnrTransferred EnrollmentState = "transferred"
	EnrWithdrawn   EnrollmentState = "withdrawn"
	EnrGraduated   EnrollmentState = "graduated"
	EnrCancelled   EnrollmentState = "cancelled"
)

// AlumniClassification is the graduation standing recorded on the alumni ledger.
type AlumniClassification string

const (
	GradPass        AlumniClassification = "pass"
	GradDistinction AlumniClassification = "distinction"
	GradHonors      AlumniClassification = "honors"
	GradGreatHonors AlumniClassification = "great_honors"
)

// SchoolEnrollment is one learner's placement for an academic year.
type SchoolEnrollment struct {
	sdk.Model `nexa:"model=school.enrollment"`

	EnrollmentNo sdk.String `nexa:"required,index,size=40,string=Enrollment No"`

	StudentID      sdk.Many2One[SchoolStudent]      `nexa:"required,index,string=Student"`
	AcademicYearID sdk.Many2One[SchoolAcademicYear] `nexa:"required,index,string=Academic Year"`
	CampusID       sdk.Many2One[SchoolCampus]       `nexa:"string=Campus"`
	ClassID        sdk.Many2One[SchoolClass]        `nexa:"index,string=Class"`
	SectionID      sdk.Many2One[SchoolSection]      `nexa:"index,string=Section"`

	EnrollDate sdk.Date                       `nexa:"string=Enroll Date,default=today"`
	ExitDate   sdk.Date                       `nexa:"string=Exit Date"`
	RollNumber sdk.String                     `nexa:"size=40,string=Roll Number"`
	State      sdk.Selection[EnrollmentState] `nexa:"string=Status,default=draft,index,selection=draft:Draft,active:Active,transferred:Transferred,withdrawn:Withdrawn,graduated:Graduated,cancelled:Cancelled"`

	Source    sdk.String                `nexa:"size=40,string=Source (admission/import)"`
	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active    sdk.Boolean               `nexa:"string=Record Active,default=true"`

	// Append-only placement/status history (child FK enrollment_id).
	History sdk.One2Many[SchoolEnrollmentHistory] `nexa:"string=History,inverse=enrollment_id"`

	// Non-stored: whether this enrollment is the learner's live placement right now.
	IsCurrent sdk.Boolean `nexa:"string=Current,compute=compute_enrollment_current"`
}

// SchoolEnrollmentHistory is the append-only trail of enrollment state moves and
// placement changes (transfer from/to). Rows are written by the engine and are never
// edited or deleted by users.
type SchoolEnrollmentHistory struct {
	sdk.Model `nexa:"model=school.enrollment.history"`

	EnrollmentID  sdk.Many2One[SchoolEnrollment] `nexa:"required,index,string=Enrollment"`
	StudentID     sdk.Many2One[SchoolStudent]    `nexa:"index,string=Student"`
	FromState     sdk.String                     `nexa:"size=30,string=From"`
	ToState       sdk.String                     `nexa:"size=30,string=To,index"`
	Action        sdk.String                     `nexa:"size=60,string=Action"`
	FromSectionID sdk.Many2One[SchoolSection]    `nexa:"string=From Section"`
	ToSectionID   sdk.Many2One[SchoolSection]    `nexa:"string=To Section"`
	Reason        sdk.Text                       `nexa:"string=Reason"`
	OperatorID    sdk.Integer                    `nexa:"string=Operator"`
	Timestamp     sdk.DateTime                   `nexa:"string=Timestamp"`
}

// SchoolAlumni is the graduate / alumni ledger, created by the Graduation action and
// kept for life after the enrollment chain closes. It also carries the graduation
// certificate reference the blueprint calls for.
type SchoolAlumni struct {
	sdk.Model `nexa:"model=school.alumni"`

	StudentID      sdk.Many2One[SchoolStudent]      `nexa:"required,index,string=Student"`
	EnrollmentID   sdk.Many2One[SchoolEnrollment]   `nexa:"string=Final Enrollment"`
	AcademicYearID sdk.Many2One[SchoolAcademicYear] `nexa:"string=Graduation Year"`

	GraduationDate sdk.Date                            `nexa:"string=Graduation Date,default=today"`
	Classification sdk.Selection[AlumniClassification] `nexa:"string=Classification,default=pass,selection=pass:Pass,distinction:Distinction,honors:Honors,great_honors:Great Honors"`
	CertificateNo  sdk.String                          `nexa:"index,size=60,string=Certificate No"`

	ContactEmail sdk.String `nexa:"size=120,string=Contact Email"`
	ContactPhone sdk.String `nexa:"size=40,string=Contact Phone"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true"`
}

func init() {
	sdk.MustRegister("school_enrollment",
		&SchoolEnrollment{},
		&SchoolEnrollmentHistory{},
		&SchoolAlumni{},
	)
}
