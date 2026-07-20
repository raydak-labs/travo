package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

// Repeater options, mode switching, repeater/AP radio layout, and WiFi health.

func (w *WifiService) loadRepeaterOptions() (models.RepeaterOptions, error) {
	if w.repeaterOptionsFile == "" {
		return models.RepeaterOptions{}, nil
	}
	data, err := os.ReadFile(w.repeaterOptionsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return models.RepeaterOptions{AllowAPOnSTARadio: false}, nil
		}
		return models.RepeaterOptions{}, err
	}
	var o models.RepeaterOptions
	if err := json.Unmarshal(data, &o); err != nil {
		return models.RepeaterOptions{}, fmt.Errorf("repeater options: %w", err)
	}
	return o, nil
}

// GetRepeaterOptions returns persisted repeater preferences (missing file => allow_ap_on_sta_radio false).
func (w *WifiService) GetRepeaterOptions() (models.RepeaterOptions, error) {
	return w.loadRepeaterOptions()
}

// SetRepeaterOptions persists repeater-options.json. When allow_ap_on_sta_radio
// transitions from true to false in repeater mode on multi-radio hardware, it
// re-applies STA/AP separation (same as ReconcileRepeaterAPLayout).
func (w *WifiService) SetRepeaterOptions(o models.RepeaterOptions) (*WirelessApplyResult, error) {
	if w.repeaterOptionsFile == "" {
		return nil, nil
	}
	prev, err := w.loadRepeaterOptions()
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(w.repeaterOptionsFile)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, err
	}
	b, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return nil, err
	}
	tmp := w.repeaterOptionsFile + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, w.repeaterOptionsFile); err != nil {
		return nil, err
	}
	if w.deriveWifiMode() != "repeater" {
		return nil, nil
	}
	if !prev.AllowAPOnSTARadio || o.AllowAPOnSTARadio {
		return nil, nil
	}
	radios, err := w.getWifiRadioNames()
	if err != nil {
		return nil, err
	}
	if len(radios) < 2 {
		return nil, nil
	}
	return w.ReconcileRepeaterAPLayout()
}

func (w *WifiService) repeaterAllowAPOnSTARadio(multiRadio bool) bool {
	if !multiRadio {
		return true
	}
	o, err := w.loadRepeaterOptions()
	if err != nil {
		return false
	}
	return o.AllowAPOnSTARadio
}

// applyRepeaterDownlinkAPPolicy enables/disables AP wifi-iface sections for repeater (and ap/client)
// mode transitions. When apOnOtherRadio && !allowSTAAP, AP sections on staRadio are disabled so
// downlink stays off the STA PHY.
func (w *WifiService) applyRepeaterDownlinkAPPolicy(
	apSections []string,
	staRadio string,
	apOnOtherRadio bool,
	allowSTAAP bool,
	enableAP bool,
) error {
	for _, section := range apSections {
		apDisabled := !enableAP
		if enableAP && apOnOtherRadio && !allowSTAAP {
			opts, _ := w.uci.GetAll("wireless", section)
			if opts["device"] == staRadio {
				apDisabled = true
			}
		}
		if err := w.setIfaceDisabled(section, apDisabled); err != nil {
			return err
		}
		if !apDisabled {
			if err := w.ensureSectionRadioEnabled(section); err != nil {
				return err
			}
		}
	}
	return nil
}

// reconcileRepeaterAPRadioLayout re-applies STA/AP radio separation in repeater mode after AP
// credential or enabled mutations (e.g. unified SSID save) so uplink PHY APs are not left on.
func (w *WifiService) reconcileRepeaterAPRadioLayout() error {
	if w.deriveWifiMode() != "repeater" {
		return nil
	}
	apSections, err := w.getWifiSectionsByMode("ap")
	if err != nil {
		return err
	}
	activeSTA, err := w.selectActiveSTA()
	if err != nil {
		return err
	}
	staRadio := ""
	if activeSTA != "" {
		if opts, err := w.uci.GetAll("wireless", activeSTA); err == nil {
			staRadio = opts["device"]
		}
	}
	radios, err := w.getWifiRadioNames()
	if err != nil {
		return err
	}
	multiRadio := len(radios) >= 2
	apOnOtherRadio := func() bool {
		if !multiRadio || staRadio == "" {
			return false
		}
		for _, section := range apSections {
			opts, err := w.uci.GetAll("wireless", section)
			if err != nil {
				continue
			}
			if opts["device"] != "" && opts["device"] != staRadio {
				return true
			}
		}
		return false
	}()
	allowSTAAP := w.repeaterAllowAPOnSTARadio(multiRadio)
	return w.applyRepeaterDownlinkAPPolicy(apSections, staRadio, apOnOtherRadio, allowSTAAP, true)
}

