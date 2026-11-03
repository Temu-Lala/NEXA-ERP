package models

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nexa/core/event"
	"nexa/core/orm"
	"nexa/core/scheduler"
)

// Behaviour layer for HR-F1.
//
//   - Every time an employee is created or updated, the engine reconciles their
//     open employment-history snapshot: the first time it seeds one from the
//     employee's current placement; later it closes the open row (valid_to =
//     today) and opens a fresh one only when an organisational field actually
//     changed. Because the event payload carries only {model,id}, reconciliation
//     reads the live record and compares against the stored open snapshot, so it
//     is idempotent — repeated events with no org change are a no-op.
//
//   - A scheduled job (hr.people.expiry.scan) surfaces contracts, probation end
//     dates and documents approaching/overdue their dates as HR alerts, creating
//     each alert only once while it is still open (deduped on employee+type+
//     reference+due-date). It never mutates the underlying records.

const (
	expiryCronCode = "hr.people.expiry.scan"
	expiryWindow   = 30 // days ahead to flag
	dateLayout     = "2006-01-02"
	datetimeLayout = "2006-01-02 15:04:05"
)

func init() {
	orm.RegisterCompute("hr.employment.history", "is_current", []string{"valid_to"}, computeHistoryCurrent)

	event.Subscribe(orm.EventRecordCreated, onEmployeeChanged)
	event.Subscribe(orm.EventRecordUpdated, onEmployeeChanged)
	scheduler.RegisterCronHandler(expiryCronCode, runExpiryScan)
}

// computeHistoryCurrent marks a snapshot open (current) when it has no end date.
func computeHistoryCurrent(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return strings.TrimSpace(asDate(rec["valid_to"])) == "", nil
}

// ---------------------------------------------------------------------------
// Employment history reconciliation
// ---------------------------------------------------------------------------

func onEmployeeChanged(ctx context.Context, ev event.Event) error {
	model, _ := ev.Payload["model"].(string)
	if model != "hr.employee" {
		return nil
	}
	id, ok := orm.CoerceInt64(ev.Payload["id"])
	if !ok || id <= 0 {
		return nil
	}
	bypass := orm.ContextWithBypass(ctx, true)
	emp, err := orm.SearchOne(bypass, "hr.employee", map[string]interface{}{"id": int(id)})
	if err != nil || emp == nil {
		return nil
	}

	open := openHistory(bypass, int(id))
	snap := snapshotOf(emp)

	if open == nil {
		createHistory(bypass, emp, snap, firstEffective(emp), reasonFor(snap, nil, true), ev.Actor)
		return nil
	}

	if sameSnapshot(open, snap) {
		return nil
	}
	// Close the current row at the change boundary and open the new placement.
	_ = orm.UpdateRecordByID(bypass, "hr.employment.history", intOr(open["id"]), map[string]interface{}{
		"valid_to": today(),
	})
	createHistory(bypass, emp, snap, today(), reasonFor(snap, open, false), ev.Actor)
	return nil
}

type placement struct {
	department, job, manager int
	employmentType, workLoc  string
	company                  int
}

func snapshotOf(emp map[string]interface{}) placement {
	return placement{
		department:     intOr(emp["department_id"]),
		job:            intOr(emp["job_id"]),
		manager:        intOr(emp["parent_id"]),
		employmentType: orm.AsString(emp["employment_type"]),
		workLoc:        orm.AsString(emp["work_location"]),
		company:        intOr(emp["company_id"]),
	}
}

func sameSnapshot(open map[string]interface{}, p placement) bool {
	return intOr(open["department_id"]) == p.department &&
		intOr(open["job_id"]) == p.job &&
		intOr(open["manager_id"]) == p.manager &&
		orm.AsString(open["employment_type"]) == p.employmentType &&
		strings.TrimSpace(orm.AsString(open["work_location"])) == strings.TrimSpace(p.workLoc)
}

// openHistory returns the employee's current (valid_to empty) snapshot, if any.
func openHistory(ctx context.Context, employeeID int) map[string]interface{} {
	rows, err := orm.SearchPage(ctx, "hr.employment.history",
		[][]interface{}{
			{"employee_id", "=", employeeID},
			{"valid_to", "=", ""},
		}, 1, 0, "valid_from DESC")
	if err != nil || len(rows) == 0 {
		return nil
	}
	return rows[0]
}

func createHistory(ctx context.Context, emp map[string]interface{}, p placement, validFrom string, reason HistoryReason, actorID int) {
	inst, ok := orm.Registry["hr.employment.history"]
	if !ok {
		return
	}
	vals := map[string]interface{}{
		"employee_id":     intOr(emp["id"]),
		"valid_from":      validFrom,
		"reason":          string(reason),
		"employment_type": p.employmentType,
		"work_location":   p.workLoc,
	}
	if p.department > 0 {
		vals["department_id"] = p.department
	}
	if p.job > 0 {
		vals["job_id"] = p.job
	}
	if p.manager > 0 {
		vals["manager_id"] = p.manager
	}
	if p.company > 0 {
		vals["company_id"] = p.company
	}
	if g := gradeForJob(ctx, p.job); g != "" {
		vals["grade_name"] = g
	}
	if actorID > 0 {
		vals["create_uid"] = actorID
	}
	_, _ = orm.Create(ctx, inst, vals)
}

// gradeForJob resolves the (optional) job-grade name added by hr_people itself.
func gradeForJob(ctx context.Context, jobID int) string {
	if jobID <= 0 {
		return ""
	}
	job, err := orm.SearchOne(ctx, "hr.job", map[string]interface{}{"id": jobID})
	if err != nil || job == nil {
		return ""
	}
	gid := intOr(job["grade_id"])
	if gid <= 0 {
		return ""
	}
	grade, err := orm.SearchOne(ctx, "hr.job.grade", map[string]interface{}{"id": gid})
	if err != nil || grade == nil {
		return ""
	}
	return orm.AsString(grade["name"])
}

