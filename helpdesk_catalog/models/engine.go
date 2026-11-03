package models

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"nexa/core/event"
	"nexa/core/orm"
)

// Behaviour layer for the service-catalog companion. A service request is treated
// as a governed business process: it advances through a state machine, is gated by
// approval steps and completed only when its required fulfilment tasks are done.
// The pure decision functions below are unit-tested without a database, mirroring
// the helpdesk_sla / helpdesk_channel engines.

const (
	datetimeLayout = "2006-01-02 15:04:05"
	dateLayout     = "2006-01-02"
	requestSeqKey  = "helpdesk.service.request"
)

func init() {
	orm.RegisterCompute("helpdesk.catalog.item", "field_count", nil, computeItemFieldCount)
	orm.RegisterCompute("helpdesk.service.request", "days_open", nil, computeRequestDaysOpen)
	orm.RegisterCompute("helpdesk.service.request", "task_progress_pct", nil, computeRequestTaskProgress)
	orm.RegisterCompute("helpdesk.service.request", "can_complete", nil, computeRequestCanComplete)
	orm.RegisterCompute("helpdesk.fulfillment.task", "is_ready", nil, computeTaskIsReady)

	event.Subscribe(orm.EventRecordCreated, onServiceRequestCreated)
}

// ---------------------------------------------------------------------------
// Pure decision functions (unit-tested)
// ---------------------------------------------------------------------------

// requestAllowedTransition maps a current state + action to the next state,
// enforcing the governed lifecycle. Returns the target state and whether the move
// is legal. Whether an approval gate exists is supplied by the caller (approvalRequired).
func requestAllowedTransition(state RequestState, action string, approvalRequired bool) (RequestState, bool) {
	switch action {
	case "submit":
		if state != ReqDraft {
			return "", false
		}
		if approvalRequired {
			return ReqInApproval, true
		}
		return ReqFulfilling, true
	case "approve":
		if state != ReqInApproval {
			return "", false
		}
		return ReqFulfilling, true
	case "reject":
		if state != ReqInApproval {
			return "", false
		}
		return ReqRejected, true
	case "complete":
		if state != ReqFulfilling {
			return "", false
		}
		return ReqCompleted, true
	case "cancel":
		switch state {
		case ReqDraft, ReqSubmitted, ReqInApproval, ReqFulfilling:
			return ReqCancelled, true
		}
		return "", false
	default:
		return "", false
	}
}

// taskIsReady reports whether a pending task is actionable, i.e. its optional
// dependency (if any) is already done or skipped.
func taskIsReady(state, depState string, hasDep bool) bool {
	if state != string(TaskPending) {
		return false
	}
	if !hasDep {
		return true
	}
	return depState == string(TaskDone) || depState == string(TaskSkipped)
}

// requiredTasksSatisfied reports whether every required task is done or skipped.
// tasks is a slice of {required, state} string pairs.
func requiredTasksSatisfied(tasks [][2]string) bool {
	if len(tasks) == 0 {
		return false
	}
	for _, t := range tasks {
		required := t[0] == "t" || t[0] == "true" || t[0] == "1"
		if required && t[1] != string(TaskDone) && t[1] != string(TaskSkipped) {
			return false
		}
	}
	return true
}

// approvalsResolved reports whether the approval gate is fully satisfied: at least
// one step exists and none are pending. Any rejected step rejects the request.
func approvalsResolved(approvalStates []string) (approved bool, rejected bool) {
	if len(approvalStates) == 0 {
		return true, false
	}
	for _, s := range approvalStates {
		switch s {
		case string(ApprovalRejected):
			return false, true
		case string(ApprovalPending):
			return false, false
		}
	}
	return true, false
}

// validateFieldValue checks a captured value against a field definition. Returns a
// non-empty message when the value is invalid. Required, select-membership and
// numeric parsing are enforced; other types accept any text.
func validateFieldValue(fieldType, required, value, options string) string {
	value = strings.TrimSpace(value)
	if (required == "t" || required == "true" || required == "1") && value == "" {
		return "value is required"
	}
	if value == "" {
		return ""
	}
	switch FieldType(fieldType) {
	case FieldInteger:
		if _, err := strconv.ParseInt(value, 10, 64); err != nil {
			return "value must be a whole number"
		}
	case FieldFloat:
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return "value must be a number"
		}
	case FieldBoolean:
		switch strings.ToLower(value) {
		case "true", "false", "1", "0", "yes", "no":
		default:
			return "value must be true or false"
		}
	case FieldSelect:
		opts := splitOptions(options)
		if len(opts) > 0 && !containsFold(opts, value) {
			return "value must be one of the allowed options"
		}
	}
	return ""
}

