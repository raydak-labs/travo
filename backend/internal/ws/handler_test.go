package ws

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	fws "github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"

	"github.com/openwrt-travel-gui/backend/internal/auth"
)

const testSecret = "ws-test-secret"

// wsTestServer wires a Fiber app with the real upgrade middleware and handler.
type wsTestServer struct {
	URL     string
	Auth    *auth.AuthService
	Block   *auth.TokenBlocklist
	Registr *auth.SessionRegistry
	Hub     *Hub
	// done is closed when the test server is torn down, so a dial retry can
	// bail out instead of waiting on a dead listener.
	done chan struct{}
}

func newWSTestServer(t *testing.T, opts ...func(*wsTestServer)) *wsTestServer {
	t.Helper()

	ts := &wsTestServer{
		done:    make(chan struct{}),
		Auth:    auth.NewAuthService("admin", testSecret),
		Block:   auth.NewTokenBlocklist(),
		Registr: auth.NewSessionRegistry(time.Hour),
	}
	ts.Auth.SetBlocklist(ts.Block)
	ts.Auth.SetSessionRegistry(ts.Registr)
	for _, o := range opts {
		o(ts)
	}

	ts.Hub = newTestHub()
	ts.Hub.BroadcastInterval = 50 * time.Millisecond
	ts.Hub.Start()

	app := fiber.New()
	app.Use("/api/v1/ws", UpgradeMiddleware(ts.Auth))
	app.Get("/api/v1/ws", Handler(ts.Hub, ts.Auth))

	// A real listener is required: the upgrade hijacks the fasthttp
	// connection, which the in-process test transports do not provide.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() {
		_ = app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true})
	}()
	t.Cleanup(func() {
		close(ts.done)
		ts.Hub.Stop()
		_ = ln.Close()
	})
	ts.URL = "ws://" + ln.Addr().String()
	return ts
}

func loginToken(t *testing.T, ts *wsTestServer) string {
	t.Helper()
	token, _, err := ts.Auth.Login("admin")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	return token
}

func dialWS(t *testing.T, ts *wsTestServer, path string, header http.Header) (*fws.Conn, *http.Response, error) {
	t.Helper()
	// The Fiber app is served from a goroutine started just before this dial,
	// so on a loaded machine the accept loop may not be running yet. Give the
	// handshake a bounded budget and retry once rather than failing a test that
	// is only measuring scheduling latency.
	d := fws.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, resp, err := d.Dial(ts.URL+path, header)
	if err == nil {
		return conn, resp, nil
	}
	select {
	case <-time.After(250 * time.Millisecond):
	case <-ts.done:
	}
	return d.Dial(ts.URL+path, header)
}

// --- upgrade auth gate -------------------------------------------------------

func TestUpgradeMiddleware_RejectsMissingToken(t *testing.T) {
	ts := newWSTestServer(t)
	_, resp, err := dialWS(t, ts, "/api/v1/ws?token=", nil)
	if err == nil {
		t.Fatal("expected the upgrade to be rejected without a token")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %v", resp)
	}
}

func TestUpgradeMiddleware_RejectsInvalidToken(t *testing.T) {
	ts := newWSTestServer(t)
	_, resp, err := dialWS(t, ts, "/api/v1/ws?token=not-a-jwt", nil)
	if err == nil {
		t.Fatal("expected the upgrade to be rejected for a garbage token")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %v", resp)
	}
}

func TestUpgradeMiddleware_RejectsExpiredToken(t *testing.T) {
	ts := newWSTestServer(t)
	expired := signToken(t, "expired-jti", time.Now().Add(-time.Hour))
	_, resp, err := dialWS(t, ts, "/api/v1/ws?token="+expired, nil)
	if err == nil {
		t.Fatal("expected the upgrade to be rejected for an expired token")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %v", resp)
	}
}

func TestUpgradeMiddleware_RejectsRevokedToken(t *testing.T) {
	ts := newWSTestServer(t)
	token := loginToken(t, ts)
	ts.Auth.RevokeSession(token)
	ts.Block.Block(token, time.Now().Add(time.Hour))

	_, resp, err := dialWS(t, ts, "/api/v1/ws?token="+token, nil)
	if err == nil {
		t.Fatal("expected the upgrade to be rejected for a revoked token")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %v", resp)
	}
}

func TestUpgradeMiddleware_AcceptsValidToken(t *testing.T) {
	ts := newWSTestServer(t)
	conn, _, err := dialWS(t, ts, "/api/v1/ws?token="+loginToken(t, ts), nil)
	if err != nil {
		t.Fatalf("expected the upgrade to succeed, got %v", err)
	}
	defer conn.Close()
	// Registration happens in the server-side handler, which runs after the
	// client's Dial returns, so the count is not observable yet. Asserting
	// immediately is a race that only shows up on a loaded runner.
	waitForClientCount(t, ts.Hub, 1)
}

