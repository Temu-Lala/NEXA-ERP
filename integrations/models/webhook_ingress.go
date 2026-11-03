package models

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"nexa/core/orm"
	"nexa/core/server/router"
)

// Inbound webhook ingress. A single public route (POST /webhook/<token>) receives deliveries on
// behalf of a governed integration.webhook. The handler does NO business work: it authenticates
// the sender by HMAC signature over the raw body, applies a timestamp replay window, deduplicates
// by idempotency key, then durably persists an integration.inbound.event row and returns. A
// delivery is only acknowledged once it is safely stored, so nothing is lost if a worker later
// fails; malformed, forged, stale or oversized deliveries are recorded truthfully as rejected and
// never enter the processing queue. The path token is the only secret in the URL; the signing
// secret itself lives in the environment and is resolved by reference at check time.

func init() {
	router.Register(http.MethodPost, ingressPathPrefix, router.AuthNone, webhookIngressHandler)
}

func webhookIngressHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	bypass := orm.ContextWithBypass(r.Context(), true)

	token := strings.Trim(strings.TrimPrefix(r.URL.Path, ingressPathPrefix), "/")
	if token == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	hook, err := orm.SearchOne(bypass, webhookModel, map[string]interface{}{"token": token})
	if err != nil || hook == nil {
		// Unknown token: store nothing (an unauthenticated probe must not fill the queue).
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	hookID := intOr(hook["id"])
	if !webhookAccepts(strOr(hook["state"]), boolOr(hook["active"])) {
		http.Error(w, "webhook intake is not active", http.StatusServiceUnavailable)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, int64(maxInboundBodyBytes)+1))
	if err != nil {
		writeIngressJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "unreadable body"})
		return
	}
	if len(body) > maxInboundBodyBytes {
		persistInbound(bypass, hookID, body, false, "", r, string(EvRejected), "payload exceeds the size limit")
		writeIngressJSON(w, http.StatusRequestEntityTooLarge, map[string]interface{}{"error": "payload too large"})
		return
	}

	sigHeaderName := headerOrDefault(strOr(hook["signature_header"]), defaultSignatureHeader)
	tsHeaderName := headerOrDefault(strOr(hook["timestamp_header"]), defaultTimestampHeader)
	providedSig := r.Header.Get(sigHeaderName)
	providedTS := r.Header.Get(tsHeaderName)
	nonce := r.Header.Get(defaultNonceHeader)
	externalID := r.Header.Get(defaultExternalIDHeader)
	sigOK := false

	if boolOr(hook["require_signature"]) {
		secret, ok := resolveSecret(strOr(hook["secret_ref"]))
		if !ok {
			// Cannot verify without the secret: refuse rather than accept an unauthenticated write.
			persistInbound(bypass, hookID, body, false, dedupKey(externalID, nonce, ""), r, string(EvRejected),
				"signing secret reference is not resolvable from the environment")
			writeIngressJSON(w, http.StatusForbidden, map[string]interface{}{"error": "signature verification unavailable"})
			return
		}
		sigOK = verifyInboundSignature(secret, body, providedSig)
		if !sigOK {
			persistInbound(bypass, hookID, body, false, dedupKey(externalID, nonce, ""), r, string(EvRejected), "invalid signature")
			writeIngressJSON(w, http.StatusUnauthorized, map[string]interface{}{"error": "invalid signature"})
			return
		}
		window := clampReplayWindow(intOr(hook["replay_window_seconds"]))
		if !timestampWithinWindow(providedTS, window, time.Now().UTC()) {
			persistInbound(bypass, hookID, body, true, dedupKey(externalID, nonce, ""), r, string(EvRejected), "timestamp outside the replay window")
			writeIngressJSON(w, http.StatusRequestTimeout, map[string]interface{}{"error": "stale or missing timestamp"})
			return
		}
	}

	key := dedupKey(externalID, nonce, inboundContentHash(body))
	if key != "" {
		if existing, _ := orm.SearchOne(bypass, inboundEventModel, map[string]interface{}{
			"webhook_id": hookID,
			"event_key":  key,
		}); existing != nil {
			// Idempotent replay of an already-stored event: acknowledge without storing twice.
			writeIngressJSON(w, http.StatusAccepted, map[string]interface{}{
				"status": "duplicate", "event_id": intOr(existing["id"]),
			})
			return
		}
	}

	eventID := persistInbound(bypass, hookID, body, sigOK, key, r, string(EvNew), "")
	if eventID <= 0 {
		writeIngressJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": "could not store event"})
		return
	}
	_ = orm.UpdateRecordByID(bypass, webhookModel, hookID, map[string]interface{}{"last_received": nowStamp()})
	writeIngressJSON(w, http.StatusAccepted, map[string]interface{}{"status": "accepted", "event_id": eventID})
}

// persistInbound durably writes one integration.inbound.event row and returns its id (0 on
// failure). Only a bounded excerpt and derived metadata are stored — never the whole payload.
func persistInbound(bypass context.Context, hookID int, body []byte, sigOK bool, key string, r *http.Request, status, reason string) int {
	vals := map[string]interface{}{
		"webhook_id":      hookID,
		"received_at":     nowStamp(),
		"status":          status,
		"signature_ok":    sigOK,
		"nonce":           truncate(r.Header.Get(defaultNonceHeader), 120),
		"source_ip":       remoteIP(r),
		"content_type":    truncate(r.Header.Get("Content-Type"), 80),
		"body_bytes":      len(body),
		"payload_excerpt": truncate(string(body), maxInboundExcerpt),
		"attempt_count":   0,
	}
	if key != "" {
		vals["event_key"] = truncate(key, 160)
	}
	if reason != "" {
		vals["error"] = reason
	}
	id, err := orm.Create(bypass, orm.RegistryModel(inboundEventModel), vals)
	if err != nil {
		return 0
	}
	return id
}

func remoteIP(r *http.Request) string {
	if fwd := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); fwd != "" {
		return truncate(fwd, 64)
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		host = strings.TrimSpace(r.RemoteAddr)
	}
	return truncate(host, 64)
}

func writeIngressJSON(w http.ResponseWriter, code int, payload map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}
