package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	hrmodels "nexa_addons/hr/models"
)

// HR-F2 "Time & Attendance". The anchor hr.attendance stores a raw check-in /
// check-out pair and derives worked hours, but has no notion of a scheduled
// shift, no exception detection (late / early / missing punch), no overtime
// request/approval and no audited correction path. This companion adds all of
// that without touching the anchor's table:
//
//   - hr.work.shift defines a work shift (start/end "HH:MM", grace windows,
//     rounding, overnight flag) and hr.work.shift.assignment is the rotating
//     roster (which employee is on which shift on which date).
//   - hr.attendance is extended (non-destructively, via inherit) with the
//     resolved shift, an exception status and measured late/early minutes.
//   - hr.attendance.correction lets an employee request a punch fix; on approval
//     the attendance is adjusted while the ORIGINAL times are preserved on the
//     immutable correction record (nothing is silently overwritten).
//   - hr.overtime.request captures overtime for approval (maker-checker: only an
//     HR manager can validate).
//
// The engine reconciles attendance exceptions idempotently and only writes when
// a value actually changes, so re-firing the same event is a no-op (no update
// loop).

type (
	HrEmployee   = hrmodels.HrEmployee
	HrAttendance = hrmodels.HrAttendance
	CoreCompany  = basemodels.CoreCompany
)

// ---------------------------------------------------------------------------
// Enumerations
// ---------------------------------------------------------------------------

// AttendanceStatus is the exception verdict for a punch pair vs its shift.
type AttendanceStatus string

const (
	AttPresent      AttendanceStatus = "present"
	AttLate         AttendanceStatus = "late"
	AttEarly        AttendanceStatus = "early_leave"
	AttLateEarly    AttendanceStatus = "late_and_early"
	AttMissingPunch AttendanceStatus = "missing_punch"
	AttNotScheduled AttendanceStatus = "not_scheduled"
)

// CorrectionState is the maker-checker workflow of a punch correction.
type CorrectionState string

const (
	CorrDraft    CorrectionState = "draft"
	CorrConfirm  CorrectionState = "confirm"
	CorrValidate CorrectionState = "validate"
	CorrRefuse   CorrectionState = "refuse"
)

// OvertimeState mirrors the leave approval machine.
type OvertimeState string

const (
	OtDraft    OvertimeState = "draft"
	OtConfirm  OvertimeState = "confirm"
	OtValidate OvertimeState = "validate"
	OtRefuse   OvertimeState = "refuse"
)

// ---------------------------------------------------------------------------
// Shift definition + roster
// ---------------------------------------------------------------------------

// HrWorkShift is a repeating work window expressed as clock times. Grace windows
// and rounding are configurable per shift so the exception engine can be tuned
// without code. Overnight marks shifts whose end time falls on the next day.
type HrWorkShift struct {
	sdk.Model `nexa:"model=hr.work.shift"`

	Name        sdk.String                `nexa:"required,string=Shift"`
	Code        sdk.String                `nexa:"string=Code,index"`
	StartTime   sdk.String                `nexa:"required,string=Start (HH:MM),default=09:00"`
	EndTime     sdk.String                `nexa:"required,string=End (HH:MM),default=17:00"`
	GraceInMin  sdk.Integer               `nexa:"string=Grace In (min),default=0"`
	GraceOutMin sdk.Integer               `nexa:"string=Grace Out (min),default=0"`
	RoundingMin sdk.Integer               `nexa:"string=Rounding (min),default=0"`
	StandardHrs sdk.Float64               `nexa:"string=Standard Hours,default=8"`
	Overnight   sdk.Boolean               `nexa:"string=Overnight,default=false"`
	Active      sdk.Boolean               `nexa:"string=Active,default=true"`
	CompanyID   sdk.Many2One[CoreCompany] `nexa:"string=Company"`
}

// HrWorkShiftAssignment is a roster line: an employee scheduled onto a shift for
// a single date (supports rotating schedules by assigning per day).
type HrWorkShiftAssignment struct {
	sdk.Model `nexa:"model=hr.work.shift.assignment"`

	EmployeeID sdk.Many2One[HrEmployee]  `nexa:"required,string=Employee,index"`
	ShiftID    sdk.Many2One[HrWorkShift] `nexa:"required,string=Shift,index"`
	Date       sdk.Date                  `nexa:"required,string=Date,index"`
	Note       sdk.Text                  `nexa:"string=Note"`
	CompanyID  sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active     sdk.Boolean               `nexa:"string=Active,default=true"`
}

