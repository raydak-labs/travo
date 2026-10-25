package network

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/platform"
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
	svc := New(Deps{UCI: u, Ubus: ub})

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

func TestGetClients(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: u, Ubus: ub})

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
	svc := New(Deps{UCI: u, Ubus: ub})

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
	ub.RegisterResponse("dhcp.ipv4leases", map[string]any{
		"device": map[string]any{},
	})
	svc := New(Deps{UCI: u, Ubus: ub})

	status, err := svc.GetNetworkStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(status.Clients) != 0 {
		t.Errorf("expected 0 clients when no DHCP leases, got %d", len(status.Clients))
	}
}

func TestGetClients_MergesAliases(t *testing.T) {
	dir := t.TempDir()
	aliasFile := filepath.Join(dir, "aliases.json")
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: u, Ubus: ub, AliasFile: aliasFile})

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

func TestAddAndGetDNSEntry(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: u, Ubus: ub})

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
	svc := New(Deps{UCI: u, Ubus: ub})

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

func TestAddAndGetDHCPReservation(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: u, Ubus: ub})

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
	svc := New(Deps{UCI: u, Ubus: ub})

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

func TestBlockMultipleClients(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: u, Ubus: ub, Cmd: &platform.MockCommandRunner{}})

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

func TestSetInterfaceState_Up(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: u, Ubus: ub, Cmd: &platform.MockCommandRunner{}})

	err := svc.SetInterfaceState("wan", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetInterfaceState_Down(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: u, Ubus: ub, Cmd: &platform.MockCommandRunner{}})

	err := svc.SetInterfaceState("lan", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetInterfaceState_Wwan(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: u, Ubus: ub, Cmd: &platform.MockCommandRunner{}})

	err := svc.SetInterfaceState("wwan", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetInterfaceState_UnknownInterface(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: u, Ubus: ub, Cmd: &platform.MockCommandRunner{}})

	err := svc.SetInterfaceState("invalid", true)
	if err == nil {
		t.Fatal("expected error for unknown interface")
	}
}

func TestSetInterfaceState_CommandFailure(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: u, Ubus: ub, Cmd: &platform.MockCommandRunner{Err: fmt.Errorf("command failed")}})

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

	cmdRunner := &platform.FuncCommandRunner{
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
	svc := New(Deps{UCI: u, Ubus: ub, Cmd: cmdRunner})

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
	cmdRunner := &platform.FuncCommandRunner{
		RunFunc: func(_ string, _ ...string) ([]byte, error) {
			return nil, fmt.Errorf("iw not found")
		},
	}

	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: u, Ubus: ub, Cmd: cmdRunner})

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

// fakeDDNSScript points svc at a ddns-scripts init script that exists, so the
// availability precondition passes. Without it every DDNS write correctly
// reports the package as missing, which is the behaviour on a real router that
// has not installed ddns-scripts.
func fakeDDNSScript(t *testing.T, svc *Service) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ddns")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake ddns init script: %v", err)
	}
	svc.SetDDNSInitScript(path)
	return path
}

func TestGetClients_HostnamesFromUbus(t *testing.T) {
	// Verify that ubus-sourced clients already have hostnames populated
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: u, Ubus: ub})

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

func TestParseInterface_EmitsContractTypes(t *testing.T) {
	ub := ubus.NewMockUbus()
	tests := []struct {
		name   string
		iface  string
		device string
		data   map[string]any
		want   string
	}{
		{"wan stays wan", "wan", "eth0", map[string]any{"device": "eth0"}, "wan"},
		{"lan stays lan", "lan", "br-lan", map[string]any{"device": "br-lan"}, "lan"},
		{"wwan is wifi", "wwan", "phy0-sta0", map[string]any{"device": "phy0-sta0"}, "wifi"},
		{"wlan ap is wifi", "guest", "phy1-ap0", map[string]any{"device": "phy1-ap0"}, "wifi"},
		{"usb tether is usb", "usbtether", "usb0", map[string]any{"device": "usb0", "proto": "dhcp"}, "usb"},
		{"wireguard is vpn", "wg0", "wg0", map[string]any{"device": "wg0", "proto": "wireguard"}, "vpn"},
		{"wireguard tunnel device is vpn", "vpn0", "tun0", map[string]any{"device": "tun0", "proto": "none"}, "vpn"},
		{"unknown keeps name", "guest", "br-guest", map[string]any{"device": "br-guest"}, "guest"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseInterface(tt.iface, tt.device, tt.data, ub).Type
			if got != tt.want {
				t.Errorf("expected type %q, got %q", tt.want, got)
			}
		})
	}
}

func TestGetNetworkStatus_EmitsContractTypes(t *testing.T) {
	ub := ubus.NewMockUbus()
	ub.RegisterResponse("network.interface.wwan.status", map[string]any{
		"up": true, "device": "phy0-sta0", "l3_device": "wwan", "proto": "dhcp",
	})
	svc := New(Deps{UCI: uci.NewMockUCI(), Ubus: ub})

	status, err := svc.GetNetworkStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]string{"wan": "wan", "lan": "lan", "wwan": "wifi"}
	for _, iface := range status.Interfaces {
		if w, ok := want[iface.Name]; ok {
			if iface.Type != w {
				t.Errorf("interface %s: expected type %q, got %q", iface.Name, w, iface.Type)
			}
			delete(want, iface.Name)
		}
	}
	for name := range want {
		t.Errorf("interface %s missing from status", name)
	}
}

// ---------------------------------------------------------------------------
// PUT /network/wan: every declared field must actually be written.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// DELETE /network/dns/entries/:section must not be able to eat the dnsmasq
// section (uci.validSectionName deliberately admits @type[N] references).
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Diagnostics: the target is the last argv entry of a root-run command.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Block / Unblock: a failed firewall reload must not leave the committed
// config and the running firewall disagreeing.
// ---------------------------------------------------------------------------
