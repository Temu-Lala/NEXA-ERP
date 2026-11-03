package models

import (
	"context"
	"math"
	"time"

	"nexa/core/orm"
	"nexa/core/scheduler"
)

// HR-F11 behaviour layer. The pure helpers carry the workforce arithmetic —
// turnover / retention with an explicit denominator, forward projection of a
// scenario — so it is unit-testable with no database. The computes project a
// position plan's live occupancy and a scenario's projection against the current
// org. The scheduled refresh re-derives every department snapshot from the source
// operational models; it reads them by runtime name and degrades to zero when a
// source module is absent, and it only ever writes hr.workforce.snapshot rows —
// no employee, payslip, ledger or requisition record is ever touched here.

const (
	dateLayout   = "2006-01-02"
	datetimeFull = "2006-01-02 15:04:05"

	refreshCronCode = "hr.workforce.snapshot.refresh"
)

const (
	snapshotModel   = "hr.workforce.snapshot"
	positionPlanMod = "hr.position.plan"
	scenarioModel   = "hr.workforce.scenario"

	employeeMod     = "hr.employee"
	departmentMod   = "hr.department"
	overtimeMod     = "hr.overtime.request"
	leaveTxnMod     = "hr.leave.transaction"
	payslipMod      = "hr.payslip"
	skillGapMod     = "hr.skill.gap"
	jobOpeningMod   = "hr.job.opening"
	positionPlanSrc = "hr.position.plan"
)

func init() {
	orm.RegisterCompute(positionPlanMod, "filled_count", []string{"department_id", "job_id"}, computePlanFilled)
	orm.RegisterCompute(positionPlanMod, "vacancy", []string{"department_id", "job_id", "planned_headcount"}, computePlanVacancy)
	orm.RegisterCompute(scenarioModel, "base_headcount", []string{"base_override"}, computeScenarioBase)
	orm.RegisterCompute(scenarioModel, "projected_headcount", []string{"base_override", "growth_pct", "attrition_pct"}, computeScenarioProjected)
	orm.RegisterCompute(scenarioModel, "projected_net_add", []string{"base_override", "growth_pct", "attrition_pct"}, computeScenarioNetAdd)
	orm.RegisterCompute(scenarioModel, "projected_cost", []string{"base_override", "growth_pct", "attrition_pct", "avg_cost_per_head"}, computeScenarioCost)
	scheduler.RegisterCronHandler(refreshCronCode, runWorkforceSnapshot)
}

// ---------------------------------------------------------------------------
// Pure logic (unit-tested)
// ---------------------------------------------------------------------------

// turnoverRate returns terminations as a percentage of the average of opening and
// closing headcount. When there is no one to measure against (avg <= 0) it is 0 —
// a rate is never invented for an empty population.
func turnoverRate(terminations, opening, closing int) float64 {
	avg := float64(opening+closing) / 2.0
	if avg <= 0 {
		return 0
	}
	return round2(float64(terminations) / avg * 100.0)
}

// retentionRate returns the share of the opening headcount retained over the
// period, as a percentage clamped to [0, 100]. With no opening population it is 0.
func retentionRate(terminations, opening int) float64 {
	if opening <= 0 {
		return 0
	}
	r := (1.0 - float64(terminations)/float64(opening)) * 100.0
	if r < 0 {
		return 0
	}
	if r > 100 {
		return 100
	}
	return round2(r)
}

// projectHeadcount applies a growth factor and an attrition factor (both in
// percent) to a base headcount and returns the rounded result, never negative.
func projectHeadcount(base int, growthPct, attritionPct float64) int {
	if base <= 0 {
		return 0
	}
	factor := (1.0 + growthPct/100.0) * (1.0 - clampPct(attritionPct)/100.0)
	p := float64(base) * factor
	if p < 0 {
		p = 0
	}
	return int(math.Round(p))
}

// average returns the mean of a set of values, or 0 for an empty set.
func average(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var s float64
	for _, v := range vals {
		s += v
	}
	return round2(s / float64(len(vals)))
}

// clampPct bounds a percentage to [0, 100] for attrition (a loss cannot exceed the
// population nor be negative).
func clampPct(p float64) float64 {
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// ---------------------------------------------------------------------------
// Field / value helpers
// ---------------------------------------------------------------------------

func intOr(v interface{}) int {
	n, ok := orm.CoerceInt64(v)
	if !ok {
		return 0
	}
	return int(n)
}

func floatOr(v interface{}) float64 {
	f, ok := orm.CoerceFloat64(v)
	if !ok {
		return 0
	}
	return f
}

func parseDate(v interface{}) time.Time {
	s := orm.AsString(v)
	if len(s) < 10 {
		return time.Time{}
	}
	if t, err := time.Parse(dateLayout, s[:10]); err == nil {
		return t.UTC()
	}
	return time.Time{}
}

func fmtDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(dateLayout)
}

func fmtDT(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(datetimeFull)
}

// inPeriod reports whether a date falls within [start, end] (inclusive). A zero
// date is never in period.
func inPeriod(t, start, end time.Time) bool {
	if t.IsZero() {
		return false
	}
	return !t.Before(start) && !t.After(end)
}

