package models

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"nexa/core/event"
	"nexa/core/orm"
	"nexa/core/scheduler"
)

// Behaviour layer for the automation companion. A rule is an event-driven
// WHEN/IF/THEN unit. The pure decision functions below are unit-tested without a
// database, mirroring the helpdesk_sla / _channel / _catalog engines. Actions are
// applied under a security-bypass context, which (per orm side-effect rules) does
// not emit nested record events, giving clean loop prevention on top of apply-once.

const (
	datetimeLayout = "2006-01-02 15:04:05"
	sweepCronCode  = "helpdesk.automation.sweep"
	ticketModel    = "helpdesk.ticket"
	openStateSet   = "new,open,in_progress,pending"
)

func init() {
	orm.RegisterCompute("helpdesk.automation.rule", "condition_count", nil, computeRuleConditionCount)
	orm.RegisterCompute("helpdesk.automation.rule", "action_count", nil, computeRuleActionCount)

	event.Subscribe(orm.EventRecordCreated, onTicketEvent)
	event.Subscribe(orm.EventRecordUpdated, onTicketEvent)
	scheduler.RegisterCronHandler(sweepCronCode, runScheduledSweep)
}

// ---------------------------------------------------------------------------
// Pure decision functions (unit-tested)
// ---------------------------------------------------------------------------

// matchesOperator compares a field's actual value to the condition's expected value.
func matchesOperator(actual string, op ConditionOperator, expected string) bool {
	a := strings.TrimSpace(actual)
	e := strings.TrimSpace(expected)
	switch op {
	case OpEquals:
		return strings.EqualFold(a, e)
	case OpNotEquals:
		return !strings.EqualFold(a, e)
	case OpContains:
		return e != "" && strings.Contains(strings.ToLower(a), strings.ToLower(e))
	case OpIsEmpty:
		return a == ""
	case OpIsNotEmpty:
		return a != ""
	case OpIn:
		return inSet(a, e)
	case OpNotIn:
		return !inSet(a, e)
	case OpGreaterThan:
		af, aok := parseFloat(a)
		ef, eok := parseFloat(e)
		return aok && eok && af > ef
	case OpLessThan:
		af, aok := parseFloat(a)
		ef, eok := parseFloat(e)
		return aok && eok && af < ef
	default:
		return false
	}
}

func inSet(actual, csv string) bool {
	if actual == "" {
		return false
	}
	for _, p := range strings.Split(csv, ",") {
		if strings.EqualFold(strings.TrimSpace(p), actual) {
			return true
		}
	}
	return false
}

func parseFloat(s string) (float64, bool) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// combineResults evaluates condition outcomes under the rule's match logic.
// With no conditions a rule matches (so trigger-only rules can act).
func combineResults(results []bool, logic MatchLogic) bool {
	if len(results) == 0 {
		return true
	}
	if logic == MatchAny {
		for _, r := range results {
			if r {
				return true
			}
		}
		return false
	}
	for _, r := range results {
		if !r {
			return false
		}
	}
	return true
}

// triggerForHistoryField maps a changed history field to its *_changed trigger.
func triggerForHistoryField(field string) (AutomationTrigger, bool) {
	switch strings.TrimSpace(field) {
	case "state":
		return TriggerStateChanged, true
	case "priority":
		return TriggerPriorityChanged, true
	case "assigned_to", "assignment":
		return TriggerAssigneeChanged, true
	default:
		return "", false
	}
}

// bumpPriority steps a ticket priority up one level for the escalate action.
func bumpPriority(current string) string {
	order := []string{"low", "normal", "high", "urgent"}
	idx := 0
	for i, p := range order {
		if strings.EqualFold(p, strings.TrimSpace(current)) {
			idx = i
			break
		}
	}
	if idx >= len(order)-1 {
		return "urgent"
	}
	return order[idx+1]
}

// nextVersionNumber is the version id assigned when publishing a rule.
func nextVersionNumber(current int) int {
	if current < 0 {
		return 1
	}
	return current + 1
}

// shouldExecute decides whether a matched rule applies its actions (vs dry-run).
func shouldExecute(dryRun bool) bool { return !dryRun }

// ---------------------------------------------------------------------------
// Computes
// ---------------------------------------------------------------------------

func computeRuleConditionCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return countLinkedBy(ctx, "helpdesk.automation.condition", "rule_id", rec["id"])
}

func computeRuleActionCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return countLinkedBy(ctx, "helpdesk.automation.action", "rule_id", rec["id"])
}

// ---------------------------------------------------------------------------
// Event-driven execution
// ---------------------------------------------------------------------------

