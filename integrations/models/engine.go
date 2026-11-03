package models

import (
	"context"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"nexa/core/orm"
)

// Behaviour layer for integrations. The control-plane rules are pure and unit-tested: a
// connection may only be enabled against a well-formed HTTPS endpoint (never loopback, a
// metadata host or a bare, path-less authority); an outbound call is successful when it
// completes without a transport error and returns a 2xx or 3xx status; the operational state
// a test drives is active on success and error on failure; a signature is a stable
// HMAC-SHA256 of the request body; the auth headers vary by mode; and health is derived from
// the last check, its status and how long ago it ran. The computes project a connection's
// health label, days-since-check and delivery count.

const (
	connectionModel   = "integration.connection"
	deliveryModel     = "integration.delivery"
	webhookModel      = "integration.webhook"
	inboundEventModel = "integration.inbound.event"

	dateLayout   = "2006-01-02"
	datetimeFull = "2006-01-02 15:04:05"

	defTimeoutSeconds   = 15
	minTimeoutSeconds   = 1
	maxTimeoutSeconds   = 60
	staleAfterDays      = 30
	maxResponseExcerpt  = 1000
	maxRequestBodyBytes = 8192

	// Inbound webhooks: the public ingress prefix, bounded body handling, the replay-window
	// bounds and the scheduled worker's identity and batch size.
	ingressPathPrefix       = "/webhook/"
	maxInboundBodyBytes     = 262144
	maxInboundExcerpt       = 2000
	defReplayWindowSeconds  = 300
	minReplayWindowSeconds  = 30
	maxReplayWindowSeconds  = 3600
	inboundBatchLimit       = 50
	inboundProcessCronCode  = "integrations.inbound.process"
	defaultSignatureHeader  = "X-Signature"
	defaultTimestampHeader  = "X-Timestamp"
	defaultNonceHeader      = "X-Webhook-Nonce"
	defaultExternalIDHeader = "X-Webhook-Id"
)

// validateEndpointPolicy is the pure, DNS-free gate applied to any configured endpoint. It
// requires an absolute HTTPS URL with a host, rejects loopback/metadata authorities and a
// bare, path-less host, and bounds the length. The dial-time IP pinning (blocking private,
// link-local and CGNAT ranges) is enforced separately in the sender.
func validateEndpointPolicy(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return errEmptyEndpoint
	}
	if len(raw) > 500 {
		return errEndpointTooLong
	}
	u, err := url.Parse(raw)
	if err != nil {
		return errMissingScheme
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme == "http" {
		return errInsecureScheme
	}
	if scheme != "https" {
		return errMissingScheme
	}
	name := strings.TrimSpace(u.Hostname())
	if name == "" {
		return errMissingScheme
	}
	lname := strings.ToLower(name)
	if lname == "localhost" || strings.HasSuffix(lname, ".localhost") || lname == "metadata.google.internal" {
		return errBlockedHost
	}
	// A bare, shallow authority with no path (e.g. https://example.com) is rejected; a usable
	// API endpoint carries a path or a sufficiently-qualified host.
	if strings.Trim(u.Path, "/") == "" && strings.Count(lname, ".") < 2 {
		return errPathlessHost
	}
	return nil
}

// smtpTarget is a parsed mail server address for an email connection. The endpoint is not an
// HTTP URL: it is a host with an optional port and a scheme that selects the transport (smtps:
// implicit TLS, smtp/submit: opportunistic STARTTLS). Parsing and host policy are pure and
// DNS-free; the dial-time IP pinning is enforced in the sender.
type smtpTarget struct {
	host string
	port int
	tls  string // "implicit" | "starttls"
}

// parseSMTPTarget turns a configured mail endpoint into a host/port/transport without touching
// the network. Accepted forms: smtp://host[:port], smtps://host[:port], submit://host[:port] or
// a bare host[:port]. localhost/.localhost/metadata authorities and out-of-range ports fail.
func parseSMTPTarget(raw string) (smtpTarget, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return smtpTarget{}, errEmptyEndpoint
	}
	if len(raw) > 500 {
		return smtpTarget{}, errEndpointTooLong
	}
	scheme := ""
	rest := raw
	if i := strings.Index(raw, "://"); i >= 0 {
		scheme = strings.ToLower(raw[:i])
		rest = raw[i+3:]
	}
	if j := strings.IndexAny(rest, "/?#"); j >= 0 {
		rest = rest[:j]
	}
	rest = strings.TrimSpace(rest)
	host, portStr := rest, ""
	if h, p, err := net.SplitHostPort(rest); err == nil {
		host, portStr = h, p
	}
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return smtpTarget{}, errMissingSMTPHost
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || host == "metadata.google.internal" {
		return smtpTarget{}, errBlockedHost
	}
	switch scheme {
	case "", "smtp", "smtps", "submit":
	default:
		return smtpTarget{}, errUnsupportedMailScheme
	}
	tlsMode := "starttls"
	port := 587
	switch scheme {
	case "smtps":
		port = 465
		tlsMode = "implicit"
	}
	if portStr != "" {
		n, err := strconv.Atoi(strings.TrimSpace(portStr))
		if err != nil || n < 1 || n > 65535 {
			return smtpTarget{}, errBadPort
		}
		port = n
		if port == 465 {
			tlsMode = "implicit"
		}
	}
	return smtpTarget{host: host, port: port, tls: tlsMode}, nil
}

