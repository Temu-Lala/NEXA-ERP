package models

import (
	"nexa/core/sdk"
	hrmodels "nexa_addons/hr/models"
	skillmodels "nexa_addons/hr_skill/models"
	trmodels "nexa_addons/hr_training/models"
)

// HR-F8 "Learning & Development". The base hr_training module owns a course
// catalog, scheduled sessions, enrollments and certificates, but none of the
// production depth the blueprint requires: providers are not modelled, training
// spend has no budget-approved request flow, enrollments track no attendance,
// there are no learning paths, no exams / graded attempts, no scheduled
// certificate-expiry reminders, and the skills a job demands is never compared
// against the skills an employee has (gap detection). This companion adds exactly
// that WITHOUT migrating the hr_training tables: it reuses the course / session /
// enrollment / certificate and hr_skill records (never recreated) via
// non-destructive inherit extensions plus new models, and every cost- or
// decision-bearing transition is a guarded, maker-checker step.

type (
	HrEmployee            = hrmodels.HrEmployee
	HrJob                 = hrmodels.HrJob
	HrSkill               = skillmodels.HrSkill
	SkillLevel            = skillmodels.SkillLevel
	HrTrainingCourse      = trmodels.HrTrainingCourse
	HrTrainingSession     = trmodels.HrTrainingSession
	HrTrainingEnrollment  = trmodels.HrTrainingEnrollment
	HrTrainingCertificate = trmodels.HrTrainingCertificate
)

// ---------------------------------------------------------------------------
// Enumerations
// ---------------------------------------------------------------------------

type ProviderType string

const (
	ProviderInternal ProviderType = "internal"
	ProviderExternal ProviderType = "external"
)

type RequestState string

const (
	ReqDraft     RequestState = "draft"
	ReqSubmitted RequestState = "submitted"
	ReqApproved  RequestState = "approved"
	ReqRefused   RequestState = "refused"
	ReqCancelled RequestState = "cancelled"
)

type AttendanceStatus string

const (
	AttNA      AttendanceStatus = "na"
	AttPresent AttendanceStatus = "present"
	AttAbsent  AttendanceStatus = "absent"
	AttPartial AttendanceStatus = "partial"
)

type AttemptState string

const (
	AttInProgress AttemptState = "in_progress"
	AttSubmitted  AttemptState = "submitted"
	AttScored     AttemptState = "scored"
)

type GapState string

const (
	GapOpen      GapState = "open"
	GapAddressed GapState = "addressed"
)

// ---------------------------------------------------------------------------
// Providers
// ---------------------------------------------------------------------------

// HrTrainingProvider is an internal department or external vendor delivering a
// course.
type HrTrainingProvider struct {
	sdk.Model `nexa:"model=hr.training.provider"`

	Name    sdk.String                  `nexa:"required,string=Name"`
	Type    sdk.Selection[ProviderType] `nexa:"string=Type,default=internal,selection=internal:Internal,external:External"`
	Contact sdk.String                  `nexa:"string=Contact"`
	Email   sdk.String                  `nexa:"string=Email"`
	Active  sdk.Boolean                 `nexa:"string=Active,default=true"`
}

// ---------------------------------------------------------------------------
// Course extension (inherit hr.training.course)
// ---------------------------------------------------------------------------

// HrTrainingCourseExtension links a course to a provider and captures whether it
// yields a certificate and that certificate's validity window (drives expiry
// reminders).
type HrTrainingCourseExtension struct {
	sdk.Model `nexa:"inherit=hr.training.course"`

	ProviderID            sdk.Many2One[HrTrainingProvider] `nexa:"string=Provider"`
	CertificationRequired sdk.Boolean                      `nexa:"string=Awards Certificate,default=false"`
	ValidityMonths        sdk.Integer                      `nexa:"string=Certificate Validity (months),default=0"`
}

// ---------------------------------------------------------------------------
// Training requests + budget approval (new)
// ---------------------------------------------------------------------------