func splitOptions(options string) []string {
	parts := strings.Split(options, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func containsFold(opts []string, v string) bool {
	for _, o := range opts {
		if strings.EqualFold(o, v) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Computed fields
// ---------------------------------------------------------------------------

func computeItemFieldCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return countLinkedBy(ctx, "helpdesk.catalog.field", "item_id", rec["id"])
}

func computeRequestDaysOpen(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	submitted := strings.TrimSpace(orm.AsString(rec["submitted_at"]))
	var start time.Time
	if t, ok := parseDT(submitted); ok {
		start = t
	} else if c, ok := parseDT(orm.AsString(rec["create_date"])); ok {
		start = c
	} else {
		return 0, nil
	}
	end := time.Now().UTC()
	if done := strings.TrimSpace(orm.AsString(rec["completed_at"])); done != "" {
		if t, ok := parseDT(done); ok {
			end = t
		}
	}
	d := int(end.Sub(start).Hours() / 24)
	if d < 0 {
		d = 0
	}
	return d, nil
}

func computeRequestTaskProgress(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	tasks := fetchTasks(ctx, rec["id"])
	if len(tasks) == 0 {
		return 0, nil
	}
	done := 0
	for _, t := range tasks {
		st := orm.AsString(t["state"])
		if st == string(TaskDone) || st == string(TaskSkipped) {
			done++
		}
	}
	return done * 100 / len(tasks), nil
}

func computeRequestCanComplete(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	if orm.AsString(rec["state"]) != string(ReqFulfilling) {
		return false, nil
	}
	tasks := fetchTasks(ctx, rec["id"])
	pairs := make([][2]string, 0, len(tasks))
	for _, t := range tasks {
		pairs = append(pairs, [2]string{orm.AsString(t["required"]), orm.AsString(t["state"])})
	}
	return requiredTasksSatisfied(pairs), nil
}

func computeTaskIsReady(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	depID, ok := orm.CoerceInt64(rec["depends_on_id"])
	hasDep := ok && depID > 0
	depState := ""
	if hasDep {
		if dep, err := orm.SearchOne(orm.ContextWithBypass(ctx, true), "helpdesk.fulfillment.task",
			map[string]interface{}{"id": int(depID)}); err == nil && dep != nil {
			depState = orm.AsString(dep["state"])
		}
	}
	return taskIsReady(orm.AsString(rec["state"]), depState, hasDep), nil
}

// ---------------------------------------------------------------------------
// Service request numbering
// ---------------------------------------------------------------------------

func onServiceRequestCreated(ctx context.Context, ev event.Event) error {
	model, _ := ev.Payload["model"].(string)
	if model != requestSeqKey {
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
	if strings.TrimSpace(orm.AsString(rec["number"])) != "" {
		return nil
	}
	num := fmt.Sprintf("SR-%05d", id)
	_ = orm.UpdateRecordByID(bypass, model, int(id), map[string]interface{}{"number": num})
	return nil
}

// ---------------------------------------------------------------------------
// Helpers (package-local)
// ---------------------------------------------------------------------------

func fetchTasks(ctx context.Context, requestID interface{}) []map[string]interface{} {
	rid, ok := orm.CoerceInt64(requestID)
	if !ok || rid <= 0 {
		return nil
	}
	rows, err := orm.Search(orm.ContextWithBypass(ctx, true), "helpdesk.fulfillment.task",
		[][]interface{}{{"request_id", "=", int(rid)}})
	if err != nil {
		return nil
	}
	return rows
}

func countLinkedBy(ctx context.Context, model, field string, id interface{}) (interface{}, error) {
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
	n, _ := strconv.Atoi(s)
	return n
}

func parseDT(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{datetimeLayout, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

func nowStamp() string { return time.Now().UTC().Format(datetimeLayout) }
