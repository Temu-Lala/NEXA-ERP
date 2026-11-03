package models

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"nexa/core/orm"
)

// Object actions drive integrations. A connector is enabled only against a valid HTTPS endpoint
// (server-side policy, not a UI hint) and can be paused at any time. The connection-test action
// performs a REAL, SSRF-guarded outbound request — resolving the secret from the environment,
// building auth/signature headers, sending the probe and reading back the true status, latency,
// response and any transport error — then records an append-only integration.delivery row and
// moves the connector to active on success or error on failure. Nothing here is simulated: the
// numbers a user sees come from an actual call. Test, enable and pause require an administrator.

func init() {
	orm.RegisterObjectAction(connectionModel, "connection_enable", actConnectionEnable)
	orm.RegisterObjectAction(connectionModel, "connection_pause", actConnectionPause)
	orm.RegisterObjectAction(connectionModel, "connection_test", actConnectionTest)
}

func nowStamp() string { return time.Now().UTC().Format(datetimeFull) }

func actConnectionEnable(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	if err := requireAdminErr(ctx, "enable an integration connection"); err != nil {
		return "", err
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, connectionModel, id)
	if err != nil {
		return "", err
	}
	endpoint := strOr(rec["endpoint"])
	if err := validateProviderEndpoint(strOr(rec["provider"]), endpoint); err != nil {
		return "", fmt.Errorf("cannot enable: %w", err)
	}
	if err := orm.UpdateRecordByID(bypass, connectionModel, id, map[string]interface{}{
		"state":      string(StActive),
		"last_error": "",
	}); err != nil {
		return "", err
	}
	return "Connection enabled", nil
}

func actConnectionPause(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	if err := requireAdminErr(ctx, "pause an integration connection"); err != nil {
		return "", err
	}
	bypass := orm.ContextWithBypass(ctx, true)
	if _, err := getRecord(bypass, connectionModel, id); err != nil {
		return "", err
	}
	if err := orm.UpdateRecordByID(bypass, connectionModel, id, map[string]interface{}{"state": string(StPaused)}); err != nil {
		return "", err
	}
	return "Connection paused", nil
}

func actConnectionTest(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	if err := requireAdminErr(ctx, "test an integration connection"); err != nil {
		return "", err
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, connectionModel, id)
	if err != nil {
		return "", err
	}
	endpoint := strOr(rec["endpoint"])
	provider := strOr(rec["provider"])
	adapter := adapterForProvider(provider)
	method := strOr(rec["http_method"])
	if method == "" {
		method = string(MPost)
	}
	methodLabel := probeMethodLabel(provider, method)

	// Reject a malformed or disallowed endpoint before touching the network, recording the truth.
	if err := validateProviderEndpoint(provider, endpoint); err != nil {
		recordDelivery(bypass, buildFailureDelivery(ctx, id, endpoint, methodLabel, adapter, err.Error()))
		_ = orm.UpdateRecordByID(bypass, connectionModel, id, map[string]interface{}{
			"state": string(StError), "last_check": nowStamp(), "last_status_code": 0, "last_error": err.Error(),
		})
		return "", fmt.Errorf("connection test failed: %w", err)
	}

	// Resolve the secret by reference from the environment; it is never stored on the record.
	mode := strOr(rec["auth_mode"])
	var secret string
	if requestHeadersRequired(mode) {
		s, ok := resolveSecret(strOr(rec["secret_ref"]))
		if !ok {
			msg := fmt.Sprintf("secret reference %q is not resolvable from the environment", strOr(rec["secret_ref"]))
			recordDelivery(bypass, buildFailureDelivery(ctx, id, endpoint, methodLabel, adapter, msg))
			_ = orm.UpdateRecordByID(bypass, connectionModel, id, map[string]interface{}{
				"state": string(StError), "last_check": nowStamp(), "last_status_code": 0, "last_error": msg,
			})
			return "", fmt.Errorf("connection test failed: %s", msg)
		}
		secret = s
	}

	// Run the transport-correct probe. SMTP performs a guarded handshake and never sends; HTTP
	// sends the configured (or default) request through the SSRF-pinned sender.
	var result outboundResult
	var headers map[string]string
	var body []byte
	sigPreview := ""
	if adapter == adapterSMTP {
		result = smtpProbe(ctx, smtpRequest{
			endpoint:   endpoint,
			user:       strOr(rec["api_user"]),
			password:   secret,
			timeoutSec: intOr(rec["timeout_seconds"]),
		})
	} else {
		body = probeBody(rec, id)
		headers = parseExtraHeaders(strOr(rec["headers"]))
		authHdr, err := authHeaders(mode, strOr(rec["header_name"]), strOr(rec["api_user"]), secret, body, nowStamp())
		if err != nil {
			recordDelivery(bypass, buildFailureDelivery(ctx, id, endpoint, methodLabel, adapter, err.Error()))
			return "", fmt.Errorf("connection test failed: %w", err)
		}
		for k, v := range authHdr {
			headers[k] = v
		}
		if boolOr(rec["sign_payload"]) || AuthMode(mode) == AuthHMAC {
			sigPreview = signaturePreview(headers["X-Signature"])
		}
		result = sendOutbound(ctx, outboundRequest{
			method:     method,
			url:        endpoint,
			headers:    headers,
			body:       body,
			timeoutSec: intOr(rec["timeout_seconds"]),
		})
	}
	success := classifyOutcome(result.transportErr, result.statusCode)

	delivery := map[string]interface{}{
		"connection_id":  id,
		"timestamp":      nowStamp(),
		"trigger":        string(TrgTest),
		"provider":       adapter,
		"method":         methodLabel,
		"url":            truncate(endpoint, 500),
		"status_code":    result.statusCode,
		"success":        success,
		"duration_ms":    result.durationMs,
		"request_digest": buildRequestDigest(methodLabel, headers, len(body)),
		"signature":      sigPreview,
		"actor_id":       orm.SecurityUID(ctx),
	}
	if result.transportErr != nil {
		delivery["error"] = result.transportErr.Error()
	} else {
		delivery["response_excerpt"] = result.response
	}
	recordDelivery(bypass, delivery)

	patch := map[string]interface{}{
		"state":            string(stateAfterTest(success)),
		"last_check":       nowStamp(),
		"last_status_code": result.statusCode,
		"last_latency_ms":  result.durationMs,
		"last_error":       "",
	}
	if !success {
		if result.transportErr != nil {
			patch["last_error"] = result.transportErr.Error()
		} else {
			patch["last_error"] = fmt.Sprintf("endpoint returned status %d", result.statusCode)
		}
	}
	if err := orm.UpdateRecordByID(bypass, connectionModel, id, patch); err != nil {
		return "", err
	}
	if success {
		return testSuccessMessage(adapter, endpoint, result), nil
	}
	if result.transportErr != nil {
		return "", fmt.Errorf("connection test failed: %w", result.transportErr)
	}
	return "", fmt.Errorf("connection test failed: endpoint returned %d", result.statusCode)
}

