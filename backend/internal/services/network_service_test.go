package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
