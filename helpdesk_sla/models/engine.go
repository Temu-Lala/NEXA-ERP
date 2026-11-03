package models

import (
	"context"
	"fmt"
	"time"

	"nexa/core/event"
	"nexa/core/orm"
	"nexa/core/scheduler"
)

// The SLA engine. It turns the (previously inert) helpdesk_sla data model into a
// working service-level system:
//
//   - On ticket creation an applicable policy is resolved and first-response /
//     resolution clocks are started with deadlines computed against the policy's
//     working calendar (or wall-clock time when unbounded).
//   - On ticket update the clocks are reconciled idempotently: the first-response
//     clock stops when a response is recorded, the resolution clock stops on
//     resolve/close, and pending tickets pause their clocks (resumed on return).
//   - A scheduled job (helpdesk.sla.check) evaluates running clocks, flipping them
//     to warning at the policy threshold and to breached past their target, firing
//     the policy's escalation rules and appending to the SLA audit trail.
//
// All transitions are guarded so a rule fires once per threshold, avoiding the
// duplicate-action / infinite-loop hazards. The ticket stays the source of truth;
// the engine only observes it and writes SLA artefacts, reusing the anchor's
// message and history models for notifications rather than duplicating them.

const slaCronCode = "helpdesk.sla.check"

func init() {
	event.Subscribe(orm.EventRecordCreated, onTicketCreatedForSLA)
	event.Subscribe(orm.EventRecordUpdated, onTicketUpdatedForSLA)
	scheduler.RegisterCronHandler(slaCronCode, runSLACheck)
}

// ---------------------------------------------------------------------------
// Ticket creation: start clocks
// ---------------------------------------------------------------------------

func onTicketCreatedForSLA(ctx context.Context, ev event.Event) error {
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
	policy, cal := resolvePolicy(bypass, rec)
	if policy == nil {
		return nil
	}
	start := time.Now().UTC()
	pid := intOrZero(policy["id"])
	if fr := intOrZero(policy["first_response_mins"]); fr > 0 {
		startClock(bypass, int(id), pid, SLATypeFirstResponse, fr, cal, start, ev.Actor)
	}
	if res := intOrZero(policy["resolution_mins"]); res > 0 {
		startClock(bypass, int(id), pid, SLATypeResolution, res, cal, start, ev.Actor)
	}
	return nil
}

func startClock(ctx context.Context, ticketID, policyID int, slaType SLAType, targetMins int, cal *slaCalendar, start time.Time, actor int) {
	inst, ok := orm.Registry["helpdesk.sla.timer"]
	if !ok {
		return
	}
	due := addBusinessMinutes(cal, start, targetMins)
	tid, err := orm.Create(ctx, inst, map[string]interface{}{
		"ticket_id":    ticketID,
		"policy_id":    policyID,
		"sla_type":     string(slaType),
		"start":        fmtDT(start),
		"due":          fmtDT(due),
		"breach_state": string(BreachStateOnTrack),
		"target_mins":  targetMins,
	})
	if err != nil {
		return
	}
	logEvent(ctx, ticketID, tid, SLAEventCreated,
		fmt.Sprintf("SLA %s clock started (target %d min).", string(slaType), targetMins), actor)
}

// ---------------------------------------------------------------------------
// Ticket update: reconcile clocks (idempotent — no before/after values needed)
// ---------------------------------------------------------------------------

func onTicketUpdatedForSLA(ctx context.Context, ev event.Event) error {
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
	reconcileTicket(ctx, bypass, int(id), rec)
	return nil
}

func reconcileTicket(ctx context.Context, bypass context.Context, ticketID int, ticket map[string]interface{}) {
	state := orm.AsString(ticket["state"])
	firstResp := trim(orm.AsString(ticket["first_response_at"]))
	timers := fetchRunningTimers(bypass, ticketID)
	for _, t := range timers {
		pol := loadPolicy(bypass, intOrZero(t["policy_id"]))
		if pol == nil {
			continue
		}
		cal := calendarFor(bypass, pol)
		now := time.Now().UTC()

		// Keep pause state consistent with the ticket state.
		if state == "pending" {
			if !orm.AsBool(t["paused"]) {
				pauseClock(bypass, t, now)
			}
			continue
		}
		if orm.AsBool(t["paused"]) {
			resumeClock(bypass, t, now)
		}

		slaType := SLAType(orm.AsString(t["sla_type"]))
		completed := false
		switch slaType {
		case SLATypeFirstResponse:
			if firstResp != "" || state == "in_progress" || state == "resolved" || state == "closed" {
				completed = true
			}
		case SLATypeResolution:
			if state == "resolved" || state == "closed" {
				completed = true
			}
		}
		if completed {
			finalizeClock(bypass, t, pol, cal, now)
		}
	}
}

