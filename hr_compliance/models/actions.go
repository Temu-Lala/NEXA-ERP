package models

import (
	"context"
	"fmt"
	"time"

	"nexa/core/orm"
)

// HR-F10 workflow actions. Case transitions (grievance / incident) and the
// disciplinary and termination approvals are gated to HR Officers / Managers;
// disciplinary issuance and the termination decision additionally require the
// Manager group and reject self-approval (maker-checker). Publishing a policy
// fans out per-employee acknowledgement rows (idempotently), and an employee may
// acknowledge only their own row. Completing an APPROVED termination is the only
// place an employee's lifecycle status is written — it is explicit, gated and
// audited, never silent. Grievance and disciplinary changes are mirrored into
// their own append-only history models; final settlement / leave are captured as
// references only and never post into payroll.

func init() {
	orm.RegisterObjectAction(policyModel, "action_policy_publish", policyPublish)
	orm.RegisterObjectAction(policyModel, "action_policy_archive", policyArchive)
	orm.RegisterObjectAction(ackModel, "action_policy_ack", policyAcknowledge)

	orm.RegisterObjectAction(grievanceModel, "action_grievance_submit", func(c context.Context, _ string, id int, _ map[string]string) (string, error) {
		return moveCaseState(c, grievanceModel, id, CaseSubmitted, true)
	})
	orm.RegisterObjectAction(grievanceModel, "action_grievance_investigate", func(c context.Context, _ string, id int, _ map[string]string) (string, error) {
		return moveCaseState(c, grievanceModel, id, CaseInvestigating, true)
	})
	orm.RegisterObjectAction(grievanceModel, "action_grievance_resolve", grievanceResolve)
	orm.RegisterObjectAction(grievanceModel, "action_grievance_close", func(c context.Context, _ string, id int, _ map[string]string) (string, error) {
		return moveCaseState(c, grievanceModel, id, CaseClosed, true)
	})
	orm.RegisterObjectAction(grievanceModel, "action_grievance_reopen", func(c context.Context, _ string, id int, _ map[string]string) (string, error) {
		return moveCaseState(c, grievanceModel, id, CaseInvestigating, true)
	})
	orm.RegisterObjectAction(grievanceModel, "action_grievance_cancel", func(c context.Context, _ string, id int, _ map[string]string) (string, error) {
		return moveCaseState(c, grievanceModel, id, CaseCancelled, true)
	})

	orm.RegisterObjectAction(discModel, "action_disc_issue", discIssue)
	orm.RegisterObjectAction(discModel, "action_disc_appeal", func(c context.Context, _ string, id int, _ map[string]string) (string, error) {
		return moveDisc(c, id, DiscAppealed)
	})
	orm.RegisterObjectAction(discModel, "action_disc_uphold", func(c context.Context, _ string, id int, _ map[string]string) (string, error) {
		return moveDisc(c, id, DiscUpheld)
	})
	orm.RegisterObjectAction(discModel, "action_disc_close", func(c context.Context, _ string, id int, _ map[string]string) (string, error) {
		return moveDisc(c, id, DiscClosed)
	})

	orm.RegisterObjectAction(incidentModel, "action_incident_submit", func(c context.Context, _ string, id int, _ map[string]string) (string, error) {
		return moveCaseState(c, incidentModel, id, CaseSubmitted, false)
	})
	orm.RegisterObjectAction(incidentModel, "action_incident_investigate", func(c context.Context, _ string, id int, _ map[string]string) (string, error) {
		return moveCaseState(c, incidentModel, id, CaseInvestigating, false)
	})
	orm.RegisterObjectAction(incidentModel, "action_incident_resolve", func(c context.Context, _ string, id int, _ map[string]string) (string, error) {
		return moveCaseState(c, incidentModel, id, CaseResolved, false)
	})
	orm.RegisterObjectAction(incidentModel, "action_incident_close", func(c context.Context, _ string, id int, _ map[string]string) (string, error) {
		return moveCaseState(c, incidentModel, id, CaseClosed, false)
	})
	orm.RegisterObjectAction(incidentModel, "action_incident_cancel", func(c context.Context, _ string, id int, _ map[string]string) (string, error) {
		return moveCaseState(c, incidentModel, id, CaseCancelled, false)
	})

	orm.RegisterObjectAction(lineModel, "action_line_complete", lineComplete)

	orm.RegisterObjectAction(terminationModel, "action_term_submit", termSubmit)
	orm.RegisterObjectAction(terminationModel, "action_term_approve", func(c context.Context, _ string, id int, _ map[string]string) (string, error) {
		return decideTerm(c, id, TermApproved)
	})
	orm.RegisterObjectAction(terminationModel, "action_term_reject", func(c context.Context, _ string, id int, _ map[string]string) (string, error) {
		return decideTerm(c, id, TermDraft)
	})
	orm.RegisterObjectAction(terminationModel, "action_term_cancel", func(c context.Context, _ string, id int, _ map[string]string) (string, error) {
		return decideTerm(c, id, TermCancelled)
	})
	orm.RegisterObjectAction(terminationModel, "action_term_complete", termComplete)

	orm.RegisterObjectAction(offLineModel, "action_obline_done", obLineDone)
	orm.RegisterObjectAction(offLineModel, "action_obline_undo", obLineUndo)
}

