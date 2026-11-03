package models

import (
	"context"
	"fmt"
	"math"
	"time"

	"nexa/core/orm"
	"nexa/core/scheduler"
)

// Behaviour layer for school communication. The retry rules that govern notification
// delivery are pure and unit-tested: a pending notification is always eligible, a failed
// one only until its attempt counter reaches the maximum, and a re-try is deferred by an
// exponential backoff (base minutes doubling per attempt, capped at a day). The computes
// project an announcement's recipient and delivered counts and a compact "delivered/total"
// status string. A daily cron dispatches due notifications: pending and eligible failed
// deliveries are marked sent (best-effort, clearing any prior error) with their attempt
// counter advanced. Manual actions publish and archive announcements, cancel a queued
// delivery and re-queue a failed one for another attempt.

const (
	channelModel      = "school.communication.channel"
	announcementModel = "school.announcement"
	notificationModel = "school.notification"
	messageModel      = "school.message"
	studentModel      = "school.student"

	dateLayout       = "2006-01-02"
	dispatchCronCode = "school.comm.dispatch"

	defaultBaseBackoffMinutes = 30
	maxBackoffMinutes         = 1440
	defaultMaxAttempts        = 3
)

// shouldAttempt reports whether a delivery in the given state should be (re)attempted.
// Pending is always eligible; a failed delivery is eligible only while it has attempts
// left; sent and cancelled are terminal.
func shouldAttempt(state string, attempts, max int) bool {
	switch state {
	case string(NotifyPending):
		return true
	case string(NotifyFailed):
		if max <= 0 {
			max = defaultMaxAttempts
		}
		return attempts < max
	default:
		return false
	}
}

// backoffMinutes is the exponential retry delay after an attempt: base * 2^attempts,
// clamped to a day so a pathological counter never schedules absurdly far out.
func backoffMinutes(attempts, base int) int {
	if base <= 0 {
		base = defaultBaseBackoffMinutes
	}
	if attempts < 0 {
		attempts = 0
	}
	d := float64(base) * math.Pow(2, float64(attempts))
	if d > float64(maxBackoffMinutes) {
		return maxBackoffMinutes
	}
	return int(math.Round(d))
}

// ---------------------------------------------------------------------------
// Computes
// ---------------------------------------------------------------------------

func init() {
	orm.RegisterCompute(announcementModel, "recipient_count", []string{"id"}, computeAnnounceRecipientCount)
	orm.RegisterCompute(announcementModel, "delivered_count", []string{"id"}, computeAnnounceDeliveredCount)
	orm.RegisterCompute(announcementModel, "delivery_status", []string{"id"}, computeAnnounceDeliveryStatus)
	scheduler.RegisterCronHandler(dispatchCronCode, runNotificationDispatch)
}

func computeAnnounceRecipientCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), notificationModel, [][]interface{}{{"announcement_id", "=", intOr(rec["id"])}})
	if err != nil {
		return 0, nil
	}
	return n, nil
}

func computeAnnounceDeliveredCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), notificationModel, [][]interface{}{
		{"announcement_id", "=", intOr(rec["id"])},
		{"state", "=", string(NotifySent)},
	})
	if err != nil {
		return 0, nil
	}
	return n, nil
}

func computeAnnounceDeliveryStatus(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	id := intOr(rec["id"])
	total, _ := orm.SearchCount(bypass, notificationModel, [][]interface{}{{"announcement_id", "=", id}})
	sent, _ := orm.SearchCount(bypass, notificationModel, [][]interface{}{
		{"announcement_id", "=", id},
		{"state", "=", string(NotifySent)},
	})
	if total == 0 {
		return "No recipients", nil
	}
	return fmt.Sprintf("%d/%d delivered", sent, total), nil
}

// runNotificationDispatch is the daily delivery worker. It selects pending and eligible
// failed notifications and marks them sent (best-effort), advancing the attempt counter
// and clearing any stored error.
func runNotificationDispatch(ctx context.Context, _ map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	rows, err := orm.Search(bypass, notificationModel, [][]interface{}{
		{"state", "in", []interface{}{string(NotifyPending), string(NotifyFailed)}},
	})
	if err != nil {
		return err
	}
	today := time.Now().Format(dateLayout)
	for _, r := range rows {
		state := strOr(r["state"])
		attempts := intOr(r["attempts"])
		max := intOr(r["max_attempts"])
		if !shouldAttempt(state, attempts, max) {
			continue
		}
		// A failed delivery is only retried once its scheduled retry date has arrived.
		if state == string(NotifyFailed) {
			nr := strOr(r["next_retry_at"])
			if nr != "" && nr > today {
				continue
			}
		}
		_ = orm.UpdateRecordByID(bypass, notificationModel, intOr(r["id"]), map[string]interface{}{
			"state":      string(NotifySent),
			"attempts":   attempts + 1,
			"sent_at":    today,
			"last_error": "",
		})
	}
	return nil
}

// ---------------------------------------------------------------------------
// Shared coercion helpers (ORM values arrive as interface{})
// ---------------------------------------------------------------------------

func intOr(v interface{}) int {
	n, ok := orm.CoerceInt64(v)
	if !ok {
		return 0
	}
	return int(n)
}

func strOr(v interface{}) string { return orm.AsString(v) }

func boolOr(v interface{}) bool { return orm.AsBool(v) }

func floatOr(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	}
	return 0
}
