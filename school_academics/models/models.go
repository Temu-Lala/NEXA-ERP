package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	schoolmodels "nexa_addons/school/models"
	setupmodels "nexa_addons/school_setup/models"
)

// School Academics (blueprint section 3). This models the academic structure and, most
// importantly, a reliable timetable conflict engine that validates a lesson before it
// can be published. A subject is a reusable registry entry; a curriculum is a versioned
// offering of a subject for a grade/year with learning outcomes and a subject-level
// prerequisite graph. A teaching assignment binds a teacher to a subject for a class and
// section (and carries the teacher's allowed weekdays and weekly period workload). A
// timetable entry schedules one lesson (weekday + period + duration + classroom) and is
// blocked from publishing when it would double-book the teacher, the section or the room,
// fall outside the teacher's availability, or exceed the classroom capacity. Lesson plans
// attach topics/objectives/materials to an assignment with a submit/approve flow.

type (
	CoreUser           = basemodels.CoreUser
	CoreCompany        = basemodels.CoreCompany
	SchoolClass        = schoolmodels.SchoolClass
	SchoolSection      = schoolmodels.SchoolSection
	SchoolAcademicYear = schoolmodels.SchoolAcademicYear
	SchoolCampus       = setupmodels.SchoolCampus
	SchoolClassroom    = setupmodels.SchoolClassroom
)

type SubjectCategory string

const (
	CatMath       SubjectCategory = "math"
	CatScience    SubjectCategory = "science"
	CatLanguage   SubjectCategory = "language"
	CatHumanities SubjectCategory = "humanities"
	CatArts       SubjectCategory = "arts"
	CatPE         SubjectCategory = "pe"
	CatOther      SubjectCategory = "other"
)

type CurriculumStatus string

const (
	CurrDraft   CurriculumStatus = "draft"
	CurrActive  CurriculumStatus = "active"
	CurrRetired CurriculumStatus = "retired"
)

type TimetableState string

const (
	TtDraft     TimetableState = "draft"
	TtPublished TimetableState = "published"
	TtCancelled TimetableState = "cancelled"
)

type LessonPlanState string

const (
	LpDraft     LessonPlanState = "draft"
	LpSubmitted LessonPlanState = "submitted"
	LpApproved  LessonPlanState = "approved"
	LpRejected  LessonPlanState = "rejected"
)

// SchoolSubject is a reusable subject (Mathematics, Biology, ...) with an optional
// prerequisite graph expressed through school.subject.prerequisite rows.
type SchoolSubject struct {
	sdk.Model `nexa:"model=school.subject"`

	Code        sdk.String                     `nexa:"required,index,size=40,string=Subject Code"`
	Name        sdk.String                     `nexa:"required,size=120,string=Subject Name"`
	Category    sdk.Selection[SubjectCategory] `nexa:"string=Category,default=other,selection=math:Mathematics,science:Science,language:Language,humanities:Humanities,arts:Arts,pe:Physical Education,other:Other"`
	CreditHours sdk.Float64                    `nexa:"string=Credit Hours,default=0"`

	Description sdk.Text                  `nexa:"string=Description"`
	CompanyID   sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active      sdk.Boolean               `nexa:"string=Active,default=true"`

	Prerequisites sdk.One2Many[SchoolSubjectPrerequisite] `nexa:"string=Prerequisites,inverse=subject_id"`

	PrereqCount sdk.Integer `nexa:"string=Prerequisites,compute=compute_subject_prereq_count"`
}

// SchoolSubjectPrerequisite is one edge of the subject prerequisite graph: this subject
// requires the referenced subject to have been taken.
type SchoolSubjectPrerequisite struct {
	sdk.Model `nexa:"model=school.subject.prerequisite"`

	SubjectID             sdk.Many2One[SchoolSubject] `nexa:"required,index,string=Subject"`
	PrerequisiteSubjectID sdk.Many2One[SchoolSubject] `nexa:"required,index,string=Prerequisite Subject"`
	Notes                 sdk.Text                    `nexa:"string=Notes"`
}

// SchoolCurriculum is a versioned offering of a subject for a grade/year, carrying
// learning outcomes.
type SchoolCurriculum struct {
	sdk.Model `nexa:"model=school.curriculum"`

	Name           sdk.String                       `nexa:"required,size=120,string=Name"`
	SubjectID      sdk.Many2One[SchoolSubject]      `nexa:"required,index,string=Subject"`
	AcademicYearID sdk.Many2One[SchoolAcademicYear] `nexa:"string=Academic Year"`
	ClassID        sdk.Many2One[SchoolClass]        `nexa:"string=Grade Level"`
	Version        sdk.Integer                      `nexa:"string=Version,default=1"`
	Status         sdk.Selection[CurriculumStatus]  `nexa:"string=Status,default=draft,selection=draft:Draft,active:Active,retired:Retired"`
	StartDate      sdk.Date                         `nexa:"string=Start Date"`
	EndDate        sdk.Date                         `nexa:"string=End Date"`

	Outcomes sdk.One2Many[SchoolCurriculumOutcome] `nexa:"string=Learning Outcomes,inverse=curriculum_id"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true"`

	OutcomeCount sdk.Integer `nexa:"string=Outcomes,compute=compute_curriculum_outcome_count"`
}