// ---------------------------------------------------------------------------
// Policies & acknowledgement
// ---------------------------------------------------------------------------

func policyPublish(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("only an HR Officer or Manager can publish a policy")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, policyModel, id)
	if err != nil {
		return "", err
	}
	if PolicyState(orm.AsString(rec["state"])) == PolicyArchived {
		return "", fmt.Errorf("an archived policy cannot be published")
	}
	if err := orm.UpdateRecordByID(bypass, policyModel, id, map[string]interface{}{"state": string(PolicyPublished)}); err != nil {
		return "", err
	}
	if orm.AsBool(rec["requires_ack"]) {
		fanOutAcks(bypass, id)
	}
	return "", nil
}

func policyArchive(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("only an HR Officer or Manager can archive a policy")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	if _, err := getRecord(bypass, policyModel, id); err != nil {
		return "", err
	}
	return "", orm.UpdateRecordByID(bypass, policyModel, id, map[string]interface{}{"state": string(PolicyArchived)})
}

// fanOutAcks creates a pending acknowledgement for every active employee that
// does not already have one, so publishing is idempotent across re-runs.
func fanOutAcks(ctx context.Context, policyID int) {
	emps, err := orm.Search(ctx, employeeMod, [][]interface{}{{"active", "=", true}})
	if err != nil {
		return
	}
	for _, e := range emps {
		empID := intOr(e["id"])
		if empID <= 0 {
			continue
		}
		existing, _ := orm.SearchOne(ctx, ackModel, map[string]interface{}{
			"policy_id":   policyID,
			"employee_id": empID,
		})
		if existing != nil {
			continue
		}
		_, _ = createRecord(ctx, ackModel, map[string]interface{}{
			"policy_id":   policyID,
			"employee_id": empID,
			"state":       string(AckPending),
		})
	}
}

func policyAcknowledge(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, ackModel, id)
	if err != nil {
		return "", err
	}
	// A member of the self-service tier may answer only their own row; an
	// officer may record it on the employee's behalf.
	if intOr(rec["employee_user_id"]) != uid && !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("you can only acknowledge your own policy request")
	}
	decision := AckAcknowledged
	if orm.AsString(args["decision"]) == string(AckDeclined) {
		decision = AckDeclined
	}
	return "", orm.UpdateRecordByID(bypass, ackModel, id, map[string]interface{}{
		"state":    string(decision),
		"ack_date": time.Now().UTC().Format(datetimeFull),
	})
}

// ---------------------------------------------------------------------------
// Grievance / incident (shared case flow)
// ---------------------------------------------------------------------------

