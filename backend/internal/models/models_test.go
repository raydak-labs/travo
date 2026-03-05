package models

import (
	"encoding/json"
	"testing"
)

func floatPtr(f float64) *float64 {
	return &f
}

func TestSystemInfoJSON(t *testing.T) {
	original := SystemInfo{
		Hostname:        "OpenWrt",
		Model:           "GL-MT3000",
		FirmwareVersion: "23.05.2",
		KernelVersion:   "5.15.137",
		UptimeSeconds:   86400,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal SystemInfo: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}
	for _, key := range []string{"hostname", "model", "firmware_version", "kernel_version", "uptime_seconds"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("expected JSON key %q not found", key)
		}
	}

	var decoded SystemInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal SystemInfo: %v", err)
	}
	if decoded != original {
		t.Errorf("roundtrip mismatch: got %+v, want %+v", decoded, original)
	}
}

func TestSystemStatsJSON(t *testing.T) {
	original := SystemStats{
		CPU: CpuStats{
			UsagePercent:       45.2,
			Cores:              4,
			TemperatureCelsius: floatPtr(55.0),
			LoadAverage:        [3]float64{0.5, 0.8, 1.2},
		},
		Memory: MemoryStats{
			TotalBytes:   1073741824,
			UsedBytes:    536870912,
			FreeBytes:    268435456,
			CachedBytes:  268435456,
			UsagePercent: 50.0,
		},
		Storage: StorageStats{
			TotalBytes:   8589934592,
			UsedBytes:    2147483648,
			FreeBytes:    6442450944,
			UsagePercent: 25.0,
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal SystemStats: %v", err)
	}

	var decoded SystemStats
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal SystemStats: %v", err)
	}

	if decoded.CPU.Cores != original.CPU.Cores {
		t.Errorf("CPU cores mismatch: got %d, want %d", decoded.CPU.Cores, original.CPU.Cores)
	}
	if decoded.Memory.TotalBytes != original.Memory.TotalBytes {
		t.Errorf("memory total mismatch: got %d, want %d", decoded.Memory.TotalBytes, original.Memory.TotalBytes)
	}
}

func TestNetworkStatusJSON(t *testing.T) {
	original := NetworkStatus{
		WAN: &NetworkInterface{
			Name:       "eth0",
			Type:       "wan",
			IPAddress:  "10.0.0.2",
			Netmask:    "255.255.255.0",
			Gateway:    "10.0.0.1",
			DNSServers: []string{"8.8.8.8"},
			MACAddress: "11:22:33:44:55:66",
			IsUp:       true,
			RxBytes:    5000000,
			TxBytes:    3000000,
		},
		LAN: NetworkInterface{
			Name:       "br-lan",
			Type:       "lan",
			IPAddress:  "192.168.1.1",
			Netmask:    "255.255.255.0",
			DNSServers: []string{},
			MACAddress: "AA:BB:CC:DD:EE:FF",
			IsUp:       true,
		},
		Interfaces:        []NetworkInterface{},
		Clients:           []Client{},
		InternetReachable: true,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal NetworkStatus: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}
	for _, key := range []string{"wan", "lan", "interfaces", "clients", "internet_reachable"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("expected JSON key %q not found", key)
		}
	}

	var decoded NetworkStatus
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal NetworkStatus: %v", err)
	}
	if decoded.InternetReachable != original.InternetReachable {
		t.Errorf("internet_reachable mismatch")
	}
}

func TestVpnStatusJSON(t *testing.T) {
	original := VpnStatus{
		Type:           "wireguard",
		Enabled:        true,
		Connected:      true,
		ConnectedSince: "2025-01-01T00:00:00Z",
		Endpoint:       "1.2.3.4:51820",
		RxBytes:        1000000,
		TxBytes:        2000000,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal VpnStatus: %v", err)
	}

	var decoded VpnStatus
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal VpnStatus: %v", err)
	}
	if decoded != original {
		t.Errorf("roundtrip mismatch: got %+v, want %+v", decoded, original)
	}
}

func TestServiceInfoJSON(t *testing.T) {
	ver := "0.107.43"
	original := ServiceInfo{
		ID:          "adguard",
		Name:        "AdGuard Home",
		Description: "DNS-based ad blocker",
		State:       "running",
		Version:     &ver,
		AutoStart:   true,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal ServiceInfo: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}
	for _, key := range []string{"id", "name", "description", "state", "version", "auto_start"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("expected JSON key %q not found", key)
		}
	}

	var decoded ServiceInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal ServiceInfo: %v", err)
	}
	if *decoded.Version != *original.Version {
		t.Errorf("version mismatch: got %s, want %s", *decoded.Version, *original.Version)
	}
}

func TestCaptivePortalStatusJSON(t *testing.T) {
	portalURL := "http://captive.example.com/login"
	original := CaptivePortalStatus{
		Detected:         true,
		PortalURL:        &portalURL,
		CanReachInternet: false,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal CaptivePortalStatus: %v", err)
	}

	var decoded CaptivePortalStatus
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal CaptivePortalStatus: %v", err)
	}
	if decoded.Detected != original.Detected {
		t.Errorf("detected mismatch")
	}
	if *decoded.PortalURL != *original.PortalURL {
		t.Errorf("portal_url mismatch")
	}
}

func TestLoginResponseJSON(t *testing.T) {
	original := LoginResponse{
		Token:     "jwt-token-here",
		ExpiresAt: "2025-01-02T00:00:00Z",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal LoginResponse: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}
	if _, ok := raw["expires_at"]; !ok {
		t.Error("expected JSON key 'expires_at' not found")
	}

	var decoded LoginResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal LoginResponse: %v", err)
	}
	if decoded != original {
		t.Errorf("roundtrip mismatch: got %+v, want %+v", decoded, original)
	}
}

func TestWireguardConfigJSON(t *testing.T) {
	original := WireguardConfig{
		PrivateKey: "key123=",
		Address:    "10.0.0.2/32",
		DNS:        []string{"1.1.1.1"},
		Peers: []WireguardPeer{
			{
				PublicKey:  "pub123=",
				Endpoint:   "1.2.3.4:51820",
				AllowedIPs: []string{"0.0.0.0/0"},
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal WireguardConfig: %v", err)
	}

	var decoded WireguardConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal WireguardConfig: %v", err)
	}
	if len(decoded.Peers) != 1 {
		t.Errorf("expected 1 peer, got %d", len(decoded.Peers))
	}
	if decoded.Peers[0].PublicKey != original.Peers[0].PublicKey {
		t.Errorf("peer public key mismatch")
	}
}
