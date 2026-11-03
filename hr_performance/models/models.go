package models

import (
	"nexa/core/sdk"
	hrmodels "nexa_addons/hr/models"
	apprmodels "nexa_addons/hr_appraisal/models"
	skillmodels "nexa_addons/hr_skill/models"
)

// HR-F7 "Performance". The base hr_appraisal module already owns review cycles,
// individual appraisals (self + manager text), goals and scored evaluation lines,
// but none of it delivers the production behaviours the blueprint demands: goals
// need a company / department / individual hierarchy plus measurable KPIs against
// targets and a period; a review needs a reusable criteria template; a 360 needs
// weighted multi-rater (manager / peer / subordinate / self / external) feedback
// rolled up to a score; there is no periodic check-in, no development plan, no
// performance-improvement plan, and no calibration session that proposes, adjusts
// and approves ratings with an append-only audit. This companion adds exactly that
// WITHOUT migrating the appraisal tables: it reuses hr_appraisal / hr_skill records
// (never recreated) via non-destructive inherit extensions plus new models, and,
// per the blueprint guardrail, a performance rating NEVER mutates pay — calibration
// and finalisation only record a decision on the appraisal, not a salary change.

type (
	HrEmployee       = hrmodels.HrEmployee
	HrDepartment     = hrmodels.HrDepartment
	HrAppraisal      = apprmodels.HrAppraisal
	HrAppraisalCycle = apprmodels.HrAppraisalCycle
	HrAppraisalGoal  = apprmodels.HrAppraisalGoal
	HrAppraisalLine  = apprmodels.HrAppraisalLine
	HrSkill          = skillmodels.HrSkill
)

// ---------------------------------------------------------------------------
// Enumerations
// ---------------------------------------------------------------------------

type GoalLevel string

const (
	LevelIndividual GoalLevel = "individual"
	LevelDepartment GoalLevel = "department"
	LevelCompany    GoalLevel = "company"
)

type FeedbackRelation string

const (
	RelManager     FeedbackRelation = "manager"
	RelPeer        FeedbackRelation = "peer"
	RelSubordinate FeedbackRelation = "subordinate"
	RelSelf        FeedbackRelation = "self"
	RelExternal    FeedbackRelation = "external"
)

type CheckinState string

const (
	CheckDraft CheckinState = "draft"
	CheckDone  CheckinState = "done"
)

type DevPlanState string

const (
	DevDraft     DevPlanState = "draft"
	DevActive    DevPlanState = "active"
	DevCompleted DevPlanState = "completed"
	DevCancelled DevPlanState = "cancelled"
)

type PipState string

const (
	PipDraft     PipState = "draft"
	PipSubmitted PipState = "submitted"
	PipActive    PipState = "active"
	PipSucceeded PipState = "succeeded"
	PipFailed    PipState = "failed"
	PipCancelled PipState = "cancelled"
)

type CalibSessionState string

const (
	CalDraft  CalibSessionState = "draft"
	CalOpen   CalibSessionState = "open"
	CalLocked CalibSessionState = "locked"
	CalClosed CalibSessionState = "closed"
)

type CalibLineState string

const (
	CalProposed CalibLineState = "proposed"
	CalAdjusted CalibLineState = "adjusted"
	CalApproved CalibLineState = "approved"
	CalRejected CalibLineState = "rejected"
)

// ---------------------------------------------------------------------------
// Goal hierarchy + KPI measurement (inherit hr.appraisal.goal)
// ---------------------------------------------------------------------------

// HrGoalHierarchy turns a flat goal into a cascadeable objective: it carries a
// level (company / department / individual), an optional parent for roll-up, an
// owning cycle, and a measurable target vs actual with an achievement percent.
type HrGoalHierarchy struct {
	sdk.Model `nexa:"inherit=hr.appraisal.goal"`

	Level          sdk.Selection[GoalLevel]       `nexa:"string=Level,default=individual,selection=individual:Individual,department:Department,company:Company,index"`
	ParentGoalID   sdk.Many2One[HrAppraisalGoal]  `nexa:"string=Parent Objective"`
	CycleID        sdk.Many2One[HrAppraisalCycle] `nexa:"string=Cycle"`
	TargetValue    sdk.Float64                    `nexa:"string=Target,default=0"`
	ActualValue    sdk.Float64                    `nexa:"string=Actual,default=0"`
	Unit           sdk.String                     `nexa:"string=Unit"`
	AchievementPct sdk.Float64                    `nexa:"string=Achievement %,compute=compute_goal_achievement,store"`
}

