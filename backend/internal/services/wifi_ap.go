package services

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/platform"
)

// Radios, AP configuration, and guest WiFi.

// ErrAPAndSTASameRadio reports a radio role that would run an access point and
// the uplink STA on the same PHY while the repeater split policy forbids it.
// ADR 0002 §2: committing that state — even transiently — is enough to crash
// ath11k/IPQ6018, and allow_ap_on_sta_radio is the explicit, documented escape
// hatch for it. SetRadioRole refuses the request instead of applying it.
var ErrAPAndSTASameRadio = errors.New("refusing to run an access point and the WiFi uplink " +
	"on the same radio: give the uplink STA its own radio and put the downlink access " +
	"point on the other one, or enable allow_ap_on_sta_radio in repeater options first")

const (
	guestNetwork = "guest"
	guestIPAddr  = "192.168.2.1"
	guestNetmask = "255.255.255.0"
)

// GetAPConfigs returns the AP configuration for all radios.
func (w *WifiService) GetAPConfigs() ([]models.APConfig, error) {
	sections, err := w.uci.GetSections("wireless")
	if err != nil {
		return nil, err
	}
	configs := make([]models.APConfig, 0, len(sections))
	for _, section := range ifaceSections(sections, "ap") {
		opts := sections[section]
		radio := opts["device"]
		radioOpts, _ := w.uci.GetAll("wireless", radio)
		band := radioOpts["band"]
		channel := 0
		if ch, ok := radioOpts["channel"]; ok {
			if v, err := strconv.Atoi(ch); err == nil {
				channel = v
			}
		}
		enabled := opts["disabled"] != "1"
		configs = append(configs, models.APConfig{
			Radio:      radio,
			Band:       band,
			SSID:       opts["ssid"],
			Encryption: opts["encryption"],
			Key:        opts["key"],
			Enabled:    enabled,
			Channel:    channel,
			Section:    section,
		})
	}
	return configs, nil
}

// SetAPConfig updates AP configuration for a specific section.
// When update.Enabled is nil, UCI disabled is not modified (for repeater credential sync).
func (w *WifiService) SetAPConfig(section string, update models.APConfigUpdate, req LockoutRequest) (*WirelessApplyResult, error) {
	// Only disabling removes the access point; every other field of this request
	// (SSID, key, re-enabling) leaves it up and must not be policed.
	if update.Enabled != nil && !*update.Enabled {
		if err := w.guardLockoutExcluding(req, section, ""); err != nil {
			return nil, err
		}
	}
	return w.mutateWireless([]string{"wireless"}, func() (*WirelessApplyResult, error) {
		opts, err := w.uci.GetAll("wireless", section)
		if err != nil {
			return nil, fmt.Errorf("AP section %s not found", section)
		}
		if opts["mode"] != "ap" {
			return nil, fmt.Errorf("section %s is not an AP interface", section)
		}
		// Enabling an AP is a single-radio decision, so it is policed the same way
		// SetRadioRole is: refuse it when this radio carries the WiFi uplink.
		// reconcileRepeaterAPRadioLayout is not a substitute — it does nothing at
		// all outside repeater mode, which is exactly the mode an operator is in
		// when they enable an AP by hand. Checked before any write so the refusal
		// leaves no staged delta behind.
		if update.Enabled != nil && *update.Enabled {
			if err := w.rejectAPOnUplinkRadio(opts["device"], "access point enabled"); err != nil {
				return nil, err
			}
		}
		if update.SSID != "" {
			if err := w.uci.Set("wireless", section, "ssid", update.SSID); err != nil {
				return nil, fmt.Errorf("setting SSID: %w", err)
			}
		}
		if update.Encryption != "" {
			if err := w.uci.Set("wireless", section, "encryption", update.Encryption); err != nil {
				return nil, fmt.Errorf("setting encryption: %w", err)
			}
		}
		if update.Encryption != "none" && update.Key != "" {
			if err := w.uci.Set("wireless", section, "key", update.Key); err != nil {
				return nil, fmt.Errorf("setting key: %w", err)
			}
		}
		if update.Enabled != nil {
			disabled := platform.BoolToEnabled(!*update.Enabled)
			if err := w.uci.Set("wireless", section, "disabled", disabled); err != nil {
				return nil, fmt.Errorf("setting disabled: %w", err)
			}
			if *update.Enabled {
				if err := w.ensureSectionRadioEnabled(section); err != nil {
					return nil, fmt.Errorf("enabling AP radio: %w", err)
				}
			}
		}
		if err := w.reconcileRepeaterAPRadioLayout(); err != nil {
			return nil, err
		}
		if err := w.uci.Commit("wireless"); err != nil {
			return nil, fmt.Errorf("committing wireless: %w", err)
		}
		return w.stageWirelessApply()
	})
}
