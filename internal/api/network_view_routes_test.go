package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/netview"
)

type fakeNetworkView struct{ err error }

func (f *fakeNetworkView) Interfaces(context.Context) (netview.Listing, error) {
	return netview.Listing{Interfaces: []netview.Interface{{Name: "wwan", Proto: "dhcp", Up: true, UptimeSeconds: 90, Uplink: true, Zone: "wan"}}, Partial: true}, f.err
}

func (f *fakeNetworkView) Zones(context.Context) ([]netview.Zone, error) { return nil, f.err }

func networkViewEnv(t *testing.T, f NetworkView) (*env, string, string) {
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.NetworkView = f })
	cookie, csrf := e.login()
	return e, cookie, csrf
}

func TestNetworkViewLists(t *testing.T) {
	e, cookie, csrf := networkViewEnv(t, &fakeNetworkView{})
	rec := e.do(req{method: "GET", path: "/api/v1/network/interfaces", cookie: cookie, csrf: csrf})
	var got struct {
		Interfaces []map[string]any `json:"interfaces"`
		Partial    bool             `json:"partial"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil || !got.Partial || got.Interfaces[0]["zone"] != "wan" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	rec = e.do(req{method: "GET", path: "/api/v1/network/firewall/zones", cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"zones\":[]}\n" {
		t.Fatalf("empty zones encode []: %d %q", rec.Code, rec.Body)
	}
}

func TestNetworkViewUnavailable(t *testing.T) {
	for _, f := range []NetworkView{nil, &fakeNetworkView{err: errors.New("ubus down")}} {
		e, cookie, csrf := networkViewEnv(t, f)
		for _, path := range []string{"/api/v1/network/interfaces", "/api/v1/network/firewall/zones"} {
			rec := e.do(req{method: "GET", path: path, cookie: cookie, csrf: csrf})
			if rec.Code != http.StatusServiceUnavailable || !json.Valid(rec.Body.Bytes()) || !strings.Contains(rec.Body.String(), "network_unavailable") {
				t.Fatalf("%s: %d %s", path, rec.Code, rec.Body)
			}
		}
	}
}
