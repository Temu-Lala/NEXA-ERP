package models

import (
	"context"
	"strings"
	"time"

	"nexa/core/orm"
)

// Benefits & compensation behaviour layer: the pure money/band maths lives here
// (unit-testable, no database) plus the stored computes that project a plan's
// contributions, a change's band check and a review line's increase onto their
// records. Nothing here mutates payroll; these are advisory derived values.

const (
	dateLayout   = "2006-01-02"
	datetimeFull = "2006-01-02 15:04:05"
)

const (
	benefitPlanModel = "hr.benefit.plan"
	enrollmentModel  = "hr.benefit.enrollment"
	dependentModel   = "hr.benefit.dependent"
	allowanceModel   = "hr.allowance"
	compChangeModel  = "hr.compensation.change"
	reviewCycleModel = "hr.comp.review.cycle"
	reviewLineModel  = "hr.comp.review.line"
	gradeModel       = "hr.job.grade"
	employeeModel    = "hr.employee"
)

func init() {
	orm.RegisterCompute(enrollmentModel, "employer_contribution", []string{"plan_id", "reference_salary"}, computeEmployerContribution)
	orm.RegisterCompute(enrollmentModel, "employee_contribution", []string{"plan_id", "reference_salary"}, computeEmployeeContribution)
	orm.RegisterCompute(compChangeModel, "within_band", []string{"new_grade_id", "new_salary"}, computeWithinBand)
	orm.RegisterCompute(reviewLineModel, "proposed_increase_pct", []string{"current_salary", "proposed_salary"}, computeReviewIncreasePct)
	orm.RegisterCompute(reviewLineModel, "within_budget", []string{"cycle_id", "proposed_increase_pct"}, computeReviewWithinBudget)
}

// ---------------------------------------------------------------------------
// Pure maths
// ---------------------------------------------------------------------------

// roundMoney snaps to 2 decimals with integer-safe half-up rounding.
func roundMoney(f float64) float64 {
	scaled := f * 100
	if scaled >= 0 {
		return float64(int64(scaled+0.5)) / 100
	}
	return float64(int64(scaled-0.5)) / 100
}

// contribution evaluates an employer contribution against a reference salary,
// honouring an optional cap (0 = no cap).
func contribution(contributionType string, rate, base, cap float64) float64 {
	var amount float64
	switch contributionType {
	case string(ContributionPct):
		amount = base * rate / 100
	default: // fixed
		amount = rate
	}
	if cap > 0 && amount > cap {
		amount = cap
	}
	return roundMoney(amount)
}

// employeeContribution is the percentage-of-salary amount withheld from the
// employee (rate is a percentage; 0 means fully employer-funded).
func employeeContribution(rate, base float64) float64 {
	return roundMoney(base * rate / 100)
}

// increasePct is the percentage change from old to new; a non-positive base
// yields 0 so an un-paid record cannot divide by zero.
func increasePct(old, newVal float64) float64 {
	if old <= 0 {
		return 0
	}
	return roundMoney((newVal - old) / old * 100)
}

// withinBand reports whether a salary sits inside a grade band. A 0 bound means
// that side is unbounded, so an un-set band always passes.
func withinBand(salary, min, max float64) bool {
	if min > 0 && salary < min {
		return false
	}
	if max > 0 && salary > max {
		return false
	}
	return true
}

// tenureDaysOK checks an employee's tenure (from hire date to the coverage/effective
// date, or today when either is empty) meets the plan's minimum. A 0 minimum always
// passes; an unknown hire date is treated as meeting no minimum only when the
// requirement is 0.
func tenureDaysOK(hire, target string, minDays int) bool {
	if minDays <= 0 {
		return true
	}
	if hire == "" {
		return false
	}
	start, err := time.Parse(dateLayout, hire)
	if err != nil {
		return false
	}
	end := start
	if target != "" {
		if t, err := time.Parse(dateLayout, target); err == nil {
			end = t
		}
	}
	days := int(end.Sub(start).Hours() / 24)
	return days >= minDays
}

// ---------------------------------------------------------------------------
// Field helpers
// ---------------------------------------------------------------------------

// activeInPeriod reports whether an effective-dated row covers the target date.
// An empty valid_from is "from the beginning" and an empty valid_to is "open
// ended", so un-dated rows always qualify.
func activeInPeriod(validFrom, validTo, target string) bool {
	if validFrom != "" && target != "" && validFrom > target {
		return false
	}
	if validTo != "" && target != "" && validTo < target {
		return false
	}
	return true
}

func dateStr(v interface{}) string {
	s := strings.TrimSpace(orm.AsString(v))
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

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

// ---------------------------------------------------------------------------
// Stored computes
// ---------------------------------------------------------------------------

func computeEmployerContribution(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	plan := loadPlan(ctx, intOr(rec["plan_id"]))
	if plan == nil {
		return 0.0, nil
	}
	return contribution(orm.AsString(plan["contribution_type"]),
		floatOr(plan["contribution_rate"]), floatOr(rec["reference_salary"]), floatOr(plan["contribution_cap"])), nil
}

func computeEmployeeContribution(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	plan := loadPlan(ctx, intOr(rec["plan_id"]))
	if plan == nil {
		return 0.0, nil
	}
	return employeeContribution(floatOr(plan["employee_rate"]), floatOr(rec["reference_salary"])), nil
}

func computeWithinBand(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	gradeID := intOr(rec["new_grade_id"])
	if gradeID <= 0 {
		return true, nil // no target grade: nothing to check
	}
	g, err := orm.SearchOne(orm.ContextWithBypass(ctx, true), gradeModel, map[string]interface{}{"id": gradeID})
	if err != nil || g == nil {
		return true, nil
	}
	return withinBand(floatOr(rec["new_salary"]), floatOr(g["salary_min"]), floatOr(g["salary_max"])), nil
}

func computeReviewIncreasePct(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	return increasePct(floatOr(rec["current_salary"]), floatOr(rec["proposed_salary"])), nil
}

func computeReviewWithinBudget(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	cycleID := intOr(rec["cycle_id"])
	if cycleID <= 0 {
		return true, nil
	}
	c, err := orm.SearchOne(orm.ContextWithBypass(ctx, true), reviewCycleModel, map[string]interface{}{"id": cycleID})
	if err != nil || c == nil {
		return true, nil
	}
	cap := floatOr(c["budget_increase_pct"])
	if cap <= 0 {
		return true, nil
	}
	pct := floatOr(rec["proposed_increase_pct"])
	if pct == 0 {
		pct = increasePct(floatOr(rec["current_salary"]), floatOr(rec["proposed_salary"]))
	}
	return pct <= cap, nil
}

func loadPlan(ctx context.Context, planID int) map[string]interface{} {
	if planID <= 0 {
		return nil
	}
	plan, err := orm.SearchOne(orm.ContextWithBypass(ctx, true), benefitPlanModel, map[string]interface{}{"id": planID})
	if err != nil || plan == nil {
		return nil
	}
	return plan
}