func moveCaseState(ctx context.Context, model string, id int, to CaseState, audit bool) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("only an HR Officer or Manager can progress this case")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, model, id)
	if err != nil {
		return "", err
	}
	from := orm.AsString(rec["state"])
	if !caseAllowed(from, string(to)) {
		return "", fmt.Errorf("illegal transition %s -> %s", from, string(to))
	}
	vals := map[string]interface{}{"state": string(to)}
	if (to == CaseClosed || to == CaseResolved) && orm.AsString(rec["closed_date"]) == "" {
		vals["closed_date"] = time.Now().UTC().Format(datetimeFull)
	}
	if to == CaseSubmitted && orm.AsString(rec["number"]) == "" {
		prefix := "INC"
		if model == grievanceModel {
			prefix = "GRV"
		}
		vals["number"] = fmt.Sprintf("%s-%05d", prefix, id)
	}
	if err := orm.UpdateRecordByID(bypass, model, id, vals); err != nil {
		return "", err
	}
	if audit {
		writeGrievanceHistory(bypass, id, HistState, "state", from, string(to), uid)
	}
	return "", nil
}

func grievanceResolve(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, grievanceModel, id)
	if err != nil {
		return "", err
	}
	if orm.AsString(rec["resolution"]) == "" {
		return "", fmt.Errorf("record a resolution before resolving the grievance")
	}
	return moveCaseState(ctx, grievanceModel, id, CaseResolved, true)
}

// ---------------------------------------------------------------------------
// Disciplinary (restricted, separate audit)
// ---------------------------------------------------------------------------

func discIssue(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireManager(ctx, uid) {
		return "", fmt.Errorf("only an HR Manager can issue a disciplinary measure")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, discModel, id)
	if err != nil {
		return "", err
	}
	if DisciplinaryState(orm.AsString(rec["state"])) != DiscDraft {
		return "", fmt.Errorf("only a draft disciplinary case can be issued")
	}
	vals := map[string]interface{}{"state": string(DiscIssued), "issued_by": uid}
	if orm.AsString(rec["issue_date"]) == "" {
		vals["issue_date"] = time.Now().UTC().Format(dateLayout)
	}
	if err := orm.UpdateRecordByID(bypass, discModel, id, vals); err != nil {
		return "", err
	}
	writeDiscHistory(bypass, id, HistIssue, "state", string(DiscDraft), string(DiscIssued), uid)
	return "", nil
}

func moveDisc(ctx context.Context, id int, to DisciplinaryState) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireManager(ctx, uid) {
		return "", fmt.Errorf("only an HR Manager can update a disciplinary case")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, discModel, id)
	if err != nil {
		return "", err
	}
	from := orm.AsString(rec["state"])
	if !discAllowed(from, string(to)) {
		return "", fmt.Errorf("illegal transition %s -> %s", from, string(to))
	}
	if err := orm.UpdateRecordByID(bypass, discModel, id, map[string]interface{}{"state": string(to)}); err != nil {
		return "", err
	}
	writeDiscHistory(bypass, id, HistState, "state", from, string(to), uid)
	return "", nil
}

func discAllowed(from, to string) bool {
	switch DisciplinaryState(from) {
	case DiscDraft:
		return to == string(DiscIssued)
	case DiscIssued:
		return to == string(DiscAppealed) || to == string(DiscClosed)
	case DiscAppealed:
		return to == string(DiscUpheld) || to == string(DiscClosed)
	case DiscUpheld:
		return to == string(DiscClosed)
	default:
		return false
	}
}

// ---------------------------------------------------------------------------
// Compliance lines
// ---------------------------------------------------------------------------

