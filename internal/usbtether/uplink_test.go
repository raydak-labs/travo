package usbtether

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/pkgmgr"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// fakeTxns commits the staged configs and runs the probe, like Begin without the watchdog.
type fakeTxns struct {
	d        *mock.Device
	configs  []string
	probeErr error
	// beforeStage runs after Set's early read and before stage, like a concurrent writer
	// that commits while Begin waits for the locks.
	beforeStage func()
}

func (f *fakeTxns) Begin(ctx context.Context, _ string, configs []string, stage func(context.Context) error, probe txn.Probe) (txn.Status, error) {
	f.configs = configs
	if f.beforeStage != nil {
		f.beforeStage()
	}
	if err := stage(ctx); err != nil {
		for _, c := range configs {
			_ = f.d.Revert(ctx, c)
		}
		return txn.Status{}, err
	}
	for _, c := range configs {
		if err := f.d.Commit(ctx, c); err != nil {
			return txn.Status{}, err
		}
	}
	if f.probeErr = probe(ctx); f.probeErr != nil {
		return txn.Status{Meta: txn.Meta{ID: "0123456789abcdef"}, State: txn.RolledBack}, txn.ErrProbeFailed
	}
	return txn.Status{Meta: txn.Meta{ID: "0123456789abcdef"}, State: txn.Pending}, nil
}

func uplinkService(t *testing.T, d *mock.Device) (*Service, *fakeTxns) {
	t.Helper()
	f := &fakeTxns{d: d}
	s := service(d, pkgmgr.NewMock())
	s.Txns, s.ProbeWait = f, 50*time.Millisecond
	return s, f
}

func section(t *testing.T, d *mock.Device, config, name string) (device.Section, bool) {
	t.Helper()
	secs, err := d.Show(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(secs, func(x device.Section) bool { return x.Name == name })
	if i < 0 {
		return device.Section{}, false
	}
	return secs[i], true
}

func TestEnableBindsTheOnlyTetherLinkAndJoinsWANZone(t *testing.T) {
	d := router(t)
	d.AddNetLink(device.NetLink{Name: "usb0", Bus: "usb", Driver: "rndis_host"})
	d.SetUbusReply("network.interface."+Interface, "status", read(t, "interface-wwan-status.json"))
	s, f := uplinkService(t, d)
	st, err := s.Set(context.Background(), Request{Enabled: true})
	if err != nil || st.State != txn.Pending {
		t.Fatalf("Set = %+v, %v (probe %v)", st, err, f.probeErr)
	}
	if !slices.Equal(f.configs, []string{"network", "firewall"}) {
		t.Fatalf("configs = %v", f.configs)
	}
	iface, ok := section(t, d, "network", Interface)
	if !ok || iface.Type != "interface" || iface.Options["proto"] != "dhcp" || iface.Options["device"] != "usb0" || iface.Options["metric"] != Metric {
		t.Fatalf("network.usbtether = %+v", iface)
	}
	zone, _ := section(t, d, "firewall", "@zone[1]")
	if !slices.Equal(zone.Lists["network"], []string{"wan", "wan6", "wwan", "usbtether"}) {
		t.Fatalf("wan zone networks = %v", zone.Lists["network"])
	}
	if _, err := s.Set(context.Background(), Request{Enabled: true}); !errors.Is(err, txn.ErrNoChanges) {
		t.Fatalf("second enable = %v, want ErrNoChanges", err)
	}
}

func TestEnableRefusesBeforeAnyWrite(t *testing.T) {
	cases := map[string]struct {
		links []device.NetLink
		req   Request
		want  error
	}{
		"no phone":     {nil, Request{Enabled: true}, ErrNoDevice},
		"unknown link": {[]device.NetLink{{Name: "usb0", Bus: "usb", Driver: "cdc_ncm"}}, Request{Enabled: true, Device: "wan"}, ErrNoDevice},
		"two phones":   {[]device.NetLink{{Name: "usb0", Bus: "usb", Driver: "cdc_ncm"}, {Name: "eth0", Bus: "usb", Driver: "ipheth"}}, Request{Enabled: true}, ErrAmbiguous},
		"usb ethernet": {[]device.NetLink{{Name: "eth1", Bus: "usb", Driver: "r8152"}}, Request{Enabled: true}, ErrNoDevice},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			d := router(t)
			for _, l := range c.links {
				d.AddNetLink(l)
			}
			s, f := uplinkService(t, d)
			if _, err := s.Set(context.Background(), c.req); !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
			if f.configs != nil {
				t.Fatal("a transaction started")
			}
		})
	}
}

