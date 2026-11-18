package clients

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

const captures = "../../testdata/device/axt1800-25.12.6/"

var now = time.Unix(1794284022-3600, 0).UTC() // one hour before the captured lease expires

func read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(captures + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func parsed(t *testing.T, config, name string, export bool) []device.Section {
	t.Helper()
	text, parse := string(read(t, name)), cli.ParseShow
	if export {
		parse = cli.ParseExport
		if !strings.HasPrefix(text, "package ") { // section excerpts of `uci export`
			text = "package " + config + "\n\n" + text
		}
	}
	secs, err := parse(config, text)
	if err != nil {
		t.Fatal(err)
	}
	return secs
}

// fakeTxns mirrors txn.Manager: Begin and Apply stage, refuse empty Begin, commit.
type fakeTxns struct {
	d       *mock.Device
	begins  []string
	applies int
}

func (f *fakeTxns) Begin(ctx context.Context, summary string, configs []string, stage func(context.Context) error, _ txn.Probe) (txn.Status, error) {
	f.begins = append(f.begins, summary)
	if err := stage(ctx); err != nil {
		return txn.Status{}, errors.Join(err, f.d.Revert(ctx, configs[0]))
	}
	if err := f.d.Commit(ctx, configs[0]); err != nil {
		return txn.Status{}, err
	}
	return txn.Status{ID: "0123456789abcdef", State: txn.Pending, Configs: configs}, nil
}

func (f *fakeTxns) Apply(ctx context.Context, configs []string, stage func(context.Context) error) error {
	if err := stage(ctx); err != nil {
		return errors.Join(err, f.d.Revert(ctx, configs[0]))
	}
	f.applies++
	return f.d.Commit(ctx, configs[0])
}

// setup builds a service on the AXT1800 captures: one WiFi station (22:4e:...) leased on
// phy1-ap0, the wired capture host (9c:eb:...) and the uplink gateway on phy0-sta0.
func setup(t *testing.T) (*Service, *mock.Device, *fakeTxns) {
	t.Helper()
	d := mock.New()
	load := func(config string, secs []device.Section) {
		if err := d.Load(t.Context(), config, secs); err != nil {
			t.Fatal(err)
		}
	}
	load("dhcp", parsed(t, "dhcp", "uci-export-dhcp.txt", true))
	load("network", parsed(t, "network", "clients/uci-show-network-lan.txt", false))
	load("firewall", parsed(t, "firewall", "uci-show-firewall.txt", false))
	d.SetUbusReply("network.interface", "dump", read(t, "interface-dump.json"))
	d.SetUbusReply("network.wireless", "status", read(t, "with-ap-client/wireless-status.json"))
	d.SetUbusReply("hostapd.phy1-ap0", "get_clients", read(t, "with-ap-client/hostapd-phy1-ap0-get_clients.json"))
	d.SetNeighbours(read(t, "with-ap-client/ip-neigh.json"))
	d.SetLeases(read(t, "with-ap-client/dhcp-leases.txt"))
	f := &fakeTxns{d: d}
	dir := t.TempDir()
	return &Service{UCI: d, Ubus: d, Neigh: d, Leases: d, Kicker: d, Txns: f,
		Guards:  guard.Dir{Path: filepath.Join(dir, "guards"), Now: time.Now},
		Aliases: &AliasStore{Path: filepath.Join(dir, "clients.json")},
		Now:     func() time.Time { return now }}, d, f
}
