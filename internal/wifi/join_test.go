package wifi

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/lockout"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/uciwrite"
)

func configFixture(t *testing.T, config, filename string) []device.Section {
	t.Helper()
	sections, err := cli.ParseShow(config, string(fixture(t, filename)))
	if err != nil {
		t.Fatal(err)
	}
	return sections
}
func TestJoinPlan(t *testing.T) {
	network := configFixture(t, "network", "uci-show-network.txt")
	firewall := configFixture(t, "firewall", "uci-show-firewall.txt")
	req := JoinRequest{SSID: "TestNetwork", Key: "example-only", Security: SecurityWPA2, BSSID: "02:00:00:00:00:01"}
	for _, tc := range []struct {
		dir    string
		create bool
	}{{"no-sta", true}, {"no-sta", false}, {"connected", false}} {
		t.Run(tc.dir, func(t *testing.T) {
			wireless := configFixture(t, "wireless", "wifi-scan/"+tc.dir+"/uci-show-wireless.txt")
			// This capture has no runtime STA but retains a disabled UCI section.
			// Drop only that section for the fresh install case.
			if tc.create {
				wireless = slices.DeleteFunc(wireless, func(s device.Section) bool { return s.Name == "wwan" })
			}
			writes, before, after, err := planJoin(wireless, network, firewall, req, "radio0")
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("AP plan changed: %v", err)
			}
			if len(writes) == 0 || slices.ContainsFunc(writes, func(w uciWrite) bool { return w.config != "wireless" || w.section != "wwan" || w.value == req.Key }) {
				t.Fatalf("unexpected plan: %#v", writes)
			}
			if (writes[0].action == "type") != tc.create {
				t.Fatal(writes)
			}
			if !slices.Contains(writes, uciWrite{"key", "wireless", "wwan", "key", ""}) {
				t.Fatal("secret setter missing")
			}
		})
	}
	if _, _, _, err := planJoin(nil, nil, nil, req, "radio0"); !errors.Is(err, ErrFirewallUnavailable) {
		t.Fatal(err)
	}
	writes, _, _, err := planJoin(nil, nil, firewallWithoutUplink(firewall), req, "radio0")
	if err != nil || !slices.Contains(writes, uciWrite{"set", "network", "wwan", "proto", "dhcp"}) ||
		!slices.ContainsFunc(writes, func(w uciWrite) bool { return w.config == "firewall" && w.action == "list" && w.value == "wwan" }) {
		t.Fatal(writes, err)
	}
}
func firewallWithoutUplink(sections []device.Section) []device.Section {
	for i := range sections {
		s := &sections[i]
		if s.Options["name"] == "wan" {
			s.Lists["network"] = slices.DeleteFunc(s.Lists["network"], func(n string) bool { return n == "wwan" })
			n := strings.Fields(s.Options["network"])
			s.Options["network"] = strings.Join(slices.DeleteFunc(n, func(n string) bool { return n == "wwan" }), " ")
		}
	}
	return sections
}

