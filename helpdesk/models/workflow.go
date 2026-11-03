package models

import (
	"context"
	"fmt"
	"time"

	"nexa/core/event"
	"nexa/core/orm"
)

// Behaviour layer for the Helpdesk ticket workspace. It adds live aggregates over
// the message and history lines, a guarded lifecycle state machine on tickets
// (start / pending / resolve / close / reopen) plus self-assignment, append-only
// audit history, and automatic ticket numbering on creation. The ticket record
// stays the single source of truth; history and links only add governance around
// it, mirroring the crm_lifecycle pattern.

func init() {
	// Non-stored live aggregates (recomputed on read).
	orm.RegisterCompute("helpdesk.ticket", "message_count", nil, computeTicketMessageCount)
	orm.RegisterCompute("helpdesk.ticket", "history_count", nil, computeTicketHistoryCount)

	// Lifecycle transitions (each records an audit entry). RunObjectAction already
	// enforces write access and record existence before the handler runs.
	orm.RegisterObjectAction("helpdesk.ticket", "action_start_progress",
		ticketTransition("start progress on", TicketStateInProgress, TicketStateNew, TicketStatePending, TicketStateOpen))
	orm.RegisterObjectAction("helpdesk.ticket", "action_pending",
		ticketTransition("set pending", TicketStatePending, TicketStateNew, TicketStateOpen, TicketStateInProgress))
	orm.RegisterObjectAction("helpdesk.ticket", "action_resolve",
		ticketTransition("resolve", TicketStateResolved, TicketStateNew, TicketStateOpen, TicketStateInProgress, TicketStatePending))
	orm.RegisterObjectAction("helpdesk.ticket", "action_close",
		ticketTransition("close", TicketStateClosed, TicketStateResolved, TicketStateCancelled))
	orm.RegisterObjectAction("helpdesk.ticket", "action_cancel",
		ticketTransition("cancel", TicketStateCancelled, TicketStateNew, TicketStateOpen, TicketStateInProgress, TicketStatePending))
	orm.RegisterObjectAction("helpdesk.ticket", "action_reopen", ticketReopen)
	orm.RegisterObjectAction("helpdesk.ticket", "action_assign_self", ticketAssignSelf)

	// Automatic numbering + creation audit on insert.
	event.Subscribe(orm.EventRecordCreated, onTicketCreated)
}

// ---------------------------------------------------------------------------
// Computed fields
// ---------------------------------------------------------------------------

func computeTicketMessageCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return countLinked(ctx, "helpdesk.ticket.message", "ticket_id", rec["id"])
}

func computeTicketHistoryCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return countLinked(ctx, "helpdesk.ticket.history", "ticket_id", rec["id"])
}

func countLinked(ctx context.Context, model, field string, ticketID interface{}) (interface{}, error) {
	tid, ok := orm.CoerceInt64(ticketID)
	if !ok || tid <= 0 {
		return 0, nil
	}
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), model,
		[][]interface{}{{field, "=", int(tid)}})
	if err != nil {
		return nil, err
	}
	return n, nil
}

// ---------------------------------------------------------------------------
// Lifecycle transitions
// ---------------------------------------------------------------------------

// ticketTransition builds a guarded state change: the current state must be one of
// the allowed sources; the move is stamped (first response / close dates) and an
// append-only history row is written describing the transition.
func ticketTransition(verb string, to TicketState, from ...TicketState) func(context.Context, string, int, map[string]string) (string, error) {
	return func(ctx context.Context, model string, id int, _ map[string]string) (string, error) {
		uid := orm.SecurityUID(ctx)
		bypass := orm.ContextWithBypass(ctx, true)
		rec, err := orm.SearchOne(bypass, model, map[string]interface{}{"id": id})
		if err != nil {
			return "", fmt.Errorf("ticket not found")
		}
		current := TicketState(orm.AsString(rec["state"]))
		allowed := false
		for _, s := range from {
			if current == s {
				allowed = true
				break
			}
		}
		if !allowed {
			return "", fmt.Errorf("cannot %s a ticket in state %q", verb, current)
		}
		upd := map[string]interface{}{"state": string(to)}
		switch to {
		case TicketStateClosed:
			upd["closed_date"] = nowDateTime()
		case TicketStateInProgress, TicketStatePending, TicketStateResolved:
			if orm.AsString(rec["first_response_at"]) == "" {
				upd["first_response_at"] = nowDateTime()
			}
		}
		if err := orm.UpdateRecordByID(bypass, model, id, upd); err != nil {
			return "", err
		}
		logHistory(bypass, id, HistoryState, "state", string(current), string(to),
			fmt.Sprintf("Ticket %s.", verb), uid)
		return "", nil
	}
}