// HrTrainingRequest is a budget-gated request to put an employee through a
// course. Approval is maker-checker (an HR Manager approves a request they did
// not raise); approving records a decision and does not post any cost.
type HrTrainingRequest struct {
	sdk.Model `nexa:"model=hr.training.request"`

	EmployeeID    sdk.Many2One[HrEmployee]        `nexa:"required,string=Employee,index"`
	CourseID      sdk.Many2One[HrTrainingCourse]  `nexa:"string=Course,index"`
	SessionID     sdk.Many2One[HrTrainingSession] `nexa:"string=Session"`
	Reason        sdk.Text                        `nexa:"string=Justification"`
	EstimatedCost sdk.Numeric                     `nexa:"string=Estimated Cost,precision=12,scale=2,default=0"`
	BudgetCode    sdk.String                      `nexa:"string=Budget Code"`
	RequestDate   sdk.Date                        `nexa:"string=Request Date"`
	State         sdk.Selection[RequestState]     `nexa:"string=Status,default=draft,selection=draft:Draft,submitted:Submitted,approved:Approved,refused:Refused,cancelled:Cancelled"`
	RequestedBy   sdk.Integer                     `nexa:"string=Requested By"`
	ApproverID    sdk.Integer                     `nexa:"string=Approved By"`
	ApproveDate   sdk.Date                        `nexa:"string=Approval Date"`
}

// ---------------------------------------------------------------------------
// Enrollment attendance (inherit hr.training.enrollment)
// ---------------------------------------------------------------------------

// HrTrainingEnrollmentAttendance records attendance and hours actually attended
// against an enrollment, making a payroll-ready training summary possible.
type HrTrainingEnrollmentAttendance struct {
	sdk.Model `nexa:"inherit=hr.training.enrollment"`

	Attendance    sdk.Selection[AttendanceStatus] `nexa:"string=Attendance,default=na,selection=na:Not Marked,present:Present,absent:Absent,partial:Partial"`
	HoursAttended sdk.Float64                     `nexa:"string=Hours Attended,default=0"`
}

// ---------------------------------------------------------------------------
// Learning paths
// ---------------------------------------------------------------------------

// HrTrainingPath is an ordered curriculum of courses.
type HrTrainingPath struct {
	sdk.Model `nexa:"model=hr.training.path"`

	Name        sdk.String                         `nexa:"required,string=Path"`
	Description sdk.Text                           `nexa:"string=Description"`
	Active      sdk.Boolean                        `nexa:"string=Active,default=true"`
	Courses     sdk.One2Many[HrTrainingPathCourse] `nexa:"string=Courses,inverse=path_id"`
}

// HrTrainingPathCourse is one ordered course in a learning path.
type HrTrainingPathCourse struct {
	sdk.Model `nexa:"model=hr.training.path.course"`

	PathID    sdk.Many2One[HrTrainingPath]   `nexa:"required,string=Path,index"`
	CourseID  sdk.Many2One[HrTrainingCourse] `nexa:"required,string=Course"`
	Sequence  sdk.Integer                    `nexa:"string=Sequence,default=10"`
	Mandatory sdk.Boolean                    `nexa:"string=Mandatory,default=true"`
}

// ---------------------------------------------------------------------------
// Exams + graded attempts
// ---------------------------------------------------------------------------

// HrTrainingExam is an assessment tied to a course with a pass mark.
type HrTrainingExam struct {
	sdk.Model `nexa:"model=hr.training.exam"`

	Name         sdk.String                           `nexa:"required,string=Exam"`
	CourseID     sdk.Many2One[HrTrainingCourse]       `nexa:"string=Course"`
	PassMark     sdk.Float64                          `nexa:"string=Pass Mark %,default=0"`
	DurationMins sdk.Integer                          `nexa:"string=Duration (mins),default=60"`
	Active       sdk.Boolean                          `nexa:"string=Active,default=true"`
	Questions    sdk.One2Many[HrTrainingExamQuestion] `nexa:"string=Questions,inverse=exam_id"`
}

