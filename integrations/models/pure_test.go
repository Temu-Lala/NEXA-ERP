package models

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestValidateEndpointPolicy(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want error
	}{
		{"empty", "", errEmptyEndpoint},
		{"plaintext http rejected", "http://hooks.example.com/a", errInsecureScheme},
		{"missing scheme", "hooks.example.com/a", errMissingScheme},
		{"loopback rejected", "https://localhost/abc", errBlockedHost},
		{"localhost suffix rejected", "https://db.localhost/abc", errBlockedHost},
		{"metadata rejected", "https://metadata.google.internal/x", errBlockedHost},
		{"bare shallow host rejected", "https://example.com", errPathlessHost},
		{"valid https path", "https://hooks.example.com/ingest", nil},
		{"valid dotted host no path", "https://api.v2.example.com", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := validateEndpointPolicy(c.url)
			if c.want == nil && got != nil {
				t.Fatalf("expected nil, got %v", got)
			}
			if c.want != nil && !errors.Is(got, c.want) {
				t.Fatalf("expected %v, got %v", c.want, got)
			}
		})
	}
}

func TestSignPayloadIsStableAndPrefixed(t *testing.T) {
	a := signPayload("secret", []byte("hello"))
	b := signPayload("secret", []byte("hello"))
	if a != b {
		t.Fatalf("signature not deterministic: %s vs %s", a, b)
	}
	if !strings.HasPrefix(a, "sha256=") {
		t.Fatalf("expected sha256= prefix, got %q", a)
	}
	if len(a) != len("sha256=")+64 {
		t.Fatalf("expected 64 hex chars, got %q", a)
	}
	if signPayload("other", []byte("hello")) == a {
		t.Fatalf("different secret produced same signature")
	}
}

func TestAuthHeadersByMode(t *testing.T) {
	if h, err := authHeaders("none", "", "", "", nil, ""); err != nil || len(h) != 0 {
		t.Fatalf("none mode should be empty, got %v err=%v", h, err)
	}
	if _, err := authHeaders("bearer", "", "", "", nil, ""); !errors.Is(err, errSecretUnresolved) {
		t.Fatalf("bearer with empty secret must error, got %v", err)
	}
	h, err := authHeaders("bearer", "", "", "tok", nil, "")
	if err != nil || h["Authorization"] != "Bearer tok" {
		t.Fatalf("bearer header wrong: %v err=%v", h, err)
	}
	h, err = authHeaders("api_key", "X-My-Key", "", "k", nil, "")
	if err != nil || h["X-My-Key"] != "k" {
		t.Fatalf("api_key header wrong: %v err=%v", h, err)
	}
	h, err = authHeaders("hmac", "", "", "sek", []byte("body"), "ts")
	if err != nil || !strings.HasPrefix(h["X-Signature"], "sha256=") || h["X-Timestamp"] != "ts" {
		t.Fatalf("hmac headers wrong: %v err=%v", h, err)
	}
}

func TestClassifyOutcome(t *testing.T) {
	if classifyOutcome(errors.New("boom"), 200) {
		t.Fatalf("transport error must be a failure")
	}
	if !classifyOutcome(nil, 204) || !classifyOutcome(nil, 302) {
		t.Fatalf("2xx/3xx must succeed")
	}
	if classifyOutcome(nil, 404) || classifyOutcome(nil, 500) || classifyOutcome(nil, 0) {
		t.Fatalf("4xx/5xx/0 must fail")
	}
}

func TestComputeHealth(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-24 * time.Hour).Format(datetimeFull)
	old := now.Add(-90 * 24 * time.Hour).Format(datetimeFull)
	if got := computeHealth(string(StDraft), "", 0, now); got != "Draft" {
		t.Fatalf("draft -> %s", got)
	}
	if got := computeHealth(string(StActive), "", 0, now); got != "Unchecked" {
		t.Fatalf("unchecked -> %s", got)
	}
	if got := computeHealth(string(StActive), recent, 500, now); got != "Failing" {
		t.Fatalf("failing -> %s", got)
	}
	if got := computeHealth(string(StActive), recent, 200, now); got != "Healthy" {
		t.Fatalf("healthy -> %s", got)
	}
	if got := computeHealth(string(StActive), old, 200, now); got != "Stale" {
		t.Fatalf("stale -> %s", got)
	}
	if got := computeHealth(string(StPaused), recent, 200, now); got != "Paused" {
		t.Fatalf("paused -> %s", got)
	}
}