// --- origin ----------------------------------------------------------------

func TestUpgradeMiddleware_RejectsCrossSiteOrigin(t *testing.T) {
	ts := newWSTestServer(t)
	h := http.Header{"Origin": []string{"http://evil.example"}}

	_, resp, err := dialWS(t, ts, "/api/v1/ws?token="+loginToken(t, ts), h)
	if err == nil {
		t.Fatal("expected a cross-site WebSocket upgrade to be rejected")
	}
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %v", resp)
	}
}

func TestUpgradeMiddleware_AllowsSameOrigin(t *testing.T) {
	ts := newWSTestServer(t)
	host := strings.TrimPrefix(ts.URL, "ws://")
	h := http.Header{"Origin": []string{"ws://" + host}}

	conn, _, err := dialWS(t, ts, "/api/v1/ws?token="+loginToken(t, ts), h)
	if err != nil {
		t.Fatalf("expected a same-origin upgrade to succeed, got %v", err)
	}
	conn.Close()
}

func TestOriginAllowed(t *testing.T) {
	tests := []struct {
		name    string
		origin  string
		host    string
		allowed []string
		want    bool
	}{
		{"no origin (non-browser client)", "", "192.168.1.1", nil, true},
		{"same host", "http://192.168.1.1", "192.168.1.1", nil, true},
		{"same host and port", "http://192.168.1.1:8080", "192.168.1.1:8080", nil, true},
		{"default port dropped", "https://travo.lan", "travo.lan:443", nil, true},
		{"different host", "http://evil.example", "192.168.1.1", nil, false},
		{"different port", "http://192.168.1.1:3001", "192.168.1.1:3000", nil, false},
		{"subdomain of same host", "http://evil.192.168.1.1", "192.168.1.1", nil, false},
		{"explicitly allowed", "http://ui.example:5173", "192.168.1.1", []string{"http://ui.example:5173"}, true},
		{"allow-list miss", "http://ui.example:5173", "192.168.1.1", []string{"http://other.example"}, false},
		{"wildcard is not same-origin", "*", "192.168.1.1", []string{"*"}, false},
		{"malformed origin", "not-a-url", "192.168.1.1", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := originAllowed(tc.origin, tc.host, tc.allowed); got != tc.want {
				t.Errorf("originAllowed(%q, %q, %v) = %v, want %v", tc.origin, tc.host, tc.allowed, got, tc.want)
			}
		})
	}
}

// A wildcard CORS setting must not become "every origin may open a socket".
func TestResolveAllowedOrigins_DropsWildcard(t *testing.T) {
	if got := resolveAllowedOrigins([]string{"*", "http://ui.example"}); len(got) != 1 || got[0] != "http://ui.example" {
		t.Errorf("expected the wildcard to be dropped, got %v", got)
	}
}

// The CORS allowlist and the WebSocket origin check must agree, otherwise an
// operator who passes --cors-origins on the command line ends up with a UI that
// loads but a socket that never opens.
func TestResolveAllowedOrigins_ReadsEnvCORSOrigins(t *testing.T) {
	env := func(k string) string {
		if k == "CORS_ORIGINS" {
			return "http://localhost:5173, https://travo.example/"
		}
		return ""
	}
	got := resolveAllowedOriginsFrom(nil, nil, env)
	want := []string{"http://localhost:5173", "https://travo.example"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("resolveAllowedOriginsFrom(env) = %v, want %v", got, want)
	}
}

func TestResolveAllowedOrigins_FallsBackToCLIArg(t *testing.T) {
	env := func(string) string { return "" }
	for _, args := range [][]string{
		{"--cors-origins=http://localhost:5173"},
		{"-cors-origins", "http://localhost:5173"},
		{"--port", "8080", "--cors-origins=http://localhost:5173"},
		{"--static-dir", "--cors-origins=http://localhost:5173"},
	} {
		got := resolveAllowedOriginsFrom(nil, args, env)
		if len(got) != 1 || got[0] != "http://localhost:5173" {
			t.Errorf("resolveAllowedOriginsFrom(%v) = %v, want [http://localhost:5173]", args, got)
		}
	}
}

func TestResolveAllowedOrigins_ExplicitListWinsOverEnvAndArgs(t *testing.T) {
	env := func(string) string { return "http://from-env.example" }
	got := resolveAllowedOriginsFrom(
		[]string{"http://explicit.example"},
		[]string{"--cors-origins=http://from-cli.example"},
		env,
	)
	if len(got) != 1 || got[0] != "http://explicit.example" {
		t.Errorf("expected the explicit list to win, got %v", got)
	}
}

