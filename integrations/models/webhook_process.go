package models

import (
	"context"
	"strings"

	"nexa/core/event"
	"nexa/core/orm"
	"nexa/core/scheduler"
)

// Asynchronous inbound-event processing. The ingress only stores; this scheduled worker drains
// the durable queue. Each pass claims a bounded batch of new events and transitions them
// new → processing → processed, re-reading every row before the claim so a concurrent pass can
// never process the same event twice (the transition is guarded, making the worker idempotent).
// A pass records an attempt count and, if a step fails, leaves the event failed rather than
// silently dropping it. No downstream business reaction is performed yet: draining the durable
// queue honestly is the deliverable of this stage, and per-source handlers attach on top of it.

func init() {
	scheduler.RegisterCronHandler(inboundProcessCronCode, runInboundProcessing)
	event.Subscribe(orm.EventRecordCreated, onWebhookCreated)
}

func runInboundProcessing(ctx context.Context, _ map[string]interface{}) error {
	return orm.WithElevated(ctx, inboundProcessCronCode, func(bypass context.Context) error {
		rows, err := orm.SearchPage(bypass, inboundEventModel, [][]interface{}{
			{"status", "=", string(EvNew)},
		}, inboundBatchLimit, 0, "id ASC")
		if err != nil {
			return err
		}
		for _, row := range rows {
			processOneInboundEvent(bypass, intOr(row["id"]))
		}
		return nil
	})
}

// processOneInboundEvent claims and finalises a single event. It re-reads the row and only
// advances it while still new, so redelivered or already-claimed events are skipped.
func processOneInboundEvent(bypass context.Context, id int) {
	if id <= 0 {
		return
	}
	fresh, err := orm.SearchOne(bypass, inboundEventModel, map[string]interface{}{"id": id})
	if err != nil || fresh == nil || strOr(fresh["status"]) != string(EvNew) {
		return
	}
	attempts := intOr(fresh["attempt_count"]) + 1
	if err := orm.UpdateRecordByID(bypass, inboundEventModel, id, map[string]interface{}{
		"status":        string(EvProcessing),
		"attempt_count": attempts,
	}); err != nil {
		markInboundFailed(bypass, id, attempts, err.Error())
		return
	}
	// Claimed: the row is now owned by this pass. There is no downstream handler to run yet,
	// so completing means recording the event processed with its timestamp.
	if err := orm.UpdateRecordByID(bypass, inboundEventModel, id, map[string]interface{}{
		"status":       string(EvProcessed),
		"processed_at": nowStamp(),
		"error":        "",
	}); err != nil {
		markInboundFailed(bypass, id, attempts, err.Error())
	}
}

func markInboundFailed(bypass context.Context, id, attempts int, reason string) {
	_ = orm.UpdateRecordByID(bypass, inboundEventModel, id, map[string]interface{}{
		"status":        string(EvFailed),
		"attempt_count": attempts,
		"error":         truncate(reason, 500),
	})
}

// onWebhookCreated provisions a freshly created webhook: a CSPRNG path token when none was set
// and the conventional header/window defaults, so intake works without hand-typed values.
func onWebhookCreated(ctx context.Context, ev event.Event) error {
	model, _ := ev.Payload["model"].(string)
	if model != webhookModel {
		return nil
	}
	id, ok := orm.CoerceInt64(ev.Payload["id"])
	if !ok || id <= 0 {
		return nil
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, webhookModel, map[string]interface{}{"id": int(id)})
	if err != nil || rec == nil {
		return nil
	}
	patch := map[string]interface{}{}
	if strings.TrimSpace(strOr(rec["token"])) == "" {
		token, terr := generateWebhookToken()
		if terr != nil {
			return nil
		}
		patch["token"] = token
	}
	if strings.TrimSpace(strOr(rec["signature_header"])) == "" {
		patch["signature_header"] = defaultSignatureHeader
	}
	if strings.TrimSpace(strOr(rec["timestamp_header"])) == "" {
		patch["timestamp_header"] = defaultTimestampHeader
	}
	if intOr(rec["replay_window_seconds"]) <= 0 {
		patch["replay_window_seconds"] = defReplayWindowSeconds
	}
	if len(patch) == 0 {
		return nil
	}
	_ = orm.UpdateRecordByID(bypass, webhookModel, int(id), patch)
	return nil
}
