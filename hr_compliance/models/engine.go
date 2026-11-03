package models

import (
	"context"
	"time"

	"nexa/core/orm"
	"nexa/core/scheduler"
)

// HR-F10 behaviour layer. The pure helpers below carry the compliance and
// offboarding arithmetic (notice period, renewal windows, workflow legality) so
// it is unit-testable with no database. The computes project an acknowledgement's
// related user id (for the ownership rule) and a policy's live pending-ack count.
// The scheduled scan provisions per-employee compliance lines from active
// requirements and refreshes their verdicts idempotently; it never fabricates
// completions — a line only becomes compliant/expired from a stored last_completed.

const (
	dateLayout   = "2006-01-02"
	datetimeFull = "2006-01-02 15:04:05"

	scanCronCode = "hr.compliance.scan"

	employeeMod = "hr.employee"
)

const (
	policyModel      = "hr.policy"
	ackModel         = "hr.policy.ack"
	grievanceModel   = "hr.grievance"
	grievanceHist    = "hr.grievance.history"
	discModel        = "hr.disciplinary.case"
	discHist         = "hr.disciplinary.history"
	incidentModel    = "hr.incident"
	requirementMod   = "hr.compliance.requirement"
	lineModel        = "hr.compliance.line"
	terminationModel = "hr.termination"
	offLineModel     = "hr.offboarding.line"
	medicalModel     = "hr.medical.record"
)

func init() {
	orm.RegisterCompute(ackModel, "employee_user_id", []string{"employee_id"}, computeAckEmployeeUser)
	orm.RegisterCompute(policyModel, "ack_required_count", []string{"id"}, computePolicyAckPending)
	scheduler.RegisterCronHandler(scanCronCode, runComplianceScan)
}

// ---------------------------------------------------------------------------
// Pure logic (unit-tested)
// ---------------------------------------------------------------------------

// addDays returns t shifted by n calendar days (UTC-truncated to date).
func addDays(t time.Time, n int) time.Time {
	if t.IsZero() {
		return time.Time{}
	}
	return t.AddDate(0, 0, n)
}

// deriveLastWorkingDay is the notice-period projection: initiation date plus the
// contractual notice, or the initiation date itself when no notice applies.
func deriveLastWorkingDay(init time.Time, noticeDays int) time.Time {
	if init.IsZero() {
		return time.Time{}
	}
	if noticeDays <= 0 {
		return init
	}
	return init.AddDate(0, 0, noticeDays)
}

// complianceStatus decides an employee's verdict for one requirement given when
// they last satisfied it and the renewal cadence. Never-completed obligations are
// "pending" (nothing is asserted on their behalf). A zero frequency is one-time:
// once completed it stays compliant. Otherwise the next due date is
// last_completed + frequency, and a past due flips the verdict to "expired".
func complianceStatus(frequencyDays int, lastCompleted, today time.Time) (ComplianceState, time.Time) {
	if lastCompleted.IsZero() {
		return CompPending, time.Time{}
	}
	if frequencyDays <= 0 {
		return CompCompliant, time.Time{}
	}
	due := lastCompleted.AddDate(0, 0, frequencyDays)
	if today.After(due) {
		return CompExpired, due
	}
	return CompCompliant, due
}

// termAllowed reports whether a termination may move from one state to another.
// The workflow is draft -> submitted -> approved -> completed, with cancellation
// from draft/submitted/approved and a rejection that returns a submitted record
// to draft. Completed and cancelled are terminal.
func termAllowed(from, to string) bool {
	switch TerminationState(from) {
	case TermDraft:
		return to == string(TermSubmitted) || to == string(TermCancelled)
	case TermSubmitted:
		return to == string(TermApproved) || to == string(TermDraft) || to == string(TermCancelled)
	case TermApproved:
		return to == string(TermCompleted) || to == string(TermCancelled)
	default:
		return false
	}
}

