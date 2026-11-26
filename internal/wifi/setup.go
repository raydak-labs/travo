package wifi

import (
	"context"
	"slices"

	"github.com/raydak-labs/travo/internal/device"
	"github.com/raydak-labs/travo/internal/txn"
)

// DefaultSSID is the name a fresh router's WiFi gets unless the user picks another one. Both
// bands share it, so moving an AP to the other radio for an uplink join is invisible to clients.
const DefaultSSID = "Travo"

// SetupEncryption is WPA2/WPA3 mixed: WPA3 devices use SAE, older ones WPA2-PSK, with one key.
const SetupEncryption = "sae-mixed"

// NotSetUp reports whether the router's own WiFi is still as OpenWrt ships it: at least one
// main AP, and every one of them off and open. An AP a schedule turned off keeps its key, so a
// WiFi that is merely off for the night does not count.
func NotSetUp(aps []APConfig) bool {
	main := 0
	for _, ap := range aps {
		if ap.Guest {
			continue
		}
		if ap.Enabled || ap.Security != "open" {
			return false
		}
		main++
	}
	return main > 0
}

// SetupRequest turns on the router's own WiFi: every AP bridged into lan gets the same name
// and key and is enabled, in one transaction.
type SetupRequest struct {
	SSID, Key string
}

func (r SetupRequest) validate() error {
	if err := validSSID(r.SSID); err != nil {
		return err
	}
	return validKey(r.Key)
}

// mainAP: an AP bridged into lan (the guest AP and APs on other networks are not touched).
func mainAP(section device.Section) bool {
	return isAP(section) && slices.Contains(sectionNetworks(section), "lan")
}

// planSetup enables every main AP whose radio carries no enabled STA (an AP and a STA on one
// radio crash ath11k, device-facts §3), with the name, the mixed-mode encryption and the key.
// A radio switched off as a whole is switched on. The probe then proves every AP up.
func planSetup(wireless []device.Section, req SetupRequest) ([]uciWrite, []apExpectation, error) {
	var writes []uciWrite
	enabled := map[string]bool{}
	enabledRadios := map[string]bool{}
	found := false
	for _, section := range wireless {
		if !mainAP(section) {
			continue
		}
		found = true
		radio := section.Options["device"]
		if radioHasSTA(wireless, radio) {
			continue
		}
		enabled[section.Name] = true
		set := func(option, value string) {
			if section.Options[option] != value {
				writes = append(writes, uciWrite{"set", "wireless", section.Name, option, value})
			}
		}
		set("ssid", req.SSID)
		set("encryption", SetupEncryption)
		if section.Options["key"] != req.Key {
			writes = append(writes, uciWrite{"key", "wireless", section.Name, "key", ""})
		}
		set("disabled", "0")
		if radioDisabled(wireless, radio) && !enabledRadios[radio] {
			writes = append(writes, uciWrite{"set", "wireless", radio, "disabled", "0"})
			enabledRadios[radio] = true
		}
	}
	switch {
	case !found:
		return nil, nil, ErrAPNotFound
	case len(enabled) == 0:
		return nil, nil, ErrRadioInUse
	}
	var expect []apExpectation
	for _, section := range wireless {
		switch {
		case enabled[section.Name]:
			expect = append(expect, apExpectation{section: section.Name, enabled: true, ssid: req.SSID})
		// Other APs on a radio this plan switches on come up too, so the probe proves them.
		case isAP(section) && section.Options["disabled"] != "1" &&
			(enabledRadios[section.Options["device"]] || !radioDisabled(wireless, section.Options["device"])):
			expect = append(expect, apExpectation{section: section.Name, enabled: true})
		}
	}
	return writes, expect, nil
}

// SetupAPs applies planSetup through a guarded transaction. It only enables APs, so it never
// needs the lockout check (invariants §2: only disabling an AP can cut the caller off).
func (s *Service) SetupAPs(ctx context.Context, req SetupRequest) (txn.Status, error) {
	if err := req.validate(); err != nil {
		return txn.Status{}, err
	}
	if s.join == nil {
		return txn.Status{}, ErrUnavailable
	}
	return s.applyAPPlan(ctx, "wifi: set up access points", []string{"wireless", "network", "firewall"}, req.Key,
		func(ctx context.Context) (apPlan, error) {
			wireless, err := s.join.UCI.Show(ctx, "wireless")
			if err != nil {
				return apPlan{}, ErrUnavailable
			}
			writes, expect, err := planSetup(wireless, req)
			return apPlan{writes, expect}, err
		})
}
