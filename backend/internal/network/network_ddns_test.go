package network

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/platform"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func TestGetDDNSConfig(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: u, Ubus: ub})

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
	svc := New(Deps{UCI: u, Ubus: ub})

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
	svc := New(Deps{UCI: u, Ubus: ub})

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
	cmdRunner := &platform.FuncCommandRunner{
		RunFunc: func(_ string, _ ...string) ([]byte, error) {
			return []byte("ok"), nil
		},
	}
	svc := New(Deps{UCI: u, Ubus: ub, Cmd: cmdRunner})
	_ = fakeDDNSScript(t, svc)

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
	cmdRunner := &platform.FuncCommandRunner{
		RunFunc: func(_ string, _ ...string) ([]byte, error) {
			return []byte("ok"), nil
		},
	}
	svc := New(Deps{UCI: u, Ubus: ub, Cmd: cmdRunner})
	_ = fakeDDNSScript(t, svc)

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
	cmdRunner := &platform.FuncCommandRunner{
		RunFunc: func(_ string, _ ...string) ([]byte, error) {
			return []byte("ok"), nil
		},
	}
	svc := New(Deps{UCI: u, Ubus: ub, Cmd: cmdRunner})
	_ = fakeDDNSScript(t, svc)

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
	cmdRunner := &platform.FuncCommandRunner{
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
	svc := New(Deps{UCI: u, Ubus: ub, Cmd: cmdRunner})

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
	cmdRunner := &platform.FuncCommandRunner{
		RunFunc: func(_ string, _ ...string) ([]byte, error) {
			return nil, fmt.Errorf("not found")
		},
	}
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: u, Ubus: ub, Cmd: cmdRunner})

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

// ddns-scripts is what can actually service a `ddns` UCI config, and it is not
// in the service catalog, so the UI cannot install it and can only explain.
// Without this check every write failed with a bare
// `uci: Entry not found` 500 that said nothing about the real cause — verified on
// the device, where PUT /api/v1/network/ddns returned
// 500 {"error":"set ddns.myddns.enabled: uci: Entry not found"}.
func TestSetDDNSConfig_ReportsMissingPackageClearly(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	cmdRunner := &platform.FuncCommandRunner{
		RunFunc: func(_ string, _ ...string) ([]byte, error) { return []byte("ok"), nil },
	}
	svc := New(Deps{UCI: u, Ubus: ub, Cmd: cmdRunner})
	// Deliberately no fakeDDNSScript: this is the router-without-ddns-scripts case.

	if svc.DDNSAvailable() {
		t.Fatal("DDNSAvailable must be false when the init script is absent")
	}

	err := svc.SetDDNSConfig(models.DDNSConfig{
		Enabled: true, Service: "no-ip.com", Domain: "test.no-ip.org",
	})
	if !errors.Is(err, ErrDDNSNotAvailable) {
		t.Fatalf("got %v, want ErrDDNSNotAvailable", err)
	}
	// The message has to name the package, or the operator cannot act on it.
	if !strings.Contains(err.Error(), "ddns-scripts") {
		t.Errorf("error %q does not name the missing package", err)
	}
	// And nothing may have been written: a refused write must not leave a
	// half-updated config behind.
	opts, getErr := u.GetAll("ddns", "myddns")
	if getErr != nil {
		t.Fatalf("read back ddns.myddns: %v", getErr)
	}
	if opts["domain"] != "myrouter.duckdns.org" || opts["enabled"] != "0" {
		t.Errorf("a refused DDNS write still modified the config: %v", opts)
	}
}

// The `myddns` section does not exist on a router where DDNS was never
// configured, and `uci set` on a missing section fails outright — so before this
// the endpoint could only ever succeed on a device that already had DDNS set up.
func TestSetDDNSConfig_CreatesSectionWhenAbsent(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	cmdRunner := &platform.FuncCommandRunner{
		RunFunc: func(_ string, _ ...string) ([]byte, error) { return []byte("ok"), nil },
	}
	svc := New(Deps{UCI: u, Ubus: ub, Cmd: cmdRunner})
	_ = fakeDDNSScript(t, svc)

	// Reproduce a router that has never had DDNS configured.
	if err := u.DeleteSection("ddns", "myddns"); err != nil {
		t.Fatalf("DeleteSection: %v", err)
	}
	if _, err := u.GetAll("ddns", "myddns"); err == nil {
		t.Fatal("precondition: the ddns.myddns section should be gone")
	}

	if err := svc.SetDDNSConfig(models.DDNSConfig{
		Enabled: true, Service: "no-ip.com", Domain: "test.no-ip.org",
	}); err != nil {
		t.Fatalf("SetDDNSConfig on a router with no existing ddns section: %v", err)
	}
	cfg, err := svc.GetDDNSConfig()
	if err != nil {
		t.Fatalf("GetDDNSConfig: %v", err)
	}
	if !cfg.Enabled || cfg.Domain != "test.no-ip.org" {
		t.Errorf("config not persisted correctly: %+v", cfg)
	}
}