// deptID extracts an integer department id from a record's department_id field,
// which the ORM surfaces as a scalar id when read via Search.
func deptID(rec map[string]interface{}) int {
	return intOr(rec["department_id"])
}

// ---------------------------------------------------------------------------
// Computes — position plan
// ---------------------------------------------------------------------------

// countActive fills a plan's occupancy: active employees in the plan's department
// (and job, when the plan is job-specific).
func countActive(ctx context.Context, departmentID, jobID int) int {
	bypass := orm.ContextWithBypass(ctx, true)
	crit := [][]interface{}{{"active", "=", true}}
	if departmentID > 0 {
		crit = append(crit, []interface{}{"department_id", "=", departmentID})
	}
	if jobID > 0 {
		crit = append(crit, []interface{}{"job_id", "=", jobID})
	}
	n, err := orm.SearchCount(bypass, employeeMod, crit)
	if err != nil {
		return 0
	}
	return int(n)
}

func computePlanFilled(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return countActive(ctx, deptID(rec), intOr(rec["job_id"])), nil
}

func computePlanVacancy(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	filled := intOr(rec["filled_count"])
	if filled == 0 {
		filled = countActive(ctx, deptID(rec), intOr(rec["job_id"]))
	}
	planned := intOr(rec["planned_headcount"])
	// Vacancy is the unfilled approved seats; a negative value flags over-staffing.
	return planned - filled, nil
}

// ---------------------------------------------------------------------------
// Computes — scenario
// ---------------------------------------------------------------------------

// orgActiveHeadcount is the live company-wide active employee count (the default
// base when a scenario does not override it).
func orgActiveHeadcount(ctx context.Context) int {
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), employeeMod, [][]interface{}{
		{"active", "=", true},
	})
	if err != nil {
		return 0
	}
	return int(n)
}

func scenarioBase(ctx context.Context, rec map[string]interface{}) int {
	if override := intOr(rec["base_override"]); override > 0 {
		return override
	}
	return orgActiveHeadcount(ctx)
}

func computeScenarioBase(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return scenarioBase(ctx, rec), nil
}

func computeScenarioProjected(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	base := scenarioBase(ctx, rec)
	return projectHeadcount(base, floatOr(rec["growth_pct"]), floatOr(rec["attrition_pct"])), nil
}

func computeScenarioNetAdd(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	base := scenarioBase(ctx, rec)
	projected := projectHeadcount(base, floatOr(rec["growth_pct"]), floatOr(rec["attrition_pct"]))
	return projected - base, nil
}

func computeScenarioCost(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	base := scenarioBase(ctx, rec)
	projected := projectHeadcount(base, floatOr(rec["growth_pct"]), floatOr(rec["attrition_pct"]))
	avg := floatOr(rec["avg_cost_per_head"])
	return round2(float64(projected) * avg), nil
}

// ---------------------------------------------------------------------------
// Workforce snapshot refresh
// ---------------------------------------------------------------------------

// deptMetrics accumulates the per-department counters before they are written.
type deptMetrics struct {
	headcountActive int
	hires           int
	terminations    int
	tenureSum       float64
	openPositions   int
	absenceDays     float64
	overtimeHours   float64
	payrollCost     float64
	skillsGapCount  int
}

