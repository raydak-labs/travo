package uplink

import (
	"testing"
)

func TestGetProfiles_EmptyByDefault(t *testing.T) {
	svc, _ := newTestVpnService(t)
	profiles, err := svc.GetProfiles()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(profiles) != 0 {
		t.Errorf("expected 0 profiles, got %d", len(profiles))
	}
}

func TestAddProfile(t *testing.T) {
	svc, _ := newTestVpnService(t)

	conf := "[Interface]\nPrivateKey = dGVzdHByaXZhdGVrZXkxMjM0NTY3ODkwMTIzNDU2\nAddress = 10.0.0.2/32\n\n[Peer]\nPublicKey = dGVzdHB1YmxpY2tleTEyMzQ1Njc4OTAxMjM0NTY=\nEndpoint = vpn.example.com:51820\nAllowedIPs = 0.0.0.0/0\n"
	profile, err := svc.AddProfile("Test VPN", conf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.Name != "Test VPN" {
		t.Errorf("expected name 'Test VPN', got %q", profile.Name)
	}
	if profile.ID == "" {
		t.Error("expected non-empty ID")
	}
	if profile.Active {
		t.Error("expected new profile to not be active")
	}

	profiles, _ := svc.GetProfiles()
	if len(profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(profiles))
	}
}

func TestAddProfile_InvalidConfig(t *testing.T) {
	svc, _ := newTestVpnService(t)
	_, err := svc.AddProfile("Bad", "not a valid config")
	if err == nil {
		t.Error("expected error for invalid config")
	}
}

func TestDeleteProfile(t *testing.T) {
	svc, _ := newTestVpnService(t)

	conf := "[Interface]\nPrivateKey = dGVzdHByaXZhdGVrZXkxMjM0NTY3ODkwMTIzNDU2\nAddress = 10.0.0.2/32\n\n[Peer]\nPublicKey = dGVzdHB1YmxpY2tleTEyMzQ1Njc4OTAxMjM0NTY=\nEndpoint = vpn.example.com:51820\nAllowedIPs = 0.0.0.0/0\n"
	profile, _ := svc.AddProfile("Test VPN", conf)

	err := svc.DeleteProfile(profile.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	profiles, _ := svc.GetProfiles()
	if len(profiles) != 0 {
		t.Errorf("expected 0 profiles after delete, got %d", len(profiles))
	}
}

func TestDeleteProfile_NotFound(t *testing.T) {
	svc, _ := newTestVpnService(t)
	err := svc.DeleteProfile("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent profile")
	}
}

func TestActivateProfile(t *testing.T) {
	svc, _ := newTestVpnService(t)

	conf := "[Interface]\nPrivateKey = dGVzdHByaXZhdGVrZXkxMjM0NTY3ODkwMTIzNDU2\nAddress = 10.0.0.2/32\n\n[Peer]\nPublicKey = dGVzdHB1YmxpY2tleTEyMzQ1Njc4OTAxMjM0NTY=\nEndpoint = vpn.example.com:51820\nAllowedIPs = 0.0.0.0/0\n"
	p1, _ := svc.AddProfile("VPN 1", conf)
	p2, _ := svc.AddProfile("VPN 2", conf)

	err := svc.ActivateProfile(p1.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	profiles, _ := svc.GetProfiles()
	for _, p := range profiles {
		if p.ID == p1.ID && !p.Active {
			t.Error("expected p1 to be active")
		}
		if p.ID == p2.ID && p.Active {
			t.Error("expected p2 to not be active")
		}
	}

	// Now activate p2
	err = svc.ActivateProfile(p2.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	profiles, _ = svc.GetProfiles()
	for _, p := range profiles {
		if p.ID == p1.ID && p.Active {
			t.Error("expected p1 to not be active after activating p2")
		}
		if p.ID == p2.ID && !p.Active {
			t.Error("expected p2 to be active")
		}
	}
}

func TestActivateProfile_NotFound(t *testing.T) {
	svc, _ := newTestVpnService(t)
	err := svc.ActivateProfile("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent profile")
	}
}
