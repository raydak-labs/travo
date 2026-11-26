package dhcpserver

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

const captures = "../../testdata/device/axt1800-25.12.6/"

func read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(captures + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func load(t *testing.T, d *mock.Device, config, name string, parse func(string, string) ([]device.Section, error)) {
	t.Helper()
	secs, err := parse(config, string(read(t, name)))
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Load(t.Context(), config, secs); err != nil {
		t.Fatal(err)
	}
}

// fakeTxns mirrors txn.Manager.Apply: stage, revert on error, commit, reload.
type fakeTxns struct {
	d       *mock.Device
	commits int
}

func (f *fakeTxns) Apply(ctx context.Context, configs []string, stage func(context.Context) error) error {
	if err := stage(ctx); err != nil {
		return errors.Join(err, f.d.Revert(ctx, configs[0]))
	}
	f.commits++
	if err := f.d.Commit(ctx, configs[0]); err != nil {
		return err
	}
	return f.d.Reload(ctx)
}

// opt reads one option of the first section matching name or, for "@type", type.
func opt(t *testing.T, d *mock.Device, section, option string) (string, bool) {
	t.Helper()
	secs, err := d.Show(t.Context(), "dhcp")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range secs {
		if s.Name == section || "@"+s.Type == section {
			v, ok := s.Options[option]
			return v, ok
		}
	}
	return "", false
}

// setup builds a service on the AXT1800 captures (dhcp.lan 100/150/12h on 192.168.1.1/24,
// one lease for 192.168.1.151).
func setup(t *testing.T) (*Service, *mock.Device, *fakeTxns) {
	t.Helper()
	d := mock.New()
	load(t, d, "dhcp", "dhcp/uci-export-dhcp.txt", cli.ParseExport)
	load(t, d, "network", "dhcp/uci-show-network-lan-guest.txt", cli.ParseShow)
	d.SetLeases(read(t, "with-ap-client/dhcp-leases.txt"))
	f := &fakeTxns{d: d}
	return &Service{UCI: d, Leases: d, Reload: d, Txns: f}, d, f
}

func TestSettingsReadsTheCapture(t *testing.T) {
	s, _, _ := setup(t)
	v, err := s.Settings(t.Context(), netip.MustParseAddr("192.168.1.151"))
	if err != nil {
		t.Fatal(err)
	}
	want := DHCPView{DHCPSettings: DHCPSettings{Enabled: true, Start: 100, Limit: 150, LeaseTime: "12h"},
		Subnet: "192.168.1.0/24", First: "192.168.1.100", Last: "192.168.1.249", CallerLease: true}
	if v != want {
		t.Fatalf("%+v", v)
	}
	if v, _ := s.Settings(t.Context(), netip.MustParseAddr("192.168.1.20")); v.CallerLease {
		t.Fatal("no lease for .20")
	}
}

func TestReadDefaultsAndOffSwitches(t *testing.T) {
	lan := func(opts map[string]string) []device.Section {
		return []device.Section{{Name: "lan", Type: "dhcp", Options: opts}}
	}
	got, err := Read(lan(map[string]string{"interface": "lan"}))
	if err != nil || got != (DHCPSettings{Enabled: true, Start: 100, Limit: 150, LeaseTime: "12h"}) {
		t.Fatalf("%+v %v", got, err)
	}
	for _, opts := range []map[string]string{{"ignore": "1"}, {"ignore": "yes"}, {"dhcpv4": "disabled"}} {
		if got, _ := Read(lan(opts)); got.Enabled {
			t.Errorf("%v reads as on", opts)
		}
	}
	if _, err := Read(lan(map[string]string{"start": "192.168.1.50"})); !errors.Is(err, ErrForeignValue) {
		t.Fatal(err)
	}
	if _, err := Read(nil); !errors.Is(err, ErrNoSection) {
		t.Fatal(err)
	}
}

func TestValidate(t *testing.T) {
	lan := netip.MustParsePrefix("192.168.1.1/24")
	ok := DHCPSettings{Enabled: true, Start: 10, Limit: 245, LeaseTime: "2m"}
	if _, err := Validate(ok, lan); err != nil {
		t.Fatal(err)
	}
	for _, in := range []DHCPSettings{
		{Start: 0, Limit: 10, LeaseTime: "1h"},
		{Start: 255, Limit: 1, LeaseTime: "1h"},
		{Start: 10, Limit: 0, LeaseTime: "1h"},
		{Start: 10, Limit: 246, LeaseTime: "1h"}, // ends on the broadcast address
		{Start: 10, Limit: 10, LeaseTime: "1m"},
		{Start: 10, Limit: 10, LeaseTime: "90s"},
		{Start: 10, Limit: 10, LeaseTime: "0h"},
		{Start: 10, Limit: 10, LeaseTime: ""},
	} {
		if _, err := Validate(in, lan); !errors.Is(err, ErrInvalid) {
			t.Errorf("%+v: %v", in, err)
		}
	}
	if _, err := Validate(DHCPSettings{Start: 100, Limit: 100, LeaseTime: "infinite"}, netip.MustParsePrefix("10.0.0.1/25")); !errors.Is(err, ErrInvalid) {
		t.Fatal("pool past a /25")
	}
}

func TestSetSettingsWritesOnlyWhatDiffers(t *testing.T) {
	s, d, f := setup(t)
	v, err := s.SetSettings(t.Context(), DHCPSettings{Enabled: false, Start: 100, Limit: 50, LeaseTime: "1d"}, netip.Addr{})
	if err != nil {
		t.Fatal(err)
	}
	if v.Enabled || v.Limit != 50 || v.Last != "192.168.1.149" || f.commits != 1 {
		t.Fatalf("%+v commits=%d", v, f.commits)
	}
	if got, _ := opt(t, d, "lan", "ignore"); got != "1" {
		t.Fatalf("ignore=%q", got)
	}
	// Unchanged: nothing committed, the idempotent reload still runs.
	reloads := d.Reloads
	if _, err := s.SetSettings(t.Context(), DHCPSettings{Start: 100, Limit: 50, LeaseTime: "1d"}, netip.Addr{}); err != nil {
		t.Fatal(err)
	}
	if f.commits != 1 || d.Reloads != reloads+1 {
		t.Fatalf("commits=%d reloads=%d", f.commits, d.Reloads)
	}
	if v, err = s.SetSettings(t.Context(), DHCPSettings{Enabled: true, Start: 100, Limit: 50, LeaseTime: "1d"}, netip.Addr{}); err != nil || !v.Enabled {
		t.Fatalf("%+v %v", v, err)
	}
	if _, set := opt(t, d, "lan", "ignore"); set {
		t.Fatal("ignore left")
	}
	// Untouched options stay.
	if got, _ := opt(t, d, "lan", "dhcpv6"); got != "server" {
		t.Fatalf("dhcpv6=%q", got)
	}
}

func TestSetSettingsEnablesAfterDhcpv4Disabled(t *testing.T) {
	s, d, _ := setup(t)
	if err := d.Set(t.Context(), "dhcp", "lan", "dhcpv4", "disabled"); err != nil {
		t.Fatal(err)
	}
	if err := d.Commit(t.Context(), "dhcp"); err != nil {
		t.Fatal(err)
	}
	v, err := s.SetSettings(t.Context(), DHCPSettings{Enabled: true, Start: 100, Limit: 150, LeaseTime: "12h"}, netip.Addr{})
	if err != nil || !v.Enabled {
		t.Fatalf("%+v %v", v, err)
	}
}

func TestSetSettingsRefusesInvalidAndStagedNetwork(t *testing.T) {
	s, d, f := setup(t)
	if _, err := s.SetSettings(t.Context(), DHCPSettings{Start: 200, Limit: 100, LeaseTime: "1h"}, netip.Addr{}); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if err := d.Set(t.Context(), "network", "lan", "ipaddr", "10.0.0.1/24"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetSettings(t.Context(), DHCPSettings{Start: 10, Limit: 10, LeaseTime: "1h"}, netip.Addr{}); !errors.Is(err, txn.ErrForeignStaged) {
		t.Fatal(err)
	}
	if f.commits != 0 {
		t.Fatal("committed")
	}
	if pending, _ := d.Pending(t.Context(), "dhcp"); pending {
		t.Fatal("dhcp left staged")
	}
}