// ---------------------------------------------------------------------------
// Appraisal performance overlay (inherit hr.appraisal)
// ---------------------------------------------------------------------------

// HrAppraisalPerformance adds template seeding, a rolled-up weighted 360 score
// (live, non-stored) and a finalised / calibrated rating gate. Finalisation is a
// guarded approval; it never touches salary.
type HrAppraisalPerformance struct {
	sdk.Model `nexa:"inherit=hr.appraisal"`

	TemplateID       sdk.Many2One[HrAppraisalTemplate] `nexa:"string=Review Template"`
	PeerScore        sdk.Float64                       `nexa:"string=360 Weighted Score,compute=compute_peer_score"`
	CalibratedRating sdk.Float64                       `nexa:"string=Calibrated Rating,default=0"`
	Finalized        sdk.Boolean                       `nexa:"string=Finalized,default=false,index"`
	FinalizedBy      sdk.Integer                       `nexa:"string=Finalized By"`
	FinalizedDate    sdk.Date                          `nexa:"string=Finalized Date"`
	Feedbacks        sdk.One2Many[HrAppraisalFeedback] `nexa:"string=360 Feedback,inverse=appraisal_id"`
}

// HrPerformanceKpi is a measurable indicator tracked against a goal over a period.
type HrPerformanceKpi struct {
	sdk.Model `nexa:"model=hr.performance.kpi"`

	GoalID         sdk.Many2One[HrAppraisalGoal] `nexa:"required,string=Goal,index"`
	Name           sdk.String                    `nexa:"required,string=KPI"`
	TargetValue    sdk.Float64                   `nexa:"string=Target,default=0"`
	ActualValue    sdk.Float64                   `nexa:"string=Actual,default=0"`
	Unit           sdk.String                    `nexa:"string=Unit"`
	PeriodStart    sdk.Date                      `nexa:"string=Period Start"`
	PeriodEnd      sdk.Date                      `nexa:"string=Period End"`
	Weight         sdk.Float64                   `nexa:"string=Weight %,default=100"`
	AchievementPct sdk.Float64                   `nexa:"string=Achievement %,compute=compute_kpi_achievement,store"`
	Notes          sdk.Text                      `nexa:"string=Notes"`
}

// ---------------------------------------------------------------------------
// Review templates
// ---------------------------------------------------------------------------

// HrAppraisalTemplate is a reusable set of evaluation criteria (optionally bound
// to competencies) that seeds a fresh appraisal's lines.
type HrAppraisalTemplate struct {
	sdk.Model `nexa:"model=hr.appraisal.template"`

	Name        sdk.String                            `nexa:"required,string=Name"`
	Description sdk.Text                              `nexa:"string=Description"`
	Active      sdk.Boolean                           `nexa:"string=Active,default=true"`
	Lines       sdk.One2Many[HrAppraisalTemplateLine] `nexa:"string=Criteria,inverse=template_id"`
}

// HrAppraisalTemplateLine is one default criteria row of a template.
type HrAppraisalTemplateLine struct {
	sdk.Model `nexa:"model=hr.appraisal.template.line"`

	TemplateID sdk.Many2One[HrAppraisalTemplate] `nexa:"required,string=Template,index"`
	Name       sdk.String                        `nexa:"required,string=Criteria"`
	SkillID    sdk.Many2One[HrSkill]             `nexa:"string=Competency"`
	Weight     sdk.Float64                       `nexa:"string=Weight %,default=100"`
	Sequence   sdk.Integer                       `nexa:"string=Sequence,default=10"`
}

// ---------------------------------------------------------------------------
// 360 feedback
// ---------------------------------------------------------------------------