func lineComplete(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("only an HR Officer or Manager can record a compliance completion")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, lineModel, id)
	if err != nil {
		return "", err
	}
	reqID := intOr(rec["requirement_id"])
	freq := 0
	if reqID > 0 {
		if req, err := orm.SearchOne(bypass, requirementMod, map[string]interface{}{"id": reqID}); err == nil && req != nil {
			freq = intOr(req["frequency_days"])
		}
	}
	today := time.Now().UTC()
	state, due := complianceStatus(freq, today, today)
	return "", orm.UpdateRecordByID(bypass, lineModel, id, map[string]interface{}{
		"last_completed": today.Format(dateLayout),
		"state":          string(state),
		"due_date":       fmtDate(due),
	})
}

// ---------------------------------------------------------------------------
// Termination / offboarding
// ---------------------------------------------------------------------------

func termSubmit(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("only an HR Officer or Manager can submit a termination")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, terminationModel, id)
	if err != nil {
		return "", err
	}
	if !termAllowed(orm.AsString(rec["state"]), string(TermSubmitted)) {
		return "", fmt.Errorf("only a draft termination can be submitted")
	}
	if intOr(rec["employee_id"]) <= 0 {
		return "", fmt.Errorf("the termination has no employee")
	}
	vals := map[string]interface{}{"state": string(TermSubmitted), "requested_by": uid}
	if orm.AsString(rec["last_working_day"]) == "" {
		lwd := deriveLastWorkingDay(parseDate(rec["init_date"]), intOr(rec["notice_days"]))
		if !lwd.IsZero() {
			vals["last_working_day"] = lwd.Format(dateLayout)
		}
	}
	if err := orm.UpdateRecordByID(bypass, terminationModel, id, vals); err != nil {
		return "", err
	}
	provisionOffboarding(bypass, id)
	return "", nil
}

func decideTerm(ctx context.Context, id int, to TerminationState) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireManager(ctx, uid) {
		return "", fmt.Errorf("only an HR Manager can decide a termination")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, terminationModel, id)
	if err != nil {
		return "", err
	}
	from := orm.AsString(rec["state"])
	if !termAllowed(from, string(to)) {
		return "", fmt.Errorf("illegal transition %s -> %s", from, string(to))
	}
	if (to == TermApproved || to == TermDraft) && intOr(rec["requested_by"]) == uid {
		return "", fmt.Errorf("maker-checker: the requester cannot decide their own termination")
	}
	// A decision is recorded only; the employee record is untouched here.
	vals := map[string]interface{}{"state": string(to), "decide_date": time.Now().UTC().Format(dateLayout)}
	if to == TermApproved {
		vals["approved_by"] = uid
	}
	return "", orm.UpdateRecordByID(bypass, terminationModel, id, vals)
}

func termComplete(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireManager(ctx, uid) {
		return "", fmt.Errorf("only an HR Manager can complete a termination")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, terminationModel, id)
	if err != nil {
		return "", err
	}
	if TerminationState(orm.AsString(rec["state"])) != TermApproved {
		return "", fmt.Errorf("only an approved termination can be completed")
	}
	if openRequiredLines(bypass, id) {
		return "", fmt.Errorf("complete all required offboarding tasks before finishing")
	}
	empID := intOr(rec["employee_id"])
	lwd := orm.AsString(rec["last_working_day"])
	if empID > 0 {
		newState := "terminated"
		if TerminationType(orm.AsString(rec["type"])) == TermRetired {
			newState = "retired"
		}
		empVals := map[string]interface{}{"state": newState, "termination_date": lwd}
		if r := termReason(rec); r != "" {
			empVals["termination_reason"] = r
		}
		_ = orm.UpdateRecordByID(bypass, employeeMod, empID, empVals)
	}
	return "", orm.UpdateRecordByID(bypass, terminationModel, id, map[string]interface{}{"state": string(TermCompleted)})
}

// ---------------------------------------------------------------------------
// Offboarding checklist lines
// ---------------------------------------------------------------------------

