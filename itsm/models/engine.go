package models

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"nexa/core/orm"
)

// ITSM engine: incident major-declaration + timeline, problem↔incident
// linking, change state machine, release stage advancement.

const (
	incModel  = "itsm.incident"
	tlModel   = "itsm.timeline"
	probModel = "itsm.problem"
	chgModel  = "itsm.change"
	relModel  = "itsm.release"
)

func init() {
	orm.RegisterObjectAction(incModel, "action_inc_major", incMajor)
	orm.RegisterObjectAction(incModel, "action_inc_resolve", incResolve)
	orm.RegisterObjectAction(probModel, "action_prob_link_incident", probLinkIncident)
	orm.RegisterObjectAction(chgModel, "action_chg_advance", chgAdvance)
	orm.RegisterObjectAction(relModel, "action_rel_advance", relAdvance)
}

func fnum(v interface{}) float64 {
	n, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(v)), 64)
	return n
}

func oid(v interface{}) int {
	if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
		n, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprint(arr[0])))
		return n
	}
	n, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprint(v)))
	return n
}

func loadRec(ctx context.Context, model string, id int) map[string]interface{} {
	rows, err := orm.SearchPage(ctx, model, [][]interface{}{{"id", "=", id}}, 1, 0, "id asc")
	if err != nil || len(rows) == 0 {
		return nil
	}
	return rows[0]
}

func addTimeline(ctx context.Context, incID int, kind, note string, userID string) {
	inst, ok := orm.Registry[tlModel]
	if !ok {
		return
	}
	_, _ = orm.Create(ctx, inst, map[string]interface{}{
		"incident_id": incID,
		"at":          time.Now().UTC().Format(time.RFC3339),
		"kind":        kind,
		"note":        note,
		"user_id":     userID,
	})
}

// incMajor declares a major incident: sets flag, priority=critical, assigns
// commander, logs a timeline event.
func incMajor(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	inc := loadRec(bypass, incModel, id)
	if inc == nil {
		return "", fmt.Errorf("incident not found")
	}
	cmd := args["commander_id"]
	if cmd == "" {
		cmd = args["user_id"]
	}
	_ = orm.UpdateRecordByID(bypass, incModel, id, map[string]interface{}{
		"major":        true,
		"priority":     "critical",
		"state":        "in_progress",
		"commander_id": cmd,
	})
	addTimeline(bypass, id, "major_declared", "Major incident declared — commander assigned", args["user_id"])
	return "Major incident declared — commander assigned, priority critical", nil
}

// incResolve closes the incident with resolution + optional root cause.
func incResolve(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	inc := loadRec(bypass, incModel, id)
	if inc == nil {
		return "", fmt.Errorf("incident not found")
	}
	if st := orm.AsString(inc["state"]); st == "closed" || st == "resolved" {
		return "", fmt.Errorf("incident already %s", st)
	}
	upd := map[string]interface{}{
		"state":       "resolved",
		"resolved_at": time.Now().UTC().Format(time.RFC3339),
	}
	if rc := strings.TrimSpace(args["root_cause"]); rc != "" {
		upd["root_cause"] = rc
	}
	if w := strings.TrimSpace(args["workaround"]); w != "" {
		upd["workaround"] = w
	}
	_ = orm.UpdateRecordByID(bypass, incModel, id, upd)
	addTimeline(bypass, id, "resolved", "Incident resolved", args["user_id"])
	return "Incident resolved", nil
}

// probLinkIncident attaches an incident to a problem (args: incident_id).
func probLinkIncident(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	prob := loadRec(bypass, probModel, id)
	if prob == nil {
		return "", fmt.Errorf("problem not found")
	}
	incID := oid(args["incident_id"])
	if incID <= 0 {
		return "", fmt.Errorf("incident_id required")
	}
	_ = orm.UpdateRecordByID(bypass, incModel, incID, map[string]interface{}{"problem_id": id})
	_ = orm.UpdateRecordByID(bypass, probModel, id, map[string]interface{}{"state": "investigating"})
	addTimeline(bypass, incID, "linked_problem", "Linked to problem "+orm.AsString(prob["name"]), args["user_id"])
	return "Incident linked to problem", nil
}

// chgAdvance moves a change through its state machine:
// draft→assessment→approval→scheduled→implementing→review→closed.
func chgAdvance(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	c := loadRec(bypass, chgModel, id)
	if c == nil {
		return "", fmt.Errorf("change not found")
	}
	flow := map[string]string{
		"draft":        "assessment",
		"assessment":   "approval",
		"approval":     "scheduled",
		"scheduled":    "implementing",
		"implementing": "review",
		"review":       "closed",
	}
	cur := orm.AsString(c["state"])
	next, ok := flow[cur]
	if !ok {
		return "", fmt.Errorf("cannot advance from state %s", cur)
	}
	upd := map[string]interface{}{"state": next}
	if next == "approval" {
		upd["requested_by_id"] = args["user_id"]
	}
	if next == "closed" {
		upd["approved_by_id"] = args["user_id"]
	}
	_ = orm.UpdateRecordByID(bypass, chgModel, id, upd)
	return fmt.Sprintf("Change advanced %s → %s", cur, next), nil
}

// relAdvance moves a release through planning→build→test→approval→deploying→done.
func relAdvance(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	r := loadRec(bypass, relModel, id)
	if r == nil {
		return "", fmt.Errorf("release not found")
	}
	flow := map[string]string{
		"planning":  "build",
		"build":     "test",
		"test":      "approval",
		"approval":  "deploying",
		"deploying": "done",
	}
	cur := orm.AsString(r["stage"])
	next, ok := flow[cur]
	if !ok {
		return "", fmt.Errorf("cannot advance from stage %s", cur)
	}
	upd := map[string]interface{}{"stage": next}
	if next == "done" {
		upd["deployed_at"] = time.Now().UTC().Format(time.RFC3339)
	}
	_ = orm.UpdateRecordByID(bypass, relModel, id, upd)
	return fmt.Sprintf("Release advanced %s → %s", cur, next), nil
}