func TestJoinRadioAndTripwire(t *testing.T) {
	wireless := configFixture(t, "wireless", "wifi-scan/no-sta/uci-show-wireless.txt")
	radios, err := facts.ParseWirelessRadios(fixture(t, "wifi-scan/no-sta/wireless-status.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		bands []string
		want  string
	}{{[]string{"2g"}, ""}, {[]string{"5g"}, "radio0"}, {[]string{"5g", "2g"}, "radio0"}} {
		if got := pickRadio(radios, tc.bands, wireless); got != tc.want {
			t.Fatalf("%v picked %q", tc.bands, got)
		}
	}
	before := []lockout.AP{{Section: "default_radio1", Ifname: "phy1-ap0", Enabled: true}}
	for _, caller := range []lockout.Caller{{Method: lockout.Ethernet}, {Method: lockout.WifiAP, AP: "phy1-ap0"}, {Method: lockout.Unknown}} {
		if err := checkJoinLockout(caller, before, before, false); err != nil {
			t.Fatal(err)
		}
		err := checkJoinLockout(caller, before, nil, false)
		if errors.Is(err, lockout.ErrLockoutRisk) != (caller.Method != lockout.Ethernet) {
			t.Fatal(caller, err)
		}
	}
}

type joinRoute struct{ iface string }

func (r joinRoute) DefaultIPv4Route(context.Context) (device.IPv4DefaultRoute, bool, error) {
	return device.IPv4DefaultRoute{Device: r.iface}, r.iface != "", nil
}

func TestJoinProbeRequiresCarrierAndSelectedDefaultRoute(t *testing.T) {
	for _, tc := range []struct {
		name, link, route, section, ifname string
		pass                               bool
	}{
		{"connected", "device-status-phy0-sta0.json", "phy0-sta0", "wwan", "phy0-sta0", true},
		{"wrong password", "wwan-down/device-status-phy0-sta0-disconnected.json", "phy0-sta0", "wwan", "phy0-sta0", false},
		{"no default", "device-status-phy0-sta0.json", "", "wwan", "phy0-sta0", false},
		{"ethernet default", "device-status-phy0-sta0.json", "wan", "wwan", "phy0-sta0", false},
		{"AP with carrier and route", "device-status-phy1-ap0.json", "phy1-ap0", "default_radio1", "phy1-ap0", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ubus := summaryUbus(t, "wifi-scan/connected/wireless-status.json")
			ubus.replies[ubusKey{"network.device", "status", tc.ifname}] = fixture(t, tc.link)
			service := New(context.Background(), ubus, nil).WithJoin(JoinDeps{Ubus: ubus, Routes: joinRoute{tc.route}, Links: fixedJoinLinks(2), ProbeBudget: 10 * time.Millisecond, ProbeEvery: time.Millisecond})
			radio := "radio0"
			if tc.ifname == "phy1-ap0" {
				radio = "radio1"
			}
			err := service.probe(joinPlan{radio: radio, section: tc.section}, map[int]struct{}{1: {}})(context.Background())
			if (err == nil) != tc.pass {
				t.Fatal(err)
			}
		})
	}
}

type joinBeginFunc func(context.Context, string, []string, func(context.Context) error, txn.Probe) (txn.Status, error)

func (f joinBeginFunc) Begin(ctx context.Context, summary string, sets []string, stage func(context.Context) error, probe txn.Probe) (txn.Status, error) {
	return f(ctx, summary, sets, stage, probe)
}

func TestJoinReplansUnderTransactionLock(t *testing.T) {
	for _, stale := range []bool{false, true} {
		t.Run(map[bool]string{false: "pending", true: "AP appeared before stage"}[stale], func(t *testing.T) {
			ctx := context.Background()
			service, _ := noSTAScanService(t)
			d := mock.New()
			seedJoinDevice(t, d)
			ssid := joinFixtureSSID(t, service)
			begin := joinBeginFunc(func(ctx context.Context, summary string, cs []string, stage func(context.Context) error, _ txn.Probe) (txn.Status, error) {
				if strings.Contains(summary, "test-password") || !slices.Equal(cs, []string{"wireless", "network", "firewall"}) {
					t.Fatal("unsafe transaction metadata")
				}
				if stale {
					if err := d.Set(ctx, "wireless", "default_radio0", "disabled", "0"); err != nil {
						t.Fatal(err)
					}
				}
				if err := stage(ctx); err != nil {
					return txn.Status{}, err
				}
				return txn.Status{State: txn.Pending}, nil
			})
			service.WithJoin(JoinDeps{UCI: d, Ubus: service.ubus, Keys: d, Links: d, Txns: begin})
			st, err := service.Join(ctx, JoinRequest{SSID: ssid, Security: SecurityWPA2, Key: "test-password"})
			if stale {
				if !errors.Is(err, ErrNoFreeRadio) {
					t.Fatal(err)
				}
			} else if err != nil || st.State != txn.Pending {
				t.Fatal(st, err)
			}
			wireless, err := d.Show(ctx, "wireless")
			if err != nil || hasSection(wireless, "wwan") == stale {
				t.Fatal("incorrect STA staging", err)
			}
		})
	}
}

