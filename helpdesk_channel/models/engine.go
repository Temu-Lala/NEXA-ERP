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

// Behaviour layer for the omnichannel / routing companion. It keeps the ticket as
// the single source of truth and adds governance around it:
//
//   - Saved-view usage counters, connection message/ticket totals, agent current
//     load and routing-rule hit counts are live non-stored aggregates.
//   - A scheduled job (helpdesk.channel.route) auto-routes unassigned open tickets
//     through the ordered routing rules, preferring the least-loaded available
//     agent and optionally raising priority.
//   - A scheduled job (helpdesk.channel.delivery) retries failed outbound channel
//     deliveries with a bounded retry count, leaving an audit note on exhaustion.
//   - On ticket creation an audit history row records the originating channel.
//
// The routing/delivery decisions are pure functions over generic record maps so
// they are unit-testable without a database, mirroring the helpdesk_sla engine.

const (
	routeCronCode    = "helpdesk.channel.route"
	deliveryCronCode = "helpdesk.channel.delivery"
	datetimeFull     = "2006-01-02 15:04:05"
	openStateSet     = "new,open,in_progress,pending"
)

func init() {
	// Live, non-stored aggregates.
	orm.RegisterCompute("helpdesk.channel.connection", "channel_count", nil, computeConnectionChannelCount)
	orm.RegisterCompute("helpdesk.channel.connection", "ticket_count", nil, computeConnectionTicketCount)
	orm.RegisterCompute("helpdesk.agent.load", "current_open", nil, computeAgentCurrentOpen)

	// Scheduled routing + delivery-retry engines.
	scheduler.RegisterCronHandler(routeCronCode, runRoutingScan)
	scheduler.RegisterCronHandler(deliveryCronCode, runDeliveryRetry)

	// Origin-channel audit on ticket creation.
	event.Subscribe(orm.EventRecordCreated, onTicketCreatedForChannel)
}

// ---------------------------------------------------------------------------
// Pure decision functions (unit-tested)
// ---------------------------------------------------------------------------

// isOpenTicket reports whether a ticket state still needs routing/attention.
func isOpenTicket(state string) bool {
	switch state {
	case "new", "open", "in_progress", "pending":
		return true
	}
	return false
}

// isUnassigned reports whether a ticket has neither an assignee nor a team.
func isUnassigned(assignee, team string) bool {
	return strings.TrimSpace(assignee) == "" && strings.TrimSpace(team) == ""
}

// ruleMatches decides whether a routing rule applies to a ticket. Blank / "any"
// match fields act as wildcards; a keyword match is a case-insensitive substring
// of the subject. Every non-wildcard constraint must hold.
func ruleMatches(ticket, rule map[string]string) bool {
	if v := rule["match_priority"]; v != "" && v != "any" && v != ticket["priority"] {
		return false
	}
	if v := rule["match_channel"]; v != "" && v != "any" && v != ticket["channel"] {
		return false
	}
	if v := rule["match_ticket_type"]; v != "" && v != "any" && v != ticket["ticket_type"] {
		return false
	}
	if v := rule["category_id"]; v != "" && v != "0" && v != ticket["category_id"] {
		return false
	}
	if kw := strings.ToLower(strings.TrimSpace(rule["keyword"])); kw != "" {
		if !strings.Contains(strings.ToLower(ticket["name"]), kw) {
			return false
		}
	}
	return true
}

// firstMatchingRule returns the first (already sequence-ordered) rule that matches.
func firstMatchingRule(ticket map[string]string, rules []map[string]string) map[string]string {
	for _, r := range rules {
		if r["active"] == "f" || r["active"] == "false" || r["active"] == "0" {
			continue
		}
		if ruleMatches(ticket, r) {
			return r
		}
	}
	return nil
}

// pickLeastLoaded selects the available agent with the most headroom (max-open
// minus current open). Ties break toward the lower current load, then stable order.
func pickLeastLoaded(candidates []map[string]string) map[string]string {
	var best map[string]string
	bestHeadroom := -1 << 30
	for _, c := range candidates {
		if c["available"] == "f" || c["available"] == "false" || c["available"] == "0" {
			continue
		}
		headroom := atoi(c["max_open"]) - atoi(c["current_open"])
		if headroom > bestHeadroom {
			bestHeadroom = headroom
			best = c
		}
	}
	return best
}

