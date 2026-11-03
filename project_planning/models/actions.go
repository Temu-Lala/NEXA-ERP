package models

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"nexa/core/orm"
)

// Planning actions (blueprint §6.4, §13, §22). Two guarantees are enforced
// server-side: dependencies are only created when they keep the schedule
// acyclic, and a deliverable follows a submit → accept/reject review in which the
// approver must be someone other than the owner (separation of duties). Deliver-
// ables that feed a milestone are surfaced but not auto-invoiced; that stays a
// Finance concern.

func init() {
	orm.RegisterObjectAction(depModel, "action_validate", depValidate)
	orm.RegisterObjectAction(depModel, "action_create", depCreate)

	orm.RegisterObjectAction(delivModel, "action_submit", delivSubmit)
	orm.RegisterObjectAction(delivModel, "action_accept", delivAccept)
	orm.RegisterObjectAction(delivModel, "action_reject", delivReject)
}

// depValidate re-checks an already stored edge against the acyclicity rules, so a
// graph edited outside this flow can still be audited.
func depValidate(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, depModel, id)
	if rec == nil {
		return "", fmt.Errorf("dependency not found")
	}
	if err := validateDependency(bypass, idOf(rec["task_id"]), idOf(rec["predecessor_id"]), s(rec["dependency_type"])); err != nil {
		return "", err
	}
	return "Dependency is valid and keeps the schedule acyclic.", nil
}

// depCreate builds a validated dependency from the request values, refusing
// self-links and cycles and defaulting the type to finish-to-start.
func depCreate(ctx context.Context, _ string, _ int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	taskID := atoiDefault(vals["task_id"], 0)
	predID := atoiDefault(vals["predecessor_id"], 0)
	depType := strings.TrimSpace(vals["dependency_type"])
	if depType == "" {
		depType = "finish_to_start"
	}
	if err := validateDependency(bypass, taskID, predID, depType); err != nil {
		return "", err
	}
	inst, ok := orm.Registry[depModel]
	if !ok {
		return "", fmt.Errorf("dependency model unavailable")
	}
	projID := 0
	if t := load(bypass, taskModel, taskID); t != nil {
		projID = idOf(t["project_id"])
	}
	lag := atoiDefault(vals["lag_days"], 0)
	newID, err := orm.Create(bypass, inst, map[string]interface{}{
		"project_id":      projID,
		"task_id":         taskID,
		"predecessor_id":  predID,
		"dependency_type": depType,
		"lag_days":        lag,
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Dependency created (id %d).", newID), nil
}

// ---------------------------------------------------------------------------
// Deliverable review lifecycle
// ---------------------------------------------------------------------------

func delivSubmit(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, delivModel, id)
	if rec == nil {
		return "", fmt.Errorf("deliverable not found")
	}
	if s(rec["state"]) != "draft" {
		return "", fmt.Errorf("only draft deliverables can be submitted for review")
	}
	if err := orm.UpdateRecordByID(bypass, delivModel, id, map[string]interface{}{"state": "submitted"}); err != nil {
		return "", err
	}
	return "Deliverable submitted for review.", nil
}

// delivAccept records formal acceptance, enforcing that the reviewer is not the
// deliverable's owner so acceptance is an independent act.
func delivAccept(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, delivModel, id)
	if rec == nil {
		return "", fmt.Errorf("deliverable not found")
	}
	if s(rec["state"]) != "submitted" {
		return "", fmt.Errorf("only submitted deliverables can be accepted")
	}
	uid := orm.UIDFromContext(ctx)
	if uid > 0 && idOf(rec["owner_id"]) == uid {
		return "", fmt.Errorf("a deliverable must be accepted by someone other than its owner")
	}
	upd := map[string]interface{}{"state": "accepted", "accepted_date": time.Now().Format("2006-01-02")}
	if uid > 0 {
		upd["note_accept_by"] = uid
	}
	if err := orm.UpdateRecordByID(bypass, delivModel, id, upd); err != nil {
		return "", err
	}
	return "Deliverable accepted.", nil
}

func delivReject(ctx context.Context, _ string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, delivModel, id)
	if rec == nil {
		return "", fmt.Errorf("deliverable not found")
	}
	if s(rec["state"]) != "submitted" {
		return "", fmt.Errorf("only submitted deliverables can be rejected")
	}
	upd := map[string]interface{}{"state": "rejected"}
	if r := strings.TrimSpace(vals["reason"]); r != "" {
		upd["description"] = appendNote(s(rec["description"]), "Rejected: "+r)
	}
	if err := orm.UpdateRecordByID(bypass, delivModel, id, upd); err != nil {
		return "", err
	}
	return "Deliverable rejected; returned for revision.", nil
}

// ---------------------------------------------------------------------------
// Small helpers
// ---------------------------------------------------------------------------

func atoiDefault(strv string, def int) int {
	if strings.TrimSpace(strv) == "" {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(strv))
	if err != nil {
		return def
	}
	return n
}

func appendNote(existing, add string) string {
	if strings.TrimSpace(existing) == "" {
		return add
	}
	return existing + "\n" + add
}