func onTicketEvent(ctx context.Context, ev event.Event) error {
	model, _ := ev.Payload["model"].(string)
	if model != ticketModel {
		return nil
	}
	id, ok := orm.CoerceInt64(ev.Payload["id"])
	if !ok || id <= 0 {
		return nil
	}
	isCreate := ev.Name == orm.EventRecordCreated
	bypass := orm.ContextWithBypass(ctx, true)

	ticket, err := orm.SearchOne(bypass, ticketModel, map[string]interface{}{"id": int(id)})
	if err != nil || ticket == nil {
		return nil
	}
	tags := fetchTicketTags(bypass, int(id))

	var triggers []AutomationTrigger
	if isCreate {
		triggers = []AutomationTrigger{TriggerTicketCreated}
	} else {
		triggers = []AutomationTrigger{TriggerTicketUpdated}
		if field, _, _ := latestHistoryField(bypass, int(id)); field != "" {
			if t, ok := triggerForHistoryField(field); ok {
				triggers = append(triggers, t)
			}
		}
	}
	for _, trig := range triggers {
		runRulesForTrigger(bypass, trig, ticket, tags)
	}
	return nil
}

// runRulesForTrigger evaluates all active rules of a trigger type against one ticket.
func runRulesForTrigger(ctx context.Context, trigger AutomationTrigger, ticket map[string]interface{}, tags []int) {
	rules := loadRulesByTrigger(ctx, trigger)
	if len(rules) == 0 {
		return
	}
	ticketID := atoi(str(ticket["id"]))
	for _, rule := range rules {
		evaluateRule(ctx, rule, ticketID, ticket, tags, trigger)
	}
}

func evaluateRule(ctx context.Context, rule map[string]interface{}, ticketID int, ticket map[string]interface{}, tags []int, trigger AutomationTrigger) {
	ruleID := atoi(str(rule["id"]))
	started := nowStamp()
	conds := loadChildren(ctx, "helpdesk.automation.condition", ruleID)
	actuals := map[string]string{}
	for _, c := range conds {
		f := str(c["field"])
		actuals[f] = fieldValue(ticket, tags, ConditionField(f))
	}
	results := make([]bool, 0, len(conds))
	for _, c := range conds {
		results = append(results, matchesOperator(actuals[str(c["field"])], ConditionOperator(str(c["operator"])), str(c["value"])))
	}
	matched := combineResults(results, MatchLogic(strOr(rule["match_logic"], "all")))

	run := map[string]interface{}{
		"rule_id":      ruleID,
		"ticket_id":    ticketID,
		"trigger_type": trigger,
		"matched":      matched,
		"dry_run":      orm.AsBool(rule["dry_run"]),
		"started_at":   started,
	}
	if !matched {
		run["status"] = RunSkipped
		run["finished_at"] = nowStamp()
		createRun(ctx, run)
		bumpCounters(ctx, ruleID, false, false, started)
		return
	}

	actions := loadChildren(ctx, "helpdesk.automation.action", ruleID)
	// Sort by sequence for deterministic execution.
	sort.SliceStable(actions, func(i, j int) bool { return atoi(str(actions[i]["sequence"])) < atoi(str(actions[j]["sequence"])) })
	run["status"] = RunMatched
	run["actions_planned"] = len(actions)
	if n, _ := orm.CoerceInt64(rule["current_version"]); n == 0 {
		// Unpublished rule: log but do not mutate. Explicit publish is required.
		run["status"] = RunSkipped
		run["detail"] = "rule has no published version; publish before applying"
		run["finished_at"] = nowStamp()
		createRun(ctx, run)
		bumpCounters(ctx, ruleID, true, false, started)
		return
	}
	if !shouldExecute(orm.AsBool(rule["dry_run"])) {
		run["status"] = RunDryRun
		run["detail"] = describeActions(actions)
		run["finished_at"] = nowStamp()
		createRun(ctx, run)
		bumpCounters(ctx, ruleID, true, false, started)
		return
	}
	if applyOnce := orm.AsBool(rule["apply_once"]); applyOnce && alreadyApplied(ctx, ruleID, ticketID) {
		run["status"] = RunSkipped
		run["detail"] = "apply-once: rule already executed for this ticket"
		run["finished_at"] = nowStamp()
		createRun(ctx, run)
		bumpCounters(ctx, ruleID, true, false, started)
		return
	}

	applied, failed, reason := applyActions(ctx, actions, ticket, ticketID)
	run["actions_applied"] = applied
	run["finished_at"] = nowStamp()
	if failed {
		run["status"] = RunFailed
		run["error_reason"] = reason
		createRun(ctx, run)
		bumpCounters(ctx, ruleID, true, true, started)
		return
	}
	run["status"] = RunExecuted
	createRun(ctx, run)
	bumpCounters(ctx, ruleID, true, false, started)
}

