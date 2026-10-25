package network

import (
	"fmt"
	"strings"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/network/networktest"
	"github.com/openwrt-travel-gui/backend/internal/platform"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// newClassifierService wires a NetworkService onto the captured payload: the
// real interface dump, the real neighbour table, and an `iw` that answers with
// the access points listed in associatedAPStations.
func newClassifierService(t *testing.T) *Service {
	t.Helper()
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	ub.RegisterResponse("network.interface.dump", networktest.DeviceInterfaceDump(t))
	svc := New(Deps{UCI: u, Ubus: ub, Cmd: &platform.MockCommandRunner{
		RunFunc: networktest.FakeIw(networktest.NoAssociatedAPs()),
	}})
	svc.arpFile = networktest.WriteTempFile(t, "arp", networktest.DeviceNeighborTable(t))
	return svc
}

func TestClassifyClient_WiredClientOnTheLANBridgeIsEthernet(t *testing.T) {
	svc := newClassifierService(t)

	got, err := svc.GetConnectionMethod(networktest.DeviceWiredClientIP)
	if err != nil {
		t.Fatalf("GetConnectionMethod: %v", err)
	}
	// The wired client and the WiFi client share br-lan and a /24. Only the MAC
	// association separates them, so answering `wifi-ap` here would refuse a
	// wired operator the one change they are safe to make.
	if got.Method != "ethernet" {
		t.Errorf("method = %q, want \"ethernet\" (client %s, MAC %s)",
			got.Method, networktest.DeviceWiredClientIP, networktest.DeviceWiredClientMAC)
	}
	if got.Interface != "br-lan" {
		t.Errorf("interface = %q, want \"br-lan\"", got.Interface)
	}
}

func TestClassifyClient_AssociatedWiFiClientIsWifiAP(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	ub.RegisterResponse("network.interface.dump", networktest.DeviceInterfaceDump(t))
	svc := New(Deps{UCI: u, Ubus: ub, Cmd: &platform.MockCommandRunner{
		RunFunc: networktest.FakeIw(map[string]string{"phy1-ap0": networktest.Phy1APStationDump}),
	}})
	svc.arpFile = networktest.WriteTempFile(t, "arp", networktest.DeviceNeighborTable(t))

	got, err := svc.GetConnectionMethod(networktest.DeviceWiFiClientIP)
	if err != nil {
		t.Fatalf("GetConnectionMethod: %v", err)
	}
	if got.Method != "wifi-ap" {
		t.Errorf("method = %q, want \"wifi-ap\" (client %s, MAC %s)",
			got.Method, networktest.DeviceWiFiClientIP, networktest.DeviceWiFiClientMAC)
	}
}

func TestClassifyClient_ClientBehindTheUplinkSTAIsWifiClient(t *testing.T) {
	svc := newClassifierService(t)

	got, err := svc.GetConnectionMethod(networktest.DeviceUplinkClientIP)
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
	dump := networktest.DeviceInterfaceDump(t)
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
	got := parseIwDev(string(networktest.ReadFixture(t, "iw_dev.txt")))
	for _, iface := range got {
		if strings.Contains(iface, "-ap") {
			t.Errorf("captured `iw dev` unexpectedly lists an AP: %v", got)
		}
	}

	// With an access point up, it is listed.
	got = parseIwDev(networktest.AssociatedAPStations)
	if len(got) != 1 || got[0] != "phy1-ap0" {
		t.Errorf("parseIwDev = %v, want [phy1-ap0]", got)
	}
}

func TestGetConnectionMethod(t *testing.T) {
	tests := []struct {
		name         string
		clientIP     string
		ubusResponse map[string]any
		// arp is the neighbour table the case runs against. A br-lan client
		// cannot be classified from the dump alone — it needs a MAC, and then
		// that MAC's association state. See client_classifier_test.go for the
		// same cases run against the payload captured from the device.
		arp            string
		assoc          string
		expectedMethod string
		expectError    bool
	}{
		{
			name:           "empty IP",
			clientIP:       "",
			expectedMethod: "unknown",
			expectError:    false,
		},
		{
			name:           "localhost IPv6",
			clientIP:       "::1",
			expectedMethod: "unknown",
			expectError:    false,
		},
		{
			name:           "localhost IPv4",
			clientIP:       "127.0.0.1",
			expectedMethod: "unknown",
			expectError:    false,
		},
		{
			name:           "invalid IP",
			clientIP:       "not-an-ip",
			expectedMethod: "unknown",
			expectError:    false,
		},
		{
			name:           "IPv6 address not supported",
			clientIP:       "2001:db8::1",
			expectedMethod: "unknown",
			expectError:    false,
		},
		{
			name:           "ubus error returns unknown",
			clientIP:       "192.168.1.100",
			expectedMethod: "unknown",
			expectError:    false,
		},
		{
			name:     "malformed ubus interface",
			clientIP: "192.168.1.100",
			ubusResponse: map[string]any{
				"interface": "not-a-map",
			},
			expectedMethod: "unknown",
			expectError:    false,
		},
		{
			name:     "wwan interface matches WiFi client",
			clientIP: "192.168.20.100",
			ubusResponse: map[string]any{
				"interface": []any{
					map[string]any{
						"interface": true,
						"up":        true,
						"l3_device": "wwan0",
						"ipv4-address": []any{
							map[string]any{"address": "192.168.20.0", "mask": float64(24)},
						},
					},
				},
			},
			expectedMethod: "wifi-client",
			expectError:    false,
		},
		{
			name:     "phySTA device matches WiFi client",
			clientIP: "192.168.30.50",
			ubusResponse: map[string]any{
				"interface": []any{
					map[string]any{
						"interface": true,
						"up":        true,
						"device":    "phy0-sta",
						"ipv4-address": []any{
							map[string]any{"address": "192.168.30.0", "mask": float64(24)},
						},
					},
				},
			},
			expectedMethod: "wifi-client",
			expectError:    false,
		},
		{
			// The REAL netifd shape: a bare address plus a separate integer mask.
			// The old fixture used an `ipv4-prefix` CIDR key that the device does
			// not emit, so this case passed without ever exercising the code the
			// router runs.
			name:     "ipv4-address with an integer mask matches a WiFi AP client",
			clientIP: "192.168.1.200",
			ubusResponse: map[string]any{
				"interface": []any{
					map[string]any{
						"interface": true,
						"up":        true,
						"l3_device": "br-lan",
						"ipv4-address": []any{
							map[string]any{"address": "192.168.1.1", "mask": float64(24)},
						},
					},
				},
			},
			arp:            "192.168.1.200  0x1  0x2  AA:BB:CC:11:22:33  *  br-lan",
			assoc:          "AA:BB:CC:11:22:33",
			expectedMethod: "wifi-ap",
			expectError:    false,
		},
		{
			name:     "a br-lan client with no association is Ethernet",
			clientIP: "192.168.1.201",
			ubusResponse: map[string]any{
				"interface": []any{
					map[string]any{
						"interface": true,
						"up":        true,
						"l3_device": "br-lan",
						"ipv4-address": []any{
							map[string]any{"address": "192.168.1.1", "mask": float64(24)},
						},
					},
				},
			},
			arp:            "192.168.1.201  0x1  0x2  9C:EB:E8:D3:F8:D1  *  br-lan",
			expectedMethod: "ethernet",
			expectError:    false,
		},
		{
			// CIDR tolerance in `ipv4-prefix`, kept because some netifd builds
			// emit it and the classifier must not regress on those.
			name:     "lan interface with a CIDR ipv4-prefix",
			clientIP: "192.168.2.50",
			ubusResponse: map[string]any{
				"interface": []any{
					map[string]any{
						"interface": true,
						"up":        true,
						"l3_device": "lan",
						"ipv4-prefix": []any{
							map[string]any{"address": "192.168.2.0/24"},
						},
					},
				},
			},
			arp:            "192.168.2.50  0x1  0x2  9C:EB:E8:D3:F8:D1  *  lan",
			expectedMethod: "ethernet",
			expectError:    false,
		},
		{
			name:     "eth0 matches Ethernet",
			clientIP: "10.0.0.5",
			ubusResponse: map[string]any{
				"interface": []any{
					map[string]any{
						"interface": true,
						"up":        true,
						"l3_device": "eth0",
						"ipv4-address": []any{
							map[string]any{"address": "10.0.0.0", "mask": float64(8)},
						},
					},
				},
			},
			expectedMethod: "ethernet",
			expectError:    false,
		},
		{
			name:     "eth1 device matches Ethernet",
			clientIP: "172.16.0.10",
			ubusResponse: map[string]any{
				"interface": []any{
					map[string]any{
						"interface": true,
						"up":        true,
						"l3_device": "eth1",
						"ipv4-address": []any{
							map[string]any{"address": "172.16.0.0", "mask": float64(12)},
						},
					},
				},
			},
			expectedMethod: "ethernet",
			expectError:    false,
		},
		{
			name:     "unknown interface returns unknown connection",
			clientIP: "192.168.100.50",
			ubusResponse: map[string]any{
				"interface": []any{
					map[string]any{
						"interface": true,
						"up":        true,
						"l3_device": "some-unknown",
						"ipv4-address": []any{
							map[string]any{"address": "192.168.100.0", "mask": float64(24)},
						},
					},
				},
			},
			expectedMethod: "unknown",
			expectError:    false,
		},
		{
			name:     "interface down not matched",
			clientIP: "192.168.1.50",
			ubusResponse: map[string]any{
				"interface": []any{
					map[string]any{
						"interface": false,
						"up":        false,
						"l3_device": "br-lan",
						"ipv4-address": []any{
							map[string]any{"address": "192.168.1.0", "mask": float64(24)},
						},
					},
				},
			},
			expectedMethod: "unknown",
			expectError:    false,
		},
		{
			name:     "no matching interface returns unknown",
			clientIP: "8.8.8.8",
			ubusResponse: map[string]any{
				"interface": []any{
					map[string]any{
						"interface": true,
						"up":        true,
						"l3_device": "br-lan",
						"ipv4-address": []any{
							map[string]any{"address": "192.168.1.0", "mask": float64(24)},
						},
					},
				},
			},
			expectedMethod: "unknown",
			expectError:    false,
		},
		{
			name:     "/23 subnet matching",
			clientIP: "192.168.0.100",
			ubusResponse: map[string]any{
				"interface": []any{
					map[string]any{
						"interface": true,
						"up":        true,
						"l3_device": "eth0",
						"ipv4-address": []any{
							map[string]any{"address": "192.168.0.0", "mask": float64(23)},
						},
					},
				},
			},
			expectedMethod: "ethernet",
			expectError:    false,
		},
		{
			name:     "/16 subnet matching",
			clientIP: "10.0.210.20",
			ubusResponse: map[string]any{
				"interface": []any{
					map[string]any{
						"interface": true,
						"up":        true,
						"l3_device": "eth0",
						"ipv4-address": []any{
							map[string]any{"address": "10.0.0.0", "mask": float64(16)},
						},
					},
				},
			},
			expectedMethod: "ethernet",
			expectError:    false,
		},
		{
			name:     "/19 subnet matching",
			clientIP: "172.16.10.5",
			ubusResponse: map[string]any{
				"interface": []any{
					map[string]any{
						"interface": true,
						"up":        true,
						"l3_device": "eth0",
						"ipv4-address": []any{
							map[string]any{"address": "172.16.0.0", "mask": float64(19)},
						},
					},
				},
			},
			expectedMethod: "ethernet",
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ub := ubus.NewMockUbus()
			if tt.ubusResponse != nil {
				ub.RegisterResponse("network.interface.dump", tt.ubusResponse)
			}

			u := uci.NewMockUCI()
			cmd := &platform.MockCommandRunner{Err: fmt.Errorf("iw: not available")}
			if tt.assoc != "" {
				cmd = &platform.MockCommandRunner{RunFunc: func(_ string, args ...string) ([]byte, error) {
					if len(args) == 1 && args[0] == "dev" {
						return []byte("phy#1\n\tInterface phy1-ap0\n\t\ttype AP\n"), nil
					}
					return []byte("Station " + tt.assoc + " (on phy1-ap0)\n"), nil
				}}
			}
			svc := New(Deps{UCI: u, Ubus: ub, Cmd: cmd})
			svc.arpFile = networktest.WriteTempFile(t, "arp",
				"IP address  HW type  Flags  HW address  Mask  Device\n"+tt.arp)

			result, err := svc.GetConnectionMethod(tt.clientIP)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result.Method != tt.expectedMethod {
				t.Errorf("expected method %q, got %q", tt.expectedMethod, result.Method)
			}
		})
	}
}
