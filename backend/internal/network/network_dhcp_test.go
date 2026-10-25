package network

import (
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func TestGetDHCPConfig(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: u, Ubus: ub})

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
	svc := New(Deps{UCI: u, Ubus: ub})

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

func TestGetDHCPLeases_FileNotExist(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: u, Ubus: ub})

	leases := svc.GetDHCPLeases()
	if len(leases) != 0 {
		t.Errorf("expected empty slice when file not found, got %d", len(leases))
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
	svc := New(Deps{UCI: u, Ubus: ub})

	reservations, err := svc.GetDHCPReservations()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reservations) != 0 {
		t.Errorf("expected 0 reservations, got %d", len(reservations))
	}
}

func TestDeleteDHCPReservation(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: u, Ubus: ub})

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
	svc := New(Deps{UCI: u, Ubus: ub})

	err := svc.DeleteDHCPReservation("host_nonexistent")
	if err == nil {
		t.Error("expected error deleting nonexistent reservation")
	}
}

func TestAddDHCPReservation_DuplicateName(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: u, Ubus: ub})

	_ = svc.AddDHCPReservation(models.DHCPReservation{Name: "laptop", MAC: "AA:BB:CC:DD:EE:01", IP: "192.168.8.50"})
	err := svc.AddDHCPReservation(models.DHCPReservation{Name: "laptop", MAC: "AA:BB:CC:DD:EE:02", IP: "192.168.8.51"})
	if err == nil {
		t.Error("expected error adding duplicate DHCP reservation")
	}
}

func TestGetDHCPReservations_SurfacesGetSectionsError(t *testing.T) {
	svc := New(Deps{UCI: &uci.FailingSectionsUCI{MockUCI: uci.NewMockUCI()}, Ubus: ubus.NewMockUbus()})

	if _, err := svc.GetDHCPReservations(); err == nil {
		t.Fatal("expected an error instead of an empty reservation list when the UCI read fails")
	}
}

func TestDeleteDHCPReservationRejectsNonReservationSections(t *testing.T) {
	u := uci.NewMockUCI()
	_ = u.AddSection("dhcp", "dnsmasq", "dnsmasq")
	_ = u.Set("dhcp", "dnsmasq", "limit", "150")
	svc := New(Deps{UCI: u, Ubus: ubus.NewMockUbus()})

	for _, section := range []string{"@dnsmasq[0]", "dnsmasq", "missing"} {
		if err := svc.DeleteDHCPReservation(section); err == nil {
			t.Errorf("DeleteDHCPReservation(%q) must be refused", section)
		}
	}
	if _, err := u.Get("dhcp", "dnsmasq", "limit"); err != nil {
		t.Errorf("the dnsmasq section must survive: %v", err)
	}
}

func TestDeleteDHCPReservationStillDeletesNamedHost(t *testing.T) {
	u := uci.NewMockUCI()
	svc := New(Deps{UCI: u, Ubus: ubus.NewMockUbus()})
	if err := svc.AddDHCPReservation(models.DHCPReservation{
		Name: "nas", MAC: "aa:bb:cc:dd:ee:ff", IP: "192.168.8.60",
	}); err != nil {
		t.Fatalf("AddDHCPReservation: %v", err)
	}
	if err := svc.DeleteDHCPReservation("host_nas"); err != nil {
		t.Fatalf("DeleteDHCPReservation: %v", err)
	}
	if _, err := u.GetAll("dhcp", "host_nas"); err == nil {
		t.Error("expected the named host section to be deleted")
	}
}