// ---------------------------------------------------------------------------
// Attendance extension
// ---------------------------------------------------------------------------

// HrAttendanceExtension enriches the anchor attendance with the resolved shift
// and exception verdict. These are stored (written by the engine) so they can be
// filtered and consumed by payroll.
type HrAttendanceExtension struct {
	sdk.Model `nexa:"inherit=hr.attendance"`

	ShiftID      sdk.Many2One[HrWorkShift]       `nexa:"string=Shift,index"`
	Status       sdk.Selection[AttendanceStatus] `nexa:"string=Status,index,default=not_scheduled,selection=present:Present,late:Late,early_leave:Early Leave,late_and_early:Late & Early,missing_punch:Missing Punch,not_scheduled:Not Scheduled"`
	MinutesLate  sdk.Integer                     `nexa:"string=Late (min),default=0"`
	MinutesEarly sdk.Integer                     `nexa:"string=Early (min),default=0"`
	MissingPunch sdk.Boolean                     `nexa:"string=Missing Punch,default=false"`
	Corrected    sdk.Boolean                     `nexa:"string=Corrected,default=false"`
}

// ---------------------------------------------------------------------------
// Attendance correction (audited)
// ---------------------------------------------------------------------------

// HrAttendanceCorrection is a request to fix a punch. It snapshots the original
// times so, when approved, the attendance can be corrected while the pre-change
// values remain on this immutable record.
type HrAttendanceCorrection struct {
	sdk.Model `nexa:"model=hr.attendance.correction"`

	AttendanceID      sdk.Many2One[HrAttendance]     `nexa:"required,string=Attendance,index"`
	EmployeeID        sdk.Many2One[HrEmployee]       `nexa:"required,string=Employee,index"`
	OrigCheckIn       sdk.DateTime                   `nexa:"string=Original Check In"`
	OrigCheckOut      sdk.DateTime                   `nexa:"string=Original Check Out"`
	RequestedCheckIn  sdk.DateTime                   `nexa:"string=New Check In"`
	RequestedCheckOut sdk.DateTime                   `nexa:"string=New Check Out"`
	Reason            sdk.Text                       `nexa:"required,string=Reason"`
	State             sdk.Selection[CorrectionState] `nexa:"string=Status,index,default=draft,selection=draft:Draft,confirm:Waiting Approval,validate:Approved,refuse:Refused"`
	Applied           sdk.Boolean                    `nexa:"string=Applied,default=false"`
	CompanyID         sdk.Many2One[CoreCompany]      `nexa:"string=Company"`
}

// ---------------------------------------------------------------------------
// Overtime request (approved)
// ---------------------------------------------------------------------------

// HrOvertimeRequest records claimed overtime for a day, approved by an HR
// manager. Validated rows are the payroll-ready overtime feed.
type HrOvertimeRequest struct {
	sdk.Model `nexa:"model=hr.overtime.request"`

	EmployeeID   sdk.Many2One[HrEmployee]     `nexa:"required,string=Employee,index"`
	Date         sdk.Date                     `nexa:"required,string=Date,index"`
	Hours        sdk.Float64                  `nexa:"required,string=Overtime Hours"`
	AttendanceID sdk.Many2One[HrAttendance]   `nexa:"string=Attendance"`
	Reason       sdk.Text                     `nexa:"string=Reason"`
	State        sdk.Selection[OvertimeState] `nexa:"string=Status,index,default=draft,selection=draft:Draft,confirm:Waiting Approval,validate:Approved,refuse:Refused"`
	CompanyID    sdk.Many2One[CoreCompany]    `nexa:"string=Company"`
}

func init() {
	sdk.MustRegister("hr_shift",
		&HrWorkShift{},
		&HrWorkShiftAssignment{},
		&HrAttendanceExtension{},
		&HrAttendanceCorrection{},
		&HrOvertimeRequest{},
	)
}
