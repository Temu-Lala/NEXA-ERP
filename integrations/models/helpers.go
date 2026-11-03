package models

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Shared helpers for the integrations module: safe accessors over the loosely-typed record
// maps the ORM hands to computes and actions, endpoint-policy sentinel errors, environment
// secret resolution (the connector never stores the secret), HMAC-SHA256 request signing,
// authentication-header construction and a secret-free request digest for the delivery log.
// Everything here is pure and offline so it can be unit-tested.

var (
	errEmptyEndpoint         = errors.New("no endpoint URL is configured")
	errEndpointTooLong       = errors.New("endpoint URL is too long")
	errInsecureScheme        = errors.New("endpoint must use HTTPS; plaintext HTTP is not allowed")
	errMissingScheme         = errors.New("endpoint must be an absolute https:// URL")
	errBlockedHost           = errors.New("endpoint host is not allowed")
	errPathlessHost          = errors.New("endpoint must include a path (a bare host is not a valid target)")
	errSecretUnresolved      = errors.New("secret reference could not be resolved from the environment")
	errMissingSMTPHost       = errors.New("mail endpoint must include a host")
	errUnsupportedMailScheme = errors.New("mail endpoint scheme must be smtp, smtps or submit")
	errBadPort               = errors.New("mail endpoint port is out of range")
)

func strOr(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	if s, ok := v.(fmt.Stringer); ok {
		return s.String()
	}
	return ""
}

func intOr(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case string:
		i, _ := strconv.Atoi(strings.TrimSpace(n))
		return i
	}
	return 0
}

func boolOr(v interface{}) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

// resolveSecret reads the connector secret from the process environment by reference name.
// The secret is never stored on the record — only its environment key is — so this returns
// an empty string (and is reported unresolved) whenever the reference is blank or unset.
func resolveSecret(ref string) (string, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", false
	}
	val := os.Getenv(ref)
	if val == "" {
		return "", false
	}
	return val, true
}

// signPayload returns an "sha256=<hex>" HMAC-SHA256 signature of body under secret, matching
// the common webhook signature convention.
func signPayload(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// authHeaders builds the authentication headers for a mode. It never logs or returns the
// secret itself. An empty secret for a mode that requires one yields an error so the caller
// can record a truthful failure.
func authHeaders(mode, headerName, apiUser, secret string, body []byte, timestamp string) (map[string]string, error) {
	out := map[string]string{}
	switch AuthMode(mode) {
	case AuthNone:
		return out, nil
	case AuthAPIKey:
		if secret == "" {
			return nil, errSecretUnresolved
		}
		h := strings.TrimSpace(headerName)
		if h == "" {
			h = "X-Api-Key"
		}
		out[h] = secret
		return out, nil
	case AuthBearer:
		if secret == "" {
			return nil, errSecretUnresolved
		}
		out["Authorization"] = "Bearer " + secret
		return out, nil
	case AuthBasic:
		if secret == "" {
			return nil, errSecretUnresolved
		}
		cred := apiUser + ":" + secret
		out["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte(cred))
		return out, nil
	case AuthHMAC:
		if secret == "" {
			return nil, errSecretUnresolved
		}
		out["X-Signature"] = signPayload(secret, body)
		if timestamp != "" {
			out["X-Timestamp"] = timestamp
		}
		return out, nil
	}
	return out, nil
}

// requestHeadersRequired reports whether an auth mode depends on a resolvable secret.
func requestHeadersRequired(mode string) bool {
	switch AuthMode(mode) {
	case AuthAPIKey, AuthBearer, AuthBasic, AuthHMAC:
		return true
	}
	return false
}

// defaultProbeBody is the JSON payload sent when no test template is configured. It carries no
// user data — only a stable marker so a receiving endpoint can recognise a connectivity probe.
func defaultProbeBody(code string, id int) []byte {
	payload := map[string]interface{}{
		"object":     "integration.connection.test",
		"connection": strings.TrimSpace(code),
		"id":         id,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return []byte(`{"object":"integration.connection.test"}`)
	}
	return b
}

// buildRequestDigest produces a secret-free summary of the outgoing request for the audit row:
// the method, the header NAMES (never their values) and the body length.
func buildRequestDigest(method string, headers map[string]string, bodyLen int) string {
	names := make([]string, 0, len(headers))
	for k := range headers {
		names = append(names, k)
	}
	// Sort for a stable digest independent of map iteration order.
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && strings.ToLower(names[j]) < strings.ToLower(names[j-1]); j-- {
			names[j], names[j-1] = names[j-1], names[j]
		}
	}
	return strings.ToUpper(strings.TrimSpace(method)) + " headers=[" + strings.Join(names, ",") + "] body_bytes=" + strconv.Itoa(bodyLen)
}

// parseExtraHeaders decodes an optional JSON object of additional non-secret headers. Malformed
// or non-object input yields no headers rather than failing the call.
func parseExtraHeaders(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	out := map[string]string{}
	if raw == "" {
		return out
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return out
	}
	for k, v := range m {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		out[k] = v
	}
	return out
}

// truncate caps a string to n runes with an ellipsis marker, for storing a response excerpt.
func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…[truncated]"
}

// signaturePreview shortens a signature for display so the full value is never stored.
func signaturePreview(sig string) string {
	sig = strings.TrimSpace(sig)
	if sig == "" {
		return ""
	}
	r := []rune(sig)
	if len(r) <= 20 {
		return sig
	}
	return string(r[:13]) + "…"
}