// SetMode sets the app-level WiFi operating mode by enabling/disabling STA and AP sections.
// Uses OpenWRT's apply+confirm flow for safety: if the device crashes or becomes unreachable,
// the rollback timer (30 seconds) will automatically revert to the previous configuration.
// The user's browser polls to confirm the router is still reachable; if confirm succeeds,
// the rollback is cancelled. This prevents soft-brick scenarios without needing a separate
// guard file (which is only required for background tasks that run without user oversight).
func (w *WifiService) SetMode(mode string) (*WirelessApplyResult, error) {
	validModes := map[string]bool{"ap": true, "client": true, "repeater": true}
	if !validModes[mode] {
		return nil, fmt.Errorf("unsupported wifi mode %q", mode)
	}

	apSections, err := w.getWifiSectionsByMode("ap")
	if err != nil {
		return nil, err
	}
	staSections, err := w.getWifiSectionsByMode("sta")
	if err != nil {
		return nil, err
	}

	enableAP := false
	enableSTA := false
	switch mode {
	case "ap":
		enableAP = true
	case "client":
		enableSTA = true
	case "repeater":
		enableAP = true
		enableSTA = true
	}

	if enableSTA && len(staSections) == 0 {
		section, err := w.ensureSTASectionForScan()
		if err != nil {
			return nil, err
		}
		staSections = append(staSections, section)
	}

	// At most one STA may be enabled at a time: two STA wifi-iface sections pointing at
	// network=wwan race for the interface in netifd, and the losing binding leaves the
	// actually-connected STA without DHCP. Pick the single preferred profile and disable the rest.
	var activeSTA string
	if enableSTA {
		var err error
		activeSTA, err = w.selectActiveSTA()
		if err != nil {
			return nil, err
		}
	}

	// In repeater mode, separate STA and AP onto different radios when possible.
	// On ath11k/IPQ6018, an AP sharing a radio with a STA is forced to follow the STA's
	// channel and cannot start until the STA associates — a failing STA takes the AP with it.
	// With only one radio, coexistence is unavoidable, so we allow it.
	staRadio := ""
	if mode == "repeater" && activeSTA != "" {
		if opts, err := w.uci.GetAll("wireless", activeSTA); err == nil {
			staRadio = opts["device"]
		}
	}
	radios, err := w.getWifiRadioNames()
	if err != nil {
		return nil, err
	}
	multiRadio := len(radios) >= 2
	apOnOtherRadio := func() bool {
		if !multiRadio || staRadio == "" {
			return false
		}
		for _, section := range apSections {
			opts, err := w.uci.GetAll("wireless", section)
			if err != nil {
				continue
			}
			if opts["device"] != "" && opts["device"] != staRadio {
				return true
			}
		}
		return false
	}()

	allowSTAAP := w.repeaterAllowAPOnSTARadio(multiRadio)

	if err := w.applyRepeaterDownlinkAPPolicy(apSections, staRadio, apOnOtherRadio, allowSTAAP, enableAP); err != nil {
		return nil, err
	}
	for _, section := range staSections {
		enable := enableSTA && section == activeSTA
		if err := w.setIfaceDisabled(section, !enable); err != nil {
			return nil, err
		}
		if enable {
			if err := w.ensureSectionRadioEnabled(section); err != nil {
				return nil, err
			}
		}
	}
	if err := w.uci.Commit("wireless"); err != nil {
		return nil, err
	}
	if w.modeFile != "" {
		if err := os.MkdirAll(filepath.Dir(w.modeFile), 0750); err == nil {
			_ = os.WriteFile(w.modeFile, []byte(mode), 0600)
		}
	}
	return w.stageWirelessApply()
}

// ReconcileRepeaterAPLayout re-applies STA/AP radio separation (repeater mode only).
func (w *WifiService) ReconcileRepeaterAPLayout() (*WirelessApplyResult, error) {
	if w.deriveWifiMode() != "repeater" {
		return nil, fmt.Errorf("repeater radio reconcile only applies in repeater mode")
	}
	if err := w.reconcileRepeaterAPRadioLayout(); err != nil {
		return nil, err
	}
	if err := w.uci.Commit("wireless"); err != nil {
		return nil, fmt.Errorf("committing wireless: %w", err)
	}
	return w.stageWirelessApply()
}

