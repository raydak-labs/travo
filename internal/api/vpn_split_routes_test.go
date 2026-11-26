package api

import (
	"context"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/vpnprofiles"
	"github.com/blackdark-org/travo-greenfield/internal/vpntunnel"
)

type splitFake struct {
	split vpntunnel.Split
	st    txn.Status
	err   error
	got   []netip.Prefix
}

func (f *splitFake) Split(context.Context) (vpntunnel.Split, error) { return f.split, f.err }
func (f *splitFake) SetSplit(_ context.Context, routes []netip.Prefix) (txn.Status, error) {
	f.got = routes
	return f.st, f.err
}

func TestVPNSplitRoutes(t *testing.T) {
	fake := &splitFake{split: vpntunnel.Split{Routes: []netip.Prefix{netip.MustParsePrefix("10.9.0.0/16")}}}
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.VPNSplit = fake })
	cookie, csrf := e.login()
	got := e.do(req{path: "/api/v1/vpn/split", cookie: cookie})
	if got.Code != http.StatusOK || strings.TrimSpace(got.Body.String()) != `{"routes":["10.9.0.0/16"]}` {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
	fake.split = vpntunnel.Split{}
	if got := e.do(req{path: "/api/v1/vpn/split", cookie: cookie}); strings.TrimSpace(got.Body.String()) != `{"routes":[]}` {
		t.Fatalf("full tunnel must encode []: %s", got.Body)
	}
	post := func(body string) (int, string) {
		r := e.do(req{method: "POST", path: "/api/v1/vpn/split", body: body, cookie: cookie, csrf: csrf})
		return r.Code, r.Body.String()
	}
	for _, body := range []string{`{"routes":["nope"]}`, `{"routes":["0.0.0.0/0"]}`, `{"routes":["2001:db8::/32"]}`} {
		if code, out := post(body); code != http.StatusBadRequest && code != http.StatusUnprocessableEntity {
			t.Fatalf("%s: %d %s", body, code, out)
		}
	}
	fake.st = txn.Status{Meta: txn.Meta{ID: "0123456789abcdef", Summary: "Configure split tunnel"}, State: txn.Pending}
	if code, out := post(`{"routes":["10.9.0.0/16"]}`); code != http.StatusAccepted || !slices.Equal(fake.got, []netip.Prefix{netip.MustParsePrefix("10.9.0.0/16")}) {
		t.Fatalf("%d %s", code, out)
	}
	fake.st, fake.err = txn.Status{}, vpnprofiles.ErrNotFound
	if r := e.do(req{method: "POST", path: "/api/v1/vpn/split", body: `{"routes":[]}`, cookie: cookie, csrf: csrf}); errCode(t, r) != "vpn_profile_required" {
		t.Fatalf("%d %s", r.Code, r.Body)
	}
}
