package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	hrmodels "nexa_addons/hr/models"
	schoolmodels "nexa_addons/school/models"
	academicsmodels "nexa_addons/school_academics/models"
	setupmodels "nexa_addons/school_setup/models"
)

// School Staff (blueprint section 8). The school does not reinvent HR: a staff member is
// a school-side profile layered on top of the reused HR record (hr.employee, which itself
// links to the login user and org department). On top of that profile the school tracks
// teaching qualifications (verified by a manager), substitutions when a teacher is away
// (reusing the academics teaching assignment as the class being covered), and classroom
// performance reviews with a submit/approve gate. Workload and leave are NOT new engines:
// weekly teaching periods are aggregated from school.teaching.assignment rows and leave is
// read straight from hr.leave, all surfaced as live computes on the profile.

type (
	CoreUser                 = basemodels.CoreUser
	CoreCompany              = basemodels.CoreCompany
	HrEmployee               = hrmodels.HrEmployee
	HrDepartment             = hrmodels.HrDepartment
	SchoolClass              = schoolmodels.SchoolClass
	SchoolSection            = schoolmodels.SchoolSection
	SchoolSubject            = academicsmodels.SchoolSubject
	SchoolTeachingAssignment = academicsmodels.SchoolTeachingAssignment
	SchoolCampus             = setupmodels.SchoolCampus
)

type StaffCategory string

const (
	CatTeacher        StaffCategory = "teacher"
	CatAdministrative StaffCategory = "administrative"
	CatSupport        StaffCategory = "support"
	CatLeadership     StaffCategory = "leadership"
	CatOther          StaffCategory = "other"
)

type QualType string

const (
	QDegree        QualType = "degree"
	QCertification QualType = "certification"
	QLicense       QualType = "license"
	QDiploma       QualType = "diploma"
	QOther         QualType = "other"
)

type SubstState string

const (
	SubRequested SubstState = "requested"
	SubAssigned  SubstState = "assigned"
	SubCompleted SubstState = "completed"
	SubCancelled SubstState = "cancelled"
)

type PerfState string

const (
	PerfDraft     PerfState = "draft"
	PerfSubmitted PerfState = "submitted"
	PerfApproved  PerfState = "approved"
)

// SchoolStaffMember is the school-side profile of a reused HR employee.
type SchoolStaffMember struct {
	sdk.Model `nexa:"model=school.staff.member"`

	StaffCode  sdk.String                   `nexa:"required,index,size=40,string=Staff Code"`
	EmployeeID sdk.Many2One[HrEmployee]     `nexa:"required,index,string=Employee (HR)"`
	TeacherID  sdk.Many2One[CoreUser]       `nexa:"index,string=System User (Teacher)"`
	Category   sdk.Selection[StaffCategory] `nexa:"string=Category,default=teacher,index,selection=teacher:Teacher,administrative:Administrative,support:Support,leadership:Leadership,other:Other"`

	CampusID     sdk.Many2One[SchoolCampus] `nexa:"index,string=Campus"`
	DepartmentID sdk.Many2One[HrDepartment] `nexa:"index,string=Department"`
	JobTitle     sdk.String                 `nexa:"size=120,string=Job Title"`
	StartDate    sdk.Date                   `nexa:"string=Start Date"`
	EndDate      sdk.Date                   `nexa:"string=End Date"`

	Qualifications sdk.One2Many[SchoolStaffQualification] `nexa:"string=Qualifications,inverse=staff_id"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true"`

	// Non-stored signals pulled from other modules.
	QualificationCount sdk.Integer `nexa:"string=Qualifications,compute=compute_staff_qual_count"`
	WeeklyPeriods      sdk.Integer `nexa:"string=Teaching Periods/wk,compute=compute_staff_weekly_periods"`
	OpenLeaveRequests  sdk.Integer `nexa:"string=Open Leave,compute=compute_staff_open_leave"`
	OnLeave            sdk.Boolean `nexa:"string=On Leave Today,compute=compute_staff_on_leave"`
}