func fetchRunningTimers(ctx context.Context, ticketID int) []map[string]interface{} {
	return fetchAll(ctx, "helpdesk.sla.timer",
		[][]interface{}{{"ticket_id", "=", ticketID}, {"end", "=", ""}}, "id ASC")
}

func pauseClock(ctx context.Context, t map[string]interface{}, now time.Time) {
	id := intOrZero(t["id"])
	_ = orm.UpdateRecordByID(ctx, "helpdesk.sla.timer", id, map[string]interface{}{
		"paused":        true,
		"pause_started": fmtDT(now),
		"breach_state":  string(BreachStatePaused),
	})
	logEvent(ctx, intOrZero(t["ticket_id"]), id, SLAEventPaused, "SLA clock paused (awaiting customer).", 0)
}

func resumeClock(ctx context.Context, t map[string]interface{}, now time.Time) {
	ps, ok := parseDT(orm.AsString(t["pause_started"]))
	if !ok {
		ps = now
	}
	cal := calendarFor(ctx, loadPolicy(ctx, intOrZero(t["policy_id"])))
	seg := businessMinutesBetween(cal, ps, now)
	paused := intOrZero(t["paused_mins"]) + seg
	id := intOrZero(t["id"])
	_ = orm.UpdateRecordByID(ctx, "helpdesk.sla.timer", id, map[string]interface{}{
		"paused":        false,
		"pause_started": "",
		"paused_mins":   paused,
		"breach_state":  string(BreachStateOnTrack),
	})
	logEvent(ctx, intOrZero(t["ticket_id"]), id, SLAEventResumed,
		fmt.Sprintf("SLA clock resumed (paused %d min).", seg), 0)
}

func finalizeClock(ctx context.Context, t, pol map[string]interface{}, cal *slaCalendar, now time.Time) {
	start, ok := parseDT(orm.AsString(t["start"]))
	if !ok {
		start = now
	}
	target := intOrZero(t["target_mins"])
	elapsed := businessMinutesBetween(cal, start, now) - intOrZero(t["paused_mins"])
	if elapsed < 0 {
		elapsed = 0
	}
	state := BreachStateMet
	breach := 0
	if elapsed > target {
		state = BreachStateBreached
		breach = elapsed - target
	}
	id := intOrZero(t["id"])
	_ = orm.UpdateRecordByID(ctx, "helpdesk.sla.timer", id, map[string]interface{}{
		"end":          fmtDT(now),
		"elapsed_mins": elapsed,
		"breach_state": string(state),
		"breach_mins":  breach,
	})
	slaType := orm.AsString(t["sla_type"])
	if state == BreachStateMet {
		logEvent(ctx, intOrZero(t["ticket_id"]), id, SLAEventMet,
			fmt.Sprintf("%s met in %d min (target %d).", slaType, elapsed, target), 0)
	} else {
		msg := fmt.Sprintf("%s breached: %d min elapsed vs %d target.", slaType, elapsed, target)
		logEvent(ctx, intOrZero(t["ticket_id"]), id, SLAEventBreached, msg, 0)
		runEscalation(ctx, t, pol, "resolution", msg)
	}
}

// ---------------------------------------------------------------------------
// Scheduled breach / warning evaluation
// ---------------------------------------------------------------------------

