package facts

import (
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

const dir = "../../../testdata/device/axt1800-25.12.6"

func read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestInterfaceDumpFromDevice(t *testing.T) {
	ifs, err := ParseInterfaceDump(read(t, "interface-dump.json"))
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]Interface{}
	for _, i := range ifs {
		by[i.Name] = i
	}
	lan := by["lan"]
	if !lan.Up || lan.L3Device != "br-lan" || len(lan.IPv4) != 1 || lan.IPv4[0] != netip.MustParsePrefix("192.168.1.1/24") {
		t.Fatalf("lan %+v", lan)
	}
	if lan.IPv4[0].Masked() != netip.MustParsePrefix("192.168.1.0/24") {
		t.Fatal("masked")
	}
	if w := by["wwan"]; !w.Up || w.L3Device != "phy0-sta0" || !w.HasDefRte || !slices.Equal(w.DNSServers, []string{"192.168.90.2"}) {
		t.Fatalf("wwan %+v", w)
	}
	if w := by["wan"]; w.Up || w.L3Device != "" || len(w.DNSServers) != 0 {
		t.Fatalf("wan (cable unplugged) %+v", w)
	}
}

func TestInterfaceDumpRejectsCIDRStrings(t *testing.T) {
	// The shape that once made every caller "unknown": a CIDR string instead of {address, mask}.
	bad := []byte(`{"interface":[{"interface":"lan","up":true,"ipv4-address":[{"address":"192.168.1.1/24"}]}]}`)
	if _, err := ParseInterfaceDump(bad); err == nil {
		t.Fatal("accepted CIDR string")
	}
	if _, err := ParseInterfaceDump([]byte(`{"interface":[{"interface":"wwan","up":true,"dns-server":["hotel"]}]}`)); err == nil {
		t.Fatal("accepted a non-address dns-server")
	}
	if _, err := ParseInterfaceDump([]byte(`{}`)); !errors.Is(err, ErrShape) {
		t.Fatal(err)
	}
}

func TestNeighFromDevice(t *testing.T) {
	ns, err := ParseNeighJSON(read(t, "ip-neigh.json"))
	if err != nil || len(ns) != 2 {
		t.Fatal(ns, err)
	}
	if ns[0].IP != netip.MustParseAddr("192.168.1.2") || ns[0].Dev != "br-lan" || ns[0].MAC != "9c:eb:e8:d3:f8:d1" {
		t.Fatalf("%+v", ns[0])
	}
	inc := []byte(`[{"dst":"192.168.1.9","dev":"br-lan","state":["INCOMPLETE"]},{"dst":"192.168.1.8","dev":"br-lan","lladdr":"aa:bb:cc:dd:ee:ff","state":["FAILED"]}]`)
	if ns, _ := ParseNeighJSON(inc); len(ns) != 0 {
		t.Fatal("unusable entries kept")
	}
}

func TestDeviceStatusFromDevice(t *testing.T) {
	for _, f := range []string{"device-status-phy1-ap0.json", "device-status-phy0-sta0.json", "device-status-br-lan.json"} {
		l, err := ParseDeviceStatus(read(t, f))
		if err != nil || !l.Present || !l.Up || !l.Carrier {
			t.Errorf("%s: %+v %v", f, l, err)
		}
	}
	if _, err := ParseDeviceStatus([]byte(`{"up":true}`)); !errors.Is(err, ErrShape) {
		t.Fatal("partial payload accepted: must be cannot-verify, not down")
	}
}

func TestWirelessStatusFromDevice(t *testing.T) {
	rs, err := ParseWirelessStatus(read(t, "wireless-status.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := []RadioIface{
		{Radio: "radio0", RadioUp: true, Section: "wifinet3", Ifname: "phy0-sta0"},
		{Radio: "radio1", RadioUp: true, Section: "default_radio1", Ifname: "phy1-ap0"},
	}
	if len(rs) != len(want) {
		t.Fatal(rs)
	}
	for i := range want {
		if rs[i] != want[i] {
			t.Errorf("%d: %+v want %+v", i, rs[i], want[i])
		}
	}
}

func TestKernelDefaultRouteFromDevice(t *testing.T) {
	if !ParseKernelDefaultRoute(string(read(t, "ip-route-default.txt"))) {
		t.Fatal("device has a default route")
	}
	if ParseKernelDefaultRoute("192.168.1.0/24 dev br-lan proto kernel scope link src 192.168.1.1\n") {
		t.Fatal("no default")
	}
}

func TestHostapdClientsFromDevice(t *testing.T) {
	empty, err := ParseHostapdClients(read(t, "hostapd-phy1-ap0-get_clients.json"))
	if err != nil || len(empty) != 0 {
		t.Fatal(empty, err)
	}
	one, err := ParseHostapdClients(read(t, "with-ap-client/hostapd-phy1-ap0-get_clients.json"))
	if err != nil || len(one) != 1 || one[0] != "22:4e:76:6c:2d:62" {
		t.Fatal(one, err)
	}
	if _, err := ParseHostapdClients([]byte(`{"clients":{}}`)); !errors.Is(err, ErrShape) {
		t.Fatal("payload without freq accepted")
	}
}