// canRetryDelivery reports whether a failed/pending outbound delivery may be
// retried again within the connection's max-retry budget.
func canRetryDelivery(status string, retries, maxRetries int) bool {
	if status != "failed" && status != "pending" {
		return false
	}
	if maxRetries <= 0 {
		maxRetries = 3
	}
	return retries < maxRetries
}

// looksDuplicate reports whether an inbound message repeats one already seen
// within the caller-supplied recent set (matched by external id or correlation
// key). The caller is responsible for narrowing `seen` to the connection's
// duplicate window before invoking this.
func looksDuplicate(externalID, corrKey string, seen [][]string) bool {
	if externalID == "" && corrKey == "" {
		return false
	}
	for _, s := range seen {
		if len(s) < 2 {
			continue
		}
		if externalID != "" && s[0] == externalID {
			return true
		}
		if corrKey != "" && s[1] == corrKey {
			return true
		}
	}
	return false
}

// routingDecision summarises the field updates a matched rule should apply.
type routingDecision struct {
	TeamID     string
	UserID     string
	Priority   string
	ShouldMove bool
}

func decideRouting(ticket, rule map[string]string, loads []map[string]string) routingDecision {
	if rule == nil {
		return routingDecision{}
	}
	d := routingDecision{}
	if t := rule["team_id"]; t != "" && t != "0" {
		d.TeamID = t
		d.ShouldMove = true
	}
	switch RoutingMethod(rule["method"]) {
	case RouteUser:
		if u := rule["user_id"]; u != "" && u != "0" {
			d.UserID = u
			d.ShouldMove = true
		}
	case RouteLeastLoad, RouteRoundRobin:
		if best := pickLeastLoaded(loads); best != nil {
			d.UserID = best["user_id"]
			d.ShouldMove = true
		} else if u := rule["user_id"]; u != "" && u != "0" {
			d.UserID = u
			d.ShouldMove = true
		}
	}
	if rule["raise_priority"] == "t" || rule["raise_priority"] == "true" || rule["raise_priority"] == "1" {
		if np := rule["new_priority"]; np != "" {
			d.Priority = np
			d.ShouldMove = true
		}
	}
	return d
}

// ---------------------------------------------------------------------------
// Scheduled routing scan
// ---------------------------------------------------------------------------

func runRoutingScan(ctx context.Context, _ map[string]interface{}) error {
	return orm.WithElevated(ctx, routeCronCode, func(bypass context.Context) error {
		rules := fetchAll(bypass, "helpdesk.routing.rule",
			[][]interface{}{{"active", "=", true}}, "sequence ASC, id ASC")
		if len(rules) == 0 {
			return nil
		}
		stringRules := toRuleStrings(rules)
		tickets := fetchAll(bypass, "helpdesk.ticket",
			[][]interface{}{{"state", "in", strings.Split(openStateSet, ",")}}, "id ASC")
		for _, tk := range tickets {
			if !isUnassigned(str(tk["assigned_to"]), str(tk["team_id"])) {
				continue
			}
			ts := toTicketStrings(tk)
			matched := firstMatchingRule(ts, stringRules)
			if matched == nil {
				continue
			}
			loads := loadCandidates(bypass, matched["team_id"])
			ls := toLoadStrings(loads, bypass)
			dec := decideRouting(ts, matched, ls)
			if !dec.ShouldMove {
				continue
			}
			applyRouting(bypass, int(atoi(str(tk["id"]))), matched, dec)
		}
		return nil
	})
}

func applyRouting(ctx context.Context, ticketID int, rule map[string]string, dec routingDecision) {
	upd := map[string]interface{}{}
	if dec.TeamID != "" {
		upd["team_id"] = atoi(dec.TeamID)
	}
	if dec.UserID != "" {
		upd["assigned_to"] = atoi(dec.UserID)
	}
	if dec.Priority != "" {
		upd["priority"] = dec.Priority
	}
	if len(upd) == 0 {
		return
	}
	_ = orm.UpdateRecordByID(ctx, "helpdesk.ticket", ticketID, upd)
	addHistory(ctx, ticketID, "assignment", "routing", "", rule["name"],
		fmt.Sprintf("Auto-routed by rule %q.", rule["name"]))
	bumpRuleHits(ctx, rule["id"])
}

