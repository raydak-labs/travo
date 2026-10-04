package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// The two real client identities this classifier has to tell apart, both taken
// from the live neighbour table on 192.168.1.1:
//
//	wired: 192.168.1.2  -> 9c:eb:e8:d3:f8:d1
//	wifi:  192.168.1.151 -> 22:4e:76:6c:2d:62 (the iPhone, associated with
//	                                   phy1-ap0 at the time)
//
// They sit in the SAME /24 on the SAME bridge. Nothing but "is this MAC
// associated with an access point" separates them, which is why matching the
// client's subnet alone is not enough to answer the lockout guard.
const (
	deviceWiredClientIP  = "192.168.1.2"
	deviceWiredClientMAC = "9c:eb:e8:d3:f8:d1"
	deviceWiFiClientIP   = "192.168.1.151"
	deviceWiFiClientMAC  = "22:4e:76:6c:2d:62"
	// A client behind the uplink STA, from the same capture.
	deviceUplinkClientIP = "10.0.1.146"
)

// deviceInterfaceDump is the verbatim `ubus call network.interface dump` from
// the test device. It is loaded from a file rather than written out here so it
// cannot drift from the capture again — the previous fixture invented an
// `ipv4-prefix` key that netifd does not emit, which is how a green suite came
// to certify a classifier that answered `unknown` for every real client.
func deviceInterfaceDump(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "network_interface_dump.json"))
	if err != nil {
		t.Fatalf("reading the captured dump: %v", err)
	}
	var dump map[string]any
	if err := json.Unmarshal(raw, &dump); err != nil {
		t.Fatalf("the captured dump is not valid JSON: %v", err)
	}
	return dump
}

// deviceNeighborTable is the verbatim /proc/net/arp from the test device.
func deviceNeighborTable(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "proc_net_arp.txt"))
	if err != nil {
		t.Fatalf("reading the captured neighbour table: %v", err)
	}
	return string(raw)
}

// newClassifierService wires a NetworkService onto the captured payload: the
// real interface dump, the real neighbour table, and an `iw` that answers with
// the access points listed in associatedAPStations.
func newClassifierService(t *testing.T) *NetworkService {
	t.Helper()
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	ub.RegisterResponse("network.interface.dump", deviceInterfaceDump(t))
	svc := NewNetworkServiceWithRunner(u, ub, &MockCommandRunner{
		RunFunc: fakeIw(noAssociatedAPs()),
	})
	svc.arpFile = writeTempFile(t, "arp", deviceNeighborTable(t))
	return svc
}

// associatedAPStations answers the two questions the classifier asks the
// radio: which interfaces run as access points, and which MACs each one has
// associated.
//
// NO AP INTERFACE WAS UP when the payload above was captured — the router was
// in uplink mode, so `iw dev` listed only phy0-sta0 and
// `iw dev phy1-ap0 station dump` answered "No such device". These two blocks
// are therefore NOT a live capture: they use the exact `iw` output format taken
// from the real `iw dev phy0-sta0 station dump` on the same device (see
// testdata/iw_station_dump.txt), and the associated MAC is the real iPhone MAC
// recorded in the neighbour table above. Everything is a real value; only the
// association is reconstructed. Do not read these as proof of a capture.
const associatedAPStations = `phy#0
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

// phy1ApStationDump is the iPhone as `iw` prints it on an AP interface. Same
// field set as the captured station dump in testdata/iw_station_dump.txt.
const phy1ApStationDump = `Station 22:4e:76:6c:2d:62 (on phy1-ap0)
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

// fakeIw answers the three `iw` invocations the classifier makes, using the
// same per-interface lookup as the real tool.
func fakeIw(stations map[string]string) func(string, ...string) ([]byte, error) {
	return func(name string, args ...string) ([]byte, error) {
		if name != "iw" {
			return nil, errCommandNotFound
		}
		switch {
		case len(args) == 1 && args[0] == "dev":
			return []byte(associatedAPStations), nil
		case len(args) == 4 && args[0] == "dev" && args[2] == "station":
			iface := args[1]
			dump, ok := stations[iface]
			if !ok {
				// An interface `iw dev` lists but that has no station dump
				// registered is a real failure, not an empty answer.
				return nil, errCommandNotFound
			}
			return []byte(dump), nil
		}
		return nil, errCommandNotFound
	}
}

var errCommandNotFound = &notFoundError{}