var offboardingTemplate = []struct {
	name string
	cat  OffboardCategory
	seq  int
}{
	{"Revoke system / email access", ObIT, 10},
	{"Return laptops, badge & equipment", ObEquipment, 20},
	{"Final settlement & outstanding dues", ObFinance, 30},
	{"Knowledge transfer / handover", ObManager, 40},
	{"Exit interview", ObEmployee, 50},
	{"Remove from directories & benefits", ObAdmin, 60},
}

func provisionOffboarding(ctx context.Context, terminationID int) {
	existing, _ := orm.SearchOne(ctx, offLineModel, map[string]interface{}{"termination_id": terminationID})
	if existing != nil {
		return
	}
	for _, t := range offboardingTemplate {
		_, _ = createRecord(ctx, offLineModel, map[string]interface{}{
			"termination_id": terminationID,
			"name":           t.name,
			"category":       string(t.cat),
			"required":       true,
			"done":           false,
			"sequence":       t.seq,
		})
	}
}

func obLineDone(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("only an HR Officer or Manager can tick off an offboarding task")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	return "", orm.UpdateRecordByID(bypass, offLineModel, id, map[string]interface{}{
		"done":      true,
		"done_date": time.Now().UTC().Format(datetimeFull),
	})
}

func obLineUndo(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("only an HR Officer or Manager can reopen an offboarding task")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	return "", orm.UpdateRecordByID(bypass, offLineModel, id, map[string]interface{}{
		"done":      false,
		"done_date": "",
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func requireOfficer(ctx context.Context, uid int) bool {
	return orm.UserHasGroupXML(ctx, uid, "hr.group_hr_user") ||
		orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager")
}

func requireManager(ctx context.Context, uid int) bool {
	return orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager")
}

func getRecord(ctx context.Context, model string, id int) (map[string]interface{}, error) {
	rec, err := orm.SearchOne(ctx, model, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return nil, fmt.Errorf("record not found")
	}
	return rec, nil
}

// openRequiredLines reports whether any required offboarding task is still open.
func openRequiredLines(ctx context.Context, terminationID int) bool {
	lines, err := orm.Search(ctx, offLineModel, [][]interface{}{
		{"termination_id", "=", terminationID},
		{"required", "=", true},
		{"done", "=", false},
	})
	if err != nil {
		return false
	}
	return len(lines) > 0
}

// termReason maps the termination type onto the anchor's reason selection.
func termReason(rec map[string]interface{}) string {
	switch TerminationType(orm.AsString(rec["type"])) {
	case TermResignation:
		return "resignation"
	case TermTermination:
		return "fired"
	case TermRetired:
		return "retired"
	case TermContractEnd:
		return "contract_end"
	default:
		return "other"
	}
}

func writeGrievanceHistory(ctx context.Context, id int, action CaseHistoryAction, field, oldVal, newVal string, byUID int) {
	vals := map[string]interface{}{
		"grievance_id": id,
		"action":       string(action),
		"field":        field,
		"old_value":    oldVal,
		"new_value":    newVal,
		"performed_at": time.Now().UTC().Format(datetimeFull),
	}
	if byUID > 0 {
		vals["performed_by"] = byUID
	}
	_, _ = createRecord(ctx, grievanceHist, vals)
}

func writeDiscHistory(ctx context.Context, id int, action CaseHistoryAction, field, oldVal, newVal string, byUID int) {
	vals := map[string]interface{}{
		"case_id":      id,
		"action":       string(action),
		"field":        field,
		"old_value":    oldVal,
		"new_value":    newVal,
		"performed_at": time.Now().UTC().Format(datetimeFull),
	}
	if byUID > 0 {
		vals["performed_by"] = byUID
	}
	_, _ = createRecord(ctx, discHist, vals)
}

func createRecord(ctx context.Context, model string, vals map[string]interface{}) (int, error) {
	inst, ok := orm.Registry[model]
	if !ok {
		return 0, fmt.Errorf("model %s is not registered", model)
	}
	return orm.Create(ctx, inst, vals)
}