// No configuration at all must mean same-origin only, never "any origin".
func TestResolveAllowedOrigins_NoConfigIsSameOriginOnly(t *testing.T) {
	env := func(string) string { return "   " }
	if got := resolveAllowedOriginsFrom(nil, []string{"--mock"}, env); len(got) != 0 {
		t.Errorf("expected an empty allowlist, got %v", got)
	}
}

// --- live stream ------------------------------------------------------------

// After logout the hub must stop pushing to the socket: the handler
// re-validates the session and closes the connection.
func TestHandler_StopsStreamingAfterLogout(t *testing.T) {
	ts := newWSTestServer(t)
	withFastKeepalive(t)
	token := loginToken(t, ts)

	conn, _, err := dialWS(t, ts, "/api/v1/ws?token="+token, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	waitForClientCount(t, ts.Hub, 1)

	// The client must be able to receive pushes while the session is live.
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, msg, err := conn.ReadMessage(); err != nil {
		t.Fatalf("expected a broadcast before logout, got %v", err)
	} else if !strings.Contains(string(msg), "system_stats") {
		t.Errorf("unexpected message %q", msg)
	}

	// Logout on the server side.
	ts.Auth.RevokeSession(token)
	ts.Block.Block(token, time.Now().Add(time.Hour))

	// The socket must be closed by the server, not left streaming.
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
	waitForClientCount(t, ts.Hub, 0)
}

func TestHandler_SetsReadLimit(t *testing.T) {
	ts := newWSTestServer(t)
	conn, _, err := dialWS(t, ts, "/api/v1/ws?token="+loginToken(t, ts), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	waitForClientCount(t, ts.Hub, 1)

	// A frame far beyond the read limit must be refused instead of buffered.
	oversized := strings.Repeat("A", 64<<10)
	if err := conn.WriteMessage(fws.TextMessage, []byte(oversized)); err != nil {
		// Acceptable too: the server may hang up as soon as the limit trips,
		// before the whole frame reached the socket.
		t.Logf("write rejected by the server: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var readErr error
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			readErr = err
			break
		}
	}
	if readErr == nil {
		t.Fatal("expected the server to close the connection after an oversized frame")
	}
	var closeErr *fws.CloseError
	if errors.As(readErr, &closeErr) && closeErr.Code != fws.CloseMessageTooBig {
		t.Errorf("expected close code %d (message too big), got %d", fws.CloseMessageTooBig, closeErr.Code)
	}
}

// A small ping from the client must be answered, proving the pong/ping
// keepalive is live on an upgraded socket.
func TestHandler_SendsPings(t *testing.T) {
	ts := newWSTestServer(t)
	withFastKeepalive(t)
	conn, _, err := dialWS(t, ts, "/api/v1/ws?token="+loginToken(t, ts), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	pongs := make(chan struct{}, 1)
	conn.SetPingHandler(func(string) error {
		select {
		case pongs <- struct{}{}:
		default:
		}
		return nil
	})
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
	select {
	case <-pongs:
	case <-time.After(3 * time.Second):
		t.Fatal("expected the server to send periodic pings")
	}
}

// --- helpers ----------------------------------------------------------------

func signToken(t *testing.T, jti string, exp time.Time) string {
	t.Helper()
	claims := jwt.RegisteredClaims{
		ID:        jti,
		ExpiresAt: jwt.NewNumericDate(exp),
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		Subject:   "admin",
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("signing token: %v", err)
	}
	return signed
}

// withFastKeepalive shrinks the keepalive/revalidation cadence so the tests do
// not have to wait 30-90s for the server to notice a revoked session.
func withFastKeepalive(t *testing.T) {
	t.Helper()
	origPing, origDeadline, origRevalidate := pingInterval, readDeadline, revalidateInterval
	pingInterval = 50 * time.Millisecond
	revalidateInterval = 50 * time.Millisecond
	readDeadline = 3 * time.Second
	t.Cleanup(func() {
		pingInterval, readDeadline, revalidateInterval = origPing, origDeadline, origRevalidate
	})
}

// waitForClientCount polls until the hub holds want clients, or fails. The
// registration runs on the server goroutine, so it is not observable the
// instant the client's handshake returns; the budget is generous because a
// loaded CI runner can schedule that goroutine well after the dial completes.
func waitForClientCount(t *testing.T, hub *Hub, want int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if hub.ClientCount() == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("client count = %d, want %d", hub.ClientCount(), want)
}
