package network

import (
	"errors"
	"fmt"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/platform"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func TestGetWanConfig(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: u, Ubus: ub})

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

func TestSetWanConfigReturnsErrorOnSetFailure(t *testing.T) {
	fu := &failingSetUCI{
		MockUCI: uci.NewMockUCI(),
		setErr:  fmt.Errorf("mock set error"),
	}
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: fu, Ubus: ub})

	cfg := models.WanConfig{
		Type: "static",
	}
	err := svc.SetWanConfig(cfg)
	if err == nil {
		t.Error("expected error when uci.Set fails")
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
			svc := New(Deps{UCI: fu, Ubus: ub})

			err := svc.SetWanConfig(tt.config)
			if err == nil {
				t.Errorf("expected error for field %s", tt.name)
			}
		})
	}
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
	svc := New(Deps{UCI: u, Ubus: ub, Cmd: cmd})

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
	cmd := &platform.MockCommandRunner{Output: []byte("1234\n")}
	svc := New(Deps{UCI: u, Ubus: ub, Cmd: cmd})

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
	cmd := &platform.MockCommandRunner{Err: fmt.Errorf("not found")}
	svc := New(Deps{UCI: u, Ubus: ub, Cmd: cmd})

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

// The handler strictly validates MTU and every DNS server and the OpenAPI body
// declares both, but the service dropped them on the floor and answered 200 —
// so the documented "GET, change a field, PUT it back" client pattern reported
// a successful save of nothing.
func TestSetWanConfigWritesMTUAndDNSServers(t *testing.T) {
	u := uci.NewMockUCI()
	svc := New(Deps{UCI: u, Ubus: ubus.NewMockUbus()})

	cfg := models.WanConfig{Type: "dhcp", MTU: 1400, DNSServers: []string{"1.1.1.1", "9.9.9.9"}}
	if err := svc.SetWanConfig(cfg); err != nil {
		t.Fatalf("SetWanConfig: %v", err)
	}

	got, err := svc.GetWanConfig()
	if err != nil {
		t.Fatalf("GetWanConfig: %v", err)
	}
	if got.MTU != 1400 {
		t.Errorf("MTU must be persisted, got %d", got.MTU)
	}
	if len(got.DNSServers) != 2 || got.DNSServers[0] != "1.1.1.1" || got.DNSServers[1] != "9.9.9.9" {
		t.Errorf("DNS servers must be persisted, got %v", got.DNSServers)
	}
	if peerdns, _ := u.Get("network", "wan", "peerdns"); peerdns != "0" {
		t.Errorf("custom DNS servers must set peerdns=0, got %q", peerdns)
	}
}

// An explicitly empty dns_servers list means "use the DHCP-provided resolvers
// again"; an absent list must not touch them at all.
func TestSetWanConfigDNSHandling(t *testing.T) {
	t.Run("absent list leaves DNS alone", func(t *testing.T) {
		u := uci.NewMockUCI()
		svc := New(Deps{UCI: u, Ubus: ubus.NewMockUbus()})
		if err := svc.SetWanConfig(models.WanConfig{Type: "dhcp", MTU: 1500}); err != nil {
			t.Fatalf("SetWanConfig: %v", err)
		}
		if dns, _ := u.Get("network", "wan", "dns"); dns == "" {
			t.Error("an absent dns_servers list must not clear the configured DNS")
		}
	})

	t.Run("empty list restores peer DNS", func(t *testing.T) {
		u := uci.NewMockUCI()
		svc := New(Deps{UCI: u, Ubus: ubus.NewMockUbus()})
		if err := svc.SetWanConfig(models.WanConfig{Type: "dhcp", DNSServers: []string{}}); err != nil {
			t.Fatalf("SetWanConfig: %v", err)
		}
		if peerdns, _ := u.Get("network", "wan", "peerdns"); peerdns != "1" {
			t.Errorf("an empty dns_servers list must set peerdns=1, got %q", peerdns)
		}
		if dns, _ := u.Get("network", "wan", "dns"); dns != "" {
			t.Errorf("an empty dns_servers list must clear the static resolvers, got %q", dns)
		}
	})
}

