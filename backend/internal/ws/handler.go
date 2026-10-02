package ws

import (
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/auth"
)

// readLimit caps a single inbound frame. The protocol is server → client
// push; the browser client never sends anything, so a few KiB is generous.
// Without a limit a single multi-GB frame is buffered on a 128 MB router.
const readLimit = 4 << 10

// HandlerOptions tunes the keepalive loop. The zero value is the production
// default; tests shorten the intervals so they do not have to wait 30-90s for
// the server to notice a revoked session.
type HandlerOptions struct {
	// PingInterval is how often the server pings an idle client. Pongs refresh
	// the read deadline, so a dead peer is detected even when the router's NAT
	// silently drops the socket.
	PingInterval time.Duration
	// ReadDeadline bounds a blocked read. It is refreshed by every pong, so a
	// client that cannot answer pings is dropped instead of holding a socket
	// open (and holding a goroutine) on the router.
	ReadDeadline time.Duration
	// RevalidateInterval is how often an already-upgraded socket re-checks its
	// session. Without it a logged-out or expired session would keep receiving
	// system_stats, alerts and network_status until it reconnects.
	RevalidateInterval time.Duration
}

// withDefaults fills in any unset field.
func (o HandlerOptions) withDefaults() HandlerOptions {
	if o.PingInterval <= 0 {
		o.PingInterval = 30 * time.Second
	}
	if o.ReadDeadline <= 0 {
		o.ReadDeadline = 90 * time.Second
	}
	if o.RevalidateInterval <= 0 {
		o.RevalidateInterval = 30 * time.Second
	}
	return o
}

// Handler returns a Fiber handler for WebSocket connections. opts tunes the
// keepalive loop; pass a zero HandlerOptions for the production defaults.
func Handler(hub *Hub, authSvc *auth.AuthService, opts HandlerOptions) fiber.Handler {
	opts = opts.withDefaults()
	return websocket.New(func(c *websocket.Conn) {
		// The middleware already validated the token; the handler keeps it
		// so the session can be re-validated while the socket is open.
		token := c.Query("token")
		if token == "" {
			closeWithPolicyViolation(c, "missing session token")
			return
		}
		// websocket.New leaves the socket open when the handler returns
		// normally, so closing is this handler's job.
		defer func() { _ = c.Close() }()

		hub.Register(c)
		defer hub.Unregister(c)

		c.SetReadLimit(readLimit)
		_ = c.SetReadDeadline(time.Now().Add(opts.ReadDeadline))
		c.SetPongHandler(func(string) error {
			return c.SetReadDeadline(time.Now().Add(opts.ReadDeadline))
		})

		stopKeepalive := make(chan struct{})
		defer close(stopKeepalive)
		go keepalive(c, authSvc, token, stopKeepalive, opts)

		for {
			if _, _, err := c.ReadMessage(); err != nil {
				// Any read error ends the session: the peer went away, sent a
				// frame beyond the read limit, or failed to answer pings
				// within ReadDeadline. The deadline is only ever refreshed by
				// a pong, so it is a real liveness check, not an idle timer.
				return
			}
		}
	})
}

// keepalive pings the client and re-validates its session. Both operations
// use WriteControl, which the WebSocket library allows concurrently with the
// hub's broadcast writes. On an invalid session the socket is closed, which
// unblocks the handler's read.
func keepalive(c *websocket.Conn, authSvc *auth.AuthService, token string, stop <-chan struct{}, opts HandlerOptions) {
	ping := time.NewTicker(opts.PingInterval)
	defer ping.Stop()
	revalidate := time.NewTicker(opts.RevalidateInterval)
	defer revalidate.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ping.C:
			if err := c.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeTimeout)); err != nil {
				return
			}
		case <-revalidate.C:
			if err := authSvc.ValidateToken(token); err != nil {
				closeWithPolicyViolation(c, "session is no longer valid")
				return
			}
			if bl := authSvc.Blocklist(); bl != nil && bl.IsBlocked(token) {
				closeWithPolicyViolation(c, "session is no longer valid")
				return
			}
		}
	}
}