type notFoundError struct{}

func (*notFoundError) Error() string { return "command not found" }

// noAssociatedAPs answers `iw` with an empty station list for every access
// point: the wired client's case.
func noAssociatedAPs() map[string]string { return map[string]string{"phy1-ap0": ""} }

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestClassifyClient_WiredClientOnTheLANBridgeIsEthernet(t *testing.T) {
	svc := newClassifierService(t)

	got, err := svc.GetConnectionMethod(deviceWiredClientIP)
	if err != nil {
		t.Fatalf("GetConnectionMethod: %v", err)
	}
	// The wired client and the WiFi client share br-lan and a /24. Only the MAC
	// association separates them, so answering `wifi-ap` here would refuse a
	// wired operator the one change they are safe to make.
	if got.Method != "ethernet" {
		t.Errorf("method = %q, want \"ethernet\" (client %s, MAC %s)",
			got.Method, deviceWiredClientIP, deviceWiredClientMAC)
	}
	if got.Interface != "br-lan" {
		t.Errorf("interface = %q, want \"br-lan\"", got.Interface)
	}
}

func TestClassifyClient_AssociatedWiFiClientIsWifiAP(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	ub.RegisterResponse("network.interface.dump", deviceInterfaceDump(t))
	svc := NewNetworkServiceWithRunner(u, ub, &MockCommandRunner{
		RunFunc: fakeIw(map[string]string{"phy1-ap0": phy1ApStationDump}),
	})
	svc.arpFile = writeTempFile(t, "arp", deviceNeighborTable(t))

	got, err := svc.GetConnectionMethod(deviceWiFiClientIP)
	if err != nil {
		t.Fatalf("GetConnectionMethod: %v", err)
	}
	if got.Method != "wifi-ap" {
		t.Errorf("method = %q, want \"wifi-ap\" (client %s, MAC %s)",
			got.Method, deviceWiFiClientIP, deviceWiFiClientMAC)
	}
}

func TestClassifyClient_ClientBehindTheUplinkSTAIsWifiClient(t *testing.T) {
	svc := newClassifierService(t)

	got, err := svc.GetConnectionMethod(deviceUplinkClientIP)
	if err != nil {
		t.Fatalf("GetConnectionMethod: %v", err)
	}
	if got.Method != "wifi-client" {
		t.Errorf("method = %q, want \"wifi-client\"", got.Method)
	}
}

// A client on br-lan with no neighbour entry cannot be proven wired, and it
// cannot be proven to be on WiFi either. The guard refuses on `unknown`, so
// this must NOT collapse into `ethernet`.
func TestClassifyClient_UnresolvableMACOnTheLANBridgeIsUnknown(t *testing.T) {
	svc := newClassifierService(t)

	got, err := svc.GetConnectionMethod("192.168.1.99")
	if err != nil {
		t.Fatalf("GetConnectionMethod: %v", err)
	}
	if got.Method != "unknown" {
		t.Errorf("method = %q, want \"unknown\"", got.Method)
	}
}

func TestClassifyClient_DegenerateInputsAreUnknown(t *testing.T) {
	svc := newClassifierService(t)
	for _, ip := range []string{"", "::1", "127.0.0.1", "not-an-ip", "2001:db8::1"} {
		got, err := svc.GetConnectionMethod(ip)
		if err != nil {
			t.Fatalf("GetConnectionMethod(%q): %v", ip, err)
		}
		if got.Method != "unknown" {
			t.Errorf("GetConnectionMethod(%q).Method = %q, want \"unknown\"", ip, got.Method)
		}
	}
}

func TestClassifyClient_IPOutsideEveryPrefixIsUnknown(t *testing.T) {
	svc := newClassifierService(t)

	got, err := svc.GetConnectionMethod("8.8.8.8")
	if err != nil {
		t.Fatalf("GetConnectionMethod: %v", err)
	}
	if got.Method != "unknown" {
		t.Errorf("method = %q, want \"unknown\"", got.Method)
	}
}

