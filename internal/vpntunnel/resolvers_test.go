package vpntunnel

import (
	"context"
	"encoding/base64"
	"errors"
	"net/netip"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/vpnprofiles"
)

func testKey(b byte) string {
	var raw [32]byte
	raw[0] = b
	return base64.StdEncoding.EncodeToString(raw[:])
}

// profileText: a WireGuard profile with its own private key; dns "" means no DNS line.
func profileText(key byte, dns string) string {
	text := "[Interface]\nPrivateKey=" + testKey(key) + "\nAddress=100.64.41.140/32\n"
	if dns != "" {
		text += "DNS=" + dns + "\n"
	}
	return text + "[Peer]\nPublicKey=" + testKey(2) + "\nAllowedIPs=0.0.0.0/0\nEndpoint=203.0.113.8:51820\n"
}

func addrs(in ...string) []netip.Addr {
	var out []netip.Addr
	for _, s := range in {
		out = append(out, netip.MustParseAddr(s))
	}
	return out
}

type tunnelEnv struct {
	s     *Service
	d     *mock.Device
	store *vpnprofiles.Service
}

func newTunnelEnv(t *testing.T) tunnelEnv {
	s, d, _ := splitService(t, "disabled")
	store := vpnprofiles.New(filepath.Join(t.TempDir(), "profiles.json"))
	s.profiles = store
	return tunnelEnv{s: s, d: d, store: store}
}

func (e tunnelEnv) importSelect(t *testing.T, key byte, dns string) {
	t.Helper()
	p, err := e.store.Import("p", profileText(key, dns))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.store.Select(p.ID); err != nil {
		t.Fatal(err)
	}
}

func (e tunnelEnv) resolvers(t *testing.T) []netip.Addr {
	t.Helper()
	network, err := e.d.Show(context.Background(), "network")
	if err != nil {
		t.Fatal(err)
	}
	dns, err := TunnelResolvers(network, e.store)
	if err != nil {
		t.Fatal(err)
	}
	return dns
}

// #289: the resolvers are the ones Enable staged, whatever is selected later.
func TestTunnelResolversFollowTheAppliedProfile(t *testing.T) {
	ctx := context.Background()
	e := newTunnelEnv(t)
	e.importSelect(t, 1, "198.18.0.1, 198.18.0.2")
	if _, err := e.s.Set(ctx, true); err != nil {
		t.Fatal(err)
	}
	if got := e.resolvers(t); !slices.Equal(got, addrs("198.18.0.1", "198.18.0.2")) {
		t.Fatalf("after apply: %v", got)
	}
	// same tunnel, other DNS: matching wg0 sections could not tell these apart
	e.importSelect(t, 1, "10.8.0.1")
	if got := e.resolvers(t); !slices.Equal(got, addrs("198.18.0.1", "198.18.0.2")) {
		t.Fatalf("after selecting another profile: %v", got)
	}
	if _, err := e.s.SetSplit(ctx, prefixes4(t, "10.9.0.0/16")); err != nil {
		t.Fatal(err)
	}
	if !hasSplitRoute(t, e.d, "198.18.0.1/32") || hasSplitRoute(t, e.d, "10.8.0.1/32") {
		t.Fatal("split routes must carry the applied resolvers")
	}
	if got, _ := e.s.Split(ctx); !slices.Equal(got.Routes, prefixes4(t, "10.9.0.0/16")) {
		t.Fatalf("split view: %v", got.Routes)
	}
	if _, err := e.s.Set(ctx, true); err != nil { // re-apply: now the selection is applied
		t.Fatal(err)
	}
	if got := e.resolvers(t); !slices.Equal(got, addrs("10.8.0.1")) {
		t.Fatalf("after re-applying: %v", got)
	}
}