// runWorkforceSnapshot rebuilds the current-year rolling snapshot for every
// department from the operational modules. It is idempotent (upserts one rolling
// row per department) and fully read-only against source data.
func runWorkforceSnapshot(ctx context.Context, _ map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	now := time.Now().UTC()
	periodStart := time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)

	depts, err := orm.Search(bypass, departmentMod, [][]interface{}{})
	if err != nil || len(depts) == 0 {
		return nil
	}

	// ---- employees: headcount, hires, terminations, tenure, emp->dept map ----
	emps, err := orm.Search(bypass, employeeMod, [][]interface{}{})
	if err != nil {
		emps = nil
	}
	empDept := make(map[int]int, len(emps))
	metrics := make(map[int]*deptMetrics, len(depts))
	metricFor := func(d int) *deptMetrics {
		if m, ok := metrics[d]; ok {
			return m
		}
		m := &deptMetrics{}
		metrics[d] = m
		return m
	}
	for _, e := range emps {
		eid := intOr(e["id"])
		d := deptID(e)
		if eid > 0 && d > 0 {
			empDept[eid] = d
		}
		if d <= 0 {
			continue
		}
		m := metricFor(d)
		if orm.AsBool(e["active"]) {
			m.headcountActive++
			if hd := parseDate(e["hire_date"]); !hd.IsZero() {
				m.tenureSum += now.Sub(hd).Hours() / 24.0
			}
		}
		if hd := parseDate(e["hire_date"]); inPeriod(hd, periodStart, now) {
			m.hires++
		}
		if td := parseDate(e["termination_date"]); inPeriod(td, periodStart, now) {
			m.terminations++
		}
	}

	// ---- secondary sources bucketed by each row's employee's department ----
	addAgg(bypass, overtimeMod, metricFor, func(r map[string]interface{}) (int, float64) {
		if orm.AsString(r["state"]) != "validate" {
			return 0, 0
		}
		if !inPeriod(parseDate(r["date"]), periodStart, now) {
			return 0, 0
		}
		return empDept[intOr(r["employee_id"])], floatOr(r["hours"])
	}, func(m *deptMetrics, v float64) { m.overtimeHours += v })

	addAgg(bypass, leaveTxnMod, metricFor, func(r map[string]interface{}) (int, float64) {
		if orm.AsString(r["category"]) != "usage" {
			return 0, 0
		}
		if !inPeriod(parseDate(r["date"]), periodStart, now) {
			return 0, 0
		}
		return empDept[intOr(r["employee_id"])], math.Abs(floatOr(r["amount"]))
	}, func(m *deptMetrics, v float64) { m.absenceDays += v })

	addAgg(bypass, payslipMod, metricFor, func(r map[string]interface{}) (int, float64) {
		if !inPeriod(parseDate(r["date_start"]), periodStart, now) {
			return 0, 0
		}
		return empDept[intOr(r["employee_id"])], floatOr(r["gross"])
	}, func(m *deptMetrics, v float64) { m.payrollCost += v })

	addAgg(bypass, skillGapMod, metricFor, func(r map[string]interface{}) (int, float64) {
		if orm.AsString(r["state"]) != "open" {
			return 0, 0
		}
		return empDept[intOr(r["employee_id"])], 1
	}, func(m *deptMetrics, v float64) { m.skillsGapCount += int(v) })

	// ---- job openings by their own department ----
	if openings, err := orm.Search(bypass, jobOpeningMod, [][]interface{}{}); err == nil {
		for _, o := range openings {
			if orm.AsString(o["state"]) != "open" {
				continue
			}
			d := deptID(o)
			if d <= 0 {
				continue
			}
			metricFor(d).openPositions += intOr(o["number_of_positions"])
		}
	}

	// ---- write rolling snapshots for departments with any signal ----
	for _, dept := range depts {
		d := intOr(dept["id"])
		if d <= 0 {
			continue
		}
		m, ok := metrics[d]
		if !ok {
			continue
		}
		if m.headcountActive == 0 && m.openPositions == 0 && m.hires == 0 && m.terminations == 0 {
			continue
		}
		closing := m.headcountActive
		opening := closing - m.hires + m.terminations
		if opening < 0 {
			opening = 0
		}
		turnover := turnoverRate(m.terminations, opening, closing)
		retention := retentionRate(m.terminations, opening)
		avgTenure := 0.0
		if m.headcountActive > 0 {
			avgTenure = round2(m.tenureSum / float64(m.headcountActive))
		}
		filled := closing
		planned := m.headcountActive + m.openPositions

		vals := map[string]interface{}{
			"department_id":     d,
			"period":            string(PeriodRolling),
			"period_start":      fmtDate(periodStart),
			"period_end":        fmtDate(now),
			"computed_on":       fmtDT(now),
			"headcount_active":  filled,
			"hires":             m.hires,
			"terminations":      m.terminations,
			"opening_headcount": opening,
			"avg_tenure_days":   avgTenure,
			"turnover_rate":     turnover,
			"retention_rate":    retention,
			"open_positions":    m.openPositions,
			"planned_positions": planned,
			"filled_positions":  filled,
			"absence_days":      round2(m.absenceDays),
			"overtime_hours":    round2(m.overtimeHours),
			"payroll_cost":      round2(m.payrollCost),
			"avg_pay_cost":      safeDiv(m.payrollCost, float64(closing)),
			"skills_gap_count":  m.skillsGapCount,
		}

		existing, _ := orm.SearchOne(bypass, snapshotModel, map[string]interface{}{
			"department_id": d,
			"period":        string(PeriodRolling),
			"period_start":  fmtDate(periodStart),
		})
		if existing == nil {
			_, _ = orm.Create(bypass, orm.RegistryModel(snapshotModel), vals)
			continue
		}
		sid := intOr(existing["id"])
		if sid > 0 {
			_ = orm.UpdateRecordByID(bypass, snapshotModel, sid, vals)
		}
	}
	return nil
}

// addAgg runs a defensive aggregation query over a source model, routing each
// qualifying row's value into its department's metrics via the shared metricFor
// accessor. A missing model (Search error) is silently skipped so this companion
// needs no hard dependency on the source modules.
func addAgg(
	ctx context.Context,
	model string,
	metricFor func(int) *deptMetrics,
	classify func(map[string]interface{}) (dept int, value float64),
	apply func(*deptMetrics, float64),
) {
	rows, err := orm.Search(ctx, model, [][]interface{}{})
	if err != nil {
		return
	}
	for _, r := range rows {
		d, v := classify(r)
		if d <= 0 {
			continue
		}
		apply(metricFor(d), v)
	}
}

func safeDiv(num, den float64) float64 {
	if den <= 0 {
		return 0
	}
	return round2(num / den)
}
