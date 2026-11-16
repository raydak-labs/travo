package connectivity

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPProbe(t *testing.T) {
	for _, tc := range []struct {
		code     int
		location string
		want     Internet
		portal   string // "probe" = the probe URL itself
	}{
		{204, "", Online, ""}, {302, "/hotel", Captive, "/hotel"}, {301, "https://example.org/", Captive, "https://example.org/"},
		{302, "ftp://example.org/", Offline, ""}, {302, "http://[bad", Offline, ""}, {302, "", Offline, ""},
		{200, "", Captive, "probe"}, {500, "", Offline, ""}, {404, "", Offline, ""},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" || r.URL.Path != "/check" || r.Header.Get("Cookie") != "" {
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				w.WriteHeader(500)
				return
			}
			if tc.location != "" {
				w.Header().Set("Location", tc.location)
			}
			w.WriteHeader(tc.code)
		}))
		got := NewHTTPProbe(server.URL+"/check", time.Second)(context.Background())
		server.Close()
		portal := tc.portal
		switch portal {
		case "probe":
			portal = server.URL + "/check"
		case "/hotel":
			portal = server.URL + "/hotel" // a relative Location resolves against the probe URL
		}
		if got.Internet != tc.want || got.Portal != portal {
			t.Errorf("%d %q: got %+v want %s %q", tc.code, tc.location, got, tc.want, portal)
		}
	}
}

func TestHTTPProbeTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	if got := NewHTTPProbe(server.URL, 25*time.Millisecond)(context.Background()); got.Internet != Offline {
		t.Fatal(got)
	}
}

func TestHTTPProbeUsesFreshConnection(t *testing.T) {
	addresses := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addresses <- r.RemoteAddr
		w.WriteHeader(204)
	}))
	defer server.Close()
	probe := NewHTTPProbe(server.URL, time.Second)
	if probe(t.Context()).Internet != Online || probe(t.Context()).Internet != Online {
		t.Fatal("expected online")
	}
	if first, second := receive(t, addresses), receive(t, addresses); first == second {
		t.Fatal("probe reused a connection")
	}
}
