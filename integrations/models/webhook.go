package models

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

// Inbound-webhook gate arithmetic. Everything here is pure and offline so it can be unit-tested
// without a network or a database: the ingress accepts a delivery only when its HMAC-SHA256
// signature matches the raw body under the referenced secret, the supplied timestamp falls
// inside the configured replay window (never a clock-skewed future or a stale past), and the
// most stable identity available forms an idempotency key so a provider retry is recognised as
// the same event rather than stored twice. Bodies are bounded on the way in and only a content
// hash plus a truncated excerpt are ever persisted — the raw payload is not archived whole.

// generateWebhookToken returns a 32-character hex path token from a CSPRNG. The token is the
// only secret embedded in a public URL, so it must be unguessable and is never derived from
// time, identifiers or names.
func generateWebhookToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// verifyInboundSignature reports whether providedSig is a valid HMAC-SHA256 of body under
// secret. It accepts the signature with or without the conventional "sha256=" prefix and
// compares digests in constant time, so neither a malformed header nor a forged signature
// leaks comparison timing. An empty secret or empty header is never a match.
func verifyInboundSignature(secret string, body []byte, providedSig string) bool {
	providedSig = strings.TrimSpace(providedSig)
	if strings.TrimSpace(secret) == "" || providedSig == "" {
		return false
	}
	want := strings.TrimPrefix(signPayload(secret, body), "sha256=")
	got := strings.TrimPrefix(strings.ToLower(providedSig), "sha256=")
	wantBytes, err := hex.DecodeString(want)
	if err != nil {
		return false
	}
	gotBytes, err := hex.DecodeString(got)
	if err != nil {
		return false
	}
	return hmac.Equal(gotBytes, wantBytes)
}

// timestampWithinWindow is the replay guard over a provider-supplied delivery time. Accepted
// forms are epoch seconds or an RFC3339 timestamp; anything else is treated as not fresh. A
// blank timestamp is not fresh when the gate requires one, and clock skew is rejected in both
// directions — a far-future stamp is as suspicious as a stale one.
func timestampWithinWindow(raw string, windowSeconds int, now time.Time) bool {
	if windowSeconds <= 0 {
		windowSeconds = defReplayWindowSeconds
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	epoch, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		t, err2 := time.Parse(time.RFC3339, raw)
		if err2 != nil {
			return false
		}
		epoch = t.Unix()
	}
	skew := now.Sub(time.Unix(epoch, 0))
	if skew < 0 {
		skew = -skew
	}
	return skew <= time.Duration(windowSeconds)*time.Second
}

// dedupKey chooses the most stable identity available for an inbound delivery: an explicit
// external event id, then a per-delivery nonce, then the content hash. Two deliveries that
// reduce to the same key are the same event.
func dedupKey(externalID, nonce, contentHash string) string {
	if s := strings.TrimSpace(externalID); s != "" {
		return "id:" + truncate(s, 150)
	}
	if s := strings.TrimSpace(nonce); s != "" {
		return "n:" + truncate(s, 150)
	}
	if s := strings.TrimSpace(contentHash); s != "" {
		return "h:" + s
	}
	return ""
}

// inboundContentHash returns the "sha256=<hex>" digest of the raw body. It is stored instead
// of the full payload and doubles as the last-resort idempotency key.
func inboundContentHash(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha256=" + hex.EncodeToString(sum[:])
}

// clampReplayWindow bounds a configured replay window into a safe range, defaulting when unset.
func clampReplayWindow(v int) int {
	if v < minReplayWindowSeconds {
		return defReplayWindowSeconds
	}
	if v > maxReplayWindowSeconds {
		return maxReplayWindowSeconds
	}
	return v
}

// webhookAccepts reports whether a webhook should currently take deliveries: intake must be
// both active and in the active state, so a paused or errored receiver stops at the gate.
func webhookAccepts(state string, active bool) bool {
	return active && ConnectionState(state) == StActive
}

// headerOrDefault resolves an incoming header under the webhook's configured name, falling
// back when the configuration left it blank.
func headerOrDefault(cfg, fallback string) string {
	if s := strings.TrimSpace(cfg); s != "" {
		return s
	}
	return fallback
}
