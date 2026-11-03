package models

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nexa/core/orm"
)

// Aggregated employee counters shown on department and job records. They are
// computed live (not stored) so they always reflect the current directory.
func init() {
	orm.RegisterCompute("hr.department", "total_employees", []string{"employee_ids"}, computeDepartmentEmployees)
	orm.RegisterCompute("hr.job", "no_of_employees", []string{"employee_ids"}, computeJobEmployees)
	orm.RegisterCompute("hr.employee.document", "status", []string{"expiry_date", "reminder_days"}, computeDocumentStatus)
	orm.RegisterCompute("hr.leave", "number_of_days", []string{"date_from", "date_to"}, computeLeaveDays)
	orm.RegisterCompute("hr.attendance", "worked_hours", []string{"check_in", "check_out"}, computeWorkedHours)
	orm.RegisterCompute("hr.contract", "state", []string{"date_end"}, computeContractState)
}

func computeDepartmentEmployees(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return countEmployees(ctx, "department_id", rec["id"])
}

func computeJobEmployees(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return countEmployees(ctx, "job_id", rec["id"])
}

// countEmployees counts non-archived employees linked to the given record.
func countEmployees(ctx context.Context, field string, rawID interface{}) (interface{}, error) {
	id, ok := orm.CoerceInt64(rawID)
	if !ok || id <= 0 {
		return 0, nil
	}
	bypass := orm.ContextWithBypass(ctx, true)
	n, err := orm.SearchCount(bypass, "hr.employee", [][]interface{}{
		{field, "=", int(id)},
		{"active", "=", true},
	})
	if err != nil {
		return nil, err
	}
	return n, nil
}

// computeDocumentStatus derives a document's validity live from its expiry date
// and reminder window: expired (past), expiring (within reminder_days), else valid.
// A document with no expiry date is always valid.
func computeDocumentStatus(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	raw := strings.TrimSpace(asDate(rec["expiry_date"]))
	if raw == "" {
		return string(DocumentValid), nil
	}
	if len(raw) > 10 {
		raw = raw[:10]
	}
	exp, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return string(DocumentValid), nil
	}
	reminder := 30
	if v, ok := orm.CoerceInt64(rec["reminder_days"]); ok && v > 0 {
		reminder = int(v)
	}
	days := int(time.Until(exp).Hours() / 24)
	switch {
	case days < 0:
		return string(DocumentExpired), nil
	case days <= reminder:
		return string(DocumentExpiring), nil
	default:
		return string(DocumentValid), nil
	}
}

// computeLeaveDays derives the inclusive number of calendar days between the
// start and end dates of a leave request (whole days, both ends counted).
func computeLeaveDays(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	from := strings.TrimSpace(asDate(rec["date_from"]))
	to := strings.TrimSpace(asDate(rec["date_to"]))
	if len(from) > 10 {
		from = from[:10]
	}
	if len(to) > 10 {
		to = to[:10]
	}
	if from == "" || to == "" {
		return 0.0, nil
	}
	d0, err0 := time.Parse("2006-01-02", from)
	d1, err1 := time.Parse("2006-01-02", to)
	if err0 != nil || err1 != nil {
		return 0.0, nil
	}
	days := int(d1.Sub(d0).Hours()/24) + 1
	if days < 0 {
		days = 0
	}
	return float64(days), nil
}

// computeWorkedHours returns the hours between check-in and check-out (0 when
// the employee has not checked out yet).
func computeWorkedHours(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	in, okIn := parseDateTime(rec["check_in"])
	out, okOut := parseDateTime(rec["check_out"])
	if !okIn || !okOut {
		return 0.0, nil
	}
	hours := out.Sub(in).Hours()
	if hours < 0 {
		hours = 0
	}
	return float64(int(hours*100+0.5)) / 100, nil
}

// computeContractState derives a contract's status from its end date: an empty
// end date is open-ended, past dates are expired, and dates within 30 days are
// flagged as expiring soon.
func computeContractState(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	raw := strings.TrimSpace(asDate(rec["date_end"]))
	if len(raw) > 10 {
		raw = raw[:10]
	}
	if raw == "" {
		return string(ContractOpen), nil
	}
	end, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return string(ContractOpen), nil
	}
	days := int(time.Until(end).Hours() / 24)
	switch {
	case days < 0:
		return string(ContractExpired), nil
	case days <= 30:
		return string(ContractExpiring), nil
	default:
		return string(ContractRunning), nil
	}
}

// parseDateTime parses a datetime field value (string or time.Time) using the
// layouts Nexa persists datetimes with.
func parseDateTime(v interface{}) (time.Time, bool) {
	switch t := v.(type) {
	case time.Time:
		return t, true
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return time.Time{}, false
		}
		for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02"} {
			if parsed, err := time.Parse(layout, s); err == nil {
				return parsed, true
			}
		}
	}
	return time.Time{}, false
}

// asDate renders a date field value (string or time.Time) as its day component.
func asDate(v interface{}) string {
	switch t := v.(type) {
	case time.Time:
		return t.Format("2006-01-02")
	case string:
		return t
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}