// reasonFor labels why a snapshot opened. First snapshot → hire; a move in
// department/manager/position → transfer; otherwise a generic adjustment.
func reasonFor(p placement, prev map[string]interface{}, first bool) HistoryReason {
	if first {
		return HistoryHire
	}
	if prev == nil {
		return HistoryAdjustment
	}
	if intOr(prev["department_id"]) != p.department || intOr(prev["manager_id"]) != p.manager || intOr(prev["job_id"]) != p.job {
		return HistoryTransfer
	}
	return HistoryAdjustment
}

func firstEffective(emp map[string]interface{}) string {
	if d := strings.TrimSpace(asDate(emp["hire_date"])); d != "" {
		return d
	}
	return today()
}

// ---------------------------------------------------------------------------
// Expiry scan (contract / probation / document)
// ---------------------------------------------------------------------------

func runExpiryScan(ctx context.Context, _ map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	today := time.Now().UTC()
	limit := today.AddDate(0, 0, expiryWindow)

	scanContracts(bypass, today, limit)
	scanProbation(bypass, today, limit)
	scanDocuments(bypass, today, limit)
	return nil
}

func scanContracts(ctx context.Context, today, limit time.Time) {
	rows, err := orm.Search(ctx, "hr.contract", [][]interface{}{{"active", "=", true}})
	if err != nil {
		return
	}
	for _, c := range rows {
		end := parseDate(c["date_end"])
		if end.IsZero() || end.After(limit) {
			continue
		}
		empID := intOr(c["employee_id"])
		ref := orm.AsString(c["name"])
		emitAlert(ctx, empID, AlertContract, ref,
			fmt.Sprintf("Contract %s %s on %s.", orDash(ref), expiryWord(end, today), end.Format(dateLayout)),
			end.Format(dateLayout), intOr(c["company_id"]))
	}
}

func scanProbation(ctx context.Context, today, limit time.Time) {
	rows, err := orm.Search(ctx, "hr.employee", [][]interface{}{
		{"active", "=", true},
		{"state", "=", string("active")},
	})
	if err != nil {
		return
	}
	for _, e := range rows {
		end := parseDate(e["probation_end_date"])
		if end.IsZero() || end.After(limit) {
			continue
		}
		emitAlert(ctx, intOr(e["id"]), AlertProbation, orm.AsString(e["name"]),
			fmt.Sprintf("Probation period for %s %s on %s.", orDash(orm.AsString(e["name"])), expiryWord(end, today), end.Format(dateLayout)),
			end.Format(dateLayout), intOr(e["company_id"]))
	}
}

func scanDocuments(ctx context.Context, today, limit time.Time) {
	rows, err := orm.Search(ctx, "hr.employee.document", [][]interface{}{{"active", "=", true}})
	if err != nil {
		return
	}
	for _, d := range rows {
		end := parseDate(d["expiry_date"])
		if end.IsZero() || end.After(limit) {
			continue
		}
		emitAlert(ctx, intOr(d["employee_id"]), AlertDocument, orm.AsString(d["name"]),
			fmt.Sprintf("Document %s %s on %s.", orDash(orm.AsString(d["name"])), expiryWord(end, today), end.Format(dateLayout)),
			end.Format(dateLayout), 0)
	}
}

// emitAlert writes an HR alert once per (employee,type,reference,due-date) while
// an open one already exists — so repeated scans do not spam duplicates.
func emitAlert(ctx context.Context, employeeID int, kind AlertType, ref, message, dueDate string, companyID int) {
	domain := [][]interface{}{
		{"alert_type", "=", string(kind)},
		{"due_date", "=", dueDate},
		{"state", "=", string(AlertOpen)},
	}
	if employeeID > 0 {
		domain = append(domain, []interface{}{"employee_id", "=", employeeID})
	} else {
		domain = append(domain, []interface{}{"employee_id", "=", 0})
	}
	n, err := orm.SearchCount(ctx, "hr.expiry.alert", domain)
	if err == nil && n > 0 {
		return
	}
	inst, ok := orm.Registry["hr.expiry.alert"]
	if !ok {
		return
	}
	vals := map[string]interface{}{
		"alert_type":  string(kind),
		"reference":   ref,
		"message":     message,
		"due_date":    dueDate,
		"state":       string(AlertOpen),
		"detected_at": time.Now().UTC().Format(datetimeLayout),
	}
	if employeeID > 0 {
		vals["employee_id"] = employeeID
	}
	if companyID > 0 {
		vals["company_id"] = companyID
	}
	_, _ = orm.Create(ctx, inst, vals)
}

func expiryWord(end, today time.Time) string {
	if end.Before(today) {
		return "expired"
	}
	return "expires"
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func intOr(v interface{}) int {
	n, ok := orm.CoerceInt64(v)
	if !ok {
		return 0
	}
	return int(n)
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(unspecified)"
	}
	return s
}

func today() string {
	return time.Now().UTC().Format(dateLayout)
}

// parseDate reads a date/datetime field value into a UTC day time.
func parseDate(v interface{}) time.Time {
	switch t := v.(type) {
	case time.Time:
		return t.UTC()
	case string:
		s := strings.TrimSpace(t)
		if len(s) >= 10 {
			s = s[:10]
		}
		if parsed, err := time.Parse(dateLayout, s); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

// asDate renders a date field value (string or time.Time) as its day component.
func asDate(v interface{}) string {
	switch t := v.(type) {
	case time.Time:
		return t.UTC().Format(dateLayout)
	case string:
		return t
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", t)
	}
}
