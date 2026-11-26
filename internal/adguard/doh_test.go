package adguard

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"sync"
	"testing"
)

const dohCaptures = "../../testdata/device/axt1800-25.12.6/adguard/"

func dohCapture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(dohCaptures + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// fakeAGH answers dns_info from the capture (upstream replaced after a dns_config POST) and
// test_upstream_dns with the captured ok or fail reply.
type fakeAGH struct {
	mu       sync.Mutex
	info     map[string]any
	testBody []byte
	tested   map[string]any
	configs  int
}

func newFakeAGH(t *testing.T, testFile string) (*fakeAGH, *Upstreams) {
	t.Helper()
	f := &fakeAGH{testBody: dohCapture(t, testFile)}
	if err := json.Unmarshal(dohCapture(t, "dns-info.json"), &f.info); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, &Upstreams{Client: srv.Client(), Base: srv.URL}
}

func (f *fakeAGH) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	body, _ := io.ReadAll(r.Body)
	var in map[string]any
	_ = json.Unmarshal(body, &in)
	switch r.URL.Path {
	case "/control/dns_info":
		_ = json.NewEncoder(w).Encode(f.info)
	case "/control/test_upstream_dns":
		f.tested = in
		_, _ = w.Write(f.testBody)
	case "/control/dns_config":
		f.configs++
		f.info["upstream_dns"] = in["upstream_dns"]
	default:
		http.NotFound(w, r)
	}
}

func TestDoHGetCaptured(t *testing.T) {
	_, u := newFakeAGH(t, "test-upstream-dns-ok.json")
	v, err := u.Get(t.Context())
	if err != nil || !v.Encrypted || !slices.Equal(v.Upstreams, []string{"https://dns10.quad9.net/dns-query"}) {
		t.Fatalf("%+v, %v", v, err)
	}
}

func TestDoHSet(t *testing.T) {
	f, u := newFakeAGH(t, "test-upstream-dns-ok.json")
	const cf = "https://cloudflare-dns.com/dns-query"
	v, err := u.Set(t.Context(), DoHRequest{URL: cf})
	if err != nil || !slices.Equal(v.Upstreams, []string{cf}) || f.configs != 1 {
		t.Fatalf("%+v, %v, configs %d", v, err, f.configs)
	}
	if boot, _ := f.tested["bootstrap_dns"].([]any); len(boot) != 4 {
		t.Errorf("test used bootstrap %v, want the captured four", f.tested["bootstrap_dns"])
	}
	if _, err := u.Set(t.Context(), DoHRequest{URL: cf}); !errors.Is(err, ErrDoHNoChange) {
		t.Errorf("repeat: %v", err)
	}
}

func TestDoHSetRefuses(t *testing.T) {
	f, u := newFakeAGH(t, "test-upstream-dns-fail.json")
	if _, err := u.Set(t.Context(), DoHRequest{URL: "https://doh.invalid/dns-query"}); !errors.Is(err, ErrDoHUnreachable) || f.configs != 0 {
		t.Errorf("unreachable: %v, configs %d", err, f.configs)
	}
	for _, bad := range []string{"http://dns.example/dns-query", "tls://one.one.one.one", "https://u:p@dns.example/q",
		"https:///dns-query", "https://dns.example/q?x=1", "https://dns.example/ q", ""} {
		if _, err := u.Set(t.Context(), DoHRequest{URL: bad}); !errors.Is(err, ErrDoHInvalid) {
			t.Errorf("%q: %v", bad, err)
		}
	}
	u.Idle = func() error { return ErrBusy }
	if _, err := u.Set(t.Context(), DoHRequest{URL: "https://dns.example/q"}); !errors.Is(err, ErrBusy) {
		t.Errorf("busy: %v", err)
	}
	down := &Upstreams{Client: http.DefaultClient, Base: "http://127.0.0.1:1"}
	if _, err := down.Get(t.Context()); !errors.Is(err, ErrAdminAPI) {
		t.Errorf("down: %v", err)
	}
}

func TestEncrypted(t *testing.T) {
	if Encrypted(nil) || Encrypted([]string{"https://a/q", "9.9.9.9"}) || !Encrypted([]string{"tls://a", "quic://b"}) {
		t.Error("Encrypted")
	}
}
