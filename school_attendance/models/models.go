package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	schoolmodels "nexa_addons/school/models"
	academicsmodels "nexa_addons/school_academics/models"
	setupmodels "nexa_addons/school_setup/models"
)

// School Attendance (blueprint section 7). Attendance is captured as roll-call
// SESSIONS (a class/section on a date, optionally tied to a timetable lesson) with one
// record LINE per learner. A session moves draft -> open -> finalized; once finalized a
// status change is no longer a silent edit: it is written to an append-only CORRECTION
// log (who moved which learner from what to what, with a reason) so the record stays
// auditable. Per-session SUMMARIES (present / absent / late / excused counts and the
// attendance rate) are exposed as non-stored computes, and a nightly job raises ABSENCE
// ALERTS for learners whose unexcused absences cross a threshold inside a rolling window.

type (
	CoreUser             = basemodels.CoreUser
	CoreCompany          = basemodels.CoreCompany
	SchoolStudent        = schoolmodels.SchoolStudent
	SchoolClass          = schoolmodels.SchoolClass
	SchoolSection        = schoolmodels.SchoolSection
	SchoolAcademicYear   = schoolmodels.SchoolAcademicYear
	SchoolSubject        = academicsmodels.SchoolSubject
	SchoolTimetableEntry = academicsmodels.SchoolTimetableEntry
	SchoolCampus         = setupmodels.SchoolCampus
)

// SessionState is the roll-call lifecycle of an attendance session.
type SessionState string

const (
	SessDraft     SessionState = "draft"
	SessOpen      SessionState = "open"
	SessFinalized SessionState = "finalized"
	SessCancelled SessionState = "cancelled"
)

// AttendanceStatus is one learner's mark for a session.
type AttendanceStatus string

const (
	AttPresent AttendanceStatus = "present"
	AttAbsent  AttendanceStatus = "absent"
	AttLate    AttendanceStatus = "late"
	AttExcused AttendanceStatus = "excused"
)

// AlertState is the workflow of an absence alert.
type AlertState string

const (
	AlertOpen         AlertState = "open"
	AlertAcknowledged AlertState = "acknowledged"
	AlertResolved     AlertState = "resolved"
)

// SchoolAttendanceSession is one roll-call for a class/section on a date.
type SchoolAttendanceSession struct {
	sdk.Model `nexa:"model=school.attendance.session"`

	SessionCode sdk.String `nexa:"required,index,size=40,string=Session Code"`

	AcademicYearID   sdk.Many2One[SchoolAcademicYear]   `nexa:"index,string=Academic Year"`
	ClassID          sdk.Many2One[SchoolClass]          `nexa:"index,string=Class"`
	SectionID        sdk.Many2One[SchoolSection]        `nexa:"index,string=Section"`
	SubjectID        sdk.Many2One[SchoolSubject]        `nexa:"index,string=Subject"`
	TimetableEntryID sdk.Many2One[SchoolTimetableEntry] `nexa:"index,string=Timetable Lesson"`
	TeacherID        sdk.Many2One[CoreUser]             `nexa:"index,string=Taken By"`

	SessionDate  sdk.Date                    `nexa:"required,index,string=Date,default=today"`
	PeriodNumber sdk.Integer                 `nexa:"string=Period,default=0"`
	State        sdk.Selection[SessionState] `nexa:"string=Status,default=draft,index,selection=draft:Draft,open:Open,finalized:Finalized,cancelled:Cancelled"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true"`

	// Per-learner marks (child FK session_id).
	Records sdk.One2Many[SchoolAttendanceLine] `nexa:"string=Records,inverse=session_id"`

	// Non-stored session summaries.
	TotalCount     sdk.Integer `nexa:"string=Marked,compute=compute_attendance_total"`
	PresentCount   sdk.Integer `nexa:"string=Present,compute=compute_attendance_present"`
	AbsentCount    sdk.Integer `nexa:"string=Absent,compute=compute_attendance_absent"`
	LateCount      sdk.Integer `nexa:"string=Late,compute=compute_attendance_late"`
	ExcusedCount   sdk.Integer `nexa:"string=Excused,compute=compute_attendance_excused"`
	AttendanceRate sdk.Float64 `nexa:"string=Rate %,compute=compute_attendance_rate"`
}

// SchoolAttendanceLine is one learner's mark within a session.
type SchoolAttendanceLine struct {
	sdk.Model `nexa:"model=school.attendance.line"`

	SessionID sdk.Many2One[SchoolAttendanceSession] `nexa:"required,index,string=Session"`
	StudentID sdk.Many2One[SchoolStudent]           `nexa:"required,index,string=Student"`

	Status  sdk.Selection[AttendanceStatus] `nexa:"string=Status,default=present,index,selection=present:Present,absent:Absent,late:Late,excused:Excused"`
	InTime  sdk.DateTime                    `nexa:"string=In Time"`
	OutTime sdk.DateTime                    `nexa:"string=Out Time"`
	Remarks sdk.Text                        `nexa:"string=Remarks"`

	// True once the mark has been amended after the session was finalized.
	Corrected sdk.Boolean `nexa:"string=Corrected,default=false"`
}

// SchoolAttendanceCorrection is the append-only audit trail of post-finalization
// status changes. Rows are written by the correction action and are never edited or
// deleted by users.
type SchoolAttendanceCorrection struct {
	sdk.Model `nexa:"model=school.attendance.correction"`

	LineID    sdk.Many2One[SchoolAttendanceLine]    `nexa:"required,index,string=Record"`
	SessionID sdk.Many2One[SchoolAttendanceSession] `nexa:"index,string=Session"`
	StudentID sdk.Many2One[SchoolStudent]           `nexa:"index,string=Student"`

	FromStatus sdk.String   `nexa:"size=20,string=From"`
	ToStatus   sdk.String   `nexa:"size=20,string=To,index"`
	Reason     sdk.Text     `nexa:"required,string=Reason"`
	OperatorID sdk.Integer  `nexa:"string=Operator"`
	Timestamp  sdk.DateTime `nexa:"string=Timestamp"`
}

// SchoolAbsenceAlert is raised for a learner whose unexcused absences cross a
// threshold within a rolling window; it is deduplicated while an alert is open.
type SchoolAbsenceAlert struct {
	sdk.Model `nexa:"model=school.absence.alert"`

	StudentID      sdk.Many2One[SchoolStudent]      `nexa:"required,index,string=Student"`
	AcademicYearID sdk.Many2One[SchoolAcademicYear] `nexa:"index,string=Academic Year"`
	SectionID      sdk.Many2One[SchoolSection]      `nexa:"index,string=Section"`

	AbsentCount sdk.Integer `nexa:"string=Unexcused Absences,default=0"`
	Threshold   sdk.Integer `nexa:"string=Threshold,default=0"`
	WindowDays  sdk.Integer `nexa:"string=Window (days),default=0"`
	FirstAbsent sdk.Date    `nexa:"string=First Absence"`
	LastAbsent  sdk.Date    `nexa:"string=Last Absence"`

	State    sdk.Selection[AlertState] `nexa:"string=Status,default=open,index,selection=open:Open,acknowledged:Acknowledged,resolved:Resolved"`
	RaisedAt sdk.DateTime              `nexa:"string=Raised At"`
	AckByID  sdk.Many2One[CoreUser]    `nexa:"string=Handled By"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
}

func init() {
	sdk.MustRegister("school_attendance",
		&SchoolAttendanceSession{},
		&SchoolAttendanceLine{},
		&SchoolAttendanceCorrection{},
		&SchoolAbsenceAlert{},
	)
}