func TestTunnelResolversWithoutDNSLine(t *testing.T) {
	e := newTunnelEnv(t)
	e.importSelect(t, 1, "")
	if _, err := e.s.Set(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	e.importSelect(t, 3, "10.8.0.1")
	if got := e.resolvers(t); len(got) != 0 {
		t.Fatalf("applied profile had no DNS line: %v", got)
	}
}

// A wg0 applied before #289 (the captures were written with plain uci) has no record:
// the selected profile's resolvers apply, as before, until the next apply.
func TestTunnelResolversFallBackForUnrecordedTunnel(t *testing.T) {
	store := vpnprofiles.New(filepath.Join(t.TempDir(), "profiles.json"))
	network, err := capturedNetwork(t, "connected").Show(context.Background(), "network")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := TunnelResolvers(network, store); !errors.Is(err, vpnprofiles.ErrNotFound) {
		t.Fatalf("nothing selected: %v", err)
	}
	p, err := store.Import("p", profileText(1, "198.18.0.1"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Select(p.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := TunnelResolvers(network, store); err != nil || !slices.Equal(got, addrs("198.18.0.1")) {
		t.Fatalf("%v %v", got, err)
	}
}

func TestRecordedResolversRejectsForeignValues(t *testing.T) {
	wg0 := func(value string) []device.Section {
		return []device.Section{{Name: "wg0", Type: "interface", Options: map[string]string{"proto": "wireguard", ResolversOption: value}}}
	}
	for _, bad := range []string{"", "not-an-ip", "fe80::1%wg0", "198.18.0.1 x"} {
		if _, _, err := recordedResolvers(wg0(bad)); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if dns, recorded, err := recordedResolvers(wg0("none")); err != nil || !recorded || len(dns) != 0 {
		t.Fatalf("none: %v %v %v", dns, recorded, err)
	}
	if _, recorded, err := recordedResolvers(nil); err != nil || recorded {
		t.Fatalf("absent: %v %v", recorded, err)
	}
}

func hasSplitRoute(t *testing.T, d *mock.Device, target string) bool {
	t.Helper()
	secs, err := d.Show(context.Background(), "network")
	if err != nil {
		t.Fatal(err)
	}
	for _, sec := range secs {
		if sec.Type == "route" && sec.Options["target"] == target {
			return true
		}
	}
	return false
}

// observingUCI reads network after every staged write, as a reconciler tick between two
// writes of an unlocked read would.
type observingUCI struct {
	device.UCI
	seen func([]device.Section)
}

func (o observingUCI) look(err error) error {
	if net, e := o.UCI.Show(context.Background(), "network"); e == nil {
		o.seen(net)
	}
	return err
}
func (o observingUCI) Set(ctx context.Context, c, s, k, v string) error {
	return o.look(o.UCI.Set(ctx, c, s, k, v))
}
func (o observingUCI) SetType(ctx context.Context, c, s, t string) error {
	return o.look(o.UCI.SetType(ctx, c, s, t))
}
func (o observingUCI) AddToList(ctx context.Context, c, s, k, v string) error {
	return o.look(o.UCI.AddToList(ctx, c, s, k, v))
}
func (o observingUCI) DeleteOption(ctx context.Context, c, s, k string) error {
	return o.look(o.UCI.DeleteOption(ctx, c, s, k))
}
func (o observingUCI) DeleteSection(ctx context.Context, c, s string) error {
	return o.look(o.UCI.DeleteSection(ctx, c, s))
}

// Every partly staged wg0, and the reverted one, reads as the old or the new resolvers:
// never "none", so the reconciler cannot pop on a half-written tunnel.
func TestTunnelResolversDuringStagingAndRollback(t *testing.T) {
	ctx := context.Background()
	for _, outcome := range []string{"commit", "abort"} {
		t.Run(outcome, func(t *testing.T) {
			e := newTunnelEnv(t)
			e.importSelect(t, 1, "198.18.0.1")
			if _, err := e.s.Set(ctx, true); err != nil {
				t.Fatal(err)
			}
			e.importSelect(t, 3, "10.8.0.1")
			old, next := addrs("198.18.0.1"), addrs("10.8.0.1")
			reads := 0
			e.s.uci = observingUCI{UCI: e.s.uci, seen: func(net []device.Section) {
				reads++
				got, err := TunnelResolvers(net, e.store)
				if err != nil || !slices.Equal(got, old) && !slices.Equal(got, next) {
					t.Errorf("mid-stage read %d: %v %v", reads, got, err)
				}
			}}
			if outcome == "abort" {
				e.s.keys = failingKeys{}
			}
			_, err := e.s.Set(ctx, true)
			if (err == nil) != (outcome == "commit") {
				t.Fatalf("Set: %v", err)
			}
			if reads == 0 {
				t.Fatal("no staged write observed")
			}
			want := next
			if outcome == "abort" {
				want = old
			}
			if got := e.resolvers(t); !slices.Equal(got, want) {
				t.Fatalf("after %s: %v, want %v", outcome, got, want)
			}
		})
	}
}

// failingKeys aborts Enable at the private key write, after other wg0 options are staged.
type failingKeys struct{}

func (failingKeys) SetWireGuardKey(context.Context, string, string, string) error {
	return errors.New("uci batch failed")
}

// A wg0 applied before #289 (no travo_dns) that already runs as the selected profile:
// Enable records the resolvers alone, without restarting wg0 or proving a handshake.
func TestEnableRecordsResolversWithoutRestart(t *testing.T) {
	ctx := context.Background()
	e := newTunnelEnv(t)
	e.importSelect(t, 1, "198.18.0.1")
	if _, err := e.s.Set(ctx, true); err != nil {
		t.Fatal(err)
	}
	d := e.d
	if err := d.DeleteOption(ctx, "network", "wg0", ResolversOption); err != nil {
		t.Fatal(err)
	}
	if err := d.Commit(ctx, "network"); err != nil {
		t.Fatal(err)
	}
	txns := e.s.txns.(*stageOnly)
	if _, err := e.s.Set(ctx, true); err != nil {
		t.Fatalf("recording the resolvers: %v", err)
	}
	if err := txns.probe(ctx); err != nil {
		t.Fatalf("probe for a resolvers-only change: %v", err)
	}
	if ups := e.s.control.(*proofDevice).ups; len(ups) != 0 {
		t.Fatalf("wg0 restarted for a resolvers-only change: %v", ups)
	}
	if got := e.resolvers(t); !slices.Equal(got, addrs("198.18.0.1")) {
		t.Fatalf("%v", got)
	}
	if _, err := e.s.Set(ctx, true); !errors.Is(err, txn.ErrNoChanges) {
		t.Fatalf("second apply: %v", err)
	}
}

// #296: with a split selection, re-applying a profile with other DNS keeps the user's
// prefixes and swaps the resolver /32s; a route change restarts and proves wg0.
func TestEnableMovesSplitResolverRoutes(t *testing.T) {
	ctx := context.Background()
	e := newTunnelEnv(t)
	e.importSelect(t, 1, "198.18.0.1")
	if _, err := e.s.Set(ctx, true); err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.SetSplit(ctx, prefixes4(t, "10.9.0.0/16")); err != nil {
		t.Fatal(err)
	}
	e.importSelect(t, 1, "10.8.0.1") // same tunnel, other DNS
	control := e.s.control.(*proofDevice)
	control.ups = nil
	if _, err := e.s.Set(ctx, true); err != nil {
		t.Fatal(err)
	}
	if !hasSplitRoute(t, e.d, "10.8.0.1/32") || hasSplitRoute(t, e.d, "198.18.0.1/32") || !hasSplitRoute(t, e.d, "10.9.0.0/16") {
		t.Fatal("split routes did not follow the new resolvers")
	}
	if got, _ := e.s.Split(ctx); !slices.Equal(got.Routes, prefixes4(t, "10.9.0.0/16")) {
		t.Fatalf("split view: %v", got.Routes)
	}
	if peer(t, e.d).Options["route_allowed_ips"] != "0" {
		t.Fatal("split mode lost")
	}
	// the fake never handshakes: only the restart matters, so bound the proof
	pctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	_ = e.s.txns.(*stageOnly).probe(pctx)
	if !slices.Equal(control.ups, []string{"wg0 down", "wg0 up"}) {
		t.Fatalf("a route change must restart wg0: %v", control.ups)
	}
}

// A disabled wg0 never takes the resolvers-only path: enabling it restarts and proves.
func TestEnableDisabledTunnelStillRestarts(t *testing.T) {
	ctx := context.Background()
	e := newTunnelEnv(t)
	e.importSelect(t, 1, "198.18.0.1")
	if _, err := e.s.Set(ctx, true); err != nil {
		t.Fatal(err)
	}
	if err := e.d.Set(ctx, "network", "wg0", "disabled", "1"); err != nil {
		t.Fatal(err)
	}
	if err := e.d.Commit(ctx, "network"); err != nil {
		t.Fatal(err)
	}
	e.importSelect(t, 1, "10.8.0.1")
	if _, err := e.s.Set(ctx, true); err != nil {
		t.Fatal(err)
	}
	secs, _ := e.d.Show(ctx, "network")
	for _, sec := range secs {
		if sec.Name == "wg0" && (sec.Options["disabled"] != "0" || sec.Options[ResolversOption] != "10.8.0.1") {
			t.Fatalf("wg0 %v", sec.Options)
		}
	}
}

// Split sections that hold only resolver /32s (user prefixes removed outside Travo) leave
// no selection: Enable returns the tunnel to full mode instead of routing nothing.
func TestEnableWithOnlyResolverRoutesReturnsToFullTunnel(t *testing.T) {
	ctx := context.Background()
	e := newTunnelEnv(t)
	e.importSelect(t, 1, "198.18.0.1")
	if _, err := e.s.Set(ctx, true); err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.SetSplit(ctx, prefixes4(t, "10.9.0.0/16")); err != nil {
		t.Fatal(err)
	}
	secs, _ := e.d.Show(ctx, "network")
	for _, sec := range secs {
		if sec.Type == "route" && sec.Options["target"] == "10.9.0.0/16" {
			if err := e.d.DeleteSection(ctx, "network", sec.Name); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := e.d.Commit(ctx, "network"); err != nil {
		t.Fatal(err)
	}
	e.importSelect(t, 1, "10.8.0.1")
	if _, err := e.s.Set(ctx, true); err != nil {
		t.Fatal(err)
	}
	if hasSplitRoute(t, e.d, "198.18.0.1/32") || hasSplitRoute(t, e.d, "10.8.0.1/32") {
		t.Fatal("resolver-only split routes left behind")
	}
	if peer(t, e.d).Options["route_allowed_ips"] != "1" {
		t.Fatal("peers stay in split mode with no routes")
	}
}

// A user /32 that equals a resolver must survive a re-apply with the same DNS:
// only a resolver change recomputes the split.
func TestEnableKeepsUserRouteEqualToResolver(t *testing.T) {
	ctx := context.Background()
	e := newTunnelEnv(t)
	e.importSelect(t, 1, "198.18.0.1")
	if _, err := e.s.Set(ctx, true); err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.SetSplit(ctx, prefixes4(t, "198.18.0.1/32")); err != nil {
		t.Fatal(err)
	}
	e.importSelect(t, 3, "198.18.0.1") // other tunnel, same DNS
	if _, err := e.s.Set(ctx, true); err != nil {
		t.Fatal(err)
	}
	if !hasSplitRoute(t, e.d, "198.18.0.1/32") || peer(t, e.d).Options["route_allowed_ips"] != "0" {
		t.Fatal("re-apply dropped the user's route and switched to full tunnel")
	}
}

// A user /32 equal to a resolver is the only selected route; a DNS change must keep it and
// add the new resolver /32, not switch to full tunnel.
func TestEnableKeepsUserRouteEqualToResolverWhenDNSChanges(t *testing.T) {
	ctx := context.Background()
	e := newTunnelEnv(t)
	e.importSelect(t, 1, "198.18.0.1")
	if _, err := e.s.Set(ctx, true); err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.SetSplit(ctx, prefixes4(t, "198.18.0.1/32")); err != nil {
		t.Fatal(err)
	}
	e.importSelect(t, 1, "10.8.0.1")
	if _, err := e.s.Set(ctx, true); err != nil {
		t.Fatal(err)
	}
	if !hasSplitRoute(t, e.d, "198.18.0.1/32") || !hasSplitRoute(t, e.d, "10.8.0.1/32") {
		t.Fatal("user route dropped or new resolver route missing")
	}
	if got, _ := e.s.Split(ctx); !slices.Equal(got.Routes, prefixes4(t, "198.18.0.1/32")) {
		t.Fatalf("split view: %v", got.Routes)
	}
}

// A split written before the marker has unmarked resolver /32s: they still count as
// resolver routes when they equal the recorded resolvers.
func TestUnmarkedLegacyResolverRoutes(t *testing.T) {
	dns := []netip.Addr{netip.MustParseAddr("198.18.0.1")}
	route := func(name, target, marker string) device.Section {
		o := map[string]string{"interface": "wg0", "target": target}
		if marker != "" {
			o[resolverOption] = marker
		}
		return device.Section{Name: name, Type: "route", Options: o}
	}
	legacy := []device.Section{route("travo_split_0", "10.9.0.0/16", ""), route("travo_split_1", "198.18.0.1/32", "")}
	got, err := currentSplit(legacy, dns)
	if err != nil || !slices.Equal(userRoutes(got), prefixes4(t, "10.9.0.0/16")) {
		t.Fatalf("legacy: %v %v", got, err)
	}
	marked := []device.Section{route("travo_split_0", "198.18.0.1/32", "0")}
	got, err = currentSplit(marked, dns)
	if err != nil || !slices.Equal(userRoutes(got), prefixes4(t, "198.18.0.1/32")) {
		t.Fatalf("a user-marked /32 must stay a user route: %v %v", got, err)
	}
	bad := []device.Section{route("travo_split_0", "10.9.0.0/16", "yes")}
	if _, err := currentSplit(bad, dns); err == nil {
		t.Fatal("foreign marker value accepted")
	}
}