// ---------------------------------------------------------------------------
// Action helpers
// ---------------------------------------------------------------------------

func probeBody(rec map[string]interface{}, id int) []byte {
	if tmpl := strOr(rec["payload_template"]); tmpl != "" {
		return []byte(tmpl)
	}
	return defaultProbeBody(strOr(rec["code"]), id)
}

func buildFailureDelivery(ctx context.Context, id int, url, method, adapter, msg string) map[string]interface{} {
	return map[string]interface{}{
		"connection_id":  id,
		"timestamp":      nowStamp(),
		"trigger":        string(TrgTest),
		"provider":       adapter,
		"method":         method,
		"url":            truncate(url, 500),
		"status_code":    0,
		"success":        false,
		"duration_ms":    0,
		"request_digest": buildRequestDigest(method, nil, 0),
		"error":          msg,
		"actor_id":       orm.SecurityUID(ctx),
	}
}

func recordDelivery(ctx context.Context, vals map[string]interface{}) {
	_, _ = orm.Create(ctx, orm.RegistryModel(deliveryModel), vals)
}

func getRecord(ctx context.Context, model string, id int) (map[string]interface{}, error) {
	rec, err := orm.SearchOne(ctx, model, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return nil, fmt.Errorf("record not found")
	}
	return rec, nil
}

func requireAdmin(ctx context.Context) bool {
	uid := orm.SecurityUID(ctx)
	if uid <= 0 {
		return false
	}
	return orm.UserHasGroupXML(ctx, uid, "base.group_system")
}

func requireAdminErr(ctx context.Context, what string) error {
	if !requireAdmin(ctx) {
		return fmt.Errorf("insufficient permission to %s", what)
	}
	return nil
}

func toUpperMethod(method string) string {
	m := strings.ToUpper(strings.TrimSpace(method))
	if m == "" {
		return "POST"
	}
	return m
}

// testSuccessMessage phrases a successful probe according to the transport that ran.
func testSuccessMessage(adapter, endpoint string, result outboundResult) string {
	if adapter == adapterSMTP {
		return fmt.Sprintf("Mail server reachable — %s responded %d in %d ms", shortHost(endpoint), result.statusCode, result.durationMs)
	}
	return fmt.Sprintf("Connection OK — %s returned %d in %d ms", shortHost(endpoint), result.statusCode, result.durationMs)
}

// shortHost renders just the scheme+host of an endpoint for a compact success message.
func shortHost(endpoint string) string {
	if u, err := url.Parse(strings.TrimSpace(endpoint)); err == nil && u.Host != "" {
		if u.Scheme != "" {
			return u.Scheme + "://" + u.Host
		}
		return u.Host
	}
	return endpoint
}
