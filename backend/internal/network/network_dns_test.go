package network

import (
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func TestGetDNSConfig(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: u, Ubus: ub})

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
	svc := New(Deps{UCI: u, Ubus: ub})

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

func TestGetDNSEntries_Empty(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: u, Ubus: ub})

	entries, err := svc.GetDNSEntries()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 DNS entries, got %d", len(entries))
	}
}

func TestDeleteDNSEntry(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: u, Ubus: ub})

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
	svc := New(Deps{UCI: u, Ubus: ub})

	err := svc.DeleteDNSEntry("dns_nonexistent")
	if err == nil {
		t.Error("expected error deleting nonexistent section")
	}
}

func TestAddDNSEntry_DuplicateName(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: u, Ubus: ub})

	_ = svc.AddDNSEntry(models.DNSEntry{Name: "myserver", IP: "192.168.1.50"})
	err := svc.AddDNSEntry(models.DNSEntry{Name: "myserver", IP: "192.168.1.60"})
	if err == nil {
		t.Error("expected error adding duplicate DNS entry")
	}
}

func TestGetDNSEntries_SurfacesGetSectionsError(t *testing.T) {
	svc := New(Deps{UCI: &uci.FailingSectionsUCI{MockUCI: uci.NewMockUCI()}, Ubus: ubus.NewMockUbus()})

	if _, err := svc.GetDNSEntries(); err == nil {
		t.Fatal("expected an error instead of an empty DNS entry list when the UCI read fails")
	}
}

func TestDeleteDNSEntryRejectsNonEntrySections(t *testing.T) {
	u := uci.NewMockUCI()
	_ = u.AddSection("dhcp", "dnsmasq", "dnsmasq")
	_ = u.Set("dhcp", "dnsmasq", "start", "100")
	svc := New(Deps{UCI: u, Ubus: ubus.NewMockUbus()})

	for _, section := range []string{"@dnsmasq[0]", "dnsmasq", "lan", "missing"} {
		if err := svc.DeleteDNSEntry(section); err == nil {
			t.Errorf("DeleteDNSEntry(%q) must be refused", section)
		}
	}
	if _, err := u.Get("dhcp", "dnsmasq", "start"); err != nil {
		t.Errorf("the dnsmasq section must survive: %v", err)
	}
	if _, err := u.Get("dhcp", "lan", "start"); err != nil {
		t.Errorf("the lan dhcp section must survive: %v", err)
	}
}

func TestDeleteDNSEntryStillDeletesNamedDomain(t *testing.T) {
	u := uci.NewMockUCI()
	svc := New(Deps{UCI: u, Ubus: ubus.NewMockUbus()})
	if err := svc.AddDNSEntry(models.DNSEntry{Name: "nas.local", IP: "192.168.8.50"}); err != nil {
		t.Fatalf("AddDNSEntry: %v", err)
	}
	if err := svc.DeleteDNSEntry("dns_nas_local"); err != nil {
		t.Fatalf("DeleteDNSEntry: %v", err)
	}
	if _, err := u.GetAll("dhcp", "dns_nas_local"); err == nil {
		t.Error("expected the named domain section to be deleted")
	}
}