func runSLACheck(ctx context.Context, _ map[string]interface{}) error {
	return orm.WithElevated(ctx, "helpdesk.sla.check", func(bypass context.Context) error {
		timers := fetchAll(bypass, "helpdesk.sla.timer", [][]interface{}{{"end", "=", ""}}, "id ASC")
		for _, t := range timers {
			ticketID := intOrZero(t["ticket_id"])
			ticket, err := orm.SearchOne(bypass, "helpdesk.ticket", map[string]interface{}{"id": ticketID})
			if err != nil || ticket == nil {
				continue
			}
			pol := loadPolicy(bypass, intOrZero(t["policy_id"]))
			if pol == nil {
				continue
			}
			cal := calendarFor(bypass, pol)
			now := time.Now().UTC()

			state := orm.AsString(ticket["state"])
			if state == "pending" {
				if !orm.AsBool(t["paused"]) {
					pauseClock(bypass, t, now)
				}
				continue
			}
			if orm.AsBool(t["paused"]) {
				resumeClock(bypass, t, now)
				t = reload(bypass, "helpdesk.sla.timer", intOrZero(t["id"]))
			}

			start, ok := parseDT(orm.AsString(t["start"]))
			if !ok {
				continue
			}
			target := intOrZero(t["target_mins"])
			if target <= 0 {
				continue
			}
			elapsed := businessMinutesBetween(cal, start, now) - intOrZero(t["paused_mins"])
			if elapsed < 0 {
				elapsed = 0
			}
			id := intOrZero(t["id"])
			cur := SLABreachState(orm.AsString(t["breach_state"]))
			warningPct := intOrZero(pol["warning_pct"])
			if warningPct <= 0 {
				warningPct = 75
			}
			pct := elapsed * 100 / target

			if elapsed >= target {
				if cur != BreachStateBreached {
					_ = orm.UpdateRecordByID(bypass, "helpdesk.sla.timer", id, map[string]interface{}{
						"elapsed_mins": elapsed,
						"breach_mins":  elapsed - target,
						"breach_state": string(BreachStateBreached),
					})
					msg := fmt.Sprintf("%s SLA breached: %d min elapsed vs %d target.",
						orm.AsString(t["sla_type"]), elapsed, target)
					logEvent(bypass, ticketID, id, SLAEventBreached, msg, 0)
					runEscalation(bypass, t, pol, "breach", msg)
				} else {
					_ = orm.UpdateRecordByID(bypass, "helpdesk.sla.timer", id, map[string]interface{}{"elapsed_mins": elapsed})
				}
				continue
			}

			if pct >= warningPct && !orm.AsBool(t["escalated"]) && cur != BreachStateWarning {
				_ = orm.UpdateRecordByID(bypass, "helpdesk.sla.timer", id, map[string]interface{}{
					"elapsed_mins": elapsed,
					"breach_state": string(BreachStateWarning),
					"escalated":    true,
				})
				msg := fmt.Sprintf("%s SLA at %d%% of target (%d/%d min).", orm.AsString(t["sla_type"]), pct, elapsed, target)
				logEvent(bypass, ticketID, id, SLAEventWarning, msg, 0)
				runEscalation(bypass, t, pol, "warning", msg)
				continue
			}

			if cur == BreachStateOnTrack || cur == BreachStateWarning {
				_ = orm.UpdateRecordByID(bypass, "helpdesk.sla.timer", id, map[string]interface{}{"elapsed_mins": elapsed})
			}
		}
		return nil
	})
}

// ---------------------------------------------------------------------------
// Escalation
// ---------------------------------------------------------------------------

func runEscalation(ctx context.Context, t, pol map[string]interface{}, kind, summary string) {
	policyID := intOrZero(t["policy_id"])
	ticketID := intOrZero(t["ticket_id"])
	rules := fetchAll(ctx, "helpdesk.escalation.rule",
		[][]interface{}{{"policy_id", "=", policyID}, {"active", "=", true}}, "sequence ASC, id ASC")
	if len(rules) == 0 {
		// Always leave an audit note even without an explicit rule.
		addTicketNote(ctx, ticketID, "[SLA "+kind+"] "+summary, 0)
		logEvent(ctx, ticketID, intOrZero(t["id"]), SLAEventEscalated, summary, 0)
		return
	}
	for _, r := range rules {
		action := EscalationAction(orm.AsString(r["action"]))
		notifyUser := intOrZero(r["notify_user_id"])
		note := fmt.Sprintf("[SLA %s escalation] %s", kind, summary)
		if uid := intOrZero(r["notify_user_id"]); uid > 0 {
			note += fmt.Sprintf(" (owner: user #%d)", uid)
		}
		if tid := intOrZero(r["notify_team_id"]); tid > 0 {
			note += fmt.Sprintf(" (team: #%d)", tid)
		}
		if action == EscalationReassign && notifyUser > 0 {
			reassignTicket(ctx, ticketID, notifyUser)
			note = fmt.Sprintf("[SLA %s escalation] reassigned & %s", kind, summary)
		}
		addTicketNote(ctx, ticketID, note, notifyUser)
		logEvent(ctx, ticketID, intOrZero(t["id"]), SLAEventEscalated,
			fmt.Sprintf("Escalation rule applied (%s): %s", string(action), summary), 0)
	}
}