// caseAllowed reports a legal transition for the shared grievance / incident flow.
func caseAllowed(from, to string) bool {
	switch CaseState(from) {
	case CaseDraft:
		return to == string(CaseSubmitted) || to == string(CaseCancelled)
	case CaseSubmitted:
		return to == string(CaseInvestigating) || to == string(CaseResolved) || to == string(CaseCancelled)
	case CaseInvestigating:
		return to == string(CaseResolved) || to == string(CaseCancelled)
	case CaseResolved:
		return to == string(CaseClosed) || to == string(CaseInvestigating)
	default:
		return false
	}
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

func parseDT(v interface{}) time.Time {
	s := orm.AsString(v)
	if len(s) < 10 {
		return time.Time{}
	}
	for _, layout := range []string{datetimeFull, "2006-01-02T15:04:05", "2006-01-02 15:04:05Z07:00", dateLayout} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
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

// employeeUser resolves hr.employee.user_id for the given employee id.
func employeeUser(ctx context.Context, employeeID int) int {
	if employeeID <= 0 {
		return 0
	}
	emp, err := orm.SearchOne(orm.ContextWithBypass(ctx, true), employeeMod, map[string]interface{}{"id": employeeID})
	if err != nil || emp == nil {
		return 0
	}
	return intOr(emp["user_id"])
}

// ---------------------------------------------------------------------------
// Computes
// ---------------------------------------------------------------------------

func computeAckEmployeeUser(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return employeeUser(ctx, intOr(rec["employee_id"])), nil
}

// computePolicyAckPending counts not-yet-answered acknowledgements for a policy.
func computePolicyAckPending(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	id := intOr(rec["id"])
	if id <= 0 {
		return 0, nil
	}
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), ackModel, [][]interface{}{
		{"policy_id", "=", id},
		{"state", "=", string(AckPending)},
	})
	if err != nil {
		return 0, nil
	}
	return n, nil
}

// ---------------------------------------------------------------------------
// Compliance scan
// ---------------------------------------------------------------------------

// scopedEmployees returns active employee ids a requirement applies to.
func scopedEmployees(ctx context.Context, req map[string]interface{}) []int {
	scope := orm.AsString(req["scope"])
	crit := [][]interface{}{{"active", "=", true}}
	switch CompScope(scope) {
	case ScopeDepartment:
		dept := intOr(req["department_id"])
		if dept <= 0 {
			return nil
		}
		crit = append(crit, []interface{}{"department_id", "=", dept})
	case ScopeJob:
		job := intOr(req["job_id"])
		if job <= 0 {
			return nil
		}
		crit = append(crit, []interface{}{"job_id", "=", job})
	}
	emps, err := orm.Search(ctx, employeeMod, crit)
	if err != nil {
		return nil
	}
	ids := make([]int, 0, len(emps))
	for _, e := range emps {
		if id := intOr(e["id"]); id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

// runComplianceScan provisions missing compliance lines for every active
// requirement and refreshes each line's verdict from its stored last_completed
// and the requirement's renewal cadence. Existing completions are never altered.
func runComplianceScan(ctx context.Context, _ map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	now := time.Now().UTC()

	reqs, err := orm.Search(bypass, requirementMod, [][]interface{}{{"active", "=", true}})
	if err != nil {
		return nil
	}
	for _, req := range reqs {
		reqID := intOr(req["id"])
		if reqID <= 0 {
			continue
		}
		freq := intOr(req["frequency_days"])
		category := orm.AsString(req["category"])
		mandatory := orm.AsBool(req["mandatory"])
		for _, empID := range scopedEmployees(bypass, req) {
			line, _ := orm.SearchOne(bypass, lineModel, map[string]interface{}{
				"employee_id":    empID,
				"requirement_id": reqID,
			})
			if line == nil {
				_, _ = orm.Create(bypass, orm.RegistryModel(lineModel), map[string]interface{}{
					"employee_id":    empID,
					"requirement_id": reqID,
					"category":       category,
					"mandatory":      mandatory,
					"state":          string(CompPending),
				})
				continue
			}
			// Refresh verdict from stored completion; never invent one.
			state, due := complianceStatus(freq, parseDate(line["last_completed"]), now)
			lineID := intOr(line["id"])
			if lineID <= 0 {
				continue
			}
			newState := string(state)
			newDue := fmtDate(due)
			if orm.AsString(line["state"]) == newState && orm.AsString(line["due_date"]) == newDue {
				continue
			}
			_ = orm.UpdateRecordByID(bypass, lineModel, lineID, map[string]interface{}{
				"state":    newState,
				"due_date": newDue,
			})
		}
	}
	return nil
}