// sameRadioRepeaterAPSTAConflict reports whether an enabled AP shares the STA radio while
// repeater split policy would disable it (multi-radio, allow_ap_on_sta off).
func (w *WifiService) sameRadioRepeaterAPSTAConflict() (bool, error) {
	if w.deriveWifiMode() != "repeater" {
		return false, nil
	}
	radios, err := w.getWifiRadioNames()
	if err != nil {
		return false, err
	}
	if len(radios) < 2 {
		return false, nil
	}
	if w.repeaterAllowAPOnSTARadio(true) {
		return false, nil
	}
	activeSTA, err := w.selectActiveSTA()
	if err != nil {
		return false, err
	}
	staDevice := ""
	if activeSTA != "" {
		opts, err := w.uci.GetAll("wireless", activeSTA)
		if err != nil {
			return false, err
		}
		staDevice = opts["device"]
	}
	if staDevice == "" {
		return false, nil
	}
	apSections, err := w.getWifiSectionsByMode("ap")
	if err != nil {
		return false, err
	}
	for _, section := range apSections {
		opts, err := w.uci.GetAll("wireless", section)
		if err != nil {
			continue
		}
		if opts["disabled"] == "1" {
			continue
		}
		if opts["device"] == staDevice {
			return true, nil
		}
	}
	return false, nil
}

// GetHealth returns a cross-checked view of the WiFi state to surface mismatches
// between iwinfo (association state) and netifd (wwan lease/binding). The classic
// failure mode it detects: a STA interface is associated to an SSID, but the wwan
// logical interface has been bound to a different device by netifd — so no DHCP
// client runs on the actually-connected STA and the router has no WAN. The
// existing WifiConnection endpoint reports SSID from iwinfo and IP from wwan
// separately, which is exactly why the broken state looked "connected".
func (w *WifiService) GetHealth() (models.WifiHealth, error) {
	h := models.WifiHealth{Status: "ok", Issues: []string{}}

	// Skip if no STA section is enabled (pure AP mode).
	sections, err := w.uci.GetSections("wireless")
	if err != nil {
		return h, err
	}
	hasEnabledSTA := false
	for _, opts := range sections {
		if opts["mode"] == "sta" && opts["disabled"] != "1" {
			hasEnabledSTA = true
			break
		}
	}
	if !hasEnabledSTA {
		return h, nil
	}

	// Find the runtime STA ifname and associated SSID via iwinfo.
	staIfname, _, _ := w.findSTADevice()
	var staSSID string
	staAssociated := false
	if staIfname != "" {
		if resp, err := w.ubus.Call("iwinfo", "info", map[string]interface{}{"device": staIfname}); err == nil {
			staSSID, _ = resp["ssid"].(string)
			staAssociated = staSSID != ""
		}
	}
	h.STA = &struct {
		Ifname     string `json:"ifname"`
		SSID       string `json:"ssid"`
		Associated bool   `json:"associated"`
	}{Ifname: staIfname, SSID: staSSID, Associated: staAssociated}

	// Read wwan interface state from netifd.
	wwanDevice := ""
	wwanUp := false
	wwanIP := ""
	if data, err := w.ubus.Call("network.interface.wwan", "status", nil); err == nil && data != nil {
		if u, ok := data["up"].(bool); ok {
			wwanUp = u
		}
		if d, _ := data["device"].(string); d != "" {
			wwanDevice = d
		} else if d, _ := data["l3_device"].(string); d != "" {
			wwanDevice = d
		}
		if addrs, ok := data["ipv4-address"].([]interface{}); ok && len(addrs) > 0 {
			if a, ok := addrs[0].(map[string]interface{}); ok {
				wwanIP, _ = a["address"].(string)
			}
		}
	}
	h.Wwan = &struct {
		Device    string `json:"device"`
		Up        bool   `json:"up"`
		IPAddress string `json:"ip_address"`
	}{Device: wwanDevice, Up: wwanUp, IPAddress: wwanIP}

	// Classify.
	switch {
	case staAssociated && wwanDevice != "" && staIfname != "" && wwanDevice != staIfname:
		h.Status = "error"
		h.Issues = append(h.Issues, fmt.Sprintf(
			"STA is associated on %s but wwan is bound to %s — reconcile by reconnecting to the intended network",
			staIfname, wwanDevice))
	case staAssociated && (!wwanUp || wwanIP == ""):
		h.Status = "warning"
		h.Issues = append(h.Issues, fmt.Sprintf(
			"STA is associated to %q but wwan has no DHCP lease yet", staSSID))
	case !staAssociated:
		h.Status = "warning"
		h.Issues = append(h.Issues, "STA enabled but not associated to any network")
	}

	conflict, err := w.sameRadioRepeaterAPSTAConflict()
	if err != nil {
		return h, err
	}
	if conflict {
		h.RepeaterSameRadioAPSTA = true
		h.Issues = append(h.Issues,
			"Repeater: Wi‑Fi uplink (STA) and an access point are on the same radio — use the other radio for downlink AP, or enable “Wi‑Fi on uplink radio” in repeater options.")
		if h.Status == "ok" {
			h.Status = "warning"
		}
	}
	return h, nil
}