// applyActions performs each action against the ticket, returning applied count and
// the first failure (if any). Ticket field mutations run under the bypass context.
func applyActions(ctx context.Context, actions []map[string]interface{}, ticket map[string]interface{}, ticketID int) (int, bool, string) {
	applied := 0
	for _, a := range actions {
		typ := ActionType(str(a["action_type"]))
		val := str(a["value"])
		switch typ {
		case ActionSetState:
			if err := orm.UpdateRecordByID(ctx, ticketModel, ticketID, map[string]interface{}{"state": val}); err != nil {
				return applied, true, "set_state: " + err.Error()
			}
		case ActionSetPriority:
			if err := orm.UpdateRecordByID(ctx, ticketModel, ticketID, map[string]interface{}{"priority": val}); err != nil {
				return applied, true, "set_priority: " + err.Error()
			}
		case ActionSetTeam:
			if err := setRef(ctx, "team_id", val, ticketID); err != nil {
				return applied, true, "set_team: " + err.Error()
			}
		case ActionSetAssignee:
			if err := setRef(ctx, "assigned_to", val, ticketID); err != nil {
				return applied, true, "set_assignee: " + err.Error()
			}
		case ActionSetCategory:
			if err := setRef(ctx, "category_id", val, ticketID); err != nil {
				return applied, true, "set_category: " + err.Error()
			}
		case ActionAddNote:
			if err := addNote(ctx, ticketID, val); err != nil {
				return applied, true, "add_note: " + err.Error()
			}
		case ActionAddTag:
			if err := addTag(ctx, ticketID, val); err != nil {
				return applied, true, "add_tag: " + err.Error()
			}
		case ActionEscalate:
			next := bumpPriority(str(ticket["priority"]))
			if err := orm.UpdateRecordByID(ctx, ticketModel, ticketID, map[string]interface{}{"priority": next, "state": "in_progress"}); err != nil {
				return applied, true, "escalate: " + err.Error()
			}
		default:
			// Unknown action type is skipped, not an error.
			continue
		}
		applied++
	}
	return applied, false, ""
}

func setRef(ctx context.Context, column, val string, ticketID int) error {
	id := atoi(val)
	return orm.UpdateRecordByID(ctx, ticketModel, ticketID, map[string]interface{}{column: id})
}

func addNote(ctx context.Context, ticketID int, body string) error {
	if strings.TrimSpace(body) == "" {
		return nil
	}
	m := orm.Registry["helpdesk.ticket.message"]
	if m == nil {
		return fmt.Errorf("message model not registered")
	}
	_, err := orm.Create(ctx, m, map[string]interface{}{
		"ticket_id":    ticketID,
		"body":         body,
		"message_type": "note",
		"is_internal":  true,
	})
	return err
}