func seedJoinDevice(t *testing.T, d *mock.Device) {
	t.Helper()
	seedConfigs(t, d, "wifi-scan/no-sta/uci-show-wireless.txt", func(s device.Section) bool { return s.Name == "wwan" })
}

// seedConfigs commits verbatim UCI captures; skip drops sections a case must not have.
func seedConfigs(t *testing.T, d *mock.Device, wirelessFile string, skip func(device.Section) bool) {
	t.Helper()
	ctx := context.Background()
	for _, cfg := range []string{"wireless", "network", "firewall", "dhcp"} {
		filename := "uci-show-" + cfg + ".txt"
		if cfg == "wireless" {
			filename = wirelessFile
		}
		for _, section := range configFixture(t, cfg, filename) {
			if cfg == "wireless" && skip != nil && skip(section) {
				continue
			}
			if err := d.SetType(ctx, cfg, section.Name, section.Type); err != nil {
				t.Fatal(err)
			}
			for field, value := range section.Options {
				if err := d.Set(ctx, cfg, section.Name, field, value); err != nil {
					t.Fatal(err)
				}
			}
			for field, values := range section.Lists {
				for _, value := range values {
					if err := d.AddToList(ctx, cfg, section.Name, field, value); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
		if err := d.Commit(ctx, cfg); err != nil {
			t.Fatal(err)
		}
	}
}

func joinFixtureSSID(t *testing.T, service *Service) string {
	t.Helper()
	ctx := context.Background()
	choices, err := service.Scan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var ssid string
	for _, n := range choices.Networks {
		if slices.Contains(n.Bands, "5g") {
			ssid = n.SSID
			break
		}
	}
	if ssid == "" {
		t.Fatal("fixture has no 5g network")
	}
	return ssid
}

type joinClock struct{}

func (joinClock) Now() time.Time { return time.Now() }

type joinFileDevice struct {
	*mock.Device
	configDir           string
	afterWirelessCommit func()
}

func (d joinFileDevice) Commit(ctx context.Context, config string) error {
	pending, err := d.Pending(ctx, config)
	if err != nil {
		return err
	}
	if pending {
		if err := os.WriteFile(filepath.Join(d.configDir, config), []byte("staged "+config), 0o600); err != nil {
			return err
		}
		if config == "wireless" {
			d.afterWirelessCommit()
		}
	}
	return d.Device.Commit(ctx, config)
}

// fileTxnManager runs a real txn.Manager whose commits and restores touch real files.
func fileTxnManager(t *testing.T, base *mock.Device, afterWirelessCommit func()) (*txn.Manager, string) {
	t.Helper()
	root := t.TempDir()
	// Restore real files, but leave durability to the OpenWrt integration tests.
	// A host-wide sync can exceed the restore budget on a busy CI runner.
	if err := os.WriteFile(filepath.Join(root, "sync"), []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	configDir := filepath.Join(root, "config")
	if err := os.Mkdir(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, config := range []string{"wireless", "network", "firewall", "dhcp"} {
		if err := os.WriteFile(filepath.Join(configDir, config), []byte("original "+config), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	device := joinFileDevice{Device: base, configDir: configDir, afterWirelessCommit: afterWirelessCommit}
	return &txn.Manager{
		Device: device, Locks: &uciwrite.Locks{},
		Guards: guard.Dir{Path: filepath.Join(root, "guards"), Now: time.Now},
		Dir:    filepath.Join(root, "txn"), ConfigDir: configDir, Reload: []string{"true"}, UCI: "true",
		Spawner: txn.ShellSpawner{Shell: "/bin/sh", RunTimeout: 5 * time.Second}, Clock: joinClock{},
		NewID: func() string { return "0000000000000001" }, Window: txn.MinWindow,
		ArmLimit: 30 * time.Second, ApplyBudget: 3 * time.Second, Settle: 3 * time.Second,
	}, configDir
}

func TestJoinWrongPasswordRestoresTransaction(t *testing.T) {
	ctx := context.Background()
	service, _ := noSTAScanService(t)
	base := mock.New()
	seedJoinDevice(t, base)
	ubus := service.ubus.(*fakeUbus)
	manager, configDir := fileTxnManager(t, base, func() {
		ubus.replies[ubusKey{"network.wireless", "status", ""}] = fixture(t, "wifi-scan/connected/wireless-status.json")
	})
	// Immediately after reload the previous association still reports carrier.
	ubus.replies[ubusKey{"network.device", "status", "phy0-sta0"}] = fixture(t, "device-status-phy0-sta0.json")
	lostCarrier := fixture(t, "wwan-down/device-status-phy0-sta0-disconnected.json")
	reads := 0
	links := joinLinks(func() (map[string]int, error) {
		reads++
		// Baseline and the first post-reload read still see the old healthy STA.
		if reads <= 2 {
			return fixedJoinLinks(2)()
		}
		ubus.replies[ubusKey{"network.device", "status", "phy0-sta0"}] = lostCarrier
		return fixedJoinLinks(3)()
	})
	route := joinRouteFunc(func() string {
		if reads > 2 {
			return ""
		}
		return "phy0-sta0"
	})
	service.WithJoin(JoinDeps{UCI: manager.Device, Ubus: ubus, Routes: route, Keys: manager.Device.(device.WirelessKeyWriter), Links: links, Txns: manager, ProbeBudget: 50 * time.Millisecond, ProbeEvery: time.Millisecond})
	st, err := service.Join(ctx, JoinRequest{SSID: joinFixtureSSID(t, service), Key: "wrong-password", Security: SecurityWPA2})
	if !errors.Is(err, txn.ErrProbeFailed) || st.State != txn.RolledBack {
		t.Fatal(st, err)
	}
	if reads < 3 {
		t.Fatal("probe never reached the replacement STA with lost carrier")
	}
	for _, config := range []string{"wireless", "network", "firewall"} {
		content, err := os.ReadFile(filepath.Join(configDir, config))
		if err != nil || string(content) != "original "+config {
			t.Fatalf("%s not restored: %v", config, err)
		}
	}
	if _, err := manager.Get(st.ID); !errors.Is(err, txn.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestJoinValidation(t *testing.T) {
	for _, tc := range []struct {
		req  JoinRequest
		code string
	}{
		{JoinRequest{Security: SecurityOpen}, "wifi_ssid_invalid"},
		{JoinRequest{SSID: strings.Repeat("ü", 17), Security: SecurityOpen}, "wifi_ssid_invalid"},
		{JoinRequest{SSID: "Test", BSSID: "wrong", Security: SecurityOpen}, "wifi_bssid_invalid"},
		{JoinRequest{SSID: "Test", Security: SecurityWPA2}, "wifi_key_required"},
		{JoinRequest{SSID: "Test", Security: SecurityWPA2, Key: "seven77"}, "wifi_key_invalid"},
		{JoinRequest{SSID: "Test", Security: SecurityWPA2, Key: strings.Repeat("x", 64)}, "wifi_key_invalid"},
		{JoinRequest{SSID: "Test", Security: SecurityOpen, Key: "secret"}, "wifi_key_not_allowed"},
	} {
		var v ValidationError
		if err := tc.req.validate(); !errors.As(err, &v) || v.Code != tc.code {
			t.Fatalf("%s: %v", tc.code, err)
		}
	}
	if err := (JoinRequest{SSID: "Test", Security: SecurityOther}).validate(); !errors.Is(err, ErrSecurityUnsupported) {
		t.Fatal(err)
	}
}
