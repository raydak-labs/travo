package facts

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// WirelessIface is one configured interface in netifd order. Liveness is read
// separately with ParseDeviceStatus; netifd has no per-interface up flag.
type WirelessIface struct {
	Section string
	Ifname  string
	Mode    string
	SSID    string
}

// Radio is the configured state of one radio in network.wireless status.
type Radio struct {
	Name     string
	Band     string
	Up       bool
	Disabled bool
	Ifaces   []WirelessIface
}

// ParseWirelessRadios retains interface-less radios, mode and SSID for the WiFi summary.
func ParseWirelessRadios(b []byte) ([]Radio, error) {
	var raw map[string]struct {
		Up       *bool `json:"up"`
		Disabled *bool `json:"disabled"`
		Config   struct {
			Band string `json:"band"`
		} `json:"config"`
		Interfaces []struct {
			Section string `json:"section"`
			Ifname  string `json:"ifname"`
			Config  struct {
				Mode string `json:"mode"`
				SSID string `json:"ssid"`
			} `json:"config"`
		} `json:"interfaces"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("wireless radios: %w", err)
	}
	if raw == nil {
		return nil, fmt.Errorf("wireless radios: %w: null payload", ErrShape)
	}
	names := make([]string, 0, len(raw))
	for name := range raw {
		names = append(names, name)
	}
	slices.Sort(names)
	out := make([]Radio, 0, len(names))
	for _, name := range names {
		r := raw[name]
		if r.Up == nil || r.Disabled == nil {
			return nil, fmt.Errorf("wireless radios %s: %w: need up and disabled", name, ErrShape)
		}
		band := r.Config.Band
		switch band {
		case "2g", "5g", "6g":
		default:
			band = "unknown"
		}
		radio := Radio{Name: name, Band: band, Up: *r.Up, Disabled: *r.Disabled, Ifaces: make([]WirelessIface, 0, len(r.Interfaces))}
		for _, iface := range r.Interfaces {
			radio.Ifaces = append(radio.Ifaces, WirelessIface{
				Section: iface.Section,
				Ifname:  iface.Ifname,
				Mode:    iface.Config.Mode,
				SSID:    iface.Config.SSID,
			})
		}
		out = append(out, radio)
	}
	return out, nil
}

// Encryption is the security shape reported by iwinfo.
type Encryption struct {
	Enabled        bool
	WPA            []int
	Authentication []string
}

// ScanAP is one BSSID returned by iwinfo scan. An empty SSID means hidden.
type ScanAP struct {
	SSID       string
	BSSID      string
	Band       int
	Channel    int
	MHz        int
	Signal     int
	Quality    int
	QualityMax int
	Encryption Encryption
}

// ParseIwinfoAPSSID parses `ubus call iwinfo info {"device": ifname}` and returns the
// SSID the AP broadcasts now. Only a Master-mode reply proves a live AP.
func ParseIwinfoAPSSID(b []byte) (string, error) {
	var raw struct {
		SSID *string `json:"ssid"`
		Mode string  `json:"mode"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return "", fmt.Errorf("iwinfo info: %w", err)
	}
	if raw.SSID == nil || raw.Mode != "Master" {
		return "", fmt.Errorf("iwinfo info: %w: need ssid in Master mode", ErrShape)
	}
	return *raw.SSID, nil
}

// ParseIwinfoScan parses every result; an unreadable entry fails the whole payload.
func ParseIwinfoScan(b []byte) ([]ScanAP, error) {
	var raw struct {
		Results []struct {
			SSID       string  `json:"ssid"`
			BSSID      *string `json:"bssid"`
			Band       *int    `json:"band"`
			Channel    *int    `json:"channel"`
			MHz        *int    `json:"mhz"`
			Signal     *int    `json:"signal"`
			Quality    *int    `json:"quality"`
			QualityMax *int    `json:"quality_max"`
			Encryption struct {
				Enabled        *bool    `json:"enabled"`
				WPA            []int    `json:"wpa"`
				Authentication []string `json:"authentication"`
			} `json:"encryption"`
		} `json:"results"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("iwinfo scan: %w", err)
	}
	if raw.Results == nil {
		return nil, fmt.Errorf("iwinfo scan: %w: no results array", ErrShape)
	}
	out := make([]ScanAP, 0, len(raw.Results))
	for i, r := range raw.Results {
		if r.BSSID == nil || *r.BSSID == "" || r.Band == nil || r.Channel == nil || r.MHz == nil ||
			r.Signal == nil || r.Quality == nil || r.QualityMax == nil || r.Encryption.Enabled == nil {
			return nil, fmt.Errorf("iwinfo scan result %d: %w: required field missing", i, ErrShape)
		}
		out = append(out, ScanAP{
			SSID: r.SSID, BSSID: strings.ToLower(*r.BSSID), Band: *r.Band,
			Channel: *r.Channel, MHz: *r.MHz, Signal: *r.Signal,
			Quality: *r.Quality, QualityMax: *r.QualityMax,
			Encryption: Encryption{Enabled: *r.Encryption.Enabled, WPA: r.Encryption.WPA, Authentication: r.Encryption.Authentication},
		})
	}
	return out, nil
}