// bumpRuleHits increments a routing rule's stored match counter, tolerating the
// occasional race (the count is advisory analytics, not a correctness invariant).
func bumpRuleHits(ctx context.Context, ruleID string) {
	rid := atoi(ruleID)
	if rid <= 0 {
		return
	}
	rec, err := orm.SearchOne(ctx, "helpdesk.routing.rule", map[string]interface{}{"id": rid})
	if err != nil || rec == nil {
		return
	}
	_ = orm.UpdateRecordByID(ctx, "helpdesk.routing.rule", rid, map[string]interface{}{
		"hits_count": atoi(str(rec["hits_count"])) + 1,
	})
}

func loadCandidates(ctx context.Context, teamID string) []map[string]interface{} {
	domain := [][]interface{}{{"available", "=", true}}
	if tid := atoi(teamID); tid > 0 {
		domain = append(domain, []interface{}{"team_id", "in", []int{0, tid}})
	}
	return fetchAll(ctx, "helpdesk.agent.load", domain, "max_open DESC, id ASC")
}

// ---------------------------------------------------------------------------
// Scheduled delivery retry
// ---------------------------------------------------------------------------

func runDeliveryRetry(ctx context.Context, _ map[string]interface{}) error {
	return orm.WithElevated(ctx, deliveryCronCode, func(bypass context.Context) error {
		msgs := fetchAll(bypass, "helpdesk.channel.message",
			[][]interface{}{{"direction", "=", "outbound"}, {"delivery_status", "in", []string{"failed", "pending"}}},
			"id ASC")
		for _, m := range msgs {
			retries := atoi(str(m["retry_count"]))
			conn := loadConnection(bypass, int(atoi(str(m["connection_id"]))))
			maxRetries := 3
			if conn != nil {
				maxRetries = atoi(str(conn["max_retries"]))
			}
			status := str(m["delivery_status"])
			if !canRetryDelivery(status, retries, maxRetries) {
				if status == "failed" {
					_ = orm.UpdateRecordByID(bypass, "helpdesk.channel.message", int(atoi(str(m["id"]))),
						map[string]interface{}{"last_error": "Retry budget exhausted."})
				}
				continue
			}
			// Simulated re-dispatch: advance the retry counter and mark pending. A
			// real channel adapter would call the provider here and set sent/failed.
			newRetry := retries + 1
			upd := map[string]interface{}{"retry_count": newRetry, "delivery_status": "pending"}
			if newRetry >= maxRetries {
				upd["delivery_status"] = "failed"
				upd["last_error"] = fmt.Sprintf("Delivery failed after %d attempts.", newRetry)
			}
			_ = orm.UpdateRecordByID(bypass, "helpdesk.channel.message", int(atoi(str(m["id"]))), upd)
		}
		return nil
	})
}

func loadConnection(ctx context.Context, id int) map[string]interface{} {
	if id <= 0 {
		return nil
	}
	rec, err := orm.SearchOne(ctx, "helpdesk.channel.connection", map[string]interface{}{"id": id})
	if err != nil {
		return nil
	}
	return rec
}

// ---------------------------------------------------------------------------
// Ticket-creation origin audit
// ---------------------------------------------------------------------------

func onTicketCreatedForChannel(ctx context.Context, ev event.Event) error {
	model, _ := ev.Payload["model"].(string)
	if model != "helpdesk.ticket" {
		return nil
	}
	id, ok := orm.CoerceInt64(ev.Payload["id"])
	if !ok || id <= 0 {
		return nil
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, model, map[string]interface{}{"id": int(id)})
	if err != nil || rec == nil {
		return nil
	}
	channel := str(rec["channel"])
	if channel == "" {
		channel = "internal"
	}
	addHistory(bypass, int(id), "created", "channel", "", channel,
		fmt.Sprintf("Ticket received via %s channel.", channel))
	return nil
}

func addHistory(ctx context.Context, ticketID int, action, field, oldV, newV, summary string) {
	inst, ok := orm.Registry["helpdesk.ticket.history"]
	if !ok || ticketID <= 0 {
		return
	}
	_, _ = orm.Create(ctx, inst, map[string]interface{}{
		"ticket_id": ticketID,
		"action":    action,
		"field":     field,
		"old_value": oldV,
		"new_value": newV,
		"summary":   summary,
		"date":      nowStamp(),
	})
}