func reassignTicket(ctx context.Context, ticketID, userID int) {
	if ticketID <= 0 || userID <= 0 {
		return
	}
	rec, err := orm.SearchOne(ctx, "helpdesk.ticket", map[string]interface{}{"id": ticketID})
	if err != nil || rec == nil {
		return
	}
	old := intOrZero(rec["assigned_to"])
	_ = orm.UpdateRecordByID(ctx, "helpdesk.ticket", ticketID, map[string]interface{}{"assigned_to": userID})
	if inst, ok := orm.Registry["helpdesk.ticket.history"]; ok {
		_, _ = orm.Create(ctx, inst, map[string]interface{}{
			"ticket_id": ticketID,
			"action":    "assignment",
			"field":     "assigned_to",
			"old_value": fmt.Sprintf("%d", old),
			"new_value": fmt.Sprintf("%d", userID),
			"summary":   "Ticket reassigned by SLA escalation.",
			"date":      fmtDT(time.Now().UTC()),
		})
	}
}

func addTicketNote(ctx context.Context, ticketID int, body string, authorID int) {
	if ticketID <= 0 || trim(body) == "" {
		return
	}
	inst, ok := orm.Registry["helpdesk.ticket.message"]
	if !ok {
		return
	}
	vals := map[string]interface{}{
		"ticket_id":    ticketID,
		"body":         body,
		"message_type": "note",
		"is_internal":  true,
	}
	if authorID > 0 {
		vals["author_id"] = authorID
	}
	_, _ = orm.Create(ctx, inst, vals)
}

// ---------------------------------------------------------------------------
// Policy resolution helpers
// ---------------------------------------------------------------------------

// resolvePolicy picks the first (lowest sequence) active policy matching the
// ticket's priority and category scopes, and loads the working calendar it uses.
func resolvePolicy(ctx context.Context, ticket map[string]interface{}) (map[string]interface{}, *slaCalendar) {
	priority := orm.AsString(ticket["priority"])
	categoryID := intOrZero(ticket["category_id"])
	policies := fetchAll(ctx, "helpdesk.sla.policy", [][]interface{}{{"active", "=", true}}, "sequence ASC, id ASC")
	for _, p := range policies {
		scope := orm.AsString(p["priority"])
		if scope != "" && scope != "any" && scope != priority {
			continue
		}
		pcat := intOrZero(p["category_id"])
		if pcat > 0 && pcat != categoryID {
			continue
		}
		var cal *slaCalendar
		if orm.AsBool(p["business_hours_only"]) {
			cal = loadCalendar(ctx, intOrZero(p["calendar_id"]))
		}
		return p, cal
	}
	return nil, nil
}

func calendarFor(ctx context.Context, pol map[string]interface{}) *slaCalendar {
	if pol == nil || !orm.AsBool(pol["business_hours_only"]) {
		return nil
	}
	return loadCalendar(ctx, intOrZero(pol["calendar_id"]))
}

func loadPolicy(ctx context.Context, id int) map[string]interface{} {
	if id <= 0 {
		return nil
	}
	rec, err := orm.SearchOne(ctx, "helpdesk.sla.policy", map[string]interface{}{"id": id})
	if err != nil {
		return nil
	}
	return rec
}

func logEvent(ctx context.Context, ticketID, timerID int, event SLAEventType, message string, authorID int) {
	if ticketID <= 0 {
		return
	}
	inst, ok := orm.Registry["helpdesk.sla.event"]
	if !ok {
		return
	}
	vals := map[string]interface{}{
		"ticket_id": ticketID,
		"event":     string(event),
		"message":   message,
		"date":      fmtDT(time.Now().UTC()),
	}
	if timerID > 0 {
		vals["timer_id"] = timerID
	}
	if authorID > 0 {
		vals["author_id"] = authorID
	}
	_, _ = orm.Create(ctx, inst, vals)
}

// ---------------------------------------------------------------------------
// Generic data helpers
// ---------------------------------------------------------------------------

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

func reload(ctx context.Context, model string, id int) map[string]interface{} {
	if id <= 0 {
		return nil
	}
	rec, err := orm.SearchOne(ctx, model, map[string]interface{}{"id": id})
	if err != nil {
		return nil
	}
	return rec
}
