package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/vpndns"
)

type vpnDNSFake struct {
	status   vpndns.DNSStatus
	verified vpndns.DNSVerification
	err      error
	verifies int
}

func (f *vpnDNSFake) Status(context.Context) (vpndns.DNSStatus, error) { return f.status, f.err }
func (f *vpnDNSFake) Verify(context.Context) (vpndns.DNSVerification, error) {
	f.verifies++
	return f.verified, f.err
}

func TestVPNDNSRoutes(t *testing.T) {
	vpn := "vpn"
	fake := &vpnDNSFake{status: vpndns.DNSStatus{Layer: vpndns.Active, Resolvers: []string{"10.64.0.1"}, Effective: []string{"10.64.0.1"}, Top: &vpn}}
	fake.verified = vpndns.DNSVerification{Result: vpndns.OK, Status: fake.status}
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.VPNDNS = fake })
	if got := e.do(req{path: "/api/v1/vpn/dns"}); got.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated: %d", got.Code)
	}
	cookie, csrf := e.login()
	got := e.do(req{path: "/api/v1/vpn/dns", cookie: cookie})
	want := `{"layer":"active","resolvers":["10.64.0.1"],"effective":["10.64.0.1"],"top":"vpn"}`
	if got.Code != http.StatusOK || strings.TrimSpace(got.Body.String()) != want {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
	if fake.verifies != 0 {
		t.Fatal("GET must not run the verify lookup")
	}
	if got := e.do(req{method: "POST", path: "/api/v1/vpn/dns/verify", cookie: cookie}); got.Code != http.StatusForbidden {
		t.Fatalf("verify without CSRF: %d %s", got.Code, got.Body)
	}
	got = e.do(req{method: "POST", path: "/api/v1/vpn/dns/verify", cookie: cookie, csrf: csrf})
	if got.Code != http.StatusOK || !strings.HasPrefix(strings.TrimSpace(got.Body.String()), `{"result":"ok","status":{"layer":"active"`) {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
	fake.status = vpndns.DNSStatus{Layer: vpndns.Inactive, Resolvers: []string{}}
	got = e.do(req{path: "/api/v1/vpn/dns", cookie: cookie})
	if want := `{"layer":"inactive","resolvers":[],"effective":null,"top":null}`; strings.TrimSpace(got.Body.String()) != want {
		t.Fatalf("%s", got.Body)
	}
	fake.err = errors.New("record unreadable")
	for _, r := range []req{{path: "/api/v1/vpn/dns", cookie: cookie}, {method: "POST", path: "/api/v1/vpn/dns/verify", cookie: cookie, csrf: csrf}} {
		if got := e.do(r); got.Code != http.StatusServiceUnavailable || errCode(t, got) != "vpn_dns_unavailable" {
			t.Fatalf("%s: %d %s", r.path, got.Code, got.Body)
		}
	}
}

func TestVPNDNSNilDependency(t *testing.T) {
	e := newEnvWithDashboard(t, nil, nil, nil)
	cookie, _ := e.login()
	if got := e.do(req{path: "/api/v1/vpn/dns", cookie: cookie}); got.Code != http.StatusServiceUnavailable || errCode(t, got) != "vpn_dns_unavailable" {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
}
