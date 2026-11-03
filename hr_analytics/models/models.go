package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	hrmodels "nexa_addons/hr/models"
)

// HR-F11 "Workforce Planning & Analytics". It aggregates the operational HR data
// produced by the other HR modules into read-oriented planning surfaces without
// re-creating or migrating any of their tables:
//
//   - hr.workforce.snapshot is a per-department, per-period roll-up of headcount,
//     hires, terminations, turnover / retention (each with an explicit numerator
//     and denominator), average tenure, open positions, absence + overtime trends,
//     payroll cost, and skills-gap counts. It is (re)built by a scheduled refresh
//     that reads source models by runtime name and degrades to zero when a source
//     module is not installed, so this companion depends only on the hr anchor.
//   - hr.position.plan is the approved-vs-occupied view: a planned headcount and
//     staffing budget against a department / job, with the filled count and the
//     resulting vacancy projected live from active employees.
//   - hr.workforce.scenario is a forward projection (growth and attrition applied
//     over a horizon) against the current base headcount, with a projected cost.
//
// Every figure here is derived: nothing in this module mutates an employee, a
// payslip, a leave ledger row, or a requisition. Turnover / retention are stored as
// explicit ratios so a dashboard never has to guess the denominator, and a period is
// always attached to a snapshot so a rate is never reported without its window.

type (
	HrEmployee   = hrmodels.HrEmployee
	HrDepartment = hrmodels.HrDepartment
	HrJob        = hrmodels.HrJob
	CoreCompany  = basemodels.CoreCompany
)

// ---------------------------------------------------------------------------
// Aggregation period bucketing
// ---------------------------------------------------------------------------

// SnapshotPeriod buckets a snapshot by reporting cadence. The refresh always
// writes the "rolling" current-year snapshot; the enum lets an operator file
// additional, manually-scoped periods alongside it for comparison.
type SnapshotPeriod string

const (
	PeriodRolling   SnapshotPeriod = "rolling"
	PeriodMonthly   SnapshotPeriod = "monthly"
	PeriodQuarterly SnapshotPeriod = "quarterly"
	PeriodAnnual    SnapshotPeriod = "annual"
)

// ---------------------------------------------------------------------------
// hr.workforce.snapshot
// ---------------------------------------------------------------------------

// HrWorkforceSnapshot is one department's KPI roll-up for one period. Each rate
// column stores the ratio; the raw numerator / denominator are kept beside it so a
// figure is always traceable and never silently re-derived in the client.
type HrWorkforceSnapshot struct {
	sdk.Model `nexa:"model=hr.workforce.snapshot"`

	DepartmentID sdk.Many2One[HrDepartment]    `nexa:"string=Department,index"`
	Period       sdk.Selection[SnapshotPeriod] `nexa:"string=Period,default=rolling,selection=rolling:Rolling Current Year,monthly:Monthly,quarterly:Quarterly,annual:Annual"`
	PeriodStart  sdk.Date                      `nexa:"string=Period Start,index"`
	PeriodEnd    sdk.Date                      `nexa:"string=Period End"`
	ComputedOn   sdk.DateTime                  `nexa:"string=Computed On"`
	CompanyID    sdk.Many2One[CoreCompany]     `nexa:"string=Company"`

	// Headcount / movement
	HeadcountActive  sdk.Integer `nexa:"string=Active Headcount,default=0"`
	Hires            sdk.Integer `nexa:"string=Hires,default=0"`
	Terminations     sdk.Integer `nexa:"string=Terminations,default=0"`
	OpeningHeadcount sdk.Integer `nexa:"string=Opening Headcount,default=0,help=Active headcount at the start of the period (turnover denominator)."`
	AvgTenureDays    sdk.Float64 `nexa:"string=Avg Tenure (days),default=0"`

	// Turnover / retention — stored ratios with an explicit denominator
	TurnoverRate  sdk.Float64 `nexa:"string=Turnover %,default=0,help=Terminations / average of opening and closing headcount."`
	RetentionRate sdk.Float64 `nexa:"string=Retention %,default=0,help=1 - terminations / opening headcount."`

	// Recruitment / planning
	OpenPositions    sdk.Integer `nexa:"string=Open Positions,default=0"`
	PlannedPositions sdk.Integer `nexa:"string=Approved Positions,default=0"`
	FilledPositions  sdk.Integer `nexa:"string=Filled Positions,default=0"`

	// Attendance cost drivers
	AbsenceDays   sdk.Float64 `nexa:"string=Absence Days,default=0"`
	OvertimeHours sdk.Float64 `nexa:"string=Overtime Hours,default=0"`

	// Money
	PayrollCost sdk.Numeric `nexa:"string=Payroll Cost,precision=18,scale=2,default=0"`
	AvgPayCost  sdk.Numeric `nexa:"string=Avg Cost / Head,precision=18,scale=2,default=0"`

	// Learning / performance / skills
	TrainingCost     sdk.Numeric `nexa:"string=Training Cost,precision=18,scale=2,default=0"`
	TrainingComplete sdk.Integer `nexa:"string=Training Completed,default=0"`
	ReviewsComplete  sdk.Integer `nexa:"string=Reviews Completed,default=0"`
	ReviewsTotal     sdk.Integer `nexa:"string=Reviews Due,default=0"`
	SkillsGapCount   sdk.Integer `nexa:"string=Skills Gaps,default=0"`
}

