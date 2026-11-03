package models

import (
	"context"
	"fmt"
	"time"

	"nexa/core/orm"
)

// Recruitment-plus workflow actions. All are maker-checker guarded: requisition
// approval, offer approval and probation confirmation require an HR Manager and,
// for approvals, forbid the record creator from approving their own request.
// Onboarding instantiation is idempotent (activities are matched by name), and
// offer approval renders the letter from the chosen template.

func init() {
	orm.RegisterObjectAction(openingModel, "action_req_approve", openApprove)
	orm.RegisterObjectAction(openingModel, "action_req_refuse", openRefuse)

	orm.RegisterObjectAction(offerModel, "action_approve_offer", offerApprove)
	orm.RegisterObjectAction(offerModel, "action_send_offer", offerSend)

	orm.RegisterObjectAction(planModel, "action_instantiate", planInstantiate)
	orm.RegisterObjectAction(planModel, "action_start_plan", planStart)
	orm.RegisterObjectAction(planModel, "action_complete_plan", planComplete)

	orm.RegisterObjectAction(employeeModel, "action_confirm_probation", probationConfirm)
	orm.RegisterObjectAction(employeeModel, "action_extend_probation", probationExtend)
	orm.RegisterObjectAction(employeeModel, "action_start_probation", probationStart)
}

// ---------------------------------------------------------------------------
// Requisition approval
// ---------------------------------------------------------------------------

func openApprove(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can approve a requisition")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, openingModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("job opening not found")
	}
	if orm.AsBool(rec["req_approved"]) {
		return "", fmt.Errorf("this requisition is already approved")
	}
	if creator := intOr(rec["create_uid"]); creator != 0 && creator == uid {
		return "", fmt.Errorf("maker-checker: the requisition creator cannot approve their own request")
	}
	if intOr(rec["headcount"]) <= 0 {
		return "", fmt.Errorf("set a positive headcount before approving the requisition")
	}
	if floatOr(rec["budget_annual"]) <= 0 {
		return "", fmt.Errorf("set an annual budget before approving the requisition")
	}
	return "", orm.UpdateRecordByID(bypass, openingModel, id, map[string]interface{}{
		"req_approved":  true,
		"approver_id":   uid,
		"approved_date": time.Now().UTC().Format(dateLayout),
	})
}

func openRefuse(ctx context.Context, _ string, id int, ctxArgs map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can refuse a requisition")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, openingModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("job opening not found")
	}
	reason := ctxArgs["reason"]
	return "", orm.UpdateRecordByID(bypass, openingModel, id, map[string]interface{}{
		"req_approved":     false,
		"approver_id":      uid,
		"rejection_reason": reason,
	})
}

// ---------------------------------------------------------------------------
// Offer approval + templated letter
// ---------------------------------------------------------------------------

func offerApprove(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can approve an offer")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	offer, err := orm.SearchOne(bypass, offerModel, map[string]interface{}{"id": id})
	if err != nil || offer == nil {
		return "", fmt.Errorf("offer not found")
	}
	if creator := intOr(offer["create_uid"]); creator != 0 && creator == uid {
		return "", fmt.Errorf("maker-checker: the offer creator cannot approve their own offer")
	}
	vals := map[string]interface{}{
		"approved":     true,
		"approver_id":  uid,
		"approve_date": time.Now().UTC().Format(dateLayout),
	}
	if tmplID := intOr(offer["template_id"]); tmplID > 0 {
		tmpl, err := orm.SearchOne(bypass, "hr.recruitment.offer.template", map[string]interface{}{"id": tmplID})
		if err == nil && tmpl != nil {
			vars := offerVars(bypass, offer)
			body := renderTemplate(orm.AsString(tmpl["body"]), vars)
			vals["letter"] = orm.AsString(tmpl["subject"]) + "\n\n" + body
		}
	}
	return "", orm.UpdateRecordByID(bypass, offerModel, id, vals)
}

func offerSend(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	offer, err := orm.SearchOne(bypass, offerModel, map[string]interface{}{"id": id})
	if err != nil || offer == nil {
		return "", fmt.Errorf("offer not found")
	}
	if !orm.AsBool(offer["approved"]) {
		return "", fmt.Errorf("the offer must be approved before it is sent")
	}
	if orm.AsString(offer["state"]) != "draft" {
		return "", fmt.Errorf("only a draft offer can be sent")
	}
	return "", orm.UpdateRecordByID(bypass, offerModel, id, map[string]interface{}{"state": "sent"})
}

func offerVars(ctx context.Context, offer map[string]interface{}) map[string]string {
	vars := map[string]string{
		"salary":     fmt.Sprintf("%.2f", floatOr(offer["salary"])),
		"start_date": dateStr(offer["expected_start"]),
		"offer_date": dateStr(offer["offer_date"]),
		"applicant":  "",
		"job":        "",
	}
	if appID := intOr(offer["applicant_id"]); appID > 0 {
		if app, err := orm.SearchOne(ctx, applicantModel, map[string]interface{}{"id": appID}); err == nil && app != nil {
			vars["applicant"] = orm.AsString(app["name"])
			vars["email"] = orm.AsString(app["email"])
		}
	}
	if openID := intOr(offer["opening_id"]); openID > 0 {
		if op, err := orm.SearchOne(ctx, openingModel, map[string]interface{}{"id": openID}); err == nil && op != nil {
			vars["job"] = orm.AsString(op["name"])
		}
	}
	return vars
}