// addTag links a tag to the ticket via the anchor's relation table (idempotent).
func addTag(ctx context.Context, ticketID int, val string) error {
	tagID := atoi(val)
	if tagID <= 0 || orm.DB == nil {
		return nil
	}
	_, err := orm.DB.ExecContext(ctx,
		`INSERT INTO helpdesk_ticket_tag_rel (ticket_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		ticketID, tagID)
	return err
}

// ---------------------------------------------------------------------------
// Scheduled sweep (cron)
// ---------------------------------------------------------------------------

func runScheduledSweep(ctx context.Context, _ map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	rules := loadRulesByTrigger(bypass, TriggerScheduled)
	if len(rules) == 0 {
		return nil
	}
	domain := [][]interface{}{{"state", "in", strings.Split(openStateSet, ",")}}
	tickets := fetchAll(bypass, ticketModel, domain, "id asc")
	for _, t := range tickets {
		tid := atoi(str(t["id"]))
		tags := fetchTicketTags(bypass, tid)
		for _, rule := range rules {
			evaluateRule(bypass, rule, tid, t, tags, TriggerScheduled)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Data helpers (package-local)
// ---------------------------------------------------------------------------

func loadRulesByTrigger(ctx context.Context, trigger AutomationTrigger) []map[string]interface{} {
	rows := fetchAll(ctx, "helpdesk.automation.rule",
		[][]interface{}{{"active", "=", true}, {"trigger_type", "=", string(trigger)}}, "sequence asc, id asc")
	return rows
}

func loadChildren(ctx context.Context, model string, ruleID int) []map[string]interface{} {
	return fetchAll(ctx, model, [][]interface{}{{"rule_id", "=", ruleID}}, "sequence asc, id asc")
}

func fieldValue(ticket map[string]interface{}, tags []int, field ConditionField) string {
	switch field {
	case FieldHasTag:
		parts := make([]string, 0, len(tags))
		for _, t := range tags {
			parts = append(parts, strconv.Itoa(t))
		}
		return strings.Join(parts, ",")
	case FieldState:
		return str(ticket["state"])
	case FieldPriority:
		return str(ticket["priority"])
	case FieldTicketType:
		return str(ticket["ticket_type"])
	case FieldChannel:
		return str(ticket["channel"])
	case FieldTeam:
		return str(ticket["team_id"])
	case FieldAssignee:
		return str(ticket["assigned_to"])
	case FieldCategory:
		return str(ticket["category_id"])
	case FieldPartner:
		return str(ticket["partner_id"])
	case FieldReopenCount:
		return str(ticket["reopened_count"])
	case FieldName:
		return str(ticket["name"])
	case FieldDescription:
		return str(ticket["description"])
	default:
		return ""
	}
}

func fetchTicketTags(ctx context.Context, ticketID int) []int {
	if ticketID <= 0 || orm.DB == nil {
		return nil
	}
	rows, err := orm.DB.QueryContext(ctx, `SELECT tag_id FROM helpdesk_ticket_tag_rel WHERE ticket_id = $1`, ticketID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []int
	for rows.Next() {
		var tid int
		if err := rows.Scan(&tid); err == nil {
			out = append(out, tid)
		}
	}
	return out
}

// latestHistoryField returns the field of the most recent history line for a ticket.
func latestHistoryField(ctx context.Context, ticketID int) (string, string, bool) {
	rows := fetchAll(ctx, "helpdesk.ticket.history", [][]interface{}{{"ticket_id", "=", ticketID}}, "date desc, id desc")
	if len(rows) == 0 {
		return "", "", false
	}
	top := rows[0]
	return str(top["field"]), str(top["new_value"]), true
}

func alreadyApplied(ctx context.Context, ruleID, ticketID int) bool {
	n, err := orm.SearchCount(ctx, "helpdesk.automation.run", [][]interface{}{
		{"rule_id", "=", ruleID},
		{"ticket_id", "=", ticketID},
		{"status", "=", string(RunExecuted)},
	})
	if err != nil {
		return false
	}
	return n > 0
}

func createRun(ctx context.Context, vals map[string]interface{}) {
	m := orm.Registry["helpdesk.automation.run"]
	if m == nil {
		return
	}
	_, _ = orm.Create(ctx, m, vals)
}

func bumpCounters(ctx context.Context, ruleID int, matched, errored bool, stamp string) {
	vals := map[string]interface{}{"last_run_at": stamp}
	rule, err := orm.SearchOne(ctx, "helpdesk.automation.rule", map[string]interface{}{"id": ruleID})
	if err == nil && rule != nil {
		vals["run_count"] = atoi(str(rule["run_count"])) + 1
		vals["match_count"] = atoi(str(rule["match_count"]))
		vals["error_count"] = atoi(str(rule["error_count"]))
		if matched {
			vals["match_count"] = atoi(str(rule["match_count"])) + 1
		}
		if errored {
			vals["error_count"] = atoi(str(rule["error_count"])) + 1
		}
	}
	_ = orm.UpdateRecordByID(ctx, "helpdesk.automation.rule", ruleID, vals)
}

func describeActions(actions []map[string]interface{}) string {
	parts := make([]string, 0, len(actions))
	for _, a := range actions {
		parts = append(parts, fmt.Sprintf("%s(%s)", str(a["action_type"]), str(a["value"])))
	}
	return strings.Join(parts, "; ")
}

// ---------------------------------------------------------------------------
// Generic helpers
// ---------------------------------------------------------------------------

func countLinkedBy(ctx context.Context, model, field string, id interface{}) (interface{}, error) {
	iid, ok := orm.CoerceInt64(id)
	if !ok || iid <= 0 {
		return 0, nil
	}
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), model, [][]interface{}{{field, "=", int(iid)}})
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

func strOr(v interface{}, def string) string {
	s := strings.TrimSpace(orm.AsString(v))
	if s == "" {
		return def
	}
	return s
}

func atoi(s string) int {
	s = strings.TrimSpace(s)
	n, _ := strconv.Atoi(s)
	return n
}

func nowStamp() string { return time.Now().UTC().Format(datetimeLayout) }
