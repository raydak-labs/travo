package services

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

const defaultRadioConfigFile = "/etc/travo/radios.json"

// RadioConfig is the persisted radio hardware discovery result.
type RadioConfig struct {
	Discovered bool                `json:"discovered"`
	Radios     []RadioConfigEntry  `json:"radios"`
}

// RadioConfigEntry is one discovered radio with assigned default role.
type RadioConfigEntry struct {
	Name        string `json:"name"`
	Band        string `json:"band"`
	Type        string `json:"type"`
	DefaultRole string `json:"default_role"`
}

// DiscoverAndPersistRadios runs radio discovery and writes the result to disk.
// Only runs if the config file doesn't already exist (first boot).
// Returns true if discovery was performed (first boot), false if already done.
func (w *WifiService) DiscoverAndPersistRadios() (bool, error) {
	configPath := defaultRadioConfigFile

	// Skip if already discovered
	if _, err := os.Stat(configPath); err == nil {
		return false, nil
	}

	radios, err := w.GetRadios()
	if err != nil {
		return false, err
	}

	config := RadioConfig{
		Discovered: true,
		Radios:     assignDefaultRoles(radios),
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return false, err
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return false, err
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return false, err
	}

	log.Printf("radio: discovered %d radio(s), config persisted to %s", len(radios), configPath)
	return true, nil
}

// assignDefaultRoles applies a sensible default policy:
// - If single radio: role=both (AP + STA on same radio)
// - If two radios: 5GHz=ap, 2.4GHz=sta (uplink on 2.4 for range, AP on 5 for speed)
// - If three+ radios: first 5GHz=ap, first 2.4GHz=sta, rest=ap
func assignDefaultRoles(radios []models.RadioInfo) []RadioConfigEntry {
	entries := make([]RadioConfigEntry, len(radios))

	if len(radios) == 1 {
		entries[0] = RadioConfigEntry{
			Name:        radios[0].Name,
			Band:        radios[0].Band,
			Type:        radios[0].Type,
			DefaultRole: "both",
		}
		return entries
	}

	// Multi-radio: assign based on band
	assigned5AP := false
	assigned24STA := false

	for i, r := range radios {
		entries[i] = RadioConfigEntry{
			Name: r.Name,
			Band: r.Band,
			Type: r.Type,
		}

		switch {
		case r.Band == "5g" && !assigned5AP:
			entries[i].DefaultRole = "ap"
			assigned5AP = true
		case (r.Band == "2g" || r.Band == "") && !assigned24STA:
			entries[i].DefaultRole = "sta"
			assigned24STA = true
		default:
			entries[i].DefaultRole = "ap"
		}
	}

	// Fallback: if no 2.4GHz found for STA, assign last radio as STA
	if !assigned24STA && len(entries) >= 2 {
		entries[len(entries)-1].DefaultRole = "sta"
	}

	return entries
}