// HrAppraisalFeedback is one rater's scored, weighted input into an appraisal.
// A reviewer submits their own record; the appraisal rolls submitted rows into a
// weighted average (see compute_peer_score). Feedback is advisory and never pays.
type HrAppraisalFeedback struct {
	sdk.Model `nexa:"model=hr.appraisal.feedback"`

	AppraisalID sdk.Many2One[HrAppraisal]       `nexa:"required,string=Appraisal,index"`
	ReviewerID  sdk.Many2One[HrEmployee]        `nexa:"string=Reviewer"`
	Relation    sdk.Selection[FeedbackRelation] `nexa:"string=Relationship,default=peer,selection=manager:Manager,peer:Peer,subordinate:Subordinate,self:Self,external:External"`
	Weight      sdk.Float64                     `nexa:"string=Weight,default=1"`
	Score       sdk.Float64                     `nexa:"string=Score,default=0"`
	Comments    sdk.Text                        `nexa:"string=Comments"`
	Submitted   sdk.Boolean                     `nexa:"string=Submitted,default=false,index"`
}

// ---------------------------------------------------------------------------
// Check-ins
// ---------------------------------------------------------------------------

// HrPerformanceCheckin is a periodic one-to-one progress note against a goal.
type HrPerformanceCheckin struct {
	sdk.Model `nexa:"model=hr.performance.checkin"`

	EmployeeID  sdk.Many2One[HrEmployee]      `nexa:"required,string=Employee,index"`
	ManagerID   sdk.Many2One[HrEmployee]      `nexa:"string=Manager"`
	GoalID      sdk.Many2One[HrAppraisalGoal] `nexa:"string=Objective"`
	CheckinDate sdk.Date                      `nexa:"required,string=Date,index"`
	Topic       sdk.String                    `nexa:"string=Topic"`
	Notes       sdk.Text                      `nexa:"string=Notes"`
	FollowUp    sdk.Text                      `nexa:"string=Follow-up"`
	State       sdk.Selection[CheckinState]   `nexa:"string=Status,default=draft,selection=draft:Draft,done:Logged"`
}

// ---------------------------------------------------------------------------
// Development plans
// ---------------------------------------------------------------------------

// HrPerformanceDevPlan groups development actions for an employee, optionally
// tied to an objective.
type HrPerformanceDevPlan struct {
	sdk.Model `nexa:"model=hr.performance.devplan"`

	EmployeeID sdk.Many2One[HrEmployee]                 `nexa:"required,string=Employee,index"`
	Name       sdk.String                               `nexa:"required,string=Plan"`
	GoalID     sdk.Many2One[HrAppraisalGoal]            `nexa:"string=Linked Objective"`
	StartDate  sdk.Date                                 `nexa:"string=Start"`
	TargetDate sdk.Date                                 `nexa:"string=Target"`
	Summary    sdk.Text                                 `nexa:"string=Summary"`
	State      sdk.Selection[DevPlanState]              `nexa:"string=Status,default=draft,selection=draft:Draft,active:Active,completed:Completed,cancelled:Cancelled"`
	Actions    sdk.One2Many[HrPerformanceDevPlanAction] `nexa:"string=Actions,inverse=plan_id"`
}

// HrPerformanceDevPlanAction is one item within a development plan.
type HrPerformanceDevPlanAction struct {
	sdk.Model `nexa:"model=hr.performance.devplan.action"`

	PlanID      sdk.Many2One[HrPerformanceDevPlan] `nexa:"required,string=Plan,index"`
	Description sdk.String                         `nexa:"required,string=Action"`
	SkillID     sdk.Many2One[HrSkill]              `nexa:"string=Competency"`
	DueDate     sdk.Date                           `nexa:"string=Due"`
	Progress    sdk.Float64                        `nexa:"string=Progress %,default=0"`
	Done        sdk.Boolean                        `nexa:"string=Done,default=false"`
}

// ---------------------------------------------------------------------------
// Performance improvement plans
// ---------------------------------------------------------------------------

// HrPerformancePip is a formal, maker-checker improvement plan: a manager drafts
// it, an HR Manager approves it into an active period, and it is closed out with
// a succeeded / failed outcome. Approval records a decision; it never changes pay.
type HrPerformancePip struct {
	sdk.Model `nexa:"model=hr.performance.pip"`

	EmployeeID  sdk.Many2One[HrEmployee]  `nexa:"required,string=Employee,index"`
	AppraisalID sdk.Many2One[HrAppraisal] `nexa:"string=Source Appraisal"`
	Reason      sdk.Text                  `nexa:"string=Reason"`
	Objective   sdk.Text                  `nexa:"string=Objective"`
	StartDate   sdk.Date                  `nexa:"required,string=Start"`
	EndDate     sdk.Date                  `nexa:"required,string=End,index"`
	ReviewDate  sdk.Date                  `nexa:"string=Review Point"`
	State       sdk.Selection[PipState]   `nexa:"string=Status,default=draft,selection=draft:Draft,submitted:Submitted,active:Active,succeeded:Succeeded,failed:Failed,cancelled:Cancelled"`
	RequestedBy sdk.Integer               `nexa:"string=Requested By"`
	ApproverID  sdk.Integer               `nexa:"string=Approved By"`
	ApproveDate sdk.Date                  `nexa:"string=Approval Date"`
	Outcome     sdk.Text                  `nexa:"string=Outcome"`
}