// ---------------------------------------------------------------------------
// hr.position.plan
// ---------------------------------------------------------------------------

// HrPositionPlan records an approved headcount + budget for a department (optionally
// a specific job). Filled and Vacancy are projected live from active employees so the
// plan always reflects current occupancy without editing any employee record.
type HrPositionPlan struct {
	sdk.Model `nexa:"model=hr.position.plan"`

	Name             sdk.String                 `nexa:"required,string=Reference"`
	DepartmentID     sdk.Many2One[HrDepartment] `nexa:"required,string=Department,index"`
	JobID            sdk.Many2One[HrJob]        `nexa:"string=Job Position,index"`
	PlannedHeadcount sdk.Integer                `nexa:"string=Planned Headcount,default=0"`
	BudgetAnnual     sdk.Numeric                `nexa:"string=Annual Staffing Budget,precision=18,scale=2,default=0"`
	EffectiveFrom    sdk.Date                   `nexa:"string=Effective From"`
	FiscalYear       sdk.Integer                `nexa:"string=Fiscal Year"`
	Approved         sdk.Boolean                `nexa:"string=Approved,default=false,index"`
	ApproverID       sdk.Integer                `nexa:"string=Approver ID"`
	Note             sdk.Text                   `nexa:"string=Note"`
	CompanyID        sdk.Many2One[CoreCompany]  `nexa:"string=Company"`

	FilledCount sdk.Integer `nexa:"string=Filled,compute=compute_plan_filled"`
	Vacancy     sdk.Integer `nexa:"string=Vacancy,compute=compute_plan_vacancy"`
}

// ---------------------------------------------------------------------------
// hr.workforce.scenario
// ---------------------------------------------------------------------------

// HrWorkforceScenario projects headcount forward from the current base by a growth
// and an attrition factor over a horizon, and estimates the cost of the outcome from
// an average cost per head. Base headcount and average cost default to the live
// org-wide figures when left at zero.
type HrWorkforceScenario struct {
	sdk.Model `nexa:"model=hr.workforce.scenario"`

	Name           sdk.String  `nexa:"required,string=Scenario"`
	HorizonMonths  sdk.Integer `nexa:"string=Horizon (months),default=12"`
	GrowthPct      sdk.Float64 `nexa:"string=Growth %,default=0,help=Net headcount growth applied across the horizon."`
	AttritionPct   sdk.Float64 `nexa:"string=Attrition %,default=0,help=Expected losses (of base) applied across the horizon."`
	BaseOverride   sdk.Integer `nexa:"string=Base Headcount Override,default=0,help=Leave 0 to use the live org-wide active headcount."`
	AvgCostPerHead sdk.Numeric `nexa:"string=Avg Cost / Head,precision=18,scale=2,default=0"`
	Active         sdk.Boolean `nexa:"string=Active,default=true"`
	Note           sdk.Text    `nexa:"string=Note"`

	BaseHeadcount      sdk.Integer `nexa:"string=Base Headcount,compute=compute_scenario_base"`
	ProjectedHeadcount sdk.Integer `nexa:"string=Projected Headcount,compute=compute_scenario_projected"`
	ProjectedNetAdd    sdk.Integer `nexa:"string=Net Add,compute=compute_scenario_net_add"`
	ProjectedCost      sdk.Numeric `nexa:"string=Projected Cost,precision=18,scale=2,compute=compute_scenario_cost"`
}

func init() {
	sdk.MustRegister("hr_analytics",
		&HrWorkforceSnapshot{},
		&HrPositionPlan{},
		&HrWorkforceScenario{},
	)
}
