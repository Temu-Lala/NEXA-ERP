package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	schoolmodels "nexa_addons/school/models"
)

// School Setup & Organization (blueprint section "School Setup"). This adds the
// institutional master data that the rest of the academic blueprint hangs off —
// campuses (multi-campus), terms/semesters within an academic year, physical
// classrooms with capacity, and a school calendar of events (holidays, exams,
// sports, parent-teacher meetings). All records are master/configuration data and
// are company- and (where relevant) campus-scoped so a single deployment can serve
// several schools/campuses without cross-contamination. Nothing here mutates fees,
// grades or attendance; later tranches (academics, attendance, examinations) read
// these structures.

type (
	CoreCompany        = basemodels.CoreCompany
	CoreUser           = basemodels.CoreUser
	SchoolAcademicYear = schoolmodels.SchoolAcademicYear
)

// RoomType classifies how a physical space is used.
type RoomType string

const (
	RoomClassroom  RoomType = "classroom"
	RoomLab        RoomType = "lab"
	RoomLibrary    RoomType = "library"
	RoomSports     RoomType = "sports"
	RoomAuditorium RoomType = "auditorium"
	RoomOther      RoomType = "other"
)

// EventType classifies a school-calendar entry.
type EventType string

const (
	EventHoliday   EventType = "holiday"
	EventExam      EventType = "exam"
	EventSports    EventType = "sports"
	EventPTM       EventType = "ptm"
	EventTermStart EventType = "term_start"
	EventTermEnd   EventType = "term_end"
	EventOther     EventType = "other"
)

// SchoolCampus is a physical location of the school. Multi-campus is a first-class
// concept: terms, classrooms, calendar events, and (in later tranches) classes and
// students reference a campus so each site keeps its own scheduling and settings.
type SchoolCampus struct {
	sdk.Model `nexa:"model=school.campus"`

	Name      sdk.String                `nexa:"required,size=120,string=Campus Name"`
	Code      sdk.String                `nexa:"size=30,string=Code,index"`
	Address   sdk.Text                  `nexa:"string=Address"`
	City      sdk.String                `nexa:"size=120,string=City"`
	Phone     sdk.String                `nexa:"size=40,string=Phone"`
	Email     sdk.String                `nexa:"size=120,string=Email"`
	ManagerID sdk.Many2One[CoreUser]    `nexa:"string=Campus Manager"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company,index"`
	Notes     sdk.Text                  `nexa:"string=Notes"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true,index"`
}

// SchoolTerm is a semester/term inside an academic year. Some schools run two
// semesters, others three terms; the structure is configurable, not hardcoded.
type SchoolTerm struct {
	sdk.Model `nexa:"model=school.term"`

	Name           sdk.String                       `nexa:"required,size=120,string=Term Name"`
	Code           sdk.String                       `nexa:"size=30,string=Code"`
	AcademicYearID sdk.Many2One[SchoolAcademicYear] `nexa:"required,index,string=Academic Year"`
	CampusID       sdk.Many2One[SchoolCampus]       `nexa:"index,string=Campus"`
	DateStart      sdk.Date                         `nexa:"required,string=Start Date"`
	DateEnd        sdk.Date                         `nexa:"required,string=End Date"`
	Sequence       sdk.Integer                      `nexa:"string=Sequence,default=1"`
	IsCurrent      sdk.Boolean                      `nexa:"string=Current Term,default=false,index"`
	CompanyID      sdk.Many2One[CoreCompany]        `nexa:"string=Company"`
	Notes          sdk.Text                         `nexa:"string=Notes"`
	Active         sdk.Boolean                      `nexa:"string=Active,default=true,index"`
}

// SchoolClassroom is a physical room with a capacity, used by the timetable engine
// (later tranche) to detect room conflicts and over-capacity bookings.
type SchoolClassroom struct {
	sdk.Model `nexa:"model=school.classroom"`

	Name      sdk.String                 `nexa:"required,size=120,string=Room Name"`
	Code      sdk.String                 `nexa:"size=30,string=Code,index"`
	CampusID  sdk.Many2One[SchoolCampus] `nexa:"index,string=Campus"`
	RoomType  sdk.Selection[RoomType]    `nexa:"string=Type,default=classroom,selection=classroom:Classroom,lab:Laboratory,library:Library,sports:Sports Hall,auditorium:Auditorium,other:Other"`
	Capacity  sdk.Integer                `nexa:"string=Capacity,default=0"`
	Building  sdk.String                 `nexa:"size=120,string=Building"`
	Floor     sdk.String                 `nexa:"size=40,string=Floor"`
	CompanyID sdk.Many2One[CoreCompany]  `nexa:"string=Company"`
	Notes     sdk.Text                   `nexa:"string=Notes"`
	Active    sdk.Boolean                `nexa:"string=Active,default=true,index"`
}

// SchoolCalendarEvent marks a dated event on the school calendar (holiday, exam
// window, sports meet, parent-teacher meeting, term start/end). All-day spans are
// expressed with a start and end date; holiday/exam events are what attendance and
// examination tranches will consult to know a school is not sitting normal lessons.
type SchoolCalendarEvent struct {
	sdk.Model `nexa:"model=school.calendar.event"`

	Name           sdk.String                       `nexa:"required,size=120,string=Event"`
	EventType      sdk.Selection[EventType]         `nexa:"string=Type,default=other,selection=holiday:Holiday,exam:Examination,sports:Sports Day,ptm:Parent-Teacher Meeting,term_start:Term Start,term_end:Term End,other:Other"`
	AcademicYearID sdk.Many2One[SchoolAcademicYear] `nexa:"index,string=Academic Year"`
	TermID         sdk.Many2One[SchoolTerm]         `nexa:"index,string=Term"`
	CampusID       sdk.Many2One[SchoolCampus]       `nexa:"index,string=Campus"`
	DateStart      sdk.Date                         `nexa:"required,string=Start Date"`
	DateEnd        sdk.Date                         `nexa:"string=End Date"`
	AllDay         sdk.Boolean                      `nexa:"string=All Day,default=true"`
	SchoolClosed   sdk.Boolean                      `nexa:"string=School Closed (holiday),default=false"`
	Description    sdk.Text                         `nexa:"string=Description"`
	CompanyID      sdk.Many2One[CoreCompany]        `nexa:"string=Company"`
	Active         sdk.Boolean                      `nexa:"string=Active,default=true,index"`
}

func init() {
	sdk.MustRegister("school_setup",
		&SchoolCampus{},
		&SchoolTerm{},
		&SchoolClassroom{},
		&SchoolCalendarEvent{},
	)
}
