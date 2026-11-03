package models

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"nexa/core/orm"
)

// ObjectActions expose the automation rule lifecycle to the UI: publish (an
// explicit versioned snapshot), test-run (a dry-run against a sample ticket that
// never mutates), enable and emergency disable.

const ruleModel = "helpdesk.automation.rule"

type ruleSnapshot struct {
	Name       string         `json:"name"`
	Trigger    string         `json:"trigger"`
	MatchLogic string         `json:"match_logic"`
	DryRun     bool           `json:"dry_run"`
	ApplyOnce  bool           `json:"apply_once"`
	Conditions []snapshotLine `json:"conditions"`
	Actions    []snapshotLine `json:"actions"`
}

type snapshotLine struct {
	S  string `json:"s"`
	K  string `json:"k"`
	Op string `json:"op,omitempty"`
	V  string `json:"v,omitempty"`
}

func init() {
	orm.RegisterObjectAction(ruleModel, "action_publish", rulePublish)
	orm.RegisterObjectAction(ruleModel, "action_test_run", ruleTestRun)
	orm.RegisterObjectAction(ruleModel, "action_enable", ruleSetActive(true))
	orm.RegisterObjectAction(ruleModel, "action_disable", ruleSetActive(false))
}

// rulePublish takes a new immutable version of the rule and marks it as current,
// satisfying the "version and publish rules explicitly" safeguard.
func rulePublish(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rule, err := orm.SearchOne(bypass, ruleModel, map[string]interface{}{"id": id})
	if err != nil || rule == nil {
		return "", fmt.Errorf("rule not found")
	}
	snap := buildSnapshot(bypass, rule)
	data, _ := json.Marshal(snap)
	newVersion := nextVersionNumber(atoi(str(rule["current_version"])))

	vm := orm.Registry["helpdesk.automation.version"]
	if vm == nil {
		return "", fmt.Errorf("version model not registered")
	}
	uid := orm.SecurityUID(ctx)
	if _, err := orm.Create(bypass, vm, map[string]interface{}{
		"rule_id":        id,
		"version_number": newVersion,
		"snapshot":       string(data),
		"change_note":    fmt.Sprintf("Published v%d", newVersion),
		"published_by":   uid,
		"published_at":   nowStamp(),
	}); err != nil {
		return "", fmt.Errorf("create version: %w", err)
	}
	if err := orm.UpdateRecordByID(bypass, ruleModel, id, map[string]interface{}{"current_version": newVersion}); err != nil {
		return "", fmt.Errorf("set current version: %w", err)
	}
	return fmt.Sprintf("Published %s as v%d.", str(rule["name"]), newVersion), nil
}

// ruleTestRun evaluates the rule against a sample ticket without applying actions.
func ruleTestRun(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rule, err := orm.SearchOne(bypass, ruleModel, map[string]interface{}{"id": id})
	if err != nil || rule == nil {
		return "", fmt.Errorf("rule not found")
	}
	ticketID := atoi(args["ticket_id"])
	if ticketID <= 0 {
		return "", fmt.Errorf("test run requires a ticket_id argument")
	}
	ticket, err := orm.SearchOne(bypass, ticketModel, map[string]interface{}{"id": ticketID})
	if err != nil || ticket == nil {
		return "", fmt.Errorf("sample ticket not found")
	}
	tags := fetchTicketTags(bypass, ticketID)
	conds := loadChildren(bypass, "helpdesk.automation.condition", id)
	results := make([]bool, 0, len(conds))
	for _, c := range conds {
		actual := fieldValue(ticket, tags, ConditionField(str(c["field"])))
		results = append(results, matchesOperator(actual, ConditionOperator(str(c["operator"])), str(c["value"])))
	}
	matched := combineResults(results, MatchLogic(strOr(rule["match_logic"], "all")))
	actions := loadChildren(bypass, "helpdesk.automation.action", id)
	sort.SliceStable(actions, func(i, j int) bool { return atoi(str(actions[i]["sequence"])) < atoi(str(actions[j]["sequence"])) })

	status := RunSkipped
	if matched {
		status = RunDryRun
	}
	rm := orm.Registry["helpdesk.automation.run"]
	if rm != nil {
		_, _ = orm.Create(bypass, rm, map[string]interface{}{
			"rule_id":         id,
			"ticket_id":       ticketID,
			"trigger_type":    str(rule["trigger_type"]),
			"status":          status,
			"matched":         matched,
			"dry_run":         true,
			"actions_planned": len(actions),
			"detail":          describeActions(actions),
			"started_at":      nowStamp(),
			"finished_at":     nowStamp(),
		})
	}
	if !matched {
		return "Test run: conditions did NOT match. No actions would run.", nil
	}
	if len(actions) == 0 {
		return "Test run: matched, but the rule has no actions.", nil
	}
	return fmt.Sprintf("Test run: matched. %d action(s) would apply -> %s", len(actions), describeActions(actions)), nil
}

func ruleSetActive(active bool) orm.ObjectActionFunc {
	return func(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
		bypass := orm.ContextWithBypass(ctx, true)
		if err := orm.UpdateRecordByID(bypass, ruleModel, id, map[string]interface{}{"active": active}); err != nil {
			return "", err
		}
		if active {
			return "Rule enabled.", nil
		}
		return "Rule disabled (emergency stop).", nil
	}
}

func buildSnapshot(ctx context.Context, rule map[string]interface{}) ruleSnapshot {
	conds := loadChildren(ctx, "helpdesk.automation.condition", atoi(str(rule["id"])))
	actions := loadChildren(ctx, "helpdesk.automation.action", atoi(str(rule["id"])))
	snap := ruleSnapshot{
		Name:       str(rule["name"]),
		Trigger:    str(rule["trigger_type"]),
		MatchLogic: strOr(rule["match_logic"], "all"),
		DryRun:     orm.AsBool(rule["dry_run"]),
		ApplyOnce:  orm.AsBool(rule["apply_once"]),
	}
	for _, c := range conds {
		snap.Conditions = append(snap.Conditions, snapshotLine{
			S: str(c["sequence"]), K: str(c["field"]), Op: str(c["operator"]), V: str(c["value"]),
		})
	}
	for _, a := range actions {
		snap.Actions = append(snap.Actions, snapshotLine{
			S: str(a["sequence"]), K: str(a["action_type"]), V: str(a["value"]),
		})
	}
	return snap
}