// ---------------------------------------------------------------------------
// Onboarding plan provisioning
// ---------------------------------------------------------------------------

func planInstantiate(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	plan, err := orm.SearchOne(bypass, planModel, map[string]interface{}{"id": id})
	if err != nil || plan == nil {
		return "", fmt.Errorf("onboarding plan not found")
	}
	tmplID := intOr(plan["template_id"])
	if tmplID <= 0 {
		return "", fmt.Errorf("choose a checklist template on the plan first")
	}
	start := dateStr(plan["start_date"])
	if start == "" {
		start = time.Now().UTC().Format(dateLayout)
	}
	lines, err := orm.Search(bypass, "hr.onboarding.template.line", [][]interface{}{
		{"template_id", "=", tmplID},
		{"active", "=", true},
	})
	if err != nil {
		return "", err
	}
	added := 0
	for _, ln := range lines {
		name := orm.AsString(ln["name"])
		existing, err := orm.SearchCount(bypass, activityModel, [][]interface{}{
			{"plan_id", "=", id},
			{"name", "=", name},
		})
		if err == nil && existing > 0 {
			continue
		}
		offset := intOr(ln["due_offset_days"])
		if _, err := createRecord(bypass, activityModel, map[string]interface{}{
			"plan_id":     id,
			"name":        name,
			"responsible": orm.AsString(ln["responsible"]),
			"due_date":    addDays(start, offset),
			"sequence":    intOr(ln["sequence"]),
			"state":       "pending",
		}); err != nil {
			return "", err
		}
		added++
	}
	if !orm.AsBool(plan["provisioned"]) {
		_ = orm.UpdateRecordByID(bypass, planModel, id, map[string]interface{}{"provisioned": true})
	}
	return fmt.Sprintf("provisioned %d new activity(ies)", added), nil
}

func planStart(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	plan, err := orm.SearchOne(bypass, planModel, map[string]interface{}{"id": id})
	if err != nil || plan == nil {
		return "", fmt.Errorf("onboarding plan not found")
	}
	if orm.AsString(plan["state"]) != "draft" {
		return "", fmt.Errorf("only a draft plan can be started")
	}
	return "", orm.UpdateRecordByID(bypass, planModel, id, map[string]interface{}{"state": "in_progress"})
}

func planComplete(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	plan, err := orm.SearchOne(bypass, planModel, map[string]interface{}{"id": id})
	if err != nil || plan == nil {
		return "", fmt.Errorf("onboarding plan not found")
	}
	if orm.AsString(plan["state"]) != "in_progress" {
		return "", fmt.Errorf("only an in-progress plan can be completed")
	}
	openCount, err := orm.SearchCount(bypass, activityModel, [][]interface{}{
		{"plan_id", "=", id},
		{"state", "=", "pending"},
	})
	if err != nil {
		return "", err
	}
	if openCount > 0 {
		return "", fmt.Errorf("%d activity(ies) still pending; complete or skip them first", openCount)
	}
	return "", orm.UpdateRecordByID(bypass, planModel, id, map[string]interface{}{"state": "done"})
}

// ---------------------------------------------------------------------------
// Probation + confirmation
// ---------------------------------------------------------------------------

func probationStart(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") && !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_user") {
		return "", fmt.Errorf("insufficient rights to start a probation period")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	emp, err := orm.SearchOne(bypass, employeeModel, map[string]interface{}{"id": id})
	if err != nil || emp == nil {
		return "", fmt.Errorf("employee not found")
	}
	if orm.AsString(emp["confirmation_state"]) == string(ConfInProgress) {
		return "", fmt.Errorf("probation is already in progress")
	}
	today := time.Now().UTC().Format(dateLayout)
	vals := map[string]interface{}{
		"confirmation_state": string(ConfInProgress),
		"probation_start":    today,
	}
	if dateStr(emp["probation_end"]) == "" {
		vals["probation_end"] = addDays(today, 90)
	}
	return "", orm.UpdateRecordByID(bypass, employeeModel, id, vals)
}

func probationConfirm(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can confirm probation")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	emp, err := orm.SearchOne(bypass, employeeModel, map[string]interface{}{"id": id})
	if err != nil || emp == nil {
		return "", fmt.Errorf("employee not found")
	}
	switch ConfirmationState(orm.AsString(emp["confirmation_state"])) {
	case ConfInProgress, ConfExtended:
	default:
		return "", fmt.Errorf("only an in-progress or extended probation can be confirmed")
	}
	return "", orm.UpdateRecordByID(bypass, employeeModel, id, map[string]interface{}{
		"confirmation_state": string(ConfConfirmed),
		"confirmed_date":     time.Now().UTC().Format(dateLayout),
	})
}

func probationExtend(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can extend probation")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	emp, err := orm.SearchOne(bypass, employeeModel, map[string]interface{}{"id": id})
	if err != nil || emp == nil {
		return "", fmt.Errorf("employee not found")
	}
	if ConfirmationState(orm.AsString(emp["confirmation_state"])) == ConfConfirmed {
		return "", fmt.Errorf("a confirmed employee's probation cannot be extended")
	}
	end := dateStr(emp["probation_end"])
	if end == "" {
		end = time.Now().UTC().Format(dateLayout)
	}
	return "", orm.UpdateRecordByID(bypass, employeeModel, id, map[string]interface{}{
		"confirmation_state": string(ConfExtended),
		"probation_end":      addDays(end, 30),
	})
}
