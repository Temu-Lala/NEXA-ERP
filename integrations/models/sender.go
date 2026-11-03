package models

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// The outbound transport. Every call is SSRF-guarded: the endpoint scheme/host policy is
// checked first, then the host is resolved once and its addresses are re-verified against the
// blocklist (loopback, private, link-local, multicast, unspecified and RFC 6598 CGNAT) before
// the dial is pinned to that validated address set — so a public name cannot be rebinded to an
// internal IP between resolution and connection. Redirects are bounded and each hop is
// re-validated. Responses are size-capped. This mirrors the platform's automation webhook
// sender so both share one security posture.

type outboundRequest struct {
	method     string
	url        string
	headers    map[string]string
	body       []byte
	timeoutSec int
}

type outboundResult struct {
	statusCode   int
	durationMs   int
	response     string
	transportErr error
}

func sendOutbound(ctx context.Context, req outboundRequest) outboundResult {
	start := time.Now()
	res := outboundResult{}
	if err := validateEndpointPolicy(req.url); err != nil {
		res.transportErr = err
		res.durationMs = int(time.Since(start).Milliseconds())
		return res
	}
	u, err := url.Parse(strings.TrimSpace(req.url))
	if err != nil {
		res.transportErr = err
		res.durationMs = int(time.Since(start).Milliseconds())
		return res
	}
	dialIPs, err := resolveDialIPs(u.Hostname())
	if err != nil {
		res.transportErr = err
		res.durationMs = int(time.Since(start).Milliseconds())
		return res
	}
	method := strings.ToUpper(strings.TrimSpace(req.method))
	if method == "" {
		method = http.MethodPost
	}
	var bodyReader io.Reader
	if len(req.body) > 0 && method != http.MethodGet && method != http.MethodDelete {
		body := req.body
		if len(body) > maxRequestBodyBytes {
			body = body[:maxRequestBodyBytes]
		}
		bodyReader = bytes.NewReader(body)
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, strings.TrimSpace(req.url), bodyReader)
	if err != nil {
		res.transportErr = err
		res.durationMs = int(time.Since(start).Milliseconds())
		return res
	}
	for k, v := range req.headers {
		httpReq.Header.Set(k, v)
	}
	if bodyReader != nil && httpReq.Header.Get("Content-Type") == "" {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{
		Timeout: time.Duration(timeoutSeconds(req.timeoutSec)) * time.Second,
		Transport: &http.Transport{
			DialContext: pinnedDialer(dialIPs),
		},
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("redirect limit exceeded")
			}
			return validateEndpointPolicy(r.URL.String())
		},
	}
	resp, err := client.Do(httpReq)
	res.durationMs = int(time.Since(start).Milliseconds())
	if err != nil {
		res.transportErr = err
		return res
	}
	defer resp.Body.Close()
	res.statusCode = resp.StatusCode
	limited := io.LimitReader(resp.Body, int64(maxResponseExcerpt)*4)
	raw, _ := io.ReadAll(limited)
	res.response = truncate(string(raw), maxResponseExcerpt)
	return res
}

func pinnedDialer(allowed []net.IP) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		// addr carries the URL host (usually a name) — we never re-resolve it here, that is
		// what reopens a DNS-rebinding race. Instead we dial the addresses validated up front,
		// each re-checked against the blocklist, and fail only if none can be reached.
		_, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		var lastErr error
		for _, ip := range allowed {
			if blockedIP(ip) {
				continue
			}
			var d net.Dialer
			conn, derr := d.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if derr == nil {
				return conn, nil
			}
			lastErr = derr
		}
		if lastErr == nil {
			lastErr = fmt.Errorf("endpoint resolved to no dialable address")
		}
		return nil, lastErr
	}
}

func resolveDialIPs(host string) ([]net.IP, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, fmt.Errorf("endpoint host required")
	}
	if ip := net.ParseIP(host); ip != nil {
		if blockedIP(ip) {
			return nil, fmt.Errorf("endpoint IP not allowed")
		}
		return []net.IP{ip}, nil
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("endpoint host lookup: %w", err)
	}
	var out []net.IP
	for _, ip := range ips {
		if blockedIP(ip) {
			return nil, fmt.Errorf("endpoint host resolves to a blocked address")
		}
		out = append(out, ip)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("endpoint host resolved to no addresses")
	}
	return out, nil
}

func blockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() ||
		ip.IsMulticast() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return true
		}
	}
	return false
}
