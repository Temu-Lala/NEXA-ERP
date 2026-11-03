package models

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// SMTP probe for email connections. A connection test must reach the real mail server, so this
// opens an actual socket — never a simulated one. Security mirrors the HTTPS sender: the host is
// resolved once, its addresses are re-checked against the blocklist and the dial is pinned to that
// validated IP (no re-resolution, so no DNS-rebinding race). Transport security is enforced: an
// smtps endpoint (or port 465) uses implicit TLS from the first byte; otherwise STARTTLS is
// required before credentials are ever sent — plaintext auth over an unprotected connection is
// refused. Credentials are only attempted when configured AND the server advertises AUTH; the
// probe never sends a message. The final SMTP reply code is returned so the shared outcome
// classifier treats a 2xx/3xx response as success and a 4xx/5xx rejection as a truthful failure.

type smtpRequest struct {
	endpoint   string
	user       string
	password   string
	timeoutSec int
}

// smtpProbe performs the guarded handshake and returns the same result shape as the HTTP sender.
func smtpProbe(ctx context.Context, req smtpRequest) outboundResult {
	start := time.Now()
	res := outboundResult{}
	finish := func() { res.durationMs = int(time.Since(start).Milliseconds()) }

	t, err := parseSMTPTarget(req.endpoint)
	if err != nil {
		res.transportErr = err
		finish()
		return res
	}
	ips, err := resolveDialIPs(t.host)
	if err != nil {
		res.transportErr = err
		finish()
		return res
	}

	to := time.Duration(timeoutSeconds(req.timeoutSec)) * time.Second
	dialCtx, cancel := context.WithTimeout(ctx, to)
	defer cancel()

	dialer := net.Dialer{Timeout: to}
	conn, err := dialer.DialContext(dialCtx, "tcp", net.JoinHostPort(ips[0].String(), strconv.Itoa(t.port)))
	if err != nil {
		res.transportErr = err
		finish()
		return res
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(to))

	br := bufio.NewReader(conn)

	// Implicit TLS: upgrade before the greeting is read.
	if t.tls == "implicit" {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: t.host, MinVersion: tls.VersionTLS12})
		if err := tlsConn.HandshakeContext(dialCtx); err != nil {
			res.transportErr = err
			finish()
			return res
		}
		conn = tlsConn
		br = bufio.NewReader(conn)
	}

	code, greet, err := readSMTPReply(br)
	if err != nil {
		res.transportErr = fmt.Errorf("reading greeting: %w", err)
		finish()
		return res
	}
	if code != 220 {
		res.statusCode = code
		res.response = truncate(greet, maxResponseExcerpt)
		finish()
		return res
	}

	ehloHost := t.host
	code, caps, err := cmdSMTP(conn, br, "EHLO "+ehloHost)
	if err != nil {
		res.transportErr = err
		finish()
		return res
	}
	if code != 250 {
		res.statusCode = code
		res.response = truncate(firstLine(greet+" "+caps), maxResponseExcerpt)
		finish()
		return res
	}

	// Opportunistic STARTTLS: required before any credential is sent.
	needAuth := strings.TrimSpace(req.user) != "" && strings.TrimSpace(req.password) != ""
	authMechs := strings.ToUpper(caps)
	if t.tls == "starttls" {
		if strings.Contains(authMechs, "STARTTLS") {
			code, _, err = cmdSMTP(conn, br, "STARTTLS")
			if err != nil {
				res.transportErr = err
				finish()
				return res
			}
			if code != 220 {
				res.statusCode = code
				res.response = truncate("STARTTLS rejected: "+firstLine(caps), maxResponseExcerpt)
				finish()
				return res
			}
			tlsConn := tls.Client(conn, &tls.Config{ServerName: t.host, MinVersion: tls.VersionTLS12})
			if err := tlsConn.HandshakeContext(dialCtx); err != nil {
				res.transportErr = err
				finish()
				return res
			}
			conn = tlsConn
			br = bufio.NewReader(conn)
			code, caps, err = cmdSMTP(conn, br, "EHLO "+ehloHost)
			if err != nil {
				res.transportErr = err
				finish()
				return res
			}
			authMechs = strings.ToUpper(caps)
		} else if needAuth {
			res.statusCode = code
			res.response = truncate("server does not advertise STARTTLS; refusing to send credentials over plaintext", maxResponseExcerpt)
			res.transportErr = errInsecureScheme
			finish()
			return res
		}
	}

	final := code
	response := firstLine(caps)

	if needAuth {
		switch {
		case strings.Contains(authMechs, "AUTH PLAIN"):
			blob := base64.StdEncoding.EncodeToString([]byte("\x00" + req.user + "\x00" + req.password))
			final, response, err = cmdSMTP(conn, br, "AUTH PLAIN "+blob)
		case strings.Contains(authMechs, "AUTH LOGIN"):
			final, response, err = cmdSMTP(conn, br, "AUTH LOGIN")
			if err == nil && final == 334 {
				final, response, err = cmdSMTP(conn, br, base64.StdEncoding.EncodeToString([]byte(req.user)))
				if err == nil && final == 334 {
					final, response, err = cmdSMTP(conn, br, base64.StdEncoding.EncodeToString([]byte(req.password)))
				}
			}
		default:
			res.statusCode = final
			res.response = truncate("server advertises no supported AUTH mechanism (PLAIN/LOGIN)", maxResponseExcerpt)
			res.transportErr = fmt.Errorf("no supported SMTP auth mechanism")
			finish()
			return res
		}
		if err != nil {
			res.transportErr = err
			finish()
			return res
		}
	}

	_, _, _ = cmdSMTPNoRead(conn, "QUIT")

	res.statusCode = final
	res.response = truncate(response, maxResponseExcerpt)
	finish()
	return res
}

// readSMTPReply reads one (possibly multi-line) SMTP reply and returns its code and joined text.
func readSMTPReply(br *bufio.Reader) (int, string, error) {
	code := 0
	var sb strings.Builder
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return code, sb.String(), err
		}
		line = strings.TrimRight(line, "\r\n")
		if len(line) < 3 {
			continue
		}
		c, cerr := strconv.Atoi(line[:3])
		if cerr == nil {
			code = c
		}
		if sb.Len() > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(line)
		// A '-' at position 3 means more continuation lines follow.
		if len(line) >= 4 && line[3] == '-' {
			continue
		}
		break
	}
	return code, strings.TrimSpace(sb.String()), nil
}

func cmdSMTP(conn net.Conn, br *bufio.Reader, cmd string) (int, string, error) {
	if _, err := fmt.Fprintf(conn, "%s\r\n", cmd); err != nil {
		return 0, "", err
	}
	return readSMTPReply(br)
}

func cmdSMTPNoRead(conn net.Conn, cmd string) (int, string, error) {
	_, err := fmt.Fprintf(conn, "%s\r\n", cmd)
	return 0, "", err
}

func firstLine(s string) string {
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		return s[:i]
	}
	return s
}