func TestEnableRefusesOptionStyleZoneNetworks(t *testing.T) {
	d := router(t)
	d.AddNetLink(device.NetLink{Name: "usb0", Bus: "usb", Driver: "rndis_host"})
	ctx := context.Background()
	must(t, d.DeleteOption(ctx, "firewall", "@zone[1]", "network"))
	must(t, d.Set(ctx, "firewall", "@zone[1]", "network", "wan wan6 wwan"))
	must(t, d.Commit(ctx, "firewall"))
	s, f := uplinkService(t, d)
	if _, err := s.Set(ctx, Request{Enabled: true}); !errors.Is(err, ErrUnsupported) || f.configs != nil {
		t.Fatalf("err = %v, txn started %v", err, f.configs != nil)
	}
}

func TestEnableProbeFailsWithoutDHCPAddress(t *testing.T) {
	d := router(t)
	d.AddNetLink(device.NetLink{Name: "usb0", Bus: "usb", Driver: "rndis_host"})
	// netifd never set the interface up: no object at all.
	s, f := uplinkService(t, d)
	if _, err := s.Set(context.Background(), Request{Enabled: true}); !errors.Is(err, txn.ErrProbeFailed) || !errors.Is(f.probeErr, ErrNotUp) {
		t.Fatalf("err = %v, probe = %v", err, f.probeErr)
	}
}

func TestDisableRemovesOnlyOwnedState(t *testing.T) {
	d := router(t)
	d.AddNetLink(device.NetLink{Name: "usb0", Bus: "usb", Driver: "rndis_host"})
	d.SetUbusReply("network.interface."+Interface, "status", read(t, "interface-wwan-status.json"))
	s, f := uplinkService(t, d)
	if _, err := s.Set(context.Background(), Request{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	d.SetUbusReply("network.interface."+Interface, "status", read(t, "wwan-down/interface-wwan-status-down.json"))
	st, err := s.Set(context.Background(), Request{Enabled: false})
	if err != nil || st.State != txn.Pending {
		t.Fatalf("disable = %+v, %v (probe %v)", st, err, f.probeErr)
	}
	if _, ok := section(t, d, "network", Interface); ok {
		t.Fatal("network.usbtether still present")
	}
	if _, ok := section(t, d, "network", "wwan"); !ok {
		t.Fatal("network.wwan was touched")
	}
	zone, _ := section(t, d, "firewall", "@zone[1]")
	if !slices.Equal(zone.Lists["network"], []string{"wan", "wan6", "wwan"}) || zone.Options["name"] != "wan" {
		t.Fatalf("wan zone = %+v", zone)
	}
	if _, err := s.Set(context.Background(), Request{Enabled: false}); !errors.Is(err, txn.ErrNoChanges) {
		t.Fatalf("second disable = %v, want ErrNoChanges", err)
	}
}

func TestEnableRereadsTheZoneUnderTheLocks(t *testing.T) {
	d := router(t)
	d.AddNetLink(device.NetLink{Name: "usb0", Bus: "usb", Driver: "rndis_host"})
	d.SetUbusReply("network.interface."+Interface, "status", read(t, "interface-wwan-status.json"))
	s, f := uplinkService(t, d)
	f.beforeStage = func() {
		ctx := context.Background()
		if err := d.AddToList(ctx, "firewall", "@zone[1]", "network", Interface); err != nil {
			t.Fatal(err)
		}
		if err := d.Commit(ctx, "firewall"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Set(context.Background(), Request{Enabled: true}); err != nil {
		t.Fatalf("Set = %v (probe %v)", err, f.probeErr)
	}
	zone, _ := section(t, d, "firewall", "@zone[1]")
	if !slices.Equal(zone.Lists["network"], []string{"wan", "wan6", "wwan", "usbtether"}) {
		t.Fatalf("wan zone networks = %v; want usbtether once", zone.Lists["network"])
	}
}
