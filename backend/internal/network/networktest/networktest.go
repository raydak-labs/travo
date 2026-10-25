// Package networktest holds device captures and fakes shared by the network
// and wifi tests: the test device's netifd dump, neighbour table and iw output.
package networktest

import (
	"embed"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

//go:embed testdata
var testdata embed.FS

// The two real client identities this classifier has to tell apart, both taken
// from the live neighbour table on 192.168.1.1:
//
//	wired: 192.168.1.2  -> 02:00:00:00:00:01
//	wifi:  192.168.1.151 -> 02:00:00:00:00:02 (the iPhone, associated with
//	                                   phy1-ap0 at the time)
//
// They sit in the SAME /24 on the SAME bridge. Nothing but "is this MAC
// associated with an access point" separates them, which is why matching the
// client's subnet alone is not enough to answer the lockout guard.
const (
	DeviceWiredClientIP  = "192.168.1.2"
	DeviceWiredClientMAC = "02:00:00:00:00:01"
	DeviceWiFiClientIP   = "192.168.1.151"
	DeviceWiFiClientMAC  = "02:00:00:00:00:02"
	// A client behind the uplink STA, from the same capture.
	DeviceUplinkClientIP = "10.0.1.146"
)

// DeviceInterfaceDump is the verbatim `ubus call network.interface dump` from
// the test device. It is loaded from a file rather than written out here so it
// cannot drift from the capture again — the previous fixture invented an
// `ipv4-prefix` key that netifd does not emit, which is how a green suite came
// to certify a classifier that answered `unknown` for every real client.
func DeviceInterfaceDump(t *testing.T) map[string]any {
	t.Helper()
	raw, err := testdata.ReadFile("testdata/network_interface_dump.json")
	if err != nil {
		t.Fatalf("reading the captured dump: %v", err)
	}
	var dump map[string]any
	if err := json.Unmarshal(raw, &dump); err != nil {
		t.Fatalf("the captured dump is not valid JSON: %v", err)
	}
	return dump
}

// DeviceNeighborTable is /proc/net/arp from the test device, verbatim apart from
// two client MACs redacted to the locally-administered 02:00:00:00:00:0x series
// (see testdata/proc_net_arp.txt).
func DeviceNeighborTable(t *testing.T) string {
	t.Helper()
	raw, err := testdata.ReadFile("testdata/proc_net_arp.txt")
	if err != nil {
		t.Fatalf("reading the captured neighbour table: %v", err)
	}
	return string(raw)
}

// AssociatedAPStations answers the two questions the classifier asks the
// radio: which interfaces run as access points, and which MACs each one has
// associated.
//
// NO AP INTERFACE WAS UP when the payload above was captured — the router was
// in uplink mode, so `iw dev` listed only phy0-sta0 and
// `iw dev phy1-ap0 station dump` answered "No such device". These two blocks
// are therefore NOT a live capture: they use the exact `iw` output format taken
// from the real `iw dev phy0-sta0 station dump` on the same device (see
// testdata/iw_station_dump.txt), and the associated MAC is the redacted client MAC
// from the neighbour table above. The format and every field shape are real; only
// the association is reconstructed. Do not read these as proof of a capture.
const AssociatedAPStations = `phy#0
	Interface phy0-sta0
		ifindex 23
		wdev 0x9
		addr 9e:6c:d4:9d:05:d0
		ssid Cappuxinno
		type managed
		channel 149 (5745 MHz), width: 80 MHz, center1: 5775 MHz
		txpower 24.00 dBm
phy#1
	Interface phy1-ap0
		ifindex 24
		wdev 0xa
		addr 9e:6c:d4:9d:05:d1
		ssid OpenWrt-Travel
		type AP
		channel 36 (5180 MHz), width: 80 MHz, center1: 5210 MHz
		txpower 23.00 dBm
`

// Phy1APStationDump is the iPhone as `iw` prints it on an AP interface. Same
// field set as the captured station dump in testdata/iw_station_dump.txt.
const Phy1APStationDump = `Station 02:00:00:00:00:02 (on phy1-ap0)
	authorized:	yes
	authenticated:	yes
	associated:	yes
	preamble:	long
	WMM/WME:	yes
	MFP:		no
	TDLS peer:	no
	inactive time:	0 ms
	rx bytes:	48231104
	rx packets:	41203
	tx bytes:	3012772
	tx packets:	21044
	tx retries:	0
	tx failed:	0
	beacon loss:	0
		beacon rx:	5588
	rx drop misc:	0
	signal:  	-52 dBm
	signal avg:  	-53 dBm
	beacon signal avg:	-52 dBm
	tx bitrate:	1200.9 MBit/s 80MHz HE-MCS 11 HE-NSS 2 HE-GI 0 HE-DCM 0
	last ack signal:-54 dBm
	expected throughput:	978.710Mbps
	DTIM period:	2
	beacon interval:	100
	short slot time:	yes
	connected time:	4247 seconds
`

// FakeIw answers the three `iw` invocations the classifier makes, using the
// same per-interface lookup as the real tool.
func FakeIw(stations map[string]string) func(string, ...string) ([]byte, error) {
	return func(name string, args ...string) ([]byte, error) {
		if name != "iw" {
			return nil, ErrCommandNotFound
		}
		switch {
		case len(args) == 1 && args[0] == "dev":
			return []byte(AssociatedAPStations), nil
		case len(args) == 4 && args[0] == "dev" && args[2] == "station":
			iface := args[1]
			dump, ok := stations[iface]
			if !ok {
				// An interface `iw dev` lists but that has no station dump
				// registered is a real failure, not an empty answer.
				return nil, ErrCommandNotFound
			}
			return []byte(dump), nil
		}
		return nil, ErrCommandNotFound
	}
}

var ErrCommandNotFound = &notFoundError{}

type notFoundError struct{}

func (*notFoundError) Error() string { return "command not found" }

// NoAssociatedAPs answers `iw` with an empty station list for every access
// point: the wired client's case.
func NoAssociatedAPs() map[string]string { return map[string]string{"phy1-ap0": ""} }

func WriteTempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func ReadFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := testdata.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