// ---------------------------------------------------------------------------
// Calibration
// ---------------------------------------------------------------------------

// HrCalibrationSession is a review meeting where a group of appraisals is
// calibrated. Locking applies each line's calibrated rating to its appraisal and
// writes an append-only history entry; it never touches payroll.
type HrCalibrationSession struct {
	sdk.Model `nexa:"model=hr.calibration.session"`

	Name         sdk.String                       `nexa:"required,string=Session"`
	CycleID      sdk.Many2One[HrAppraisalCycle]   `nexa:"string=Cycle"`
	DepartmentID sdk.Many2One[HrDepartment]       `nexa:"string=Department"`
	MeetingDate  sdk.Date                         `nexa:"string=Meeting Date"`
	Notes        sdk.Text                         `nexa:"string=Notes"`
	State        sdk.Selection[CalibSessionState] `nexa:"string=Status,default=draft,selection=draft:Draft,open:Calibrating,locked:Locked,closed:Closed"`
	Lines        sdk.One2Many[HrCalibrationLine]  `nexa:"string=Lines,inverse=session_id"`
}

// HrCalibrationLine is one appraisal under calibration: the proposed vs the
// calibrated rating, the adjustment reason, and an approval gate.
type HrCalibrationLine struct {
	sdk.Model `nexa:"model=hr.calibration.line"`

	SessionID        sdk.Many2One[HrCalibrationSession] `nexa:"required,string=Session,index"`
	AppraisalID      sdk.Many2One[HrAppraisal]          `nexa:"required,string=Appraisal,index"`
	ProposedRating   sdk.Float64                        `nexa:"string=Proposed,default=0"`
	CalibratedRating sdk.Float64                        `nexa:"string=Calibrated,default=0"`
	Delta            sdk.Float64                        `nexa:"string=Change,compute=compute_calib_delta,store"`
	AdjustReason     sdk.Text                           `nexa:"string=Adjustment Reason"`
	ApprovedBy       sdk.Integer                        `nexa:"string=Approved By"`
	State            sdk.Selection[CalibLineState]      `nexa:"string=Status,default=proposed,selection=proposed:Proposed,adjusted:Adjusted,approved:Approved,rejected:Rejected"`
}

// HrCalibrationHistory is an append-only audit trail of every calibration action
// (proposed / adjusted / approved / applied) with the from/to rating and actor.
type HrCalibrationHistory struct {
	sdk.Model `nexa:"model=hr.calibration.history"`

	SessionID   sdk.Many2One[HrCalibrationSession] `nexa:"string=Session,index"`
	LineID      sdk.Many2One[HrCalibrationLine]    `nexa:"string=Line,index"`
	AppraisalID sdk.Many2One[HrAppraisal]          `nexa:"string=Appraisal,index"`
	Action      sdk.String                         `nexa:"string=Action"`
	FromRating  sdk.Float64                        `nexa:"string=From"`
	ToRating    sdk.Float64                        `nexa:"string=To"`
	PerformedBy sdk.Integer                        `nexa:"string=Performed By"`
	PerformedAt sdk.DateTime                       `nexa:"string=Timestamp"`
}

func init() {
	sdk.MustRegister("hr_performance",
		&HrGoalHierarchy{},
		&HrAppraisalPerformance{},
		&HrPerformanceKpi{},
		&HrAppraisalTemplate{},
		&HrAppraisalTemplateLine{},
		&HrAppraisalFeedback{},
		&HrPerformanceCheckin{},
		&HrPerformanceDevPlan{},
		&HrPerformanceDevPlanAction{},
		&HrPerformancePip{},
		&HrCalibrationSession{},
		&HrCalibrationLine{},
		&HrCalibrationHistory{},
	)
}