// Switching modes must not leave the previous mode's options behind.
func TestSetWanConfigSwitchesModesCleanly(t *testing.T) {
	u := uci.NewMockUCI()
	_ = u.Set("network", "wan", "proto", "static")
	_ = u.Set("network", "wan", "ip4addr", "203.0.113.5")
	_ = u.Set("network", "wan", "netmask", "255.255.255.0")
	_ = u.Set("network", "wan", "gateway", "203.0.113.1")
	svc := New(Deps{UCI: u, Ubus: ubus.NewMockUbus()})

	if err := svc.SetWanConfig(models.WanConfig{Type: "dhcp"}); err != nil {
		t.Fatalf("SetWanConfig: %v", err)
	}
	for _, option := range wanStaticOptions {
		if value, _ := u.Get("network", "wan", option); value != "" {
			t.Errorf("switching to dhcp must delete %s, got %q", option, value)
		}
	}
	if proto, _ := u.Get("network", "wan", "proto"); proto != "dhcp" {
		t.Errorf("proto = %q, want dhcp", proto)
	}

	// And back the other way: the static triple survives a no-op type write.
	if err := svc.SetWanConfig(models.WanConfig{
		Type: "static", IPAddress: "198.51.100.7", Netmask: "255.255.255.0", Gateway: "198.51.100.1",
	}); err != nil {
		t.Fatalf("SetWanConfig static: %v", err)
	}
	if value, _ := u.Get("network", "wan", "ip4addr"); value != "198.51.100.7" {
		t.Errorf("ip4addr = %q, want 198.51.100.7", value)
	}
}

// Writing proto=pppoe without credentials creates a WAN that can never
// authenticate, so it must be refused rather than reported as a save.
func TestSetWanConfigRejectsPPPoEWithoutCredentials(t *testing.T) {
	t.Run("switching into pppoe", func(t *testing.T) {
		u := uci.NewMockUCI() // proto dhcp, no credentials
		svc := New(Deps{UCI: u, Ubus: ubus.NewMockUbus()})
		err := svc.SetWanConfig(models.WanConfig{Type: "pppoe"})
		if !errors.Is(err, errPPPoEUnsupported) {
			t.Fatalf("expected errPPPoEUnsupported, got %v", err)
		}
		if proto, _ := u.Get("network", "wan", "proto"); proto != "dhcp" {
			t.Errorf("a rejected save must not change the proto, got %q", proto)
		}
	})

	t.Run("pppoe section without credentials", func(t *testing.T) {
		u := uci.NewMockUCI()
		_ = u.Set("network", "wan", "proto", "pppoe")
		svc := New(Deps{UCI: u, Ubus: ubus.NewMockUbus()})
		err := svc.SetWanConfig(models.WanConfig{Type: "pppoe", MTU: 1400})
		if !errors.Is(err, errPPPoEUnsupported) {
			t.Fatalf("expected errPPPoEUnsupported, got %v", err)
		}
	})

	t.Run("already configured pppoe stays editable", func(t *testing.T) {
		u := uci.NewMockUCI()
		_ = u.Set("network", "wan", "proto", "pppoe")
		_ = u.Set("network", "wan", "username", "user@example.net")
		_ = u.Set("network", "wan", "password", "secret")
		svc := New(Deps{UCI: u, Ubus: ubus.NewMockUbus()})
		if err := svc.SetWanConfig(models.WanConfig{Type: "pppoe", MTU: 1400}); err != nil {
			t.Fatalf("editing a configured PPPoE WAN must be allowed: %v", err)
		}
		if mtu, _ := u.Get("network", "wan", "mtu"); mtu != "1400" {
			t.Errorf("MTU must be written, got %q", mtu)
		}
	})
}

func TestSetWanConfigRejectsOutOfRangeMTU(t *testing.T) {
	u := uci.NewMockUCI()
	svc := New(Deps{UCI: u, Ubus: ubus.NewMockUbus()})
	if err := svc.SetWanConfig(models.WanConfig{Type: "dhcp", MTU: 9001}); err == nil {
		t.Fatal("expected an out-of-range MTU to be rejected")
	}
}

// `uci delete` on an option that is not set is itself an error, so a mode switch
// must only clear the options that are actually there.
func TestSetWanConfigModeSwitchToleratesMissingOptions(t *testing.T) {
	u := uci.NewMockUCI()
	_ = u.Set("network", "wan", "proto", "static")
	_ = u.Set("network", "wan", "ip4addr", "203.0.113.5")
	_ = u.DeleteOption("network", "wan", "netmask")
	_ = u.DeleteOption("network", "wan", "gateway")
	svc := New(Deps{UCI: u, Ubus: ubus.NewMockUbus()})

	if err := svc.SetWanConfig(models.WanConfig{Type: "dhcp"}); err != nil {
		t.Fatalf("SetWanConfig: %v", err)
	}
	if proto, _ := u.Get("network", "wan", "proto"); proto != "dhcp" {
		t.Errorf("proto = %q, want dhcp", proto)
	}
}
