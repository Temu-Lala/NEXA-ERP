package models

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nexa/core/orm"
	"nexa/core/scheduler"
)

// Learning & development behaviour layer: the pure scoring / gap maths lives here
// (unit-testable, no database) plus the stored computes that project an attempt's
// percentage and pass, and a skill gap's shortfall onto their records. A scheduled
// job surfaces certificates approaching / past their expiry as reminders, created
// idempotently so repeated runs never duplicate an open alert.

const (
	dateLayout     = "2006-01-02"
	datetimeFull   = "2006-01-02 15:04:05"
	certCronCode   = "hr.training.cert.scan"
	expiryWindow   = 30 // days ahead to flag a certificate
	certificateMod = "hr.training.certificate"
)

const (
	providerModel      = "hr.training.provider"
	courseModel        = "hr.training.course"
	requestModel       = "hr.training.request"
	enrollmentModel    = "hr.training.enrollment"
	pathModel          = "hr.training.path"
	examModel          = "hr.training.exam"
	attemptModel       = "hr.training.attempt"
	skillGapModel      = "hr.skill.gap"
	certAlertModel     = "hr.training.cert.alert"
	employeeModel      = "hr.employee"
	jobSkillModel      = "hr.job.skill"
	employeeSkillModel = "hr.employee.skill"
)

func init() {
	orm.RegisterCompute(attemptModel, "percentage", []string{"total_score", "max_score"}, computeAttemptPercentage)
	orm.RegisterCompute(attemptModel, "passed", []string{"percentage", "exam_id"}, computeAttemptPassed)
	orm.RegisterCompute(skillGapModel, "gap", []string{"current_level", "required_level"}, computeSkillGap)
	scheduler.RegisterCronHandler(certCronCode, runCertScan)
}

// ---------------------------------------------------------------------------
// Pure maths
// ---------------------------------------------------------------------------

// round2 snaps to 2 decimals with integer-safe half-up rounding.
func round2(f float64) float64 {
	scaled := f * 100
	if scaled >= 0 {
		return float64(int64(scaled+0.5)) / 100
	}
	return float64(int64(scaled-0.5)) / 100
}

// attemptPct is total as a percent of max. A non-positive max yields 0 so an
// un-scored attempt never reports a percentage.
func attemptPct(total, max float64) float64 {
	if max <= 0 {
		return 0
	}
	return round2(total / max * 100)
}

// passedFor decides a pass from a percentage and the exam's pass mark. An unset
// (<= 0) pass mark is not a pass/fail gate, so it never marks an attempt passed.
func passedFor(pct, passMark float64) bool {
	if passMark <= 0 {
		return false
	}
	return pct >= passMark
}

// levelIndex orders skill levels beginner < intermediate < advanced < expert.
func levelIndex(level string) int {
	switch level {
	case "intermediate":
		return 1
	case "advanced":
		return 2
	case "expert":
		return 3
	default: // beginner or unknown
		return 0
	}
}

// gapBetween is the shortfall in levels (never negative — exceeding the
// requirement is not a gap).
func gapBetween(current, required string) int {
	g := levelIndex(required) - levelIndex(current)
	if g < 0 {
		return 0
	}
	return g
}

// ---------------------------------------------------------------------------
// Field helpers
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

func dateStr(v interface{}) string {
	s := strings.TrimSpace(orm.AsString(v))
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func parseDate(v interface{}) time.Time {
	d := dateStr(v)
	if d == "" {
		return time.Time{}
	}
	t, err := time.Parse(dateLayout, d)
	if err != nil {
		return time.Time{}
	}
	return t
}

// ---------------------------------------------------------------------------
// Computes
// ---------------------------------------------------------------------------

func computeAttemptPercentage(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	return attemptPct(floatOr(rec["total_score"]), floatOr(rec["max_score"])), nil
}

func computeAttemptPassed(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	pct := floatOr(rec["percentage"])
	if pct == 0 {
		pct = attemptPct(floatOr(rec["total_score"]), floatOr(rec["max_score"]))
	}
	examID := intOr(rec["exam_id"])
	if examID <= 0 {
		return false, nil
	}
	exam, err := orm.SearchOne(orm.ContextWithBypass(ctx, true), examModel, map[string]interface{}{"id": examID})
	if err != nil || exam == nil {
		return false, nil
	}
	return passedFor(pct, floatOr(exam["pass_mark"])), nil
}

func computeSkillGap(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	return gapBetween(orm.AsString(rec["current_level"]), orm.AsString(rec["required_level"])), nil
}

// ---------------------------------------------------------------------------
// Certificate expiry scan
// ---------------------------------------------------------------------------

func runCertScan(ctx context.Context, _ map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	now := time.Now().UTC()
	limit := now.AddDate(0, 0, expiryWindow)

	certs, err := orm.Search(bypass, certificateMod, [][]interface{}{})
	if err != nil {
		return nil
	}
	for _, c := range certs {
		expiry := parseDate(c["expiry_date"])
		if expiry.IsZero() || expiry.After(limit) {
			continue
		}
		days := int(expiry.Sub(now).Hours() / 24)
		emitCertAlert(bypass, c, expiry.Format(dateLayout), days)
	}
	return nil
}

// emitCertAlert creates one open alert per certificate while one is still open,
// so repeated runs stay idempotent.
func emitCertAlert(ctx context.Context, cert map[string]interface{}, expiry string, days int) {
	certID := intOr(cert["id"])
	if certID <= 0 {
		return
	}
	open, _ := orm.SearchCount(ctx, certAlertModel, [][]interface{}{{"certificate_id", "=", certID}, {"open", "=", true}})
	if open > 0 {
		return
	}
	word := "expires"
	if days < 0 {
		word = "expired"
	}
	inst, ok := orm.Registry[certAlertModel]
	if !ok {
		return
	}
	vals := map[string]interface{}{
		"certificate_id": certID,
		"expiry_date":    expiry,
		"days_remaining": days,
		"open":           true,
		"message":        fmt.Sprintf("Certificate %s %s on %s.", orDash(orm.AsString(cert["name"])), word, expiry),
	}
	if emp := intOr(cert["employee_id"]); emp > 0 {
		vals["employee_id"] = emp
	}
	if crs := intOr(cert["course_id"]); crs > 0 {
		vals["course_id"] = crs
	}
	_, _ = orm.Create(ctx, inst, vals)
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(unnamed)"
	}
	return s
}