// closeWithPolicyViolation tells the peer why the socket goes away before
// closing it, so the client can stop reconnecting with a dead token.
func closeWithPolicyViolation(c *websocket.Conn, reason string) {
	_ = c.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.ClosePolicyViolation, reason),
		time.Now().Add(writeTimeout),
	)
	_ = c.Close()
}

// UpgradeMiddleware checks if the request is a WebSocket upgrade and validates
// the JWT token. allowedOrigins is the same list the CORS middleware was built
// from (cfg.CorsOrigins, split and passed in by the caller), so the two
// policies cannot disagree; a mismatch would let a UI load and then fail to
// open a socket. An empty list means same-origin only.
func UpgradeMiddleware(authSvc *auth.AuthService, allowedOrigins []string) fiber.Handler {
	origins := normalizeOrigins(allowedOrigins)
	return func(c fiber.Ctx) error {
		if !websocket.IsWebSocketUpgrade(c) {
			return fiber.ErrUpgradeRequired
		}

		// Origin is checked before the token: a cross-site page must not be
		// able to probe which tokens are valid.
		if !originAllowed(c.Get("Origin"), c.Host(), origins) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "origin not allowed",
			})
		}

		// Validate JWT from query parameter. NOTE: a query token lands in
		// access logs, browser history and Referer headers; the frontend
		// still uses it, so it stays for now and should move to a header or
		// Sec-WebSocket-Protocol in a follow-up.
		token := c.Query("token")
		if token == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "missing token",
			})
		}

		if err := authSvc.ValidateToken(token); err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "invalid token",
			})
		}

		// Check blocklist
		bl := authSvc.Blocklist()
		if bl != nil && bl.IsBlocked(token) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "token has been revoked",
			})
		}

		return c.Next()
	}
}

// normalizeOrigins trims a configured origin list. A wildcard ("*") is
// deliberately dropped: unlike a CORS preflight, a cross-site WebSocket is a
// credentialed request, so the effective policy is same-origin plus whatever
// origins were actually configured.
func normalizeOrigins(list []string) []string {
	out := make([]string, 0, len(list))
	for _, o := range list {
		o = strings.TrimRight(strings.TrimSpace(o), "/")
		if o == "" || o == "*" {
			continue
		}
		out = append(out, o)
	}
	return out
}

// originAllowed reports whether a request with this Origin may open a socket.
// An absent Origin means a non-browser client (curl, a Go client), which
// cannot be a cross-site request, so it is accepted.
func originAllowed(origin, host string, allowed []string) bool {
	if origin == "" {
		return true
	}
	if sameOrigin(origin, host) {
		return true
	}
	for _, a := range allowed {
		// "*" never matches here: a cross-site WebSocket carries the user's
		// cookies/credentials, which is exactly what a CORS wildcard may not
		// enable.
		if a != "" && a != "*" && strings.EqualFold(a, origin) {
			return true
		}
	}
	return false
}

// sameOrigin compares an Origin with the request Host. The hostnames must be
// equal and the ports must agree, except that the scheme's default port is
// ignored (http://host == Host: host:80). A missing port on either side is
// treated as "unspecified" — a router is reached as http://host from the
// browser while the request Host may carry the listener's port.
func sameOrigin(origin, host string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	originHost, originPort := splitHostPort(u.Host)
	reqHost, reqPort := splitHostPort(host)
	if !strings.EqualFold(originHost, reqHost) {
		return false
	}
	switch {
	case originPort == reqPort:
		return true
	case originPort == "":
		return reqPort == "" || isDefaultPort(reqPort, u.Scheme)
	case reqPort == "":
		return true
	default:
		return false
	}
}

func splitHostPort(hostport string) (host, port string) {
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		// No port, or a bare IPv6 literal without brackets.
		return strings.ToLower(hostport), ""
	}
	return strings.ToLower(host), port
}

func isDefaultPort(port, scheme string) bool {
	return (scheme == "http" && port == "80") || (scheme == "https" && port == "443")
}