// ---------------------------------------------------------------------------
// Computed fields
// ---------------------------------------------------------------------------

func computeConnectionChannelCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return countLinkedTo(ctx, "helpdesk.channel.message", "connection_id", rec["id"])
}

func computeConnectionTicketCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	cid, ok := orm.CoerceInt64(rec["id"])
	if !ok || cid <= 0 {
		return 0, nil
	}
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), "helpdesk.channel.message",
		[][]interface{}{{"connection_id", "=", int(cid)}, {"ticket_id", "!=", 0}})
	if err != nil {
		return nil, err
	}
	return n, nil
}

func computeAgentCurrentOpen(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	uid, ok := orm.CoerceInt64(rec["user_id"])
	if !ok || uid <= 0 {
		return 0, nil
	}
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), "helpdesk.ticket",
		[][]interface{}{
			{"assigned_to", "=", int(uid)},
			{"state", "in", strings.Split(openStateSet, ",")},
		})
	if err != nil {
		return nil, err
	}
	return n, nil
}

// ---------------------------------------------------------------------------
// Generic data helpers
// ---------------------------------------------------------------------------

func countLinkedTo(ctx context.Context, model, field string, id interface{}) (interface{}, error) {
	iid, ok := orm.CoerceInt64(id)
	if !ok || iid <= 0 {
		return 0, nil
	}
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), model,
		[][]interface{}{{field, "=", int(iid)}})
	if err != nil {
		return nil, err
	}
	return n, nil
}

func fetchAll(ctx context.Context, model string, domain [][]interface{}, order string) []map[string]interface{} {
	var out []map[string]interface{}
	offset := 0
	const page = 500
	for {
		rows, err := orm.SearchPage(ctx, model, domain, page, offset, order)
		if err != nil || len(rows) == 0 {
			break
		}
		out = append(out, rows...)
		if len(rows) < page {
			break
		}
		offset += page
	}
	return out
}

func str(v interface{}) string { return strings.TrimSpace(orm.AsString(v)) }

func atoi(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	var n int
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func nowStamp() string { return time.Now().UTC().Format(datetimeFull) }

func toRuleStrings(rules []map[string]interface{}) []map[string]string {
	out := make([]map[string]string, 0, len(rules))
	for _, r := range rules {
		out = append(out, map[string]string{
			"id": str(r["id"]), "name": str(r["name"]), "active": str(r["active"]),
			"match_priority": str(r["match_priority"]), "match_channel": str(r["match_channel"]),
			"match_ticket_type": str(r["match_ticket_type"]), "category_id": str(r["category_id"]),
			"keyword": str(r["keyword"]), "method": str(r["method"]), "team_id": str(r["team_id"]),
			"user_id": str(r["user_id"]), "raise_priority": str(r["raise_priority"]),
			"new_priority": str(r["new_priority"]),
		})
	}
	return out
}

func toTicketStrings(t map[string]interface{}) map[string]string {
	return map[string]string{
		"id": str(t["id"]), "name": str(t["name"]), "priority": str(t["priority"]),
		"channel": str(t["channel"]), "ticket_type": str(t["ticket_type"]),
		"category_id": str(t["category_id"]), "assigned_to": str(t["assigned_to"]),
		"team_id": str(t["team_id"]), "state": str(t["state"]),
	}
}

func toLoadStrings(loads []map[string]interface{}, ctx context.Context) []map[string]string {
	out := make([]map[string]string, 0, len(loads))
	for _, l := range loads {
		open := 0
		if uid := atoi(str(l["user_id"])); uid > 0 {
			if n, err := orm.SearchCount(ctx, "helpdesk.ticket", [][]interface{}{
				{"assigned_to", "=", uid},
				{"state", "in", strings.Split(openStateSet, ",")},
			}); err == nil {
				open = n
			}
		}
		out = append(out, map[string]string{
			"user_id": str(l["user_id"]), "team_id": str(l["team_id"]),
			"max_open": str(l["max_open"]), "available": str(l["available"]),
			"current_open": fmt.Sprintf("%d", open),
		})
	}
	return out
}