func TestDaysSinceAndTimeout(t *testing.T) {
	now := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	if got := daysSince(now.Add(-73*time.Hour).Format(datetimeFull), now); got != 3 {
		t.Fatalf("daysSince got %d want 3", got)
	}
	if got := daysSince("", now); got != 0 {
		t.Fatalf("empty daysSince should be 0, got %d", got)
	}
	if timeoutSeconds(0) != defTimeoutSeconds || timeoutSeconds(999) != maxTimeoutSeconds || timeoutSeconds(7) != 7 {
		t.Fatalf("timeout clamp wrong")
	}
}

func TestTruncateAndDigest(t *testing.T) {
	if got := truncate("abcdef", 3); !strings.HasPrefix(got, "abc") || !strings.Contains(got, "truncated") {
		t.Fatalf("truncate got %q", got)
	}
	d := buildRequestDigest("post", map[string]string{"Zeta": "secret-value", "Alpha": "x"}, 12)
	if !strings.Contains(d, "headers=[Alpha,Zeta]") || !strings.Contains(d, "body_bytes=12") || strings.Contains(d, "secret-value") {
		t.Fatalf("digest wrong or leaked a value: %q", d)
	}
}

func TestParseSMTPTarget(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		host    string
		port    int
		tls     string
		wantErr error
	}{
		{"smtps implicit", "smtps://mail.example.com", "mail.example.com", 465, "implicit", nil},
		{"submit starttls default", "submit://smtp.example.com", "smtp.example.com", 587, "starttls", nil},
		{"bare host defaults starttls", "smtp.example.com", "smtp.example.com", 587, "starttls", nil},
		{"bare host port 465 implicit", "smtp.example.com:465", "smtp.example.com", 465, "implicit", nil},
		{"smtp explicit port", "smtp://mail.example.com:25", "mail.example.com", 25, "starttls", nil},
		{"path stripped", "smtps://mail.example.com:465/x", "mail.example.com", 465, "implicit", nil},
		{"empty", "", "", 0, "", errEmptyEndpoint},
		{"localhost rejected", "smtp://localhost:25", "", 0, "", errBlockedHost},
		{"metadata rejected", "smtp://metadata.google.internal:25", "", 0, "", errBlockedHost},
		{"bad port", "smtp://mail.example.com:99999", "", 0, "", errBadPort},
		{"https scheme rejected", "https://mail.example.com:443", "", 0, "", errUnsupportedMailScheme},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseSMTPTarget(c.raw)
			if c.wantErr != nil {
				if !errors.Is(err, c.wantErr) {
					t.Fatalf("expected %v, got %v", c.wantErr, err)
				}
				return
			}
			if err != nil || got.host != c.host || got.port != c.port || got.tls != c.tls {
				t.Fatalf("parseSMTPTarget(%q)=%+v err=%v want host=%s port=%d tls=%s", c.raw, got, err, c.host, c.port, c.tls)
			}
		})
	}
}

func TestAdapterForProviderAndProbeLabel(t *testing.T) {
	if adapterForProvider("email") != adapterSMTP {
		t.Fatalf("email must route to smtp adapter")
	}
	for _, p := range []string{"webhook", "rest_api", "sms", "storage", "other"} {
		if adapterForProvider(p) != adapterHTTP {
			t.Fatalf("%s must route to http adapter", p)
		}
	}
	if probeMethodLabel("email", "post") != "SMTP" {
		t.Fatalf("smtp probe label should be SMTP")
	}
	if probeMethodLabel("rest_api", "get") != "GET" {
		t.Fatalf("http probe label should upper the method")
	}
}

func TestValidateProviderEndpointRoutesByTransport(t *testing.T) {
	// An SMTP host is invalid under HTTP policy but valid under the mail policy, and vice versa.
	if err := validateProviderEndpoint("email", "smtps://mail.example.com:465"); err != nil {
		t.Fatalf("smtp endpoint should validate for email: %v", err)
	}
	if err := validateProviderEndpoint("email", "https://hooks.example.com/ingest"); err == nil {
		t.Fatalf("https url must fail the mail policy")
	}
	if err := validateProviderEndpoint("rest_api", "https://hooks.example.com/ingest"); err != nil {
		t.Fatalf("https url should validate for rest_api: %v", err)
	}
	if err := validateProviderEndpoint("rest_api", "smtps://mail.example.com:465"); err == nil {
		t.Fatalf("mail host must fail the https policy")
	}
}