// SchoolCurriculumOutcome is one measurable learning outcome of a curriculum version.
type SchoolCurriculumOutcome struct {
	sdk.Model `nexa:"model=school.curriculum.outcome"`

	CurriculumID sdk.Many2One[SchoolCurriculum] `nexa:"required,index,string=Curriculum"`
	Code         sdk.String                     `nexa:"size=20,string=Code"`
	Statement    sdk.Text                       `nexa:"required,string=Outcome"`
	Level        sdk.String                     `nexa:"size=40,string=Bloom Level"`
}

// SchoolTeachingAssignment binds a teacher to a subject for a class/section in a year and
// carries the teacher's availability + weekly workload for timetable validation.
type SchoolTeachingAssignment struct {
	sdk.Model `nexa:"model=school.teaching.assignment"`

	SubjectID      sdk.Many2One[SchoolSubject]      `nexa:"required,index,string=Subject"`
	AcademicYearID sdk.Many2One[SchoolAcademicYear] `nexa:"required,index,string=Academic Year"`
	ClassID        sdk.Many2One[SchoolClass]        `nexa:"index,string=Class"`
	SectionID      sdk.Many2One[SchoolSection]      `nexa:"index,string=Section"`
	TeacherID      sdk.Many2One[CoreUser]           `nexa:"index,string=Teacher"`

	PeriodsPerWeek  sdk.Integer `nexa:"string=Periods / Week,default=0"`
	AllowedWeekdays sdk.String  `nexa:"size=40,string=Allowed Weekdays (0=Sun..6=Sat)"`
	StartDate       sdk.Date    `nexa:"string=Start Date"`
	EndDate         sdk.Date    `nexa:"string=End Date"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true"`
}

// SchoolTimetableEntry schedules one lesson. It denormalises section/teacher/room so the
// conflict engine can validate without loading the assignment every time.
type SchoolTimetableEntry struct {
	sdk.Model `nexa:"model=school.timetable.entry"`

	AssignmentID   sdk.Many2One[SchoolTeachingAssignment] `nexa:"required,index,string=Assignment"`
	AcademicYearID sdk.Many2One[SchoolAcademicYear]       `nexa:"string=Academic Year"`
	SubjectID      sdk.Many2One[SchoolSubject]            `nexa:"index,string=Subject"`
	SectionID      sdk.Many2One[SchoolSection]            `nexa:"index,string=Section"`
	TeacherID      sdk.Many2One[CoreUser]                 `nexa:"index,string=Teacher"`
	ClassroomID    sdk.Many2One[SchoolClassroom]          `nexa:"string=Classroom"`

	Weekday      sdk.Integer `nexa:"required,string=Weekday (0=Sun..6=Sat)"`
	PeriodNumber sdk.Integer `nexa:"required,string=Period"`
	Duration     sdk.Integer `nexa:"string=Duration (periods),default=1"`
	StudentCount sdk.Integer `nexa:"string=Expected Students,default=0"`

	State sdk.Selection[TimetableState] `nexa:"string=Status,default=draft,selection=draft:Draft,published:Published,cancelled:Cancelled"`
	Notes sdk.Text                      `nexa:"string=Notes"`

	HasConflict sdk.Boolean `nexa:"string=Conflict?,compute=compute_timetable_conflict"`
}

// SchoolLessonPlan attaches a topic, objectives and materials to an assignment and moves
// through a submit/approve review.
type SchoolLessonPlan struct {
	sdk.Model `nexa:"model=school.lesson.plan"`

	AssignmentID sdk.Many2One[SchoolTeachingAssignment] `nexa:"required,index,string=Assignment"`
	SubjectID    sdk.Many2One[SchoolSubject]            `nexa:"index,string=Subject"`
	PlannedDate  sdk.Date                               `nexa:"string=Planned Date"`
	Topic        sdk.String                             `nexa:"required,size=200,string=Topic"`
	Objectives   sdk.Text                               `nexa:"string=Objectives"`
	Materials    sdk.Text                               `nexa:"string=Teaching Materials"`

	State        sdk.Selection[LessonPlanState] `nexa:"string=Status,default=draft,selection=draft:Draft,submitted:Submitted,approved:Approved,rejected:Rejected"`
	ApprovedByID sdk.Many2One[CoreUser]         `nexa:"string=Reviewed By"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
}

func init() {
	sdk.MustRegister("school_academics",
		&SchoolSubject{},
		&SchoolSubjectPrerequisite{},
		&SchoolCurriculum{},
		&SchoolCurriculumOutcome{},
		&SchoolTeachingAssignment{},
		&SchoolTimetableEntry{},
		&SchoolLessonPlan{},
	)
}
