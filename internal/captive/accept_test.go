package captive

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/connectivity"
)

const simTerms = `<form method="post" action="/accept"><input type="hidden" name="token" value="sim-1">%s
<input type="checkbox" name="terms" value="yes"><button type="submit" name="go" value="connect">Connect</button></form>`

// simPortal mirrors scripts/portal-sim.py: unauthenticated until a valid accept, which
// also needs the session cookie set on the login page.
type simPortal struct {
	mode     string // redirect, intercept, room, meta
	accepted atomic.Bool
	server   *httptest.Server
}

func newSimPortal(t *testing.T, mode string) *simPortal {
	p := &simPortal{mode: mode}
	p.server = httptest.NewServer(http.HandlerFunc(p.serve))
	t.Cleanup(p.server.Close)
	return p
}

func (p *simPortal) form() string {
	extra := ""
	if p.mode == "room" {
		extra = `<input type="text" name="room">`
	}
	return fmt.Sprintf(simTerms, extra)
}

func (p *simPortal) serve(w http.ResponseWriter, r *http.Request) {
	switch {
	case p.accepted.Load():
		w.WriteHeader(204)
	case r.Method == http.MethodPost && r.URL.Path == "/accept":
		_ = r.ParseForm()
		if c, err := r.Cookie("sid"); err == nil && c.Value == "s1" && r.PostForm.Get("terms") == "yes" && r.PostForm.Get("token") == "sim-1" {
			p.accepted.Store(true)
			http.Redirect(w, r, "/welcome", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte(p.form()))
	case p.mode == "intercept":
		http.SetCookie(w, &http.Cookie{Name: "sid", Value: "s1"})
		_, _ = w.Write([]byte(p.form()))
	case r.URL.Path == "/login" && p.mode == "meta":
		_, _ = w.Write([]byte(`<meta http-equiv="refresh" content="0; url=/terms">`))
	case r.URL.Path == "/login" || r.URL.Path == "/terms":
		http.SetCookie(w, &http.Cookie{Name: "sid", Value: "s1"})
		_, _ = w.Write([]byte(p.form()))
	default:
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

// portalConn reports captive at the portal until it accepted us.
type portalConn struct {
	portal      *simPortal
	url         string
	invalidated atomic.Int32
}

func (c *portalConn) Status(context.Context) connectivity.Status {
	if c.portal.accepted.Load() {
		return connectivity.Status{Internet: connectivity.Online}
	}
	u := c.url
	return connectivity.Status{Internet: connectivity.Captive, PortalURL: &u}
}
func (c *portalConn) Invalidate() { c.invalidated.Add(1) }

type fakeRenew struct {
	state RenewState
	err   error
	calls int
	after func()
}

func (f *fakeRenew) Availability(context.Context, connectivity.Status) RenewState { return f.state }
func (f *fakeRenew) Renew(context.Context) error {
	f.calls++
	if f.after != nil {
		f.after()
	}
	return f.err
}

func testClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar, CheckRedirect: limitRedirects}
}

func TestAcceptWalksClickThroughPortals(t *testing.T) {
	for _, tc := range []struct {
		mode, start string
		want        AcceptOutcome
		steps       []string
	}{
		{"redirect", "/generate_204", AcceptOnline, []string{"fetch", "submit"}},
		{"intercept", "/generate_204", AcceptOnline, []string{"fetch", "submit"}},
		{"meta", "/login", AcceptOnline, []string{"fetch", "follow", "submit"}},
		{"room", "/login", AcceptNeedsUser, []string{"fetch"}},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			p := newSimPortal(t, tc.mode)
			conn := &portalConn{portal: p, url: p.server.URL + tc.start}
			renew := &fakeRenew{state: RenewAvailable}
			got, err := NewAcceptor(conn, renew, testClient).Accept(t.Context())
			if err != nil || got.Outcome != tc.want || renew.calls != 0 || got.Renewed {
				t.Fatalf("got %+v %v, renews %d", got, err, renew.calls)
			}
			if kinds := stepKinds(got); !slices.Equal(kinds, tc.steps) {
				t.Fatalf("steps %v, want %v", kinds, tc.steps)
			}
			if got.Steps[0].Status != 200 || got.Steps[0].Host == "" {
				t.Fatalf("first step %+v", got.Steps[0])
			}
		})
	}
}

