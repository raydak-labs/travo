package lockout

import (
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	dfacts "github.com/blackdark-org/travo-greenfield/internal/device/facts"
)

const dev = "../../testdata/device/axt1800-25.12.6"

// End to end on verbatim device payloads: Ethernet laptop vs phone on the AP, same /24.
func TestClassifyOnDevicePayloads(t *testing.T) {
	rd := func(n string) []byte {
		b, err := os.ReadFile(filepath.Join(dev, n))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	ifs, err := dfacts.ParseInterfaceDump(rd("interface-dump.json"))
	if err != nil {
		t.Fatal(err)
	}
	ns, err := dfacts.ParseNeighJSON(rd("with-ap-client/ip-neigh.json"))
	if err != nil {
		t.Fatal(err)
	}
	sta, err := dfacts.ParseHostapdClients(rd("with-ap-client/hostapd-phy1-ap0-get_clients.json"))
	if err != nil {
		t.Fatal(err)
	}
	f := Facts{Neighbours: map[netip.Addr]string{}, Stations: map[string][]string{"phy1-ap0": sta}, EnabledAPs: []string{"phy1-ap0"}}
	for _, n := range ns {
		f.Neighbours[n.IP] = n.MAC
	}
	for _, i := range ifs {
		for _, p := range i.IPv4 {
			switch i.Name {
			case "lan":
				f.BridgeNets = append(f.BridgeNets, p.Masked())
			case "wan", "wwan", "usbtether":
				f.UplinkNets = append(f.UplinkNets, p.Masked())
			}
		}
	}
	cases := map[string]Caller{
		"192.168.1.2":   {Method: Ethernet},               // capture laptop on the cable
		"192.168.1.151": {Method: WifiAP, AP: "phy1-ap0"}, // phone on the AP
		"192.168.90.2":  {Method: WifiClient},             // upstream router, uplink side
		"192.168.1.200": {Method: Unknown},                // no neighbour entry
	}
	for ip, want := range cases {
		if got := Classify(netip.MustParseAddr(ip), f); got != want {
			t.Errorf("%s: got %+v want %+v", ip, got, want)
		}
	}
}