// HrTrainingExamQuestion is one scored question of an exam.
type HrTrainingExamQuestion struct {
	sdk.Model `nexa:"model=hr.training.exam.question"`

	ExamID   sdk.Many2One[HrTrainingExam] `nexa:"required,string=Exam,index"`
	Question sdk.Text                     `nexa:"required,string=Question"`
	MaxScore sdk.Float64                  `nexa:"string=Max Score,default=1"`
	Sequence sdk.Integer                  `nexa:"string=Sequence,default=10"`
}

// HrTrainingAttempt is one employee's graded sitting of an exam; percentage and
// pass are derived from the total against the max and the exam's pass mark.
type HrTrainingAttempt struct {
	sdk.Model `nexa:"model=hr.training.attempt"`

	ExamID      sdk.Many2One[HrTrainingExam] `nexa:"required,string=Exam,index"`
	EmployeeID  sdk.Many2One[HrEmployee]     `nexa:"required,string=Employee,index"`
	AttemptDate sdk.DateTime                 `nexa:"string=Attempt Date"`
	TotalScore  sdk.Float64                  `nexa:"string=Total Score,default=0"`
	MaxScore    sdk.Float64                  `nexa:"string=Max Score,default=0"`
	Percentage  sdk.Float64                  `nexa:"string=Percentage,compute=compute_attempt_percentage,store"`
	Passed      sdk.Boolean                  `nexa:"string=Passed,compute=compute_attempt_passed,store"`
	State       sdk.Selection[AttemptState]  `nexa:"string=Status,default=in_progress,selection=in_progress:In Progress,submitted:Submitted,scored:Scored"`
}

// ---------------------------------------------------------------------------
// Skill / competency gap detection
// ---------------------------------------------------------------------------

// HrSkillGap snapshots the difference between the level a job requires for a
// skill and the level the employee currently has (a positive gap is a shortfall).
type HrSkillGap struct {
	sdk.Model `nexa:"model=hr.skill.gap"`

	EmployeeID    sdk.Many2One[HrEmployee]  `nexa:"required,string=Employee,index"`
	JobID         sdk.Many2One[HrJob]       `nexa:"string=Job"`
	SkillID       sdk.Many2One[HrSkill]     `nexa:"required,string=Skill,index"`
	CurrentLevel  sdk.Selection[SkillLevel] `nexa:"string=Current Level,default=beginner"`
	RequiredLevel sdk.Selection[SkillLevel] `nexa:"string=Required Level,default=beginner"`
	Gap           sdk.Integer               `nexa:"string=Gap,compute=compute_skill_gap,store"`
	DetectedDate  sdk.Date                  `nexa:"string=Detected"`
	State         sdk.Selection[GapState]   `nexa:"string=Status,default=open,selection=open:Open,addressed:Addressed"`
}

// ---------------------------------------------------------------------------
// Certificate expiry reminders
// ---------------------------------------------------------------------------

// HrTrainingCertAlert is a scan-produced reminder for a certificate approaching /
// past its expiry, created idempotently by the cron while still open.
type HrTrainingCertAlert struct {
	sdk.Model `nexa:"model=hr.training.cert.alert"`

	EmployeeID    sdk.Many2One[HrEmployee]            `nexa:"required,string=Employee,index"`
	CertificateID sdk.Many2One[HrTrainingCertificate] `nexa:"string=Certificate,index"`
	CourseID      sdk.Many2One[HrTrainingCourse]      `nexa:"string=Course"`
	ExpiryDate    sdk.Date                            `nexa:"string=Expiry Date,index"`
	DaysRemaining sdk.Integer                         `nexa:"string=Days Remaining"`
	Message       sdk.Text                            `nexa:"string=Message"`
	Open          sdk.Boolean                         `nexa:"string=Open,default=true,index"`
}

func init() {
	sdk.MustRegister("hr_training_plus",
		&HrTrainingProvider{},
		&HrTrainingCourseExtension{},
		&HrTrainingRequest{},
		&HrTrainingEnrollmentAttendance{},
		&HrTrainingPath{},
		&HrTrainingPathCourse{},
		&HrTrainingExam{},
		&HrTrainingExamQuestion{},
		&HrTrainingAttempt{},
		&HrSkillGap{},
		&HrTrainingCertAlert{},
	)
}
