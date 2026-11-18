package usbtether

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/pkgmgr"
)

const captures = "../../testdata/device/axt1800-25.12.6/"

func read(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(captures + path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// router is the AXT1800 as captured: wwan uplink, wan zone `@zone[1]`, no phone attached.
func router(t *testing.T) *mock.Device {
	t.Helper()
	d := mock.New()
	for _, c := range []string{"network", "firewall"} {
		secs, err := cli.ParseExport(c, string(read(t, "usbtether/uci-export-"+c+".txt")))
		if err != nil {
			t.Fatal(err)
		}
		if err := d.Load(context.Background(), c, secs); err != nil {
			t.Fatal(err)
		}
	}
	if err := mock.SeedNetLinks(d); err != nil {
		t.Fatal(err)
	}
	d.SetUbusReply("network.device", "status", read(t, "device-status-phy0-sta0.json"))
	return d
}

func service(d *mock.Device, pm pkgmgr.PackageManager) *Service {
	return &Service{UCI: d, Ubus: d, Links: d, Packages: pm}
}

func TestStatusWithoutPhone(t *testing.T) {
	st, err := service(router(t), pkgmgr.NewMock()).Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Links) != 0 || st.Configured || st.InWANZone || st.Up || st.Device != "" {
		t.Fatalf("status = %+v, want nothing detected or configured", st)
	}
	if len(st.Drivers) != 4 {
		t.Fatalf("drivers = %+v", st.Drivers)
	}
	for _, d := range st.Drivers {
		if d.State != "absent" {
			t.Errorf("%s = %s, want absent", d.Package, d.State)
		}
	}
}

func TestStatusDetectsTetherLinkByDriver(t *testing.T) {
	d := router(t)
	d.AddNetLink(device.NetLink{Name: "eth0", Bus: "usb", Driver: "ipheth"})
	d.AddNetLink(device.NetLink{Name: "eth1", Bus: "usb", Driver: "r8152"}) // USB Ethernet, not a phone
	st, err := service(d, pkgmgr.NewMock()).Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Links) != 1 || st.Links[0].Name != "eth0" || st.Links[0].Kind != "ipheth" {
		t.Fatalf("links = %+v", st.Links)
	}
	if c := st.Links[0].Carrier; c == nil || !*c {
		t.Fatalf("carrier = %v, want true from the captured device status", c)
	}
}

func TestStatusConfiguredInterface(t *testing.T) {
	d := router(t)
	ctx := context.Background()
	must(t, d.SetType(ctx, "network", Interface, "interface"))
	must(t, d.Set(ctx, "network", Interface, "device", "usb0"))
	must(t, d.AddToList(ctx, "firewall", "@zone[1]", "network", Interface))
	must(t, d.Commit(ctx, "network"))
	must(t, d.Commit(ctx, "firewall"))
	// No usbtether capture exists yet; netifd's dhcp interface payload has the same shape.
	d.SetUbusReply("network.interface."+Interface, "status", read(t, "interface-wwan-status.json"))
	st, err := service(d, pkgmgr.NewMock()).Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Configured || st.Device != "usb0" || !st.InWANZone || !st.Up || st.IPv4 == "" {
		t.Fatalf("status = %+v", st)
	}
}

func TestStatusConfiguredButNetifdHasNoObject(t *testing.T) {
	d := router(t)
	ctx := context.Background()
	must(t, d.SetType(ctx, "network", Interface, "interface"))
	must(t, d.Commit(ctx, "network"))
	st, err := service(d, pkgmgr.NewMock()).Status(ctx)
	if err != nil || !st.Configured || st.Up {
		t.Fatalf("status = %+v, %v; want configured and down", st, err)
	}
}

type brokenPM struct{ pkgmgr.PackageManager }

func (brokenPM) Version(context.Context, string) (string, error) {
	return "", errors.New("apk: lock busy")
}

func TestStatusFailedPackageQueryIsUnknownNotAbsent(t *testing.T) {
	st, err := service(router(t), brokenPM{}).Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range st.Drivers {
		if d.State != "unknown" {
			t.Errorf("%s = %s, want unknown", d.Package, d.State)
		}
	}
}

func TestStatusWithoutPortsIsUnavailable(t *testing.T) {
	if _, err := (&Service{}).Status(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v", err)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
