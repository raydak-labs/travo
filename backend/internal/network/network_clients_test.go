package network

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/platform"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

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
	svc := New(Deps{UCI: u, Ubus: ub, AliasFile: aliasFile})

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
	svc := New(Deps{UCI: u, Ubus: ub, AliasFile: aliasFile})

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
	svc := New(Deps{UCI: u, Ubus: ub, AliasFile: aliasFile})

	_ = svc.SetAlias("aa:bb:cc:11:22:33", "Laptop")

	aliases := svc.loadAliases()
	if aliases["AA:BB:CC:11:22:33"] != "Laptop" {
		t.Errorf("expected alias stored as uppercase MAC, got %v", aliases)
	}
}

func TestLoadAliases_NoFile(t *testing.T) {
	dir := t.TempDir()
	aliasFile := filepath.Join(dir, "nonexistent.json")
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: u, Ubus: ub, AliasFile: aliasFile})

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
	svc := New(Deps{UCI: u, Ubus: ub, AliasFile: aliasFile})

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

func TestFetchDHCPClients_ConnectedSinceTreatsExpiresAsEpoch(t *testing.T) {
	ub := ubus.NewMockUbus()
	// A lease that started 1h ago with a 12h lease expires in 11h: 11*3600
	// seconds *after* the epoch.
	expires := float64(time.Now().Unix() + 11*3600)
	ub.RegisterResponse("dhcp.ipv4leases", map[string]any{
		"device": map[string]any{
			"br-lan": map[string]any{
				"leases": []any{
					map[string]any{"mac": "AA:BB:CC:11:22:33", "ip": "192.168.8.100", "hostname": "laptop", "expires": expires},
				},
			},
		},
	})
	runner := &platform.MockCommandRunner{Err: fmt.Errorf("iw not available")}
	svc := New(Deps{UCI: uci.NewMockUCI(), Ubus: ub, Cmd: runner})

	clients := svc.fetchDHCPClients()
	if len(clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(clients))
	}
	connected, err := time.Parse(time.RFC3339, clients[0].ConnectedSince)
	if err != nil {
		t.Fatalf("ConnectedSince %q is not RFC3339: %v", clients[0].ConnectedSince, err)
	}
	age := time.Since(connected)
	if age < 30*time.Minute || age > 90*time.Minute {
		t.Errorf("expected ~1h uptime from the epoch expiry, got %s", age.Round(time.Minute))
	}
}

func TestFetchDHCPClients_ConnectedSinceTreatsExpiresAsRemainingDuration(t *testing.T) {
	ub := ubus.NewMockUbus()
	ub.RegisterResponse("dhcp.ipv4leases", map[string]any{
		"device": map[string]any{
			"br-lan": map[string]any{
				"leases": []any{
					// 11h left of a 12h lease => connected ~1h ago (old build semantics).
					map[string]any{"mac": "AA:BB:CC:11:22:33", "ip": "192.168.8.100", "hostname": "laptop", "expires": float64(11 * 3600)},
				},
			},
		},
	})
	runner := &platform.MockCommandRunner{Err: fmt.Errorf("iw not available")}
	svc := New(Deps{UCI: uci.NewMockUCI(), Ubus: ub, Cmd: runner})

	clients := svc.fetchDHCPClients()
	if len(clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(clients))
	}
	connected, err := time.Parse(time.RFC3339, clients[0].ConnectedSince)
	if err != nil {
		t.Fatalf("ConnectedSince %q is not RFC3339: %v", clients[0].ConnectedSince, err)
	}
	if age := time.Since(connected); age < 30*time.Minute || age > 90*time.Minute {
		t.Errorf("expected ~1h uptime from the remaining-duration expiry, got %s", age.Round(time.Minute))
	}
}
