package services

import (
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

func TestAssignDefaultRoles_SingleRadio(t *testing.T) {
	radios := []models.RadioInfo{
		{Name: "radio0", Band: "2g", Type: "mac80211"},
	}
	entries := assignDefaultRoles(radios)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].DefaultRole != "both" {
		t.Errorf("single radio should be 'both', got %q", entries[0].DefaultRole)
	}
}

func TestAssignDefaultRoles_DualRadio(t *testing.T) {
	radios := []models.RadioInfo{
		{Name: "radio0", Band: "5g", Type: "mac80211"},
		{Name: "radio1", Band: "2g", Type: "mac80211"},
	}
	entries := assignDefaultRoles(radios)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].DefaultRole != "ap" {
		t.Errorf("5GHz radio should be 'ap', got %q", entries[0].DefaultRole)
	}
	if entries[1].DefaultRole != "sta" {
		t.Errorf("2.4GHz radio should be 'sta', got %q", entries[1].DefaultRole)
	}
}

func TestAssignDefaultRoles_TripleRadio(t *testing.T) {
	radios := []models.RadioInfo{
		{Name: "radio0", Band: "5g", Type: "mac80211"},
		{Name: "radio1", Band: "2g", Type: "mac80211"},
		{Name: "radio2", Band: "5g", Type: "mac80211"},
	}
	entries := assignDefaultRoles(radios)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].DefaultRole != "ap" {
		t.Errorf("first 5GHz should be 'ap', got %q", entries[0].DefaultRole)
	}
	if entries[1].DefaultRole != "sta" {
		t.Errorf("2.4GHz should be 'sta', got %q", entries[1].DefaultRole)
	}
	if entries[2].DefaultRole != "ap" {
		t.Errorf("second 5GHz should be 'ap', got %q", entries[2].DefaultRole)
	}
}
