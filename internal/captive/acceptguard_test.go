package captive

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"sync/atomic"
	"testing"
)

const captureDir = "../../testdata/device/axt1800-25.12.6/"

// captureInterfaces reads the router's interfaces from verbatim captures: `ip -j -4 addr
// show` (lo, br-lan, the wwan STA) and the guest interface status (br-guest).
func captureInterfaces(t *testing.T) []IfaceAddrs {
	t.Helper()
	var addr []struct {
		Ifname   string `json:"ifname"`
		AddrInfo []struct {
			Local     string `json:"local"`
			Prefixlen int    `json:"prefixlen"`
		} `json:"addr_info"`
	}
	readJSON(t, captureDir+"ip-addr.json", &addr)
	var out []IfaceAddrs
	for _, it := range addr {
		entry := IfaceAddrs{Name: it.Ifname}
		for _, a := range it.AddrInfo {
			entry.Prefixes = append(entry.Prefixes, netip.PrefixFrom(netip.MustParseAddr(a.Local), a.Prefixlen))
		}
		out = append(out, entry)
	}
	var guest struct {
		L3Device string `json:"l3_device"`
		IPv4     []struct {
			Address string `json:"address"`
			Mask    int    `json:"mask"`
		} `json:"ipv4-address"`
	}
	readJSON(t, captureDir+"guest/interface-guest-status.json", &guest)
	entry := IfaceAddrs{Name: guest.L3Device}
	for _, a := range guest.IPv4 {
		entry.Prefixes = append(entry.Prefixes, netip.PrefixFrom(netip.MustParseAddr(a.Address), a.Mask))
	}
	return append(out, entry)
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(err)
	}
}

func TestTargetPolicyOnCapturedRouter(t *testing.T) {
	// Uplink phy0-sta0 is 192.168.90.241/24 with gateway 192.168.90.2 (ip-route-default.txt).
	p := NewTargetPolicy(captureInterfaces(t), "phy0-sta0", nil)
	for _, tc := range []struct {
		ip    string
		allow bool
		why   string
	}{
		{"192.168.1.1", false, "router LAN IP"},
		{"192.168.1.50", false, "LAN host"},
		{"192.168.2.1", false, "router guest IP"},
		{"192.168.2.50", false, "guest subnet host"},
		{"192.168.90.241", false, "router's own wwan IP"},
		{"127.0.0.1", false, "loopback"},
		{"0.0.0.0", false, "unspecified"},
		{"169.254.10.1", false, "link-local"},
		{"224.0.0.251", false, "multicast"},
		{"::ffff:192.168.1.1", false, "IPv4-mapped LAN IP"},
		{"192.168.90.2", true, "portal on the uplink subnet (the hotel gateway)"},
		{"10.66.0.1", true, "hotel private net routed via the uplink"},
		{"203.0.113.10", true, "public address"},
	} {
		err := p.Check(netip.MustParseAddr(tc.ip))
		if (err == nil) != tc.allow || (err != nil && !errors.Is(err, ErrTargetRefused)) {
			t.Errorf("%s (%s): %v", tc.ip, tc.why, err)
		}
	}
}

// The kernel route decides the rest: a target routed out of a tunnel or the LAN (a
// static route, a VPN or tailnet peer) has a source address that is not the uplink's.
func TestTargetPolicyRouteCheck(t *testing.T) {
	uplink, lan, tunnel := netip.MustParseAddr("192.168.90.241"), netip.MustParseAddr("192.168.1.1"), netip.MustParseAddr("10.200.0.2")
	routes := map[string]netip.Addr{"203.0.113.10": uplink, "10.99.0.5": lan, "10.201.0.1": tunnel}
	source := func(ip netip.Addr) (netip.Addr, error) {
		if src, ok := routes[ip.String()]; ok {
			return src, nil
		}
		return netip.Addr{}, errors.New("no route")
	}
	p := NewTargetPolicy(captureInterfaces(t), "phy0-sta0", source)
	for ip, allow := range map[string]bool{"203.0.113.10": true, "10.99.0.5": false, "10.201.0.1": false, "198.51.100.1": false} {
		if err := p.Check(netip.MustParseAddr(ip)); (err == nil) != allow {
			t.Errorf("%s: %v", ip, err)
		}
	}
}

