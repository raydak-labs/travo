package api

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/apitokens"
	"github.com/blackdark-org/travo-greenfield/internal/tailscale"
)

const pendingAuthURL = "https://login.tailscale.com/a/x"

func newTailscaleTokenEnv(t *testing.T) *env {
	t.Helper()
	fake := &fakeTailscale{status: tailscale.Status{Installed: true, Running: true,
		Backend: tailscale.Backend{State: tailscale.StateNeedsLogin, AuthURL: pendingAuthURL}}}
	store := apitokens.New(filepath.Join(t.TempDir(), "api-tokens.json"))
	return newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.Tokens = store; d.Tailscale = fake })
}

func TestTailscaleTokenCannotLoginLogoutOrChangeSettings(t *testing.T) {
	e := newTailscaleTokenEnv(t)
	_, secret := e.mint("admin")
	for _, r := range []req{
		{method: "POST", path: "/api/v1/tailscale/login", body: "{}"},
		{method: "POST", path: "/api/v1/tailscale/logout", body: "{}"},
		{method: "PUT", path: "/api/v1/tailscale/settings", body: `{"exit_node":"","advertise_lan":false,"ssh":true}`},
	} {
		r.auth = "Bearer " + secret
		if rec := e.do(r); rec.Code != http.StatusForbidden || errCode(t, rec) != "session_required" {
			t.Errorf("%s %s: %d %s", r.method, r.path, rec.Code, rec.Body)
		}
	}
}

func TestTailscaleAuthURLHiddenFromTokens(t *testing.T) {
	e := newTailscaleTokenEnv(t)
	_, secret := e.mint("read")
	cookie, _ := e.login()
	for _, tc := range []struct {
		r    req
		want string
	}{
		{req{path: "/api/v1/tailscale", auth: "Bearer " + secret}, ""},
		{req{path: "/api/v1/tailscale", cookie: cookie}, pendingAuthURL},
	} {
		rec := e.do(tc.r)
		if rec.Code != http.StatusOK || bodyField(t, rec, "auth_url") != tc.want {
			t.Errorf("auth=%q: %d %s", tc.r.auth, rec.Code, rec.Body)
		}
	}

	server := httptest.NewServer(e.h)
	defer server.Close()
	session := openEventStream(t, server.URL, cookie)
	defer session.Body.Close()
	token := openTokenEventStream(t, server.URL, secret)
	defer token.Body.Close()
	if got := tailscaleEventAuthURL(t, bufio.NewReader(session.Body)); got != pendingAuthURL {
		t.Errorf("session event auth_url %q", got)
	}
	if got := tailscaleEventAuthURL(t, bufio.NewReader(token.Body)); got != "" {
		t.Errorf("token event auth_url %q", got)
	}
}

func openTokenEventStream(t *testing.T, base, secret string) *http.Response {
	t.Helper()
	r, _ := http.NewRequest(http.MethodGet, base+"/api/v1/dashboard/events", nil)
	r.Header.Set("Authorization", "Bearer "+secret)
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("open token stream: %d", resp.StatusCode)
	}
	return resp
}

func tailscaleEventAuthURL(t *testing.T, reader *bufio.Reader) string {
	t.Helper()
	_ = readSSEFrame(t, reader)
	for range 10 {
		name, data := parseSSEEvent(t, readSSEFrame(t, reader))
		if name == "tailscale" {
			got, _ := data["auth_url"].(string)
			return got
		}
	}
	t.Fatal("no tailscale event")
	return ""
}
