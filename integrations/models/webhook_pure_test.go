package models

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestVerifyInboundSignature(t *testing.T) {
	secret := "s3cr3t"
	body := []byte(`{"event":"ping"}`)
	valid := signPayload(secret, body) // "sha256=<hex>"
	if !verifyInboundSignature(secret, body, valid) {
		t.Fatalf("valid prefixed signature must verify")
	}
	if !verifyInboundSignature(secret, body, strings.TrimPrefix(valid, "sha256=")) {
		t.Fatalf("bare-hex signature must verify")
	}
	if verifyInboundSignature(secret, body, "sha256="+strings.Repeat("ab", 32)) {
		t.Fatalf("forged signature must not verify")
	}
	if verifyInboundSignature(secret, []byte("tampered"), valid) {
		t.Fatalf("signature over a different body must not verify")
	}
	if verifyInboundSignature("", body, valid) {
		t.Fatalf("empty secret must not verify")
	}
	if verifyInboundSignature(secret, body, "") {
		t.Fatalf("empty header must not verify")
	}
	if verifyInboundSignature(secret, body, "not-hex") {
		t.Fatalf("non-hex header must not verify")
	}
}

func TestTimestampWithinWindow(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	if !timestampWithinWindow(strconv.FormatInt(now.Unix(), 10), 300, now) {
		t.Fatalf("current epoch should be within window")
	}
	if !timestampWithinWindow("1761400000", 400, time.Unix(1761400000, 0)) {
		t.Fatalf("exact epoch should be within window")
	}
	if timestampWithinWindow(strconv.FormatInt(now.Add(-10*time.Minute).Unix(), 10), 300, now) {
		t.Fatalf("stale timestamp must be rejected")
	}
	if timestampWithinWindow(strconv.FormatInt(now.Add(10*time.Minute).Unix(), 10), 300, now) {
		t.Fatalf("future-skewed timestamp must be rejected")
	}
	if timestampWithinWindow("", 300, now) {
		t.Fatalf("blank timestamp must be rejected when required")
	}
	if timestampWithinWindow("garbage", 300, now) {
		t.Fatalf("unparseable timestamp must be rejected")
	}
	if !timestampWithinWindow(now.Format(time.RFC3339), 60, now) {
		t.Fatalf("RFC3339 form should be accepted")
	}
}

func TestDedupKeyPrecedence(t *testing.T) {
	if got := dedupKey("evt-1", "nonce", "hash"); got != "id:evt-1" {
		t.Fatalf("external id should win, got %q", got)
	}
	if got := dedupKey("", "nonce-9", "hash"); got != "n:nonce-9" {
		t.Fatalf("nonce should be second, got %q", got)
	}
	if got := dedupKey("", "", "sha256=abc"); got != "h:sha256=abc" {
		t.Fatalf("content hash should be last resort, got %q", got)
	}
	if got := dedupKey("", "", ""); got != "" {
		t.Fatalf("no identity should yield empty key, got %q", got)
	}
}

func TestClampReplayWindow(t *testing.T) {
	if clampReplayWindow(0) != defReplayWindowSeconds || clampReplayWindow(5) != defReplayWindowSeconds {
		t.Fatalf("too-small window should fall back to default")
	}
	if clampReplayWindow(999999) != maxReplayWindowSeconds {
		t.Fatalf("oversized window should clamp to max")
	}
	if clampReplayWindow(600) != 600 {
		t.Fatalf("in-range window should pass through")
	}
}

func TestWebhookAcceptsAndHeaderDefault(t *testing.T) {
	if !webhookAccepts(string(StActive), true) {
		t.Fatalf("active+true should accept")
	}
	for _, s := range []string{string(StDraft), string(StPaused), string(StError)} {
		if webhookAccepts(s, true) {
			t.Fatalf("state %s must not accept", s)
		}
	}
	if webhookAccepts(string(StActive), false) {
		t.Fatalf("inactive must not accept")
	}
	if headerOrDefault("", defaultSignatureHeader) != defaultSignatureHeader {
		t.Fatalf("blank config should fall back")
	}
	if headerOrDefault("X-Custom-Sig", defaultSignatureHeader) != "X-Custom-Sig" {
		t.Fatalf("configured header should win")
	}
}

func TestGenerateWebhookToken(t *testing.T) {
	a, err := generateWebhookToken()
	if err != nil {
		t.Fatalf("token generation failed: %v", err)
	}
	if len(a) != 32 {
		t.Fatalf("expected 32 hex chars, got %d (%q)", len(a), a)
	}
	b, _ := generateWebhookToken()
	if a == b {
		t.Fatalf("two tokens must differ")
	}
}