// The kernel edges on the test host: interfaces are listed and loopback routes from itself.
func TestKernelEdges(t *testing.T) {
	ifaces, err := KernelInterfaces()
	if err != nil || len(ifaces) == 0 {
		t.Fatal(ifaces, err)
	}
	src, err := KernelRouteSource(netip.MustParseAddr("127.0.0.1"))
	if err != nil || !src.IsLoopback() {
		t.Fatal(src, err)
	}
}

// loopbackLAN serves on [::1] as the "LAN"; the policy keeps 127.0.0.0/8 for the uplink.
func loopbackLAN(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	ln, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skip("no IPv6 loopback:", err)
	}
	var hits atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(204)
	}))
	server.Listener = ln
	server.Start()
	t.Cleanup(server.Close)
	return server, &hits
}

func loopbackPolicy(served ...string) TargetPolicy {
	p := TargetPolicy{allowLoopback: true}
	for _, s := range served {
		p.Served = append(p.Served, netip.MustParsePrefix(s))
	}
	return p
}

func TestPortalClientRefusesNameResolvingToLAN(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer server.Close()
	_, port, _ := net.SplitHostPort(server.Listener.Addr().String())
	// localhost resolves to 127.0.0.1 and/or ::1; both are "LAN" here, so every
	// address the name resolves to is refused at dial time.
	_, err := NewPortalClient(loopbackPolicy("127.0.0.0/8", "::1/128")).Get("http://localhost:" + port + "/")
	if !errors.Is(err, ErrTargetRefused) || hits.Load() != 0 {
		t.Fatal(err, hits.Load())
	}
}

func TestPortalClientRefusesRedirectToLAN(t *testing.T) {
	lan, lanHits := loopbackLAN(t)
	portal := httptest.NewServer(http.RedirectHandler(lan.URL+"/admin", http.StatusFound))
	defer portal.Close()
	_, err := NewPortalClient(loopbackPolicy("::1/128")).Get(portal.URL)
	if !errors.Is(err, ErrTargetRefused) || lanHits.Load() != 0 {
		t.Fatal(err, lanHits.Load())
	}
}

// The walk end to end on the production client: a portal on the uplink subnet logs in,
// a portal that redirects into the LAN fails without touching it and without a renew.
func TestAcceptWithTargetPolicy(t *testing.T) {
	lan, lanHits := loopbackLAN(t)
	edge := PortalEdge{
		Interfaces: func() ([]IfaceAddrs, error) {
			return []IfaceAddrs{
				{Name: "phy0-sta0", Prefixes: []netip.Prefix{netip.MustParsePrefix("127.0.0.2/8")}},
				{Name: "br-lan", Prefixes: []netip.Prefix{netip.MustParsePrefix("::1/128")}},
			}, nil
		},
		NewClient: func(p TargetPolicy) *http.Client {
			p.allowLoopback = true
			return NewPortalClient(p)
		},
	}
	p := newSimPortal(t, "redirect")
	got, err := NewAcceptor(&portalConn{portal: p, url: p.server.URL + "/generate_204"}, nil, edge).Accept(t.Context())
	if err != nil || got.Outcome != AcceptOnline {
		t.Fatalf("uplink portal: %+v %v", got, err)
	}
	evil := httptest.NewServer(http.RedirectHandler(lan.URL+"/cgi-bin/luci", http.StatusFound))
	defer evil.Close()
	renew := &fakeRenew{state: RenewAvailable}
	got, err = NewAcceptor(&portalConn{portal: &simPortal{}, url: evil.URL}, renew, edge).Accept(t.Context())
	if err != nil || got.Outcome != AcceptFailed || renew.calls != 0 || lanHits.Load() != 0 {
		t.Fatalf("redirect to LAN: %+v %v renews %d hits %d", got, err, renew.calls, lanHits.Load())
	}
}