// ticketReopen moves a closed/resolved/cancelled ticket back to open, clears the
// close stamp, increments the reopen counter, and records the audit entry.
func ticketReopen(ctx context.Context, model string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, model, map[string]interface{}{"id": id})
	if err != nil {
		return "", fmt.Errorf("ticket not found")
	}
	current := TicketState(orm.AsString(rec["state"]))
	if current != TicketStateClosed && current != TicketStateResolved && current != TicketStateCancelled {
		return "", fmt.Errorf("cannot reopen a ticket in state %q", current)
	}
	reopens := 0
	if n, ok := orm.CoerceInt64(rec["reopened_count"]); ok {
		reopens = int(n)
	}
	if err := orm.UpdateRecordByID(bypass, model, id, map[string]interface{}{
		"state":          string(TicketStateOpen),
		"closed_date":    "",
		"reopened_count": reopens + 1,
	}); err != nil {
		return "", err
	}
	logHistory(bypass, id, HistoryState, "state", string(current), string(TicketStateOpen),
		fmt.Sprintf("Ticket reopened (#%d).", reopens+1), uid)
	return "", nil
}

// ticketAssignSelf sets the current user as the assignee (when unassigned) and
// records the assignment audit entry.
func ticketAssignSelf(ctx context.Context, model string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, model, map[string]interface{}{"id": id})
	if err != nil {
		return "", fmt.Errorf("ticket not found")
	}
	if orm.AsString(rec["assigned_to"]) != "" {
		return "", fmt.Errorf("ticket is already assigned")
	}
	if err := orm.UpdateRecordByID(bypass, model, id, map[string]interface{}{
		"assigned_to": uid,
	}); err != nil {
		return "", err
	}
	name := "you"
	if u, e := orm.SearchOne(bypass, "core.user", map[string]interface{}{"id": uid}); e == nil {
		if n := orm.AsString(u["name"]); n != "" {
			name = n
		}
	}
	logHistory(bypass, id, HistoryAssignment, "assigned_to", "", name,
		"Ticket claimed by assignee.", uid)
	return "", nil
}

// ---------------------------------------------------------------------------
// Creation hook: numbering + creation audit
// ---------------------------------------------------------------------------

func onTicketCreated(ctx context.Context, ev event.Event) error {
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
	if err != nil {
		return nil
	}
	updates := map[string]interface{}{}
	if orm.AsString(rec["number"]) == "" {
		updates["number"] = nextTicketNumber(bypass)
	}
	if len(updates) > 0 {
		if err := orm.UpdateRecordByID(bypass, model, int(id), updates); err != nil {
			return err
		}
	}
	logHistory(bypass, int(id), HistoryCreated, "", "", orm.AsString(updates["number"]),
		"Ticket created.", ev.Actor)
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func nextTicketNumber(ctx context.Context) string {
	if n, err := orm.NextSequence(ctx, "helpdesk.ticket"); err == nil && n != "" {
		return n
	}
	return fmt.Sprintf("HD/%05d", time.Now().UnixNano()%100000)
}

func logHistory(ctx context.Context, ticketID int, action HistoryAction, field, oldVal, newVal, summary string, uid int) {
	inst, ok := orm.Registry["helpdesk.ticket.history"]
	if !ok {
		return
	}
	_, _ = orm.Create(ctx, inst, map[string]interface{}{
		"ticket_id": ticketID,
		"action":    string(action),
		"field":     field,
		"old_value": oldVal,
		"new_value": newVal,
		"summary":   summary,
		"author_id": uid,
		"date":      nowDateTime(),
	})
}

func nowDateTime() string {
	return time.Now().UTC().Format("2006-01-02 15:04:05")
}
