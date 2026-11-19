package api

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/dhcpserver"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// fakeDHCP answers with err when set and records what the routes passed on.
type fakeDHCP struct {
	err     error
	put     dhcpserver.DHCPSettings
	caller  netip.Addr
	removed string
}

func (f *fakeDHCP) Settings(_ context.Context, caller netip.Addr) (dhcpserver.DHCPView, error) {
	f.caller = caller
	return dhcpserver.DHCPView{DHCPSettings: dhcpserver.DHCPSettings{Enabled: true, Start: 100, Limit: 150, LeaseTime: "12h"}, Subnet: "192.168.1.0/24"}, f.err
}

func (f *fakeDHCP) SetSettings(_ context.Context, in dhcpserver.DHCPSettings, caller netip.Addr) (dhcpserver.DHCPView, error) {
	f.put, f.caller = in, caller
	return dhcpserver.DHCPView{DHCPSettings: in}, f.err
}

func (f *fakeDHCP) DNSNames(context.Context) ([]dhcpserver.DNSName, error) { return nil, f.err }

func (f *fakeDHCP) AddDNSName(_ context.Context, name, ip string) (dhcpserver.DNSName, error) {
	return dhcpserver.DNSName{ID: "travo_dns_1", Names: []string{name}, IP: ip, Owned: true}, f.err
}

func (f *fakeDHCP) RemoveDNSName(_ context.Context, id string) error {
	f.removed = id
	return f.err
}

func dhcpEnv(t *testing.T, d DHCPServer) (*env, string, string) {
	e := newEnvWithDashboard(t, nil, nil, nil, func(deps *Deps) { deps.DHCP = d })
	cookie, csrf := e.login()
	return e, cookie, csrf
}

func TestDHCPRoutes(t *testing.T) {
	f := &fakeDHCP{}
	e, cookie, csrf := dhcpEnv(t, f)
	rec := e.do(req{method: "GET", path: "/api/v1/network/dhcp", cookie: cookie})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"leasetime":"12h"`) || !f.caller.IsValid() {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	body := `{"enabled":false,"start":10,"limit":20,"leasetime":"1d"}`
	rec = e.do(req{method: "PUT", path: "/api/v1/network/dhcp", body: body, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusOK || f.put != (dhcpserver.DHCPSettings{Start: 10, Limit: 20, LeaseTime: "1d"}) {
		t.Fatalf("%d %s %+v", rec.Code, rec.Body, f.put)
	}
	// Strict whole-config PUT: missing or unknown fields and out-of-range offsets are refused.
	for _, b := range []string{`{"enabled":true,"start":10,"limit":20}`, body[:len(body)-1] + `,"x":1}`,
		`{"enabled":true,"start":0,"limit":20,"leasetime":"1h"}`, `{"enabled":true,"start":10,"limit":255,"leasetime":"1h"}`} {
		if rec = e.do(req{method: "PUT", path: "/api/v1/network/dhcp", body: b, cookie: cookie, csrf: csrf}); rec.Code < 400 || rec.Code >= 500 {
			t.Errorf("%s: %d %s", b, rec.Code, rec.Body)
		}
	}
	rec = e.do(req{method: "GET", path: "/api/v1/network/dns-names", cookie: cookie})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"names":[]`) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	rec = e.do(req{method: "POST", path: "/api/v1/network/dns-names", body: `{"name":"nas","ip":"192.168.1.20"}`, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"travo_dns_1"`) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	rec = e.do(req{method: "DELETE", path: "/api/v1/network/dns-names/travo_dns_1", cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusOK || f.removed != "travo_dns_1" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if rec = e.do(req{method: "DELETE", path: "/api/v1/network/dns-names/a.b", cookie: cookie, csrf: csrf}); rec.Code != http.StatusUnprocessableEntity && rec.Code != http.StatusBadRequest {
		t.Fatalf("id pattern: %d", rec.Code)
	}
}

func TestDHCPErrorCodes(t *testing.T) {
	f := &fakeDHCP{}
	e, cookie, csrf := dhcpEnv(t, f)
	for err, want := range map[error]string{
		&dhcpserver.FieldError{Field: "limit", Reason: "x"}: "400 invalid_request",
		dhcpserver.ErrForeignValue:                          "409 dhcp_foreign_config",
		dhcpserver.ErrNameExists:                            "409 dns_name_exists",
		dhcpserver.ErrNamesFull:                             "409 dns_names_full",
		dhcpserver.ErrNameForeign:                           "409 dns_name_foreign",
		dhcpserver.ErrNameNotFound:                          "404 not_found",
		fmt.Errorf("txn: stage: %w", txn.ErrForeignStaged):  "409 uci_busy",
		dhcpserver.ErrUnavailable:                           "503 dhcp_unavailable",
	} {
		f.err = err
		rec := e.do(req{method: "POST", path: "/api/v1/network/dns-names", body: `{"name":"nas","ip":"192.168.1.20"}`, cookie: cookie, csrf: csrf})
		if got := fmt.Sprint(rec.Code); !strings.HasPrefix(want, got) || !strings.Contains(rec.Body.String(), strings.Fields(want)[1]) {
			t.Errorf("%v: %d %s", err, rec.Code, rec.Body)
		}
	}
	e2, cookie2, _ := dhcpEnv(t, nil)
	if rec := e2.do(req{method: "GET", path: "/api/v1/network/dhcp", cookie: cookie2}); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil service: %d", rec.Code)
	}
}
