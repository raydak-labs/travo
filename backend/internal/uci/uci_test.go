package uci

import "testing"

func TestMockUCIGet(t *testing.T) {
	m := NewMockUCI()
	val, err := m.Get("network", "wan", "proto")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "dhcp" {
		t.Errorf("expected 'dhcp', got %q", val)
	}
}

func TestMockUCIGetNotFound(t *testing.T) {
	m := NewMockUCI()
	_, err := m.Get("nonexistent", "x", "y")
	if err == nil {
		t.Error("expected error for nonexistent key")
	}
}

func TestMockUCISet(t *testing.T) {
	m := NewMockUCI()
	if err := m.Set("network", "wan", "proto", "static"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, err := m.Get("network", "wan", "proto")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "static" {
		t.Errorf("expected 'static', got %q", val)
	}
}

func TestMockUCISetNewConfig(t *testing.T) {
	m := NewMockUCI()
	if err := m.Set("newconfig", "newsection", "opt", "val"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, err := m.Get("newconfig", "newsection", "opt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "val" {
		t.Errorf("expected 'val', got %q", val)
	}
}

func TestMockUCIGetAll(t *testing.T) {
	m := NewMockUCI()
	opts, err := m.GetAll("network", "wan")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts["proto"] != "dhcp" {
		t.Errorf("expected proto 'dhcp', got %q", opts["proto"])
	}
	if opts["ifname"] != "eth0" {
		t.Errorf("expected ifname 'eth0', got %q", opts["ifname"])
	}
}

func TestMockUCIGetAllNotFound(t *testing.T) {
	m := NewMockUCI()
	_, err := m.GetAll("nonexistent", "x")
	if err == nil {
		t.Error("expected error for nonexistent section")
	}
}

func TestMockUCICommit(t *testing.T) {
	m := NewMockUCI()
	if err := m.Commit("network"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMockUCIAddSection(t *testing.T) {
	m := NewMockUCI()
	if err := m.AddSection("network", "wg1", "interface"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	opts, err := m.GetAll("network", "wg1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts[".type"] != "interface" {
		t.Errorf("expected .type 'interface', got %q", opts[".type"])
	}
}

func TestMockUCIAddSectionAlreadyExists(t *testing.T) {
	m := NewMockUCI()
	err := m.AddSection("network", "wan", "interface")
	if err == nil {
		t.Error("expected error adding existing section")
	}
}

func TestMockUCIDeleteSection(t *testing.T) {
	m := NewMockUCI()
	if err := m.DeleteSection("network", "wan"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err := m.Get("network", "wan", "proto")
	if err == nil {
		t.Error("expected error after deleting section")
	}
}

func TestMockUCIDeleteSectionNotFound(t *testing.T) {
	m := NewMockUCI()
	err := m.DeleteSection("network", "nonexistent")
	if err == nil {
		t.Error("expected error deleting nonexistent section")
	}
}

func TestMockUCIPrePopulated(t *testing.T) {
	m := NewMockUCI()
	tests := []struct {
		config, section, option, expected string
	}{
		{"network", "lan", "ipaddr", "192.168.8.1"},
		{"wireless", "radio0", "channel", "6"},
		{"wireless", "default_radio0", "ssid", "OpenWrt-Travel"},
		{"system", "system", "hostname", "OpenWrt"},
		{"dhcp", "lan", "leasetime", "12h"},
		{"firewall", "defaults", "forward", "REJECT"},
	}
	for _, tt := range tests {
		val, err := m.Get(tt.config, tt.section, tt.option)
		if err != nil {
			t.Errorf("Get(%s,%s,%s): unexpected error: %v", tt.config, tt.section, tt.option, err)
			continue
		}
		if val != tt.expected {
			t.Errorf("Get(%s,%s,%s): expected %q, got %q", tt.config, tt.section, tt.option, tt.expected, val)
		}
	}
}

func TestMockUCIGetSections(t *testing.T) {
	m := NewMockUCI()
	sections, err := m.GetSections("dhcp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := sections["lan"]; !ok {
		t.Error("expected 'lan' section in dhcp config")
	}
	if sections["lan"]["start"] != "100" {
		t.Errorf("expected start '100', got %q", sections["lan"]["start"])
	}
}

func TestMockUCIGetSections_EmptyConfig(t *testing.T) {
	m := NewMockUCI()
	sections, err := m.GetSections("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected empty map, got %d sections", len(sections))
	}
}

func TestMockUCIGetSections_WithAddedSection(t *testing.T) {
	m := NewMockUCI()
	_ = m.AddSection("dhcp", "dns_test", "domain")
	_ = m.Set("dhcp", "dns_test", "name", "test")
	_ = m.Set("dhcp", "dns_test", "ip", "192.168.1.10")

	sections, err := m.GetSections("dhcp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sections["dns_test"][".type"] != "domain" {
		t.Errorf("expected .type 'domain', got %q", sections["dns_test"][".type"])
	}
	if sections["dns_test"]["name"] != "test" {
		t.Errorf("expected name 'test', got %q", sections["dns_test"]["name"])
	}
}
