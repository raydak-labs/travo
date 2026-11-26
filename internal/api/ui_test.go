package api

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/auth"
)

func TestUIRoutes(t *testing.T) {
	ui := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ui")) })
	build := func(ui http.Handler) http.Handler {
		h, _ := New(Deps{
			Version: "x", Txns: fakeTxns{}, Sessions: auth.NewStore(zeroEntropy()), Limiter: auth.NewRateLimiter(),
			Clock:     &fakeClock{t: time.Now()},
			Trusted:   func(a netip.Addr) bool { return a == netip.MustParseAddr("192.168.1.50") },
			RouterIPs: func() []netip.Addr { return []netip.Addr{netip.MustParseAddr("192.168.1.1")} },
			UI:        ui,
		})
		return h
	}
	for _, tc := range []struct {
		name, remote, host, path string
		status                   int
		code                     string
	}{
		{"lan ok", lanAddr, "192.168.1.1", "/", 200, ""},
		{"asset", lanAddr, "192.168.1.1", "/app-AbCd1234.js", 200, ""},
		{"api keeps precedence", lanAddr, "192.168.1.1", "/api/health", 200, ""},
		{"not lan", "8.8.8.8:1234", "192.168.1.1", "/", 403, "not_lan"},
		{"bad host", lanAddr, "evil.example", "/", 421, "bad_host"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", tc.path, nil)
			r.RemoteAddr, r.Host = tc.remote, tc.host
			rec := httptest.NewRecorder()
			build(ui).ServeHTTP(rec, r)
			if rec.Code != tc.status {
				t.Fatalf("status %d, want %d: %s", rec.Code, tc.status, rec.Body)
			}
			if tc.code != "" && bodyField(t, rec, "code") != tc.code {
				t.Errorf("body %s", rec.Body)
			}
		})
	}
	t.Run("nil UI is 404", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr, r.Host = lanAddr, "192.168.1.1"
		rec := httptest.NewRecorder()
		build(nil).ServeHTTP(rec, r)
		if rec.Code != 404 {
			t.Errorf("status %d", rec.Code)
		}
	})
}
