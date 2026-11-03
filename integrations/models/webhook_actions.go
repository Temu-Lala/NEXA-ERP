package models

import (
	"context"
	"fmt"

	"nexa/core/orm"
)

// Webhook governance actions. Enabling intake is server-guarded: a webhook may only go active
// once it has a path token and — when a signature is required — a Secret Reference, so a
// misconfigured receiver cannot silently accept unauthenticated writes. Pausing stops intake at
// the gate immediately. Rotating the token issues a fresh CSPRNG value (old URLs stop working).
// Requeue returns a settled event to the queue for the worker to re-drain. All four are gated to
// administrators; the signing secret is never stored or returned, only referenced.

func init() {
	orm.RegisterObjectAction(webhookModel, "webhook_enable", actWebhookEnable)
	orm.RegisterObjectAction(webhookModel, "webhook_pause", actWebhookPause)
	orm.RegisterObjectAction(webhookModel, "webhook_rotate_token", actWebhookRotateToken)
	orm.RegisterObjectAction(inboundEventModel, "inbound_event_requeue", actInboundEventRequeue)
}

func actWebhookEnable(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	if err := requireAdminErr(ctx, "enable an inbound webhook"); err != nil {
		return "", err
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, webhookModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["token"]) == "" {
		return "", fmt.Errorf("cannot enable: the webhook has no path token yet (save and reload it first)")
	}
	if boolOr(rec["require_signature"]) && strOr(rec["secret_ref"]) == "" {
		return "", fmt.Errorf("cannot enable: a signature is required but no Secret Reference is set")
	}
	if err := orm.UpdateRecordByID(bypass, webhookModel, id, map[string]interface{}{
		"state": string(StActive), "active": true,
	}); err != nil {
		return "", err
	}
	return "Webhook intake enabled", nil
}

func actWebhookPause(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	if err := requireAdminErr(ctx, "pause an inbound webhook"); err != nil {
		return "", err
	}
	bypass := orm.ContextWithBypass(ctx, true)
	if _, err := getRecord(bypass, webhookModel, id); err != nil {
		return "", err
	}
	if err := orm.UpdateRecordByID(bypass, webhookModel, id, map[string]interface{}{"state": string(StPaused)}); err != nil {
		return "", err
	}
	return "Webhook intake paused", nil
}

func actWebhookRotateToken(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	if err := requireAdminErr(ctx, "rotate an inbound webhook token"); err != nil {
		return "", err
	}
	bypass := orm.ContextWithBypass(ctx, true)
	if _, err := getRecord(bypass, webhookModel, id); err != nil {
		return "", err
	}
	token, err := generateWebhookToken()
	if err != nil {
		return "", fmt.Errorf("could not generate a token: %w", err)
	}
	if err := orm.UpdateRecordByID(bypass, webhookModel, id, map[string]interface{}{"token": token}); err != nil {
		return "", err
	}
	return "Ingress URL rotated — update the sender's configured address", nil
}

func actInboundEventRequeue(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	if err := requireAdminErr(ctx, "replay an inbound event"); err != nil {
		return "", err
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, inboundEventModel, id)
	if err != nil {
		return "", err
	}
	switch InboundEventStatus(strOr(rec["status"])) {
	case EvNew, EvProcessing:
		return "", fmt.Errorf("event is already queued")
	}
	if err := orm.UpdateRecordByID(bypass, inboundEventModel, id, map[string]interface{}{
		"status": string(EvNew), "processed_at": "", "error": "",
	}); err != nil {
		return "", err
	}
	return "Event requeued for processing", nil
}
