package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/auth"
	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/trust"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/danielgtaylor/huma/v2"
)

const (
	rootPW  = "correct-horse"
	lanAddr = "192.168.1.50:40000"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// accounts wraps the mock device so tests can inject panics and count writes.
type accounts struct {
	*mock.Device
	mu            sync.Mutex
	panicOnShadow bool
	sets          int
}

func (a *accounts) RootPasswordSet(ctx context.Context) (bool, error) {
	if a.panicOnShadow {
		panic("boom")
	}
	return a.Device.RootPasswordSet(ctx)
}

func (a *accounts) SetRootPassword(ctx context.Context, pw string) error {
	a.mu.Lock()
	a.sets++
	a.mu.Unlock()
	return a.Device.SetRootPassword(ctx, pw)
}

type fakeTxns struct{ statuses map[string]txn.Status }

func (f fakeTxns) Get(id string) (txn.Status, error) {
	if st, ok := f.statuses[id]; ok {
		return st, nil
	}
	return txn.Status{}, txn.ErrNotFound
}

func (f fakeTxns) Confirm(id string) (txn.Status, error) {
	st, err := f.Get(id)
	if err != nil {
		return st, err
	}
	if st.State != txn.Pending {
		return st, txn.ErrClosed
	}
	st.State = txn.Confirmed
	return st, nil
}

func (f fakeTxns) Rollback(id string) (txn.Status, error) {
	st, err := f.Get(id)
	if err != nil {
		return st, err
	}
	st.State = txn.RolledBack
	return st, nil
}

type env struct {
	t     *testing.T
	deps  Deps
	h     http.Handler
	api   huma.API
	clock *fakeClock
	acc   *accounts
	dev   *mock.Device
	store *auth.Store
	epoch []uint64
	// saveErr makes SaveEpoch fail.
	saveErr error
}

const pendingTxn = "0123456789abcdef"

func newEnv(t *testing.T) *env {
	t.Helper()
	return newEnvWithBackup(t, nil)
}

func newEnvWithBackup(t *testing.T, backup device.ConfigBackupper) *env {
	t.Helper()
	return newEnvWithDevices(t, backup, nil)
}

func newEnvWithDevices(t *testing.T, backup device.ConfigBackupper, throughput ThroughputReader) *env {
	t.Helper()
	return newEnvWithDashboard(t, backup, throughput, nil)
}

func newEnvWithDashboard(t *testing.T, backup device.ConfigBackupper, throughput ThroughputReader, connectivity ConnectivityReader, configure ...func(*Deps)) *env {
	t.Helper()
	dev := mock.New()
	dev.SetPassword(rootPW)
	e := &env{t: t, dev: dev, acc: &accounts{Device: dev}, clock: &fakeClock{t: time.Now()}}
	e.store = auth.NewStore(zeroEntropy())
	lan := []netip.Prefix{netip.MustParsePrefix("192.168.1.0/24")}
	deps := Deps{
		Version: "1.2.3", Accounts: e.acc, Ubus: dev, Backup: backup, Throughput: throughput, Connectivity: connectivity,
		Txns: fakeTxns{map[string]txn.Status{
			pendingTxn:         {Meta: txn.Meta{ID: pendingTxn, Summary: "wifi", WindowSeconds: 60}, State: txn.Pending},
			"fedcba9876543210": {Meta: txn.Meta{ID: "fedcba9876543210"}, State: txn.Confirmed},
		}},
		Sessions: e.store, Limiter: auth.NewRateLimiter(), Clock: e.clock,
		Trusted:   func(a netip.Addr) bool { return trust.Allowed(a, lan) },
		RouterIPs: func() []netip.Addr { return []netip.Addr{netip.MustParseAddr("192.168.1.1")} },
		SaveEpoch: func(n uint64) error {
			if e.saveErr != nil {
				return e.saveErr
			}
			e.epoch = append(e.epoch, n)
			return nil
		},
	}
	for _, fn := range configure {
		fn(&deps)
	}
	e.deps = deps
	e.h, e.api = New(deps)
	return e
}

type req struct {
	method, path, body string
	remote, host       string
	cookie, csrf       string
	auth               string // raw Authorization header
	contentType        string // replaces application/json
	contentLength      int64  // replaces the body's length when set
	tls                bool
}

func (e *env) do(r req) *httptest.ResponseRecorder {
	e.t.Helper()
	var rd *strings.Reader
	if r.body != "" {
		rd = strings.NewReader(r.body)
	} else {
		rd = strings.NewReader("")
	}
	hr := httptest.NewRequest(r.method, r.path, rd)
	hr.RemoteAddr = lanAddr
	hr.Host = "192.168.1.1" // httptest defaults to example.com, which the host guard refuses
	if r.host != "" {
		hr.Host = r.host
	}
	if r.remote != "" {
		hr.RemoteAddr = r.remote
	}
	if r.body != "" {
		hr.Header.Set("Content-Type", "application/json")
	}
	if r.contentType != "" {
		hr.Header.Set("Content-Type", r.contentType)
	}
	if r.contentLength != 0 {
		hr.ContentLength = r.contentLength
	}
	if r.cookie != "" {
		hr.AddCookie(&http.Cookie{Name: cookieName, Value: r.cookie})
	}
	if r.csrf != "" {
		hr.Header.Set(csrfHeader, r.csrf)
	}
	if r.auth != "" {
		hr.Header.Set("Authorization", r.auth)
	}
	if r.tls {
		hr.TLS = &tlsState
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, hr)
	return rec
}

// login performs a real login and returns its cookie and csrf token.
func (e *env) login() (cookie, csrf string) {
	e.t.Helper()
	rec := e.do(req{method: "POST", path: "/api/v1/auth/login", body: `{"password":"` + rootPW + `"}`})
	if rec.Code != http.StatusOK {
		e.t.Fatalf("login: %d %s", rec.Code, rec.Body)
	}
	return cookieOf(e.t, rec), bodyField(e.t, rec, "csrf_token")
}

func cookieOf(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == cookieName {
			return c.Value
		}
	}
	t.Fatalf("no %s cookie in %v", cookieName, rec.Header())
	return ""
}

func bodyField(t *testing.T, rec *httptest.ResponseRecorder, key string) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("body %s: %v", rec.Body, err)
	}
	s, _ := m[key].(string)
	return s
}

func errCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var e struct{ Error, Code string }
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatalf("not an error body: %s", rec.Body)
	}
	if e.Error == "" {
		t.Fatalf("error body without message: %s", rec.Body)
	}
	return e.Code
}

var errTest = errors.New("injected")

func bodyBool(t *testing.T, rec *httptest.ResponseRecorder, key string) bool {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("body %s: %v", rec.Body, err)
	}
	b, ok := m[key].(bool)
	if !ok {
		t.Fatalf("%s missing in %s", key, rec.Body)
	}
	return b
}
