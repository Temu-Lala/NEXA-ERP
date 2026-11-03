package models

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"nexa/core/orm"
)

// Behaviour layer for the Lending - Collections companion. It adds live aggregates
// over the delinquency snapshot and the collection case activity lines, a guarded
// lifecycle state machine on collection cases (contact -> promise -> arranged ->
// resolved / escalated -> closed, with reopen), and keep/break decisions on
// promises to pay. Escalation and closure are Lending Manager operations.

func init() {
	// Live aggregates (non-stored, recomputed on read).
	orm.RegisterCompute("loan.delinquency.snapshot", "overdue_total", nil, computeSnapshotOverdueTotal)
	orm.RegisterCompute("loan.collection.case", "action_count", nil, computeCaseActionCount)
	orm.RegisterCompute("loan.collection.case", "promise_count", nil, computeCasePromiseCount)

	// Collection case lifecycle.
	orm.RegisterObjectAction("loan.collection.case", "action_case_contact", caseTransition("mark as contacted", CollectionStateContacted, CollectionStateOpen))
	orm.RegisterObjectAction("loan.collection.case", "action_case_promise", caseTransition("record a promise on", CollectionStatePromise, CollectionStateOpen, CollectionStateContacted))
	orm.RegisterObjectAction("loan.collection.case", "action_case_arrange", caseTransition("arrange a settlement on", CollectionStateArranged, CollectionStateOpen, CollectionStateContacted, CollectionStatePromise))
	orm.RegisterObjectAction("loan.collection.case", "action_case_resolve", caseTransition("resolve", CollectionStateResolved, CollectionStateOpen, CollectionStateContacted, CollectionStatePromise, CollectionStateArranged, CollectionStateEscalated))
	orm.RegisterObjectAction("loan.collection.case", "action_case_escalate", caseTransitionManager("escalate", CollectionStateEscalated, CollectionStateOpen, CollectionStateContacted, CollectionStatePromise, CollectionStateArranged))
	orm.RegisterObjectAction("loan.collection.case", "action_case_close", caseTransitionManager("close", CollectionStateClosed, CollectionStateOpen, CollectionStateContacted, CollectionStatePromise, CollectionStateArranged, CollectionStateResolved, CollectionStateEscalated))
	orm.RegisterObjectAction("loan.collection.case", "action_case_reopen", caseTransition("reopen", CollectionStateOpen, CollectionStateClosed, CollectionStateResolved, CollectionStateEscalated))

	// Promise to pay decisions.
	orm.RegisterObjectAction("loan.promise.to.pay", "action_promise_keep", promiseDecide("keep", PromiseStatusKept))
	orm.RegisterObjectAction("loan.promise.to.pay", "action_promise_break", promiseDecide("mark broken", PromiseStatusBroken))
}

// ---------------------------------------------------------------------------
// Computed fields
// ---------------------------------------------------------------------------

func computeSnapshotOverdueTotal(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return round2(numericOf(rec["overdue_principal"]) + numericOf(rec["overdue_interest"])), nil
}

func computeCaseActionCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return countLinked(ctx, "loan.collection.action", rec["id"])
}

func computeCasePromiseCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return countLinked(ctx, "loan.promise.to.pay", rec["id"])
}

func countLinked(ctx context.Context, model string, caseID interface{}) (interface{}, error) {
	cid, ok := orm.CoerceInt64(caseID)
	if !ok || cid <= 0 {
		return 0, nil
	}
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), model,
		[][]interface{}{{"case_id", "=", int(cid)}})
	if err != nil {
		return nil, err
	}
	return n, nil
}

// ---------------------------------------------------------------------------
// Collection case lifecycle
// ---------------------------------------------------------------------------

// caseTransition builds a guarded state transition: the current state must be one
// of the allowed sources, the actor needs write access, and the case is stamped
// with the open/close dates as appropriate.
func caseTransition(verb string, to CollectionState, from ...CollectionState) func(context.Context, string, int, map[string]string) (string, error) {
	return caseTransitionGated(false, verb, to, from...)
}

// caseTransitionManager is a caseTransition restricted to Lending Managers.
func caseTransitionManager(verb string, to CollectionState, from ...CollectionState) func(context.Context, string, int, map[string]string) (string, error) {
	return caseTransitionGated(true, verb, to, from...)
}

func caseTransitionGated(managerOnly bool, verb string, to CollectionState, from ...CollectionState) func(context.Context, string, int, map[string]string) (string, error) {
	return func(ctx context.Context, model string, id int, _ map[string]string) (string, error) {
		uid := orm.SecurityUID(ctx)
		if err := orm.CheckModelAccess(ctx, uid, model, "write"); err != nil {
			return "", err
		}
		if managerOnly && !orm.UserHasGroupXML(ctx, uid, "lending.group_lending_manager") {
			return "", fmt.Errorf("only a Lending Manager can %s a collection case", verb)
		}
		bypass := orm.ContextWithBypass(ctx, true)
		rec, err := orm.SearchOne(bypass, model, map[string]interface{}{"id": id})
		if err != nil {
			return "", fmt.Errorf("collection case not found")
		}
		current := CollectionState(orm.AsString(rec["state"]))
		allowed := false
		for _, s := range from {
			if current == s {
				allowed = true
				break
			}
		}
		if !allowed {
			return "", fmt.Errorf("cannot %s a case in state %q", verb, current)
		}
		upd := map[string]interface{}{"state": string(to)}
		switch to {
		case CollectionStateClosed:
			upd["close_date"] = nowDate()
		case CollectionStateOpen:
			upd["close_date"] = ""
		}
		if orm.AsString(rec["opened_date"]) == "" {
			upd["opened_date"] = nowDate()
		}
		if err := orm.UpdateRecordByID(bypass, model, id, upd); err != nil {
			return "", err
		}
		return "", nil
	}
}

// ---------------------------------------------------------------------------
// Promise to pay decisions
// ---------------------------------------------------------------------------

// promiseDecide records a keep/break outcome on a pending promise to pay.
func promiseDecide(verb string, to PromiseStatus) func(context.Context, string, int, map[string]string) (string, error) {
	return func(ctx context.Context, model string, id int, _ map[string]string) (string, error) {
		uid := orm.SecurityUID(ctx)
		if err := orm.CheckModelAccess(ctx, uid, model, "write"); err != nil {
			return "", err
		}
		bypass := orm.ContextWithBypass(ctx, true)
		rec, err := orm.SearchOne(bypass, model, map[string]interface{}{"id": id})
		if err != nil {
			return "", fmt.Errorf("promise to pay not found")
		}
		if PromiseStatus(orm.AsString(rec["status"])) != PromiseStatusPending {
			return "", fmt.Errorf("cannot %s a promise that is not pending", verb)
		}
		if err := orm.UpdateRecordByID(bypass, model, id, map[string]interface{}{"status": string(to)}); err != nil {
			return "", err
		}
		if cid := intOr(rec["case_id"], 0); cid > 0 {
			return fmt.Sprintf("/web?action=lending_collections.action_loan_collection_case&view_type=form&id=%d", cid), nil
		}
		return "", nil
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func nowDate() string {
	return time.Now().UTC().Format("2006-01-02")
}

func round2(f float64) float64 {
	v, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", f), 64)
	return v
}

func intOr(v interface{}, def int) int {
	n, ok := orm.CoerceInt64(v)
	if !ok {
		return def
	}
	return int(n)
}

func numericOf(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int64:
		return float64(t)
	case int:
		return float64(t)
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f
	default:
		return 0
	}
}
