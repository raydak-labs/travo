package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// failingSetUCI wraps MockUCI but returns an error from Set.
type failingSetUCI struct {
	*uci.MockUCI
	setErr error
}

func (f *failingSetUCI) Set(_, _, _, _ string) error {
	return f.setErr
}

func TestGetConnectionMethod(t *testing.T) {
	tests := []struct {
		name           string
		clientIP       string
		ubusResponse   map[string]interface{}
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
			ubusResponse: map[string]interface{}{
				"interface": "not-a-map",
			},
			expectedMethod: "unknown",
			expectError:    false,
		},
		{
			name:     "wwan interface matches WiFi client",
			clientIP: "192.168.20.100",
			ubusResponse: map[string]interface{}{
				"interface": []interface{}{
					map[string]interface{}{
						"interface": true,
						"up":        true,
						"l3_device": "wwan0",
						"ipv4-prefix": []interface{}{
							map[string]interface{}{
								"address": "192.168.20.0/24",
							},
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
			ubusResponse: map[string]interface{}{
				"interface": []interface{}{
					map[string]interface{}{
						"interface": true,
						"up":        true,
						"device":    "phy0-sta",
						"ipv4-prefix": []interface{}{
							map[string]interface{}{
								"address": "192.168.30.0/24",
							},
						},
					},
				},
			},
			expectedMethod: "wifi-client",
			expectError:    false,
		},
		{
			name:     "br-lan interface matches WiFi AP",
			clientIP: "192.168.1.200",
			ubusResponse: map[string]interface{}{
				"interface": []interface{}{
					map[string]interface{}{
						"interface": true,
						"up":        true,
						"l3_device": "br-lan",
						"ipv4-prefix": []interface{}{
							map[string]interface{}{
								"address": "192.168.1.0/24",
							},
						},
					},
				},
			},
			expectedMethod: "wifi-ap",
			expectError:    false,
		},
		{
			name:     "lan interface matches WiFi AP",
			clientIP: "192.168.2.50",
			ubusResponse: map[string]interface{}{
				"interface": []interface{}{
					map[string]interface{}{
						"interface": true,
						"up":        true,
						"l3_device": "lan",
						"ipv4-prefix": []interface{}{
							map[string]interface{}{
								"address": "192.168.2.0/24",
							},
						},
					},
				},
			},
			expectedMethod: "wifi-ap",
			expectError:    false,
		},
		{
			name:     "eth0 matches Ethernet",
			clientIP: "10.0.0.5",
			ubusResponse: map[string]interface{}{
				"interface": []interface{}{
					map[string]interface{}{
						"interface": true,
						"up":        true,
						"l3_device": "eth0",
						"ipv4-prefix": []interface{}{
							map[string]interface{}{
								"address": "10.0.0.0/8",
							},
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
			ubusResponse: map[string]interface{}{
				"interface": []interface{}{
					map[string]interface{}{
						"interface": true,
						"up":        true,
						"l3_device": "eth1",
						"ipv4-prefix": []interface{}{
							map[string]interface{}{
								"address": "172.16.0.0/12",
							},
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
			ubusResponse: map[string]interface{}{
				"interface": []interface{}{
					map[string]interface{}{
						"interface": true,
						"up":        true,
						"l3_device": "some-unknown",
						"ipv4-prefix": []interface{}{
							map[string]interface{}{
								"address": "192.168.100.0/24",
							},
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
			ubusResponse: map[string]interface{}{
				"interface": []interface{}{
					map[string]interface{}{
						"interface": false,
						"up":        false,
						"l3_device": "br-lan",
						"ipv4-prefix": []interface{}{
							map[string]interface{}{
								"address": "192.168.1.0/24",
							},
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
			ubusResponse: map[string]interface{}{
				"interface": []interface{}{
					map[string]interface{}{
						"interface": true,
						"up":        true,
						"l3_device": "br-lan",
						"ipv4-prefix": []interface{}{
							map[string]interface{}{
								"address": "192.168.1.0/24",
							},
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
			ubusResponse: map[string]interface{}{
				"interface": []interface{}{
					map[string]interface{}{
						"interface": true,
						"up":        true,
						"l3_device": "eth0",
						"ipv4-prefix": []interface{}{
							map[string]interface{}{
								"address": "192.168.0.0/23",
							},
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
			ubusResponse: map[string]interface{}{
				"interface": []interface{}{
					map[string]interface{}{
						"interface": true,
						"up":        true,
						"l3_device": "eth0",
						"ipv4-prefix": []interface{}{
							map[string]interface{}{
								"address": "10.0.0.0/16",
							},
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
			ubusResponse: map[string]interface{}{
				"interface": []interface{}{
					map[string]interface{}{
						"interface": true,
						"up":        true,
						"l3_device": "eth0",
						"ipv4-prefix": []interface{}{
							map[string]interface{}{
								"address": "172.16.0.0/19",
							},
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
			} else if strings.Contains(tt.name, "ubus error") {
				// Let it fall through to the unknown method which returns error
			}

			u := uci.NewMockUCI()
			svc := NewNetworkService(u, ub)

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

func TestGetNetworkStatus(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(u, ub)

	status, err := svc.GetNetworkStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.WAN == nil {
		t.Fatal("expected WAN interface")
	}
	if !status.WAN.IsUp {
		t.Error("expected WAN to be up")
	}
	if status.WAN.IPAddress == "" {
		t.Error("expected WAN IP address")
	}
	if !status.InternetReachable {
		t.Error("expected internet reachable")
	}
	if len(status.Clients) == 0 {
		t.Error("expected clients")
	}
}

func TestGetWanConfig(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(u, ub)

	config, err := svc.GetWanConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Type != "dhcp" {
		t.Errorf("expected type 'dhcp', got %q", config.Type)
	}
	if config.MTU != 1500 {
		t.Errorf("expected MTU 1500, got %d", config.MTU)
	}
}

func TestGetClients(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(u, ub)

	clients, err := svc.GetClients()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(clients) < 2 {
		t.Errorf("expected at least 2 clients, got %d", len(clients))
	}
}

func TestGetNetworkStatus_IncludesClients(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(u, ub)

	status, err := svc.GetNetworkStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(status.Clients) < 2 {
		t.Fatalf("expected at least 2 clients from DHCP, got %d", len(status.Clients))
	}

	// Verify clients come from DHCP leases, not hardcoded
	foundLaptop := false
	foundPhone := false
	for _, c := range status.Clients {
		if c.Hostname == "laptop" && c.IPAddress == "192.168.8.100" && c.MACAddress == "AA:BB:CC:11:22:33" {
			foundLaptop = true
			if c.InterfaceName != "br-lan" {
				t.Errorf("expected laptop on br-lan, got %q", c.InterfaceName)
			}
		}
		if c.Hostname == "phone" && c.IPAddress == "192.168.8.101" {
			foundPhone = true
		}
	}
	if !foundLaptop {
		t.Error("expected to find laptop client from DHCP leases")
	}
	if !foundPhone {
		t.Error("expected to find phone client from DHCP leases")
	}
}

func TestGetNetworkStatus_NoDHCP_EmptyClients(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	// Override DHCP response with empty data
	ub.RegisterResponse("dhcp.ipv4leases", map[string]interface{}{
		"device": map[string]interface{}{},
	})
	svc := NewNetworkService(u, ub)

	status, err := svc.GetNetworkStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(status.Clients) != 0 {
		t.Errorf("expected 0 clients when no DHCP leases, got %d", len(status.Clients))
	}
}

func TestSetWanConfigReturnsErrorOnSetFailure(t *testing.T) {
	fu := &failingSetUCI{
		MockUCI: uci.NewMockUCI(),
		setErr:  fmt.Errorf("mock set error"),
	}
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(fu, ub)

	cfg := models.WanConfig{
		Type: "static",
	}
	err := svc.SetWanConfig(cfg)
	if err == nil {
		t.Error("expected error when uci.Set fails")
	}
}

func TestGetDHCPConfig(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(u, ub)

	config, err := svc.GetDHCPConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Start != 100 {
		t.Errorf("expected start 100, got %d", config.Start)
	}
	if config.Limit != 150 {
		t.Errorf("expected limit 150, got %d", config.Limit)
	}
	if config.LeaseTime != "12h" {
		t.Errorf("expected lease_time '12h', got '%s'", config.LeaseTime)
	}
}

func TestSetDHCPConfig(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(u, ub)

	err := svc.SetDHCPConfig(models.DHCPConfig{Start: 50, Limit: 100, LeaseTime: "24h"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	config, err := svc.GetDHCPConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Start != 50 {
		t.Errorf("expected start 50, got %d", config.Start)
	}
	if config.Limit != 100 {
		t.Errorf("expected limit 100, got %d", config.Limit)
	}
	if config.LeaseTime != "24h" {
		t.Errorf("expected lease_time '24h', got '%s'", config.LeaseTime)
	}
}

func TestGetDNSConfig(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(u, ub)

	config, err := svc.GetDNSConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Default mock has no peerdns set, so use_custom_dns should be false
	if config.UseCustomDNS {
		t.Error("expected use_custom_dns to be false by default")
	}
}

func TestSetDNSConfig(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(u, ub)

	// Enable custom DNS
	err := svc.SetDNSConfig(models.DNSConfig{
		UseCustomDNS: true,
		Servers:      []string{"8.8.8.8", "1.1.1.1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	config, _ := svc.GetDNSConfig()
	if !config.UseCustomDNS {
		t.Error("expected use_custom_dns to be true")
	}
	if len(config.Servers) != 2 {
		t.Fatalf("expected 2 DNS servers, got %d", len(config.Servers))
	}
	if config.Servers[0] != "8.8.8.8" {
		t.Errorf("expected first server '8.8.8.8', got '%s'", config.Servers[0])
	}

	// Disable custom DNS
	err = svc.SetDNSConfig(models.DNSConfig{UseCustomDNS: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	config, _ = svc.GetDNSConfig()
	if config.UseCustomDNS {
		t.Error("expected use_custom_dns to be false")
	}
}

func TestSetWanConfigPropagatesEachFieldError(t *testing.T) {
	tests := []struct {
		name   string
		config models.WanConfig
	}{
		{"Type", models.WanConfig{Type: "static"}},
		{"IPAddress", models.WanConfig{IPAddress: "10.0.0.1"}},
		{"Netmask", models.WanConfig{Netmask: "255.255.255.0"}},
		{"Gateway", models.WanConfig{Gateway: "10.0.0.1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fu := &failingSetUCI{
				MockUCI: uci.NewMockUCI(),
				setErr:  fmt.Errorf("set failed"),
			}
			ub := ubus.NewMockUbus()
			svc := NewNetworkService(fu, ub)

			err := svc.SetWanConfig(tt.config)
			if err == nil {
				t.Errorf("expected error for field %s", tt.name)
			}
		})
	}
}

func TestParseDHCPLeases(t *testing.T) {
	data := "1718900000 aa:bb:cc:dd:ee:ff 192.168.1.100 laptop-1 *\n1718903600 11:22:33:44:55:66 192.168.1.101 phone-2 01:11:22:33:44:55:66\n"
	leases := parseDHCPLeases(data)
	if len(leases) != 2 {
		t.Fatalf("expected 2 leases, got %d", len(leases))
	}
	if leases[0].MAC != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("expected MAC aa:bb:cc:dd:ee:ff, got %s", leases[0].MAC)
	}
	if leases[0].IP != "192.168.1.100" {
		t.Errorf("expected IP 192.168.1.100, got %s", leases[0].IP)
	}
	if leases[0].Hostname != "laptop-1" {
		t.Errorf("expected hostname laptop-1, got %s", leases[0].Hostname)
	}
	if leases[0].Expiry != 1718900000 {
		t.Errorf("expected expiry 1718900000, got %d", leases[0].Expiry)
	}
	if leases[1].MAC != "11:22:33:44:55:66" {
		t.Errorf("expected MAC 11:22:33:44:55:66, got %s", leases[1].MAC)
	}
	if leases[1].Hostname != "phone-2" {
		t.Errorf("expected hostname phone-2, got %s", leases[1].Hostname)
	}
}

func TestParseDHCPLeases_Empty(t *testing.T) {
	leases := parseDHCPLeases("")
	if len(leases) != 0 {
		t.Fatalf("expected no leases, got %d", len(leases))
	}
}

func TestSetAlias(t *testing.T) {
	dir := t.TempDir()
	aliasFile := filepath.Join(dir, "aliases.json")
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkServiceWithAliasFile(u, ub, aliasFile)

	// Set an alias
	err := svc.SetAlias("AA:BB:CC:11:22:33", "John's Laptop")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file written
	data, err := os.ReadFile(aliasFile)
	if err != nil {
		t.Fatalf("failed to read alias file: %v", err)
	}
	var aliases map[string]string
	if err := json.Unmarshal(data, &aliases); err != nil {
		t.Fatalf("failed to parse alias file: %v", err)
	}
	if aliases["AA:BB:CC:11:22:33"] != "John's Laptop" {
		t.Errorf("expected alias 'John's Laptop', got %q", aliases["AA:BB:CC:11:22:33"])
	}
}

func TestSetAlias_Remove(t *testing.T) {
	dir := t.TempDir()
	aliasFile := filepath.Join(dir, "aliases.json")
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkServiceWithAliasFile(u, ub, aliasFile)

	// Set then remove
	_ = svc.SetAlias("AA:BB:CC:11:22:33", "Laptop")
	err := svc.SetAlias("AA:BB:CC:11:22:33", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(aliasFile)
	var aliases map[string]string
	_ = json.Unmarshal(data, &aliases)
	if _, ok := aliases["AA:BB:CC:11:22:33"]; ok {
		t.Error("expected alias to be removed")
	}
}

func TestSetAlias_CaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	aliasFile := filepath.Join(dir, "aliases.json")
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkServiceWithAliasFile(u, ub, aliasFile)

	_ = svc.SetAlias("aa:bb:cc:11:22:33", "Laptop")

	aliases := svc.loadAliases()
	if aliases["AA:BB:CC:11:22:33"] != "Laptop" {
		t.Errorf("expected alias stored as uppercase MAC, got %v", aliases)
	}
}

func TestGetClients_MergesAliases(t *testing.T) {
	dir := t.TempDir()
	aliasFile := filepath.Join(dir, "aliases.json")
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkServiceWithAliasFile(u, ub, aliasFile)

	// Set alias for one of the mock clients
	_ = svc.SetAlias("AA:BB:CC:11:22:33", "Work Laptop")

	clients, err := svc.GetClients()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, c := range clients {
		if c.MACAddress == "AA:BB:CC:11:22:33" {
			found = true
			if c.Alias != "Work Laptop" {
				t.Errorf("expected alias 'Work Laptop', got %q", c.Alias)
			}
		}
	}
	if !found {
		t.Error("expected to find client with MAC AA:BB:CC:11:22:33")
	}
}

func TestLoadAliases_NoFile(t *testing.T) {
	dir := t.TempDir()
	aliasFile := filepath.Join(dir, "nonexistent.json")
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkServiceWithAliasFile(u, ub, aliasFile)

	aliases := svc.loadAliases()
	if len(aliases) != 0 {
		t.Errorf("expected empty map for nonexistent file, got %v", aliases)
	}
}

func TestLoadAliases_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	aliasFile := filepath.Join(dir, "aliases.json")
	_ = os.WriteFile(aliasFile, []byte("not json"), 0600)
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkServiceWithAliasFile(u, ub, aliasFile)

	aliases := svc.loadAliases()
	if len(aliases) != 0 {
		t.Errorf("expected empty map for invalid JSON, got %v", aliases)
	}
}

func TestParseDHCPLeases_WildcardHostname(t *testing.T) {
	data := "1718900000 aa:bb:cc:dd:ee:ff 192.168.1.100 * *\n"
	leases := parseDHCPLeases(data)
	if len(leases) != 1 {
		t.Fatalf("expected 1 lease, got %d", len(leases))
	}
	if leases[0].Hostname != "" {
		t.Errorf("expected empty hostname for *, got %q", leases[0].Hostname)
	}
}

func TestParseDHCPLeases_InvalidLine(t *testing.T) {
	data := "invalid line\n1718900000 aa:bb:cc:dd:ee:ff 192.168.1.100 laptop *\n"
	leases := parseDHCPLeases(data)
	if len(leases) != 1 {
		t.Fatalf("expected 1 lease (skipping invalid), got %d", len(leases))
	}
}

func TestGetDHCPLeases_FileNotExist(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(u, ub)

	leases := svc.GetDHCPLeases()
	if len(leases) != 0 {
		t.Errorf("expected empty slice when file not found, got %d", len(leases))
	}
}

func TestGetDNSEntries_Empty(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(u, ub)

	entries, err := svc.GetDNSEntries()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 DNS entries, got %d", len(entries))
	}
}

func TestAddAndGetDNSEntry(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(u, ub)

	err := svc.AddDNSEntry(models.DNSEntry{Name: "myserver", IP: "192.168.1.50"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, err := svc.GetDNSEntries()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 DNS entry, got %d", len(entries))
	}
	if entries[0].Name != "myserver" {
		t.Errorf("expected name 'myserver', got %q", entries[0].Name)
	}
	if entries[0].IP != "192.168.1.50" {
		t.Errorf("expected IP '192.168.1.50', got %q", entries[0].IP)
	}
	if entries[0].Section != "dns_myserver" {
		t.Errorf("expected section 'dns_myserver', got %q", entries[0].Section)
	}
}

func TestAddMultipleDNSEntries(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(u, ub)

	_ = svc.AddDNSEntry(models.DNSEntry{Name: "server1", IP: "192.168.1.50"})
	_ = svc.AddDNSEntry(models.DNSEntry{Name: "server2", IP: "192.168.1.60"})

	entries, err := svc.GetDNSEntries()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 DNS entries, got %d", len(entries))
	}
}

func TestDeleteDNSEntry(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(u, ub)

	_ = svc.AddDNSEntry(models.DNSEntry{Name: "myserver", IP: "192.168.1.50"})

	err := svc.DeleteDNSEntry("dns_myserver")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, err := svc.GetDNSEntries()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 DNS entries after delete, got %d", len(entries))
	}
}

func TestDeleteDNSEntry_NotFound(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(u, ub)

	err := svc.DeleteDNSEntry("dns_nonexistent")
	if err == nil {
		t.Error("expected error deleting nonexistent section")
	}
}

func TestAddDNSEntry_DuplicateName(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(u, ub)

	_ = svc.AddDNSEntry(models.DNSEntry{Name: "myserver", IP: "192.168.1.50"})
	err := svc.AddDNSEntry(models.DNSEntry{Name: "myserver", IP: "192.168.1.60"})
	if err == nil {
		t.Error("expected error adding duplicate DNS entry")
	}
}

func TestSanitizeSectionName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"myserver", "myserver"},
		{"my-server", "my_server"},
		{"My.Server", "my_server"},
		{"test123", "test123"},
	}
	for _, tt := range tests {
		got := sanitizeSectionName(tt.input)
		if got != tt.expected {
			t.Errorf("sanitizeSectionName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestGetDHCPReservations_Empty(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(u, ub)

	reservations, err := svc.GetDHCPReservations()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reservations) != 0 {
		t.Errorf("expected 0 reservations, got %d", len(reservations))
	}
}

func TestAddAndGetDHCPReservation(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(u, ub)

	err := svc.AddDHCPReservation(models.DHCPReservation{
		Name: "laptop",
		MAC:  "AA:BB:CC:DD:EE:FF",
		IP:   "192.168.8.50",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reservations, err := svc.GetDHCPReservations()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reservations) != 1 {
		t.Fatalf("expected 1 reservation, got %d", len(reservations))
	}
	if reservations[0].Name != "laptop" {
		t.Errorf("expected name 'laptop', got %q", reservations[0].Name)
	}
	if reservations[0].MAC != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("expected MAC 'AA:BB:CC:DD:EE:FF', got %q", reservations[0].MAC)
	}
	if reservations[0].IP != "192.168.8.50" {
		t.Errorf("expected IP '192.168.8.50', got %q", reservations[0].IP)
	}
	if reservations[0].Section != "host_laptop" {
		t.Errorf("expected section 'host_laptop', got %q", reservations[0].Section)
	}
}

func TestAddMultipleDHCPReservations(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(u, ub)

	_ = svc.AddDHCPReservation(models.DHCPReservation{Name: "laptop", MAC: "AA:BB:CC:DD:EE:01", IP: "192.168.8.50"})
	_ = svc.AddDHCPReservation(models.DHCPReservation{Name: "phone", MAC: "AA:BB:CC:DD:EE:02", IP: "192.168.8.51"})

	reservations, err := svc.GetDHCPReservations()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reservations) != 2 {
		t.Fatalf("expected 2 reservations, got %d", len(reservations))
	}
}

func TestDeleteDHCPReservation(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(u, ub)

	_ = svc.AddDHCPReservation(models.DHCPReservation{Name: "laptop", MAC: "AA:BB:CC:DD:EE:FF", IP: "192.168.8.50"})

	err := svc.DeleteDHCPReservation("host_laptop")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reservations, err := svc.GetDHCPReservations()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reservations) != 0 {
		t.Errorf("expected 0 reservations after delete, got %d", len(reservations))
	}
}

func TestDeleteDHCPReservation_NotFound(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(u, ub)

	err := svc.DeleteDHCPReservation("host_nonexistent")
	if err == nil {
		t.Error("expected error deleting nonexistent reservation")
	}
}

func TestAddDHCPReservation_DuplicateName(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(u, ub)

	_ = svc.AddDHCPReservation(models.DHCPReservation{Name: "laptop", MAC: "AA:BB:CC:DD:EE:01", IP: "192.168.8.50"})
	err := svc.AddDHCPReservation(models.DHCPReservation{Name: "laptop", MAC: "AA:BB:CC:DD:EE:02", IP: "192.168.8.51"})
	if err == nil {
		t.Error("expected error adding duplicate DHCP reservation")
	}
}

func TestBlockClient(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkServiceWithRunner(u, ub, &MockCommandRunner{})

	err := svc.BlockClient("AA:BB:CC:DD:EE:FF")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	blocked, err := svc.GetBlockedClients()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocked) != 1 {
		t.Fatalf("expected 1 blocked client, got %d", len(blocked))
	}
	if blocked[0] != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("expected blocked MAC 'AA:BB:CC:DD:EE:FF', got %q", blocked[0])
	}
}

func TestUnblockClient(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkServiceWithRunner(u, ub, &MockCommandRunner{})

	_ = svc.BlockClient("AA:BB:CC:DD:EE:FF")

	err := svc.UnblockClient("AA:BB:CC:DD:EE:FF")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	blocked, err := svc.GetBlockedClients()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocked) != 0 {
		t.Errorf("expected 0 blocked clients after unblock, got %d", len(blocked))
	}
}

func TestGetBlockedClients_Empty(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(u, ub)

	blocked, err := svc.GetBlockedClients()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocked) != 0 {
		t.Errorf("expected 0 blocked clients, got %d", len(blocked))
	}
}

func TestBlockMultipleClients(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkServiceWithRunner(u, ub, &MockCommandRunner{})

	_ = svc.BlockClient("AA:BB:CC:DD:EE:01")
	_ = svc.BlockClient("AA:BB:CC:DD:EE:02")

	blocked, err := svc.GetBlockedClients()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocked) != 2 {
		t.Fatalf("expected 2 blocked clients, got %d", len(blocked))
	}
}

func TestBlockClient_AlreadyBlocked(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkServiceWithRunner(u, ub, &MockCommandRunner{})

	_ = svc.BlockClient("AA:BB:CC:DD:EE:FF")
	err := svc.BlockClient("AA:BB:CC:DD:EE:FF")
	if err == nil {
		t.Error("expected error blocking already-blocked client")
	}
}

func TestUnblockClient_NotBlocked(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkServiceWithRunner(u, ub, &MockCommandRunner{})

	err := svc.UnblockClient("AA:BB:CC:DD:EE:FF")
	if err == nil {
		t.Error("expected error unblocking non-blocked client")
	}
}

func TestKickClient(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkServiceWithRunner(u, ub, &MockCommandRunner{})

	err := svc.KickClient("AA:BB:CC:DD:EE:FF")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBlockClient_CaseInsensitive(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkServiceWithRunner(u, ub, &MockCommandRunner{})

	err := svc.BlockClient("aa:bb:cc:dd:ee:ff")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	blocked, _ := svc.GetBlockedClients()
	if len(blocked) != 1 {
		t.Fatalf("expected 1 blocked client, got %d", len(blocked))
	}
	if blocked[0] != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("expected MAC stored as uppercase, got %q", blocked[0])
	}
}

func TestSetInterfaceState_Up(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkServiceWithRunner(u, ub, &MockCommandRunner{})

	err := svc.SetInterfaceState("wan", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetInterfaceState_Down(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkServiceWithRunner(u, ub, &MockCommandRunner{})

	err := svc.SetInterfaceState("lan", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetInterfaceState_Wwan(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkServiceWithRunner(u, ub, &MockCommandRunner{})

	err := svc.SetInterfaceState("wwan", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetInterfaceState_UnknownInterface(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkServiceWithRunner(u, ub, &MockCommandRunner{})

	err := svc.SetInterfaceState("invalid", true)
	if err == nil {
		t.Fatal("expected error for unknown interface")
	}
}

func TestSetInterfaceState_CommandFailure(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkServiceWithRunner(u, ub, &MockCommandRunner{Err: fmt.Errorf("command failed")})

	err := svc.SetInterfaceState("wan", true)
	if err == nil {
		t.Fatal("expected error when command fails")
	}
}

// mapCommandRunner returns different outputs based on the command name.
type mapCommandRunner struct {
	responses map[string]struct {
		output []byte
		err    error
	}
}

func (m *mapCommandRunner) Run(name string, _ ...string) ([]byte, error) {
	if r, ok := m.responses[name]; ok {
		return r.output, r.err
	}
	return nil, fmt.Errorf("command not found: %s", name)
}

func TestDetectWanType_DHCP(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	cmd := &mapCommandRunner{responses: map[string]struct {
		output []byte
		err    error
	}{
		"pgrep": {nil, fmt.Errorf("not found")},
	}}
	// Mock returns error for pgrep (no pppd, no udhcpc) → falls back to UCI config (dhcp)
	svc := NewNetworkServiceWithRunner(u, ub, cmd)

	result, err := svc.DetectWanType()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.CurrentType != "dhcp" {
		t.Errorf("expected current_type 'dhcp', got %q", result.CurrentType)
	}
	if result.DetectedType != "dhcp" {
		t.Errorf("expected detected_type 'dhcp', got %q", result.DetectedType)
	}
}

func TestDetectWanType_PPPoE(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	// pgrep succeeds on first call (checking pppd) → PPPoE detected
	cmd := &MockCommandRunner{Output: []byte("1234\n")}
	svc := NewNetworkServiceWithRunner(u, ub, cmd)

	result, err := svc.DetectWanType()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.DetectedType != "pppoe" {
		t.Errorf("expected detected_type 'pppoe', got %q", result.DetectedType)
	}
	if result.CurrentType != "dhcp" {
		t.Errorf("expected current_type 'dhcp', got %q", result.CurrentType)
	}
}

func TestDetectWanType_FallbackToCurrentConfig(t *testing.T) {
	u := uci.NewMockUCI()
	// Set current config to static
	_ = u.Set("network", "wan", "proto", "static")
	ub := ubus.NewMockUbus()
	// All pgrep calls fail → falls back to current config
	cmd := &MockCommandRunner{Err: fmt.Errorf("not found")}
	svc := NewNetworkServiceWithRunner(u, ub, cmd)

	result, err := svc.DetectWanType()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.CurrentType != "static" {
		t.Errorf("expected current_type 'static', got %q", result.CurrentType)
	}
	if result.DetectedType != "static" {
		t.Errorf("expected detected_type 'static' (fallback), got %q", result.DetectedType)
	}
}

func TestParseStationDump(t *testing.T) {
	output := `Station aa:bb:cc:11:22:33 (on phy0-ap0)
	inactive time:	1234 ms
	rx bytes:	500000
	rx packets:	1234
	tx bytes:	1000000
	tx packets:	4321
	signal:  	-45 dBm
Station AA:BB:CC:44:55:66 (on phy0-ap0)
	inactive time:	5678 ms
	rx bytes:	200000
	rx packets:	7890
	tx bytes:	400000
	tx packets:	3456`

	stats := parseStationDump(output)

	if len(stats) != 2 {
		t.Fatalf("expected 2 stations, got %d", len(stats))
	}

	// Station 1: AP rx 500000 from client = client TX, AP tx 1000000 to client = client RX
	s1 := stats["AA:BB:CC:11:22:33"]
	if s1[0] != 1000000 {
		t.Errorf("station1 RxBytes: expected 1000000, got %d", s1[0])
	}
	if s1[1] != 500000 {
		t.Errorf("station1 TxBytes: expected 500000, got %d", s1[1])
	}

	s2 := stats["AA:BB:CC:44:55:66"]
	if s2[0] != 400000 {
		t.Errorf("station2 RxBytes: expected 400000, got %d", s2[0])
	}
	if s2[1] != 200000 {
		t.Errorf("station2 TxBytes: expected 200000, got %d", s2[1])
	}
}

func TestParseStationDump_Empty(t *testing.T) {
	stats := parseStationDump("")
	if len(stats) != 0 {
		t.Errorf("expected empty map, got %d entries", len(stats))
	}
}

func TestParseIwDev(t *testing.T) {
	output := `phy#0
	Interface phy0-ap0
		ifindex 6
		wdev 0x2
		addr 00:11:22:33:44:55
		ssid MyNetwork
		type AP
		channel 6 (2437 MHz), width: 20 MHz
	Interface phy0-sta0
		ifindex 7
		wdev 0x3
		addr 00:11:22:33:44:56
		type managed
phy#1
	Interface phy1-ap0
		ifindex 8
		wdev 0x100000002
		addr 00:11:22:33:44:57
		ssid MyNetwork-5G
		type AP`

	ifaces := parseIwDev(output)
	if len(ifaces) != 2 {
		t.Fatalf("expected 2 AP interfaces, got %d: %v", len(ifaces), ifaces)
	}
	if ifaces[0] != "phy0-ap0" {
		t.Errorf("expected phy0-ap0, got %s", ifaces[0])
	}
	if ifaces[1] != "phy1-ap0" {
		t.Errorf("expected phy1-ap0, got %s", ifaces[1])
	}
}

func TestParseIwDev_NoAP(t *testing.T) {
	output := `phy#0
	Interface phy0-sta0
		type managed`

	ifaces := parseIwDev(output)
	if len(ifaces) != 0 {
		t.Errorf("expected 0 AP interfaces, got %d", len(ifaces))
	}
}

// FuncCommandRunner lets tests provide a function-based command runner.
type FuncCommandRunner struct {
	RunFunc func(name string, args ...string) ([]byte, error)
}

func (f *FuncCommandRunner) Run(name string, args ...string) ([]byte, error) {
	return f.RunFunc(name, args...)
}

func TestGetClients_WithTrafficStats(t *testing.T) {
	iwDevOutput := `phy#0
	Interface phy0-ap0
		type AP`

	stationDump := `Station AA:BB:CC:11:22:33 (on phy0-ap0)
	rx bytes:	300000
	tx bytes:	600000
Station AA:BB:CC:44:55:66 (on phy0-ap0)
	rx bytes:	100000
	tx bytes:	200000`

	cmdRunner := &FuncCommandRunner{
		RunFunc: func(name string, args ...string) ([]byte, error) {
			if name == "iw" && len(args) > 0 && args[0] == "dev" {
				if len(args) == 1 {
					return []byte(iwDevOutput), nil
				}
				if len(args) == 4 && args[2] == "station" && args[3] == "dump" {
					return []byte(stationDump), nil
				}
			}
			return nil, fmt.Errorf("unknown command: %s %v", name, args)
		},
	}

	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkServiceWithRunner(u, ub, cmdRunner)

	clients, err := svc.GetClients()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, c := range clients {
		switch c.MACAddress {
		case "AA:BB:CC:11:22:33":
			if c.RxBytes != 600000 {
				t.Errorf("laptop RxBytes: expected 600000, got %d", c.RxBytes)
			}
			if c.TxBytes != 300000 {
				t.Errorf("laptop TxBytes: expected 300000, got %d", c.TxBytes)
			}
		case "AA:BB:CC:44:55:66":
			if c.RxBytes != 200000 {
				t.Errorf("phone RxBytes: expected 200000, got %d", c.RxBytes)
			}
			if c.TxBytes != 100000 {
				t.Errorf("phone TxBytes: expected 100000, got %d", c.TxBytes)
			}
		}
	}
}

func TestGetClients_NoIwCommand(t *testing.T) {
	cmdRunner := &FuncCommandRunner{
		RunFunc: func(_ string, _ ...string) ([]byte, error) {
			return nil, fmt.Errorf("iw not found")
		},
	}

	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkServiceWithRunner(u, ub, cmdRunner)

	clients, err := svc.GetClients()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should still return clients, just without traffic stats
	if len(clients) < 2 {
		t.Errorf("expected at least 2 clients, got %d", len(clients))
	}
	for _, c := range clients {
		if c.RxBytes != 0 || c.TxBytes != 0 {
			t.Errorf("expected zero traffic stats when iw fails, got rx=%d tx=%d", c.RxBytes, c.TxBytes)
		}
	}
}

func TestGetDDNSConfig(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(u, ub)

	config, err := svc.GetDDNSConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Enabled {
		t.Error("expected DDNS disabled by default")
	}
	if config.Service != "duckdns.org" {
		t.Errorf("expected service 'duckdns.org', got %q", config.Service)
	}
	if config.Domain != "myrouter.duckdns.org" {
		t.Errorf("expected domain 'myrouter.duckdns.org', got %q", config.Domain)
	}
	if config.Username != "mytoken" {
		t.Errorf("expected username 'mytoken', got %q", config.Username)
	}
	if config.LookupHost != "myrouter.duckdns.org" {
		t.Errorf("expected lookup_host 'myrouter.duckdns.org', got %q", config.LookupHost)
	}
	if config.UpdateURL != "" {
		t.Errorf("expected empty update_url by default, got %q", config.UpdateURL)
	}
}

func TestGetDDNSConfig_CustomUpdateURL(t *testing.T) {
	u := uci.NewMockUCI()
	if err := u.DeleteOption("ddns", "myddns", "service_name"); err != nil {
		t.Fatalf("delete service_name: %v", err)
	}
	customURL := "https://dyn.example.com/nic/update?hostname=[DOMAIN]&myip=[IP]"
	if err := u.Set("ddns", "myddns", "update_url", customURL); err != nil {
		t.Fatalf("set update_url: %v", err)
	}
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(u, ub)

	config, err := svc.GetDDNSConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Service != "custom" {
		t.Errorf("expected service 'custom', got %q", config.Service)
	}
	if config.UpdateURL != customURL {
		t.Errorf("expected update_url %q, got %q", customURL, config.UpdateURL)
	}
}

func TestGetDDNSConfig_CustomUpdateURL_ServiceDash(t *testing.T) {
	u := uci.NewMockUCI()
	if err := u.Set("ddns", "myddns", "service_name", "-"); err != nil {
		t.Fatalf("set service_name: %v", err)
	}
	customURL := "https://dyn.example.com/update?ip=[IP]"
	if err := u.Set("ddns", "myddns", "update_url", customURL); err != nil {
		t.Fatalf("set update_url: %v", err)
	}
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(u, ub)

	config, err := svc.GetDDNSConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Service != "custom" {
		t.Errorf("expected service 'custom', got %q", config.Service)
	}
	if config.UpdateURL != customURL {
		t.Errorf("expected update_url %q, got %q", customURL, config.UpdateURL)
	}
}

func TestSetDDNSConfig(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	cmdRunner := &FuncCommandRunner{
		RunFunc: func(_ string, _ ...string) ([]byte, error) {
			return []byte("ok"), nil
		},
	}
	svc := NewNetworkServiceWithRunner(u, ub, cmdRunner)

	newConfig := models.DDNSConfig{
		Enabled:    true,
		Service:    "no-ip.com",
		Domain:     "test.no-ip.org",
		Username:   "user",
		Password:   "pass",
		LookupHost: "test.no-ip.org",
	}
	if err := svc.SetDDNSConfig(newConfig); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify config was written
	config, err := svc.GetDDNSConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !config.Enabled {
		t.Error("expected DDNS enabled")
	}
	if config.Service != "no-ip.com" {
		t.Errorf("expected service 'no-ip.com', got %q", config.Service)
	}
	if config.Domain != "test.no-ip.org" {
		t.Errorf("expected domain 'test.no-ip.org', got %q", config.Domain)
	}
	if config.Username != "user" {
		t.Errorf("expected username 'user', got %q", config.Username)
	}
	if config.Password != "pass" {
		t.Errorf("expected password 'pass', got %q", config.Password)
	}
	if config.LookupHost != "test.no-ip.org" {
		t.Errorf("expected lookup_host 'test.no-ip.org', got %q", config.LookupHost)
	}
	if config.UpdateURL != "" {
		t.Errorf("expected empty update_url for built-in provider, got %q", config.UpdateURL)
	}
}

func TestSetDDNSConfig_CustomUpdateURL(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	cmdRunner := &FuncCommandRunner{
		RunFunc: func(_ string, _ ...string) ([]byte, error) {
			return []byte("ok"), nil
		},
	}
	svc := NewNetworkServiceWithRunner(u, ub, cmdRunner)

	customURL := "https://provider.example/nic/update?hostname=[DOMAIN]&myip=[IP]"
	newConfig := models.DDNSConfig{
		Enabled:    true,
		Service:    "custom",
		Domain:     "home.example.com",
		Username:   "user",
		Password:   "pass",
		LookupHost: "home.example.com",
		UpdateURL:  customURL,
	}
	if err := svc.SetDDNSConfig(newConfig); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	config, err := svc.GetDDNSConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Service != "custom" {
		t.Errorf("expected service 'custom', got %q", config.Service)
	}
	if config.UpdateURL != customURL {
		t.Errorf("expected update_url %q, got %q", customURL, config.UpdateURL)
	}
	opts, err := u.GetAll("ddns", "myddns")
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if opts["service_name"] != "" {
		t.Errorf("expected service_name removed for custom, got %q", opts["service_name"])
	}
	if opts["update_url"] != customURL {
		t.Errorf("expected UCI update_url %q, got %q", customURL, opts["update_url"])
	}
}

func TestSetDDNSConfig_FromCustomToBuiltInClearsUpdateURL(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	cmdRunner := &FuncCommandRunner{
		RunFunc: func(_ string, _ ...string) ([]byte, error) {
			return []byte("ok"), nil
		},
	}
	svc := NewNetworkServiceWithRunner(u, ub, cmdRunner)

	customURL := "https://provider.example/update"
	if err := svc.SetDDNSConfig(models.DDNSConfig{
		Enabled:    true,
		Service:    "custom",
		Domain:     "h.example.com",
		Username:   "",
		Password:   "",
		LookupHost: "h.example.com",
		UpdateURL:  customURL,
	}); err != nil {
		t.Fatalf("set custom: %v", err)
	}
	if err := svc.SetDDNSConfig(models.DDNSConfig{
		Enabled:    true,
		Service:    "duckdns.org",
		Domain:     "x.duckdns.org",
		Username:   "t",
		Password:   "",
		LookupHost: "x.duckdns.org",
		UpdateURL:  "",
	}); err != nil {
		t.Fatalf("set duckdns: %v", err)
	}
	config, err := svc.GetDDNSConfig()
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if config.Service != "duckdns.org" {
		t.Errorf("expected duckdns.org, got %q", config.Service)
	}
	if config.UpdateURL != "" {
		t.Errorf("expected update_url cleared, got %q", config.UpdateURL)
	}
	opts, err := u.GetAll("ddns", "myddns")
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if opts["update_url"] != "" {
		t.Errorf("expected UCI update_url deleted, got %q", opts["update_url"])
	}
}

func TestGetDDNSStatus(t *testing.T) {
	cmdRunner := &FuncCommandRunner{
		RunFunc: func(cmd string, args ...string) ([]byte, error) {
			if cmd == "pgrep" {
				return []byte("123"), nil
			}
			if cmd == "cat" && len(args) > 0 {
				if args[0] == "/var/run/ddns/myddns.ip" {
					return []byte("203.0.113.42\n"), nil
				}
				if args[0] == "/var/run/ddns/myddns.update" {
					return []byte("2026-03-12 10:00:00\n"), nil
				}
			}
			return nil, fmt.Errorf("command not found")
		},
	}
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkServiceWithRunner(u, ub, cmdRunner)

	status, err := svc.GetDDNSStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Running {
		t.Error("expected DDNS running")
	}
	if status.PublicIP != "203.0.113.42" {
		t.Errorf("expected public IP '203.0.113.42', got %q", status.PublicIP)
	}
	if status.LastUpdate != "2026-03-12 10:00:00" {
		t.Errorf("expected last update '2026-03-12 10:00:00', got %q", status.LastUpdate)
	}
}

func TestGetDDNSStatus_NotRunning(t *testing.T) {
	cmdRunner := &FuncCommandRunner{
		RunFunc: func(_ string, _ ...string) ([]byte, error) {
			return nil, fmt.Errorf("not found")
		},
	}
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkServiceWithRunner(u, ub, cmdRunner)

	status, err := svc.GetDDNSStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Running {
		t.Error("expected DDNS not running")
	}
	if status.PublicIP != "" {
		t.Errorf("expected empty public IP, got %q", status.PublicIP)
	}
}

func TestParseEtcHosts(t *testing.T) {
	hosts := parseEtcHosts()
	// On macOS/CI /etc/hosts exists but only has localhost entries which we skip.
	// Just verify it returns a map without error.
	if hosts == nil {
		t.Fatal("expected non-nil map")
	}
}

func TestParseEtcHosts_Format(t *testing.T) {
	// Test the parsing logic directly by verifying known format handling.
	// The function reads /etc/hosts which varies by environment,
	// so we test the internal parsing via the exported result structure.
	hosts := parseEtcHosts()
	// Should not contain localhost entries
	if _, ok := hosts["127.0.0.1"]; ok {
		t.Error("expected localhost entries to be skipped")
	}
	if _, ok := hosts["::1"]; ok {
		t.Error("expected ipv6 localhost entries to be skipped")
	}
}

func TestParseDHCPLeasesFile_ReturnsHostnames(t *testing.T) {
	// parseDHCPLeasesFile reads /tmp/dhcp.leases which doesn't exist in test.
	// Verify it returns empty map gracefully.
	leases := parseDHCPLeasesFile()
	if leases == nil {
		t.Fatal("expected non-nil map")
	}
	if len(leases) != 0 {
		t.Errorf("expected empty map when file missing, got %d entries", len(leases))
	}
}

func TestGetClients_HostnamesFromUbus(t *testing.T) {
	// Verify that ubus-sourced clients already have hostnames populated
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(u, ub)

	clients, err := svc.GetClients()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hostnameCount := 0
	for _, c := range clients {
		if c.Hostname != "" {
			hostnameCount++
		}
		if c.MACAddress == "AA:BB:CC:11:22:33" && c.Hostname != "laptop" {
			t.Errorf("expected hostname 'laptop' for AA:BB:CC:11:22:33, got %q", c.Hostname)
		}
		if c.MACAddress == "AA:BB:CC:44:55:66" && c.Hostname != "phone" {
			t.Errorf("expected hostname 'phone' for AA:BB:CC:44:55:66, got %q", c.Hostname)
		}
	}
	if hostnameCount == 0 {
		t.Error("expected at least one client with a hostname")
	}
}