func TestAcceptNotCaptive(t *testing.T) {
	conn := &fakeConn{internet: connectivity.Online}
	if _, err := NewAcceptor(conn, nil, testClient).Accept(t.Context()); !errors.Is(err, ErrAcceptNotCaptive) {
		t.Fatal(err)
	}
}

func TestAcceptBusy(t *testing.T) {
	a := NewAcceptor(&fakeConn{internet: connectivity.Captive}, nil, testClient)
	a.inflight.Lock()
	defer a.inflight.Unlock()
	if _, err := a.Accept(t.Context()); !errors.Is(err, ErrAcceptBusy) {
		t.Fatal(err)
	}
}

// Step 0 renews once after a transport failure and retries once; a retry that still
// fails ends the walk.
func TestAcceptRenewsOnceAtStep0(t *testing.T) {
	dead := httptest.NewServer(http.NotFoundHandler())
	dead.Close()
	conn := &portalConn{portal: &simPortal{}, url: dead.URL + "/login"}
	renew := &fakeRenew{state: RenewAvailable}
	got, err := NewAcceptor(conn, renew, testClient).Accept(t.Context())
	if err != nil || got.Outcome != AcceptFailed || renew.calls != 1 || !got.Renewed {
		t.Fatalf("got %+v %v renews %d", got, err, renew.calls)
	}
	if kinds := stepKinds(got); !slices.Equal(kinds, []string{"fetch", "renew", "fetch"}) {
		t.Fatal(kinds)
	}
}

func TestAcceptRenewThenLogin(t *testing.T) {
	p := newSimPortal(t, "redirect")
	var down atomic.Bool
	down.Store(true)
	flaky := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if down.Load() {
			panic(http.ErrAbortHandler) // a transport failure, like a stale lease
		}
		p.serve(w, r)
	}))
	defer flaky.Close()
	conn := &portalConn{portal: p, url: flaky.URL + "/generate_204"}
	renew := &fakeRenew{state: RenewAvailable, after: func() { down.Store(false) }}
	got, err := NewAcceptor(conn, renew, testClient).Accept(t.Context())
	if err != nil || got.Outcome != AcceptOnline || renew.calls != 1 || !got.Renewed {
		t.Fatalf("got %+v %v renews %d", got, err, renew.calls)
	}
}

func TestAcceptSkipsRenew(t *testing.T) {
	for _, renew := range []*fakeRenew{
		{state: RenewBlocked},       // a kept guard: the feature skips
		{state: RenewNotApplicable}, // Ethernet uplink or wwan not the default route
	} {
		dead := httptest.NewServer(http.NotFoundHandler())
		u := dead.URL
		dead.Close()
		conn := &portalConn{portal: &simPortal{}, url: u}
		got, err := NewAcceptor(conn, renew, testClient).Accept(t.Context())
		if err != nil || got.Outcome != AcceptFailed || renew.calls != 0 || got.Renewed {
			t.Fatalf("%s: got %+v %v", renew.state, got, err)
		}
	}
}

// A later step that fails at transport level never renews.
func TestAcceptNoRenewAfterStep0(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) > 1 {
			panic(http.ErrAbortHandler)
		}
		_, _ = w.Write([]byte(`<meta http-equiv="refresh" content="0; url=/next">`))
	}))
	defer server.Close()
	conn := &portalConn{portal: &simPortal{}, url: server.URL}
	renew := &fakeRenew{state: RenewAvailable}
	got, _ := NewAcceptor(conn, renew, testClient).Accept(t.Context())
	if got.Outcome != AcceptFailed || renew.calls != 0 {
		t.Fatalf("got %+v renews %d", got, renew.calls)
	}
}

func TestAcceptStepBound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<meta http-equiv="refresh" content="0; url=/again">`))
	}))
	defer server.Close()
	conn := &portalConn{portal: &simPortal{}, url: server.URL}
	got, _ := NewAcceptor(conn, nil, testClient).Accept(t.Context())
	if got.Outcome != AcceptFailed || len(got.Steps) != 1+acceptSteps || conn.invalidated.Load() != acceptSteps {
		t.Fatalf("got %+v invalidated %d", got, conn.invalidated.Load())
	}
}

func TestPortalClientRefusesLocalTargets(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, err := NewPortalClient().Get(server.URL)
	if !errors.Is(err, errLocalTarget) {
		t.Fatal(err)
	}
}

func stepKinds(r AcceptResult) []string {
	out := []string{}
	for _, s := range r.Steps {
		out = append(out, s.Kind)
	}
	return out
}
