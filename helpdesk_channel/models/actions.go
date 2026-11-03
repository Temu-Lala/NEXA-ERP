package models

import (
	"context"
	"fmt"
	"strings"

	"nexa/core/orm"
)

// Operator-facing actions for the channel companion: connection activation,
// saved-view default/apply, agent availability, routing-rule dry-run, and manual
// application of routing to a single ticket. Actions enforce their own guards and
// write append-only history where they change a ticket, mirroring the anchor's
// workflow.go action style.

func init() {
	orm.RegisterObjectAction("helpdesk.channel.connection", "action_enable", connectionEnable)
	orm.RegisterObjectAction("helpdesk.channel.connection", "action_disable", connectionDisable)

	orm.RegisterObjectAction("helpdesk.saved.view", "action_set_default", savedViewSetDefault)
	orm.RegisterObjectAction("helpdesk.saved.view", "action_apply", savedViewApply)

	orm.RegisterObjectAction("helpdesk.agent.load", "action_toggle_available", agentLoadToggle)

	orm.RegisterObjectAction("helpdesk.routing.rule", "action_test_run", routingRuleTestRun)

	orm.RegisterObjectAction("helpdesk.ticket", "action_route_now", ticketRouteNow)
}

// ---------------------------------------------------------------------------
// Connection activation
// ---------------------------------------------------------------------------

func connectionEnable(ctx context.Context, model string, id int, _ map[string]string) (string, error) {
	if err := orm.UpdateRecordByID(ctx, model, id, map[string]interface{}{"active": true}); err != nil {
		return "", err
	}
	return "", nil
}

func connectionDisable(ctx context.Context, model string, id int, _ map[string]string) (string, error) {
	if err := orm.UpdateRecordByID(ctx, model, id, map[string]interface{}{"active": false}); err != nil {
		return "", err
	}
	return "", nil
}

// ---------------------------------------------------------------------------
// Saved views
// ---------------------------------------------------------------------------

// savedViewSetDefault marks this view as the owner's default for its model,
// clearing any previous default so exactly one is active per (owner, model).
func savedViewSetDefault(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, "helpdesk.saved.view", map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("saved view not found")
	}
	uid := str(rec["user_id"])
	target := str(rec["target_model"])
	if target == "" {
		target = "helpdesk.ticket"
	}
	domain := [][]interface{}{{"target_model", "=", target}, {"is_default", "=", true}}
	if uid != "" {
		domain = append(domain, []interface{}{"user_id", "=", atoi(uid)})
	}
	others := fetchAll(bypass, "helpdesk.saved.view", domain, "id ASC")
	for _, o := range others {
		if str(o["id"]) == fmt.Sprintf("%d", id) {
			continue
		}
		_ = orm.UpdateRecordByID(bypass, "helpdesk.saved.view", atoi(str(o["id"])),
			map[string]interface{}{"is_default": false})
	}
	if err := orm.UpdateRecordByID(bypass, "helpdesk.saved.view", id,
		map[string]interface{}{"is_default": true}); err != nil {
		return "", err
	}
	return "View set as default.", nil
}

// savedViewApply records a use of the view (increments the stored usage counter)
// and returns the filter so the client can apply it.
func savedViewApply(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, "helpdesk.saved.view", map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("saved view not found")
	}
	if orm.AsString(rec["active"]) == "f" {
		return "", fmt.Errorf("cannot apply an inactive view")
	}
	_ = orm.UpdateRecordByID(bypass, "helpdesk.saved.view", id, map[string]interface{}{
		"usage_count": atoi(str(rec["usage_count"])) + 1,
	})
	return "", nil
}

// ---------------------------------------------------------------------------
// Agent availability
// ---------------------------------------------------------------------------

func agentLoadToggle(ctx context.Context, model string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, model, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("agent load record not found")
	}
	nowAvailable := orm.AsBool(rec["available"])
	if err := orm.UpdateRecordByID(bypass, model, id, map[string]interface{}{
		"available": !nowAvailable,
	}); err != nil {
		return "", err
	}
	if nowAvailable {
		return "Agent marked unavailable.", nil
	}
	return "Agent marked available.", nil
}

// ---------------------------------------------------------------------------
// Routing rule dry-run + manual routing
// ---------------------------------------------------------------------------

// routingRuleTestRun counts currently-open unassigned tickets that would match
// this rule without changing any record (a dry run the blueprint calls for).
func routingRuleTestRun(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rule, err := orm.SearchOne(bypass, "helpdesk.routing.rule", map[string]interface{}{"id": id})
	if err != nil || rule == nil {
		return "", fmt.Errorf("routing rule not found")
	}
	rs := toRuleStrings([]map[string]interface{}{rule})[0]
	matched := 0
	for _, tk := range openUnassignedTickets(bypass) {
		if ruleMatches(toTicketStrings(tk), rs) {
			matched++
		}
	}
	return fmt.Sprintf("Dry run: %d open unassigned ticket(s) match %q.", matched, str(rule["name"])), nil
}

// ticketRouteNow applies the first matching routing rule to a single ticket
// immediately, so an agent can route one record without waiting for the cron.
func ticketRouteNow(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	tk, err := orm.SearchOne(bypass, "helpdesk.ticket", map[string]interface{}{"id": id})
	if err != nil || tk == nil {
		return "", fmt.Errorf("ticket not found")
	}
	if !isUnassigned(str(tk["assigned_to"]), str(tk["team_id"])) {
		return "", fmt.Errorf("ticket is already assigned")
	}
	rules := fetchAll(bypass, "helpdesk.routing.rule",
		[][]interface{}{{"active", "=", true}}, "sequence ASC, id ASC")
	if len(rules) == 0 {
		return "", fmt.Errorf("no active routing rules configured")
	}
	stringRules := toRuleStrings(rules)
	ts := toTicketStrings(tk)
	matched := firstMatchingRule(ts, stringRules)
	if matched == nil {
		return "", fmt.Errorf("no routing rule matches this ticket")
	}
	loads := toLoadStrings(loadCandidates(bypass, matched["team_id"]), bypass)
	dec := decideRouting(ts, matched, loads)
	if !dec.ShouldMove {
		return "", fmt.Errorf("matched rule %q specifies no destination", matched["name"])
	}
	applyRouting(bypass, id, matched, dec)
	if dec.UserID != "" {
		return fmt.Sprintf("Routed to agent #%s by rule %q.", dec.UserID, matched["name"]), nil
	}
	return fmt.Sprintf("Routed by rule %q.", matched["name"]), nil
}

func openUnassignedTickets(ctx context.Context) []map[string]interface{} {
	tickets := fetchAll(ctx, "helpdesk.ticket",
		[][]interface{}{{"state", "in", strings.Split(openStateSet, ",")}}, "id ASC")
	out := make([]map[string]interface{}, 0, len(tickets))
	for _, t := range tickets {
		if isUnassigned(str(t["assigned_to"]), str(t["team_id"])) {
			out = append(out, t)
		}
	}
	return out
}