func TestParseIfaceIPv4Prefixes_NetifdShapes(t *testing.T) {
	tests := []struct {
		name  string
		iface map[string]any
		want  []string
	}{
		{
			// THE REAL SHAPE. A bare address and a separate integer netmask.
			name:  "ipv4-address with an integer mask",
			iface: map[string]any{"ipv4-address": []any{map[string]any{"address": "192.168.1.1", "mask": float64(24)}}},
			want:  []string{"192.168.1.0/24"},
		},
		{
			name:  "a /32 mask",
			iface: map[string]any{"ipv4-address": []any{map[string]any{"address": "10.0.1.110", "mask": float64(32)}}},
			want:  []string{"10.0.1.110/32"},
		},
		{
			// Tolerance for netifd builds that put a CIDR in the same key.
			name:  "ipv4-address carrying a CIDR string",
			iface: map[string]any{"ipv4-address": []any{map[string]any{"address": "192.168.8.1/24"}}},
			want:  []string{"192.168.8.0/24"},
		},
		{
			// Tolerance for the key the old fixture invented.
			name:  "ipv4-prefix carrying a CIDR string",
			iface: map[string]any{"ipv4-prefix": []any{map[string]any{"address": "192.168.8.0/24"}}},
			want:  []string{"192.168.8.0/24"},
		},
		{
			name: "both keys together",
			iface: map[string]any{
				"ipv4-address": []any{map[string]any{"address": "192.168.1.1", "mask": float64(24)}},
				"ipv4-prefix":  []any{map[string]any{"address": "172.16.0.0/12"}},
			},
			want: []string{"192.168.1.0/24", "172.16.0.0/12"},
		},
		{
			// A malformed entry is skipped, and the good one beside it survives.
			// Dropping the whole list would put every client back on `unknown`.
			name: "a malformed entry does not hide a good one",
			iface: map[string]any{"ipv4-address": []any{
				map[string]any{"address": "not-an-ip", "mask": float64(24)},
				map[string]any{"address": "192.168.1.1", "mask": float64(24)},
			}},
			want: []string{"192.168.1.0/24"},
		},
		{
			name:  "an out-of-range mask is skipped",
			iface: map[string]any{"ipv4-address": []any{map[string]any{"address": "192.168.1.1", "mask": float64(33)}}},
			want:  nil,
		},
		{
			name:  "a missing mask yields no prefix rather than a /0",
			iface: map[string]any{"ipv4-address": []any{map[string]any{"address": "192.168.1.1"}}},
			want:  nil,
		},
		{
			name:  "a non-address entry is skipped",
			iface: map[string]any{"ipv4-address": []any{"192.168.1.1/24"}},
			want:  nil,
		},
		{
			name:  "no address keys at all",
			iface: map[string]any{"interface": "lan"},
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseIfaceIPv4Prefixes(tt.iface)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i, want := range tt.want {
				if got[i].String() != want {
					t.Errorf("prefix[%d] = %s, want %s", i, got[i], want)
				}
			}
		})
	}
}

// The captured dump has to actually parse, or every test above is running on a
// fixture that does not describe the device.
func TestParseIfaceIPv4Prefixes_CapturedDumpParses(t *testing.T) {
	dump := deviceInterfaceDump(t)
	ifaces, ok := dump["interface"].([]any)
	if !ok {
		t.Fatalf("captured dump has no interface array")
	}
	want := map[string]string{
		"br-lan":    "192.168.1.0/24",
		"lo":        "127.0.0.0/8",
		"phy0-sta0": "10.0.1.0/24",
	}
	found := map[string]string{}
	for _, raw := range ifaces {
		m, _ := raw.(map[string]any)
		l3, _ := m["l3_device"].(string)
		for _, p := range parseIfaceIPv4Prefixes(m) {
			found[l3] = p.String()
		}
	}
	for device, prefix := range want {
		if found[device] != prefix {
			t.Errorf("captured dump: %s parsed as %q, want %q",
				device, found[device], prefix)
		}
	}
}

func TestParseIwDev_APAndSTAInterfaces(t *testing.T) {
	// `iw dev` on the device right now lists ONLY the STA, because no access
	// point is up. An empty result is a real answer and must stay empty.
	got := parseIwDev(string(mustReadFile(t, "iw_dev.txt")))
	for _, iface := range got {
		if strings.Contains(iface, "-ap") {
			t.Errorf("captured `iw dev` unexpectedly lists an AP: %v", got)
		}
	}

	// With an access point up, it is listed.
	got = parseIwDev(associatedAPStations)
	if len(got) != 1 || got[0] != "phy1-ap0" {
		t.Errorf("parseIwDev = %v, want [phy1-ap0]", got)
	}
}

func mustReadFile(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