// classifyOutcome reports whether an outbound call succeeded: no transport error and a 2xx or
// 3xx status code.
func classifyOutcome(transportErr error, statusCode int) bool {
	if transportErr != nil {
		return false
	}
	return statusCode >= 200 && statusCode < 400
}

// stateAfterTest is the state a test run leaves a connection in.
func stateAfterTest(success bool) ConnectionState {
	if success {
		return StActive
	}
	return StError
}

// computeHealth maps the stored last-check fields to a human status label. A connection that
// has never been tested is Unchecked; a failing last status is Failing; a passing one is
// Healthy unless it has gone stale; a paused connector reports its own state.
func computeHealth(state, lastCheck string, lastStatusCode int, now time.Time) string {
	switch ConnectionState(state) {
	case StPaused:
		return "Paused"
	case StDraft:
		return "Draft"
	}
	if lastCheck == "" {
		return "Unchecked"
	}
	if lastStatusCode == 0 {
		return "Failed"
	}
	if lastStatusCode < 200 || lastStatusCode >= 400 {
		return "Failing"
	}
	t, err := time.Parse(datetimeFull, lastCheck)
	if err == nil && now.Sub(t) > time.Duration(staleAfterDays)*time.Hour*24 {
		return "Stale"
	}
	return "Healthy"
}

// daysSince renders a stored datetime string as whole days elapsed to now (0 when absent or
// unparseable, never negative).
func daysSince(lastCheck string, now time.Time) int {
	if strings.TrimSpace(lastCheck) == "" {
		return 0
	}
	t, err := time.Parse(datetimeFull, lastCheck)
	if err != nil {
		return 0
	}
	d := int(now.Sub(t).Hours() / 24)
	if d < 0 {
		return 0
	}
	return d
}

// timeoutSeconds clamps a configured timeout into a safe range, defaulting when unset.
func timeoutSeconds(v int) int {
	if v < minTimeoutSeconds {
		return defTimeoutSeconds
	}
	if v > maxTimeoutSeconds {
		return maxTimeoutSeconds
	}
	return v
}

// ---------------------------------------------------------------------------
// Computes
// ---------------------------------------------------------------------------

func init() {
	orm.RegisterCompute(connectionModel, "health", []string{"state", "last_check", "last_status_code"}, computeConnectionHealth)
	orm.RegisterCompute(connectionModel, "days_since_check", []string{"last_check"}, computeConnectionDaysSinceCheck)
	orm.RegisterCompute(connectionModel, "delivery_count", []string{"id"}, computeConnectionDeliveryCount)
	orm.RegisterCompute(webhookModel, "ingress_url", []string{"token"}, computeWebhookIngressUrl)
	orm.RegisterCompute(webhookModel, "received_count", []string{"id"}, computeWebhookReceivedCount)
	orm.RegisterCompute(webhookModel, "pending_count", []string{"id"}, computeWebhookPendingCount)
}

func computeConnectionHealth(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return computeHealth(strOr(rec["state"]), strOr(rec["last_check"]), intOr(rec["last_status_code"]), time.Now().UTC()), nil
}

func computeConnectionDaysSinceCheck(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return daysSince(strOr(rec["last_check"]), time.Now().UTC()), nil
}

func computeConnectionDeliveryCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), deliveryModel, [][]interface{}{
		{"connection_id", "=", intOr(rec["id"])},
	})
	if err != nil {
		return 0, nil
	}
	return n, nil
}

func computeWebhookIngressUrl(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	token := strings.TrimSpace(strOr(rec["token"]))
	if token == "" {
		return "(save the webhook to generate its ingress URL)", nil
	}
	return ingressPathPrefix + token, nil
}

func computeWebhookReceivedCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), inboundEventModel, [][]interface{}{
		{"webhook_id", "=", intOr(rec["id"])},
	})
	if err != nil {
		return 0, nil
	}
	return n, nil
}

func computeWebhookPendingCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), inboundEventModel, [][]interface{}{
		{"webhook_id", "=", intOr(rec["id"])},
		{"status", "in", []interface{}{string(EvNew), string(EvProcessing)}},
	})
	if err != nil {
		return 0, nil
	}
	return n, nil
}