// SchoolStaffQualification is a credential held (and manager-verified) for a staff member.
type SchoolStaffQualification struct {
	sdk.Model `nexa:"model=school.staff.qualification"`

	StaffID      sdk.Many2One[SchoolStaffMember] `nexa:"required,index,string=Staff"`
	QualType     sdk.Selection[QualType]         `nexa:"string=Type,default=degree,selection=degree:Degree,certification:Certification,license:License,diploma:Diploma,other:Other"`
	Title        sdk.String                      `nexa:"required,size=200,string=Title"`
	Institution  sdk.String                      `nexa:"size=150,string=Institution"`
	YearObtained sdk.Integer                     `nexa:"string=Year"`
	ExpiryDate   sdk.Date                        `nexa:"index,string=Expiry Date"`
	ReferenceNo  sdk.String                      `nexa:"size=60,string=Reference No"`

	Verified     sdk.Boolean            `nexa:"string=Verified,default=false,index"`
	VerifiedByID sdk.Many2One[CoreUser] `nexa:"string=Verified By"`

	Notes sdk.Text `nexa:"string=Notes"`
}

// SchoolStaffSubstitution records another teacher covering a class while the assigned
// teacher is away; the covered class is referenced through the academics assignment.
type SchoolStaffSubstitution struct {
	sdk.Model `nexa:"model=school.staff.substitution"`

	AbsentStaffID     sdk.Many2One[SchoolStaffMember]        `nexa:"required,index,string=Absent Teacher"`
	SubstituteStaffID sdk.Many2One[SchoolStaffMember]        `nexa:"required,index,string=Substitute"`
	AssignmentID      sdk.Many2One[SchoolTeachingAssignment] `nexa:"index,string=Assignment Covered"`
	SubjectID         sdk.Many2One[SchoolSubject]            `nexa:"index,string=Subject"`
	ClassID           sdk.Many2One[SchoolClass]              `nexa:"index,string=Class"`
	SectionID         sdk.Many2One[SchoolSection]            `nexa:"index,string=Section"`

	DateFrom     sdk.Date                  `nexa:"required,index,string=From Date"`
	DateTo       sdk.Date                  `nexa:"required,index,string=To Date"`
	PeriodNumber sdk.Integer               `nexa:"string=Period,default=0"`
	State        sdk.Selection[SubstState] `nexa:"string=Status,default=requested,index,selection=requested:Requested,assigned:Assigned,completed:Completed,cancelled:Cancelled"`

	Reason    sdk.Text                  `nexa:"string=Reason"`
	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
}

// SchoolStaffPerformanceReview is a periodic classroom/performance appraisal of a member
// with a draft -> submitted -> approved review. The overall rating is derived from the
// observation score through the grading engine.
type SchoolStaffPerformanceReview struct {
	sdk.Model `nexa:"model=school.staff.performance.review"`

	StaffID    sdk.Many2One[SchoolStaffMember] `nexa:"required,index,string=Staff"`
	Period     sdk.String                      `nexa:"required,size=40,string=Review Period"`
	ReviewerID sdk.Many2One[CoreUser]          `nexa:"index,string=Reviewer"`

	ObservationScore sdk.Float64 `nexa:"string=Observation Score (0-100),default=0"`
	OverallRating    sdk.String  `nexa:"string=Rating,compute=compute_perf_rating"`

	Strengths    sdk.Text                 `nexa:"string=Strengths"`
	Improvements sdk.Text                 `nexa:"string=Areas To Improve"`
	State        sdk.Selection[PerfState] `nexa:"string=Status,default=draft,selection=draft:Draft,submitted:Submitted,approved:Approved,index"`

	ReviewedByID sdk.Many2One[CoreUser] `nexa:"string=Approved By"`
	ReviewDate   sdk.Date               `nexa:"string=Review Date"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
}

func init() {
	sdk.MustRegister("school_staff",
		&SchoolStaffMember{},
		&SchoolStaffQualification{},
		&SchoolStaffSubstitution{},
		&SchoolStaffPerformanceReview{},
	)
}
