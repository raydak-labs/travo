package services

import (
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/models"
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

// The guest subnet is fixed, so enabling guest WiFi means claiming
// 192.168.2.0/24 outright. On a router whose LAN is already on that subnet the
// claim would create a second interface on one network — DHCP handing out
// addresses the LAN already routes — so the request is refused instead.
// ErrGuestSubnetOverlap is returned so the API can map it to a 4xx.
var ErrGuestSubnetOverlap = errors.New("refusing to create the guest network: its subnet " +
	"overlaps an existing network interface on this router")

const (
	guestNetwork = "guest"
	guestIPAddr  = "192.168.2.1"
	guestNetmask = "255.255.255.0"
)

// ifaceSections returns the wifi-iface sections of one mode that are bound to a
// radio, sorted by section name.
//
// The sort is the point: callers that pick a radio out of such a list have to
// make the same pick on every call. Go randomises map iteration, so an unsorted
// list silently turned "the first radio with this band" into a coin toss.
func ifaceSections(sections map[string]map[string]string, mode string) []string {
	var names []string
	for name, opts := range sections {
		if opts["mode"] != mode || opts["device"] == "" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// activeIfaces narrows ifaceSections to the ones the config leaves enabled.
func activeIfaces(sections map[string]map[string]string, mode string) []string {
	var names []string
	for _, name := range ifaceSections(sections, mode) {
		if sections[name]["disabled"] != "1" {
			names = append(names, name)
		}
	}
	return names
}

// uplinkRadio returns the radio carrying the WiFi uplink, or "" when no WiFi
// client interface is enabled.
//
// This is the single definition of "the uplink radio" for this package: ANY
// enabled mode=sta wifi-iface counts, not only one bound to network=wwan.
// network=wwan is what makes a STA the routed internet uplink, and a wwan STA
// wins when several radios qualify — but a hand-written client interface without
// it still runs on that PHY, so it must occupy the radio for every guard. The
// looser definition (wwan only) let the same-radio guards see an empty uplink
// while the health API, which counts every enabled STA, reported the radio as
// running "both": the API declared safe the state the guard had just refused.
// ErrAPAndSTASameRadio exists because the PHY constraint is physical, so the
// strict direction — more radios count as the uplink — is the safe one.
func (w *WifiService) uplinkRadio() (string, error) {
	sections, err := w.uci.GetSections("wireless")
	if err != nil {
		return "", err
	}
	names := activeIfaces(sections, "sta")
	for _, name := range names {
		if sections[name]["network"] == "wwan" {
			return sections[name]["device"], nil
		}
	}
	if len(names) == 0 {
		return "", nil
	}
	return sections[names[0]]["device"], nil
}

// preferredGuestRadio picks the radio the guest access point goes on: the
// 2.4 GHz one when it is free, otherwise the best radio that does NOT carry the
// enabled WiFi uplink.
//
// Skipping the uplink radio is what keeps guest WiFi reachable at all on the
// common travel-router layout — uplink STA on 2.4 GHz — instead of answering a
// refusal whose only escape is allow_ap_on_sta_radio, i.e. deliberately
// re-enabling the crash state the guard exists to prevent. It is the same split
// applyRepeaterDownlinkAPPolicy makes for the downlink AP: the guest AP and the
// uplink get separate PHYs. A guest network on 5 GHz is slower but usable; a
// refusal is not.
//
// When no alternative exists (single-radio hardware) the uplink radio is
// returned anyway and rejectAPOnUplinkRadio decides whether coexistence stands —
// the same trade-off SetMode("repeater") and SetRadioRole already accept.
func (w *WifiService) preferredGuestRadio() (string, error) {
	radios, err := w.getWifiRadioNames()
	if err != nil {
		return "", err
	}
	if len(radios) == 0 {
		return "", fmt.Errorf("no radio found for guest wifi")
	}
	uplinkRadio, err := w.uplinkRadio()
	if err != nil {
		return "", err
	}
	firstFree := ""
	for _, radio := range radios {
		if radio == uplinkRadio {
			continue
		}
		opts, _ := w.uci.GetAll("wireless", radio)
		if firstFree == "" {
			firstFree = radio
		}
		if opts["band"] == "2g" {
			return radio, nil
		}
	}
	if firstFree != "" {
		return firstFree, nil
	}
	return radios[0], nil
}

// GetRadioStatus returns whether any WiFi radio is enabled.
func (w *WifiService) GetRadioStatus() (bool, error) {
	radios, err := w.getWifiRadioNames()
	if err != nil {
		return false, err
	}
	for _, radio := range radios {
		opts, _ := w.uci.GetAll("wireless", radio)
		if opts["disabled"] != "1" {
			return true, nil
		}
	}
	return false, nil
}

// SetRadioEnabled enables or disables all WiFi radios.
func (w *WifiService) SetRadioEnabled(enabled bool) (*WirelessApplyResult, error) {
	return w.mutateWireless([]string{"wireless"}, func() (*WirelessApplyResult, error) {
		value := "0"
		if !enabled {
			value = "1"
		}
		radios, err := w.getWifiRadioNames()
		if err != nil {
			return nil, err
		}
		for _, radio := range radios {
			if err := w.uci.Set("wireless", radio, "disabled", value); err != nil {
				return nil, err
			}
		}
		if err := w.uci.Commit("wireless"); err != nil {
			return nil, err
		}
		return w.stageWirelessApply()
	})
}

// GetRadios returns information about all WiFi radio hardware.
func (w *WifiService) GetRadios() ([]models.RadioInfo, error) {
	sections, err := w.uci.GetSections("wireless")
	if err != nil {
		// Reporting "no radios" on a failed read would tell the operator the
		// hardware vanished instead of surfacing the read failure.
		return nil, fmt.Errorf("reading wireless sections: %w", err)
	}
	// Build role map: for each radio name, detect active AP/STA ifaces.
	// activeIfaces is the same predicate the same-radio guards read, so the
	// role reported here can never contradict what the guards refuse.
	type roleFlags struct{ ap, sta bool }
	roles := map[string]roleFlags{}
	for _, name := range activeIfaces(sections, "ap") {
		roles[sections[name]["device"]] = roleFlags{ap: true}
	}
	for _, name := range activeIfaces(sections, "sta") {
		rf := roles[sections[name]["device"]]
		rf.sta = true
		roles[sections[name]["device"]] = rf
	}
	radios := make([]models.RadioInfo, 0, len(sections))
	names := make([]string, 0, len(sections))
	for name, opts := range sections {
		// wifi-device sections have a "type" option (e.g. "mac80211")
		if opts["type"] == "" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names) // band switching picks a radio out of this list
	for _, name := range names {
		opts := sections[name]
		channel := 0
		if ch, ok := opts["channel"]; ok {
			if v, err := strconv.Atoi(ch); err == nil {
				channel = v
			}
		}
		rf := roles[name]
		role := "none"
		switch {
		case rf.ap && rf.sta:
			role = "both"
		case rf.ap:
			role = "ap"
		case rf.sta:
			role = "sta"
		}
		radios = append(radios, models.RadioInfo{
			Name:     name,
			Band:     opts["band"],
			Channel:  channel,
			HTMode:   opts["htmode"],
			Type:     opts["type"],
			Disabled: opts["disabled"] == "1",
			Role:     role,
		})
	}
	return radios, nil
}

// SetRadioRole assigns a role (ap/sta/both/none) to a specific radio.
// It enables/disables existing iface sections and creates them if needed.
//
// Role "both" is refused with ErrAPAndSTASameRadio when it would put an access
// point and the uplink STA on one radio and the repeater split policy forbids
// it (see rejectSameRadioAPSTA).
func (w *WifiService) SetRadioRole(radioName, role string) (*WirelessApplyResult, error) {
	return w.mutateWireless([]string{"wireless", "network", "firewall"}, func() (*WirelessApplyResult, error) {
		// abort drops the staged wireless delta before surfacing a failure: the uci
		// CLI delta is process-global, so an abandoned write would be committed by a
		// later, unrelated `uci commit wireless`.
		switch role {
		case "ap", "sta", "both", "none":
		default:
			return nil, fmt.Errorf("invalid role %q: must be ap, sta, both, or none", role)
		}
		// Role "ap" is deliberately not policed by rejectSameRadioAPSTA, and does
		// not need to be: asking for an access point on the uplink radio also
		// disables that radio's STA sections below, so the end state is AP-only
		// and never the AP+STA-on-one-PHY state ADR 0002 §2 warns about. Refusing
		// instead would break the ordinary "make this radio an AP" action the
		// operator asked for — losing the uplink is the consequence of their
		// choice, not something to second-guess here. Pinned by
		// TestSetRadioRole_APRoleOnUplinkRadioLeavesAPOnly and
		// TestSetRadioRole_APRoleOnUplinkRadioCreatesAPOnly.
		enableAP := role == "ap" || role == "both"
		enableSTA := role == "sta" || role == "both"
		// Role "both" is the only role that puts an access point and the uplink
		// STA on one PHY, so it is the only one the repeater split policy has to
		// police here. validateWirelessConsistency (below) is radio-blind — it
		// counts only active mode=sta/network=wwan sections — so without this the
		// request would commit and apply AP+STA on a single radio and bypass
		// allow_ap_on_sta_radio entirely. See rejectSameRadioAPSTA for why this
		// refuses instead of silently moving the downlink AP to the other radio.
		//
		// The refusal runs FIRST, before any write: it runs before
		// ensureWwanNetwork, which COMMITS network.wwan and adds wwan to the wan
		// firewall zone. A commit cannot be undone by revertUCIConfig, so a
		// request answered as refused would otherwise still have created an
		// interface and a firewall change nobody asked for.
		if err := w.rejectSameRadioAPSTA(radioName, enableAP, enableSTA); err != nil {
			return nil, err
		}
		// Set when this call had to invent a WPA key for a default AP.
		var generatedKey string
		sections, err := w.uci.GetSections("wireless")
		if err != nil {
			return nil, err
		}
		// Collect existing AP and STA sections for this radio.
		var apSections, staSections []string
		for name, opts := range sections {
			if opts["device"] != radioName {
				continue
			}
			switch opts["mode"] {
			case "ap":
				apSections = append(apSections, name)
			case "sta":
				staSections = append(staSections, name)
			}
		}
		// A radio that is about to gain an STA section must not leave another saved
		// STA enabled: two active wifi-ifaces bound to network=wwan is the
		// inconsistent config that rpcd rollback cannot repair.
		newSTASection := ""
		if enableSTA && len(staSections) == 0 {
			newSTASection = "sta_" + radioName
		}
		// Only when a new STA section is created: this radio gains a second active
		// wwan client otherwise. Other STA profiles are disabled, not deleted, so
		// saved networks survive.
		if newSTASection != "" {
			if err := w.disableOtherSTASections(newSTASection); err != nil {
				return nil, err
			}
		}
		// Handle AP sections.
		if enableAP && len(apSections) == 0 {
			apName := "ap_" + radioName
			if err := w.uci.AddSection("wireless", apName, "wifi-iface"); err != nil {
				return nil, fmt.Errorf("creating AP section: %w", err)
			}
			// A shipped default passphrase is a public credential: every unit
			// running this firmware would expose the same open-ish network. Generate
			// a random WPA2 key and report it back so the operator can use it.
			key, err := generateRandomWPAKey()
			if err != nil {
				return nil, fmt.Errorf("generating default AP key: %w", err)
			}
			generatedKey = key
			if err := w.uci.Set("wireless", apName, "device", radioName); err != nil {
				return nil, err
			}
			if err := w.uci.Set("wireless", apName, "mode", "ap"); err != nil {
				return nil, err
			}
			if err := w.uci.Set("wireless", apName, "ssid", "OpenWRT"); err != nil {
				return nil, err
			}
			if err := w.uci.Set("wireless", apName, "encryption", "psk2"); err != nil {
				return nil, err
			}
			if err := w.uci.Set("wireless", apName, "key", key); err != nil {
				return nil, err
			}
			if err := w.uci.Set("wireless", apName, "network", "lan"); err != nil {
				return nil, err
			}
			apSections = append(apSections, apName)
		}
		for _, section := range apSections {
			if err := w.setIfaceDisabled(section, !enableAP); err != nil {
				return nil, err
			}
			if enableAP {
				if err := w.ensureSectionRadioEnabled(section); err != nil {
					return nil, err
				}
			}
		}
		// Handle STA sections.
		if enableSTA && len(staSections) == 0 {
			if err := w.ensureWwanNetwork(); err != nil {
				return nil, fmt.Errorf("ensuring wwan network: %w", err)
			}
			staName := newSTASection
			if err := w.uci.AddSection("wireless", staName, "wifi-iface"); err != nil {
				return nil, fmt.Errorf("creating STA section: %w", err)
			}
			if err := w.uci.Set("wireless", staName, "device", radioName); err != nil {
				return nil, err
			}
			if err := w.uci.Set("wireless", staName, "mode", "sta"); err != nil {
				return nil, err
			}
			if err := w.uci.Set("wireless", staName, "network", "wwan"); err != nil {
				return nil, err
			}
			if err := w.uci.Set("wireless", staName, "ssid", ""); err != nil {
				return nil, err
			}
			if err := w.uci.Set("wireless", staName, "disabled", "0"); err != nil {
				return nil, err
			}
			staSections = append(staSections, staName)
		}
		for _, section := range staSections {
			if err := w.setIfaceDisabled(section, !enableSTA); err != nil {
				return nil, err
			}
			if enableSTA {
				if err := w.ensureSectionRadioEnabled(section); err != nil {
					return nil, err
				}
			}
		}
		// Validate BEFORE Commit: stageWirelessApply would otherwise run the same
		// check after the bad config is already committed, and the rpcd rollback it
		// triggers would restore the very same broken config.
		if err := w.validateWirelessConsistency(); err != nil {
			return nil, err
		}
		if err := w.uci.Commit("wireless"); err != nil {
			return nil, err
		}
		apply, err := w.stageWirelessApply()
		if err != nil {
			return nil, err
		}
		if apply != nil {
			apply.GeneratedKey = generatedKey
		}
		return apply, nil
	})
}

// rejectSameRadioAPSTA refuses a radio role that would leave an enabled access
// point and the uplink STA on the same PHY when the repeater split policy
// forbids it. It runs BEFORE uci.Commit("wireless"), so the refused write never
// reaches the running config — an rpcd rollback could not undo it, because the
// rollback would restore the very same broken state.
//
// Why a refusal and not the repeater reconcile (reconcileRepeaterAPRadioLayout,
// which ADR 0002 §2 mandates for functions that activate a STA):
//
//   - The reconcile is a whole-config operation. applyRepeaterDownlinkAPPolicy
//     runs with enableAP=true, so it re-enables access points on the OTHER radios.
//     SetRadioRole is a per-radio request, and the operator may just have set that
//     other radio to "none" or "ap". Reconciling here would silently undo it.
//   - Reconciling would not give the operator what they asked for anyway: the
//     radio they marked "both" would come back as STA-only (or AP-only), with no
//     error, and the split AP would appear on a radio they did not name.
//   - The layout they actually want is reachable without ambiguity: give the STA
//     its own radio and the downlink AP the other one (SetMode("repeater") plus
//     per-radio roles), or flip the documented escape hatch.
//
// Single-radio hardware keeps coexistence: with one PHY there is no split to
// make, which is the same trade-off SetMode("repeater") already accepts.
func (w *WifiService) rejectSameRadioAPSTA(radioName string, enableAP, enableSTA bool) error {
	if !enableAP || !enableSTA {
		return nil
	}
	radios, err := w.getWifiRadioNames()
	if err != nil {
		return err
	}
	if len(radios) < 2 {
		return nil
	}
	// allow_ap_on_sta_radio is the operator's explicit "I accept AP+STA on one
	// radio" switch; when it is set, the request is honoured as asked.
	if w.repeaterAllowAPOnSTARadio(true) {
		return nil
	}
	return fmt.Errorf("%w: role 'both' requested on %s", ErrAPAndSTASameRadio, radioName)
}

// rejectAPOnUplinkRadio refuses enabling an access point on the radio that
// carries the enabled WiFi uplink STA. It is the same physical constraint
// rejectSameRadioAPSTA polices for SetRadioRole, expressed the other way round:
// here the uplink already exists and the caller is adding the AP, so the
// decision is "is this radio the uplink's radio" rather than "does this role ask
// for both".
//
// Every writer that creates or enables an AP section has to go through it. The
// uplink STA on radio0 with an AP enabled on radio0 is the AP+STA-on-one-PHY
// state ADR 0002 §2 says is enough to crash ath11k/IPQ6018, and reconcileRepeater-
// APRadioLayout is no defence outside repeater mode: there it does nothing at
// all, so SetGuestWifi and SetAPConfig reached the crash state without ever
// passing the refusal SetRadioRole applies.
//
// reason names the request in the error (e.g. "guest access point enabled"), so
// the operator can see which of their two radios is the problem.
func (w *WifiService) rejectAPOnUplinkRadio(radioName, reason string) error {
	uplinkRadio, err := w.uplinkRadio()
	if err != nil {
		return err
	}
	if uplinkRadio == "" || uplinkRadio != radioName {
		return nil
	}
	radios, err := w.getWifiRadioNames()
	if err != nil {
		return err
	}
	// Single-radio hardware has no split to make; the same trade-off
	// SetMode("repeater") already accepts.
	if len(radios) < 2 {
		return nil
	}
	if w.repeaterAllowAPOnSTARadio(true) {
		return nil
	}
	return fmt.Errorf("%w: %s on %s, which carries the WiFi uplink",
		ErrAPAndSTASameRadio, reason, radioName)
}

// splitAPOffUplinkRadio keeps the radio an uplink STA is about to be enabled on
// from also running an access point.
//
// It is the same split reconcileRepeaterAPRadioLayout performs, applied for
// every mode. Connect used to rely on that reconcile alone, and it returns
// immediately outside repeater mode — so on a device in client or ap mode
// nothing stood between the uplink and an enabled AP on the same radio, which
// is the state ADR 0002 §2 says is enough to crash ath11k/IPQ6018.
//
// Two outcomes, both deliberate:
//
//   - Another radio hosts an access point: the APs on the uplink radio are
//     disabled and the downlink moves to the other band, exactly what
//     SetMode("repeater") already does. The uplink is what the operator asked
//     for; the downlink follows.
//   - No other radio hosts one: the request is refused. Disabling the only
//     access point to free a radio would silently take the router's WiFi away,
//     which is not what a "connect" request means. allow_ap_on_sta_radio is the
//     documented escape.
//
// Single-radio hardware keeps coexistence: there is no split to make, which is
// the trade-off SetMode("repeater") already accepts.
func (w *WifiService) splitAPOffUplinkRadio(radioName string) error {
	if radioName == "" {
		return nil
	}
	radios, err := w.getWifiRadioNames()
	if err != nil {
		return err
	}
	if len(radios) < 2 {
		return nil
	}
	apSections, err := w.getWifiSectionsByMode("ap")
	if err != nil {
		return err
	}
	apOnTarget, apOnOtherRadio := false, false
	for _, section := range apSections {
		opts, err := w.uci.GetAll("wireless", section)
		if err != nil || opts["disabled"] == "1" || opts["device"] == "" {
			continue
		}
		if opts["device"] == radioName {
			apOnTarget = true
		} else {
			apOnOtherRadio = true
		}
	}
	if !apOnTarget {
		return nil
	}
	allowSTAAP := w.repeaterAllowAPOnSTARadio(true)
	if !allowSTAAP && !apOnOtherRadio {
		return fmt.Errorf("%w: WiFi uplink connected on %s, which is the only radio "+
			"running an access point", ErrAPAndSTASameRadio, radioName)
	}
	return w.applyRepeaterDownlinkAPPolicy(apSections, radioName, apOnOtherRadio, allowSTAAP, true)
}

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
func (w *WifiService) SetAPConfig(section string, update models.APConfigUpdate) (*WirelessApplyResult, error) {
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
			disabled := boolToEnabled(!*update.Enabled)
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

// GetGuestWifi returns the guest WiFi configuration.
func (w *WifiService) GetGuestWifi() (*models.GuestWifiConfig, error) {
	opts, err := w.uci.GetAll("wireless", "guest")
	if err != nil {
		return &models.GuestWifiConfig{Enabled: false}, nil
	}
	return &models.GuestWifiConfig{
		Enabled:    opts["disabled"] != "1",
		SSID:       opts["ssid"],
		Encryption: opts["encryption"],
		Key:        opts["key"],
	}, nil
}

// SetGuestWifi creates or updates the guest WiFi network with full isolation.
func (w *WifiService) SetGuestWifi(cfg models.GuestWifiConfig) (*WirelessApplyResult, error) {
	return w.mutateWireless([]string{"wireless", "network", "dhcp", "firewall"}, func() (*WirelessApplyResult, error) {
		if !cfg.Enabled {
			return w.teardownGuestWifi()
		}
		// The guest subnet is fixed, so the only question is whether this
		// router already has it. Refuse first: the network, dhcp and firewall
		// commits below cannot be undone by revertUCIConfig.
		if err := w.rejectGuestSubnetOverlap(); err != nil {
			return nil, err
		}
		// Pick the radio and police it before writing anything. The refusal has to
		// come first: the network and dhcp commits below cannot be undone by
		// revertUCIConfig, so a request answered as "refused" must not already
		// have created network.guest, dhcp.guest and the guest firewall zone.
		guestRadio, err := w.preferredGuestRadio()
		if err != nil {
			return nil, err
		}
		if err := w.rejectAPOnUplinkRadio(guestRadio, "guest access point enabled"); err != nil {
			return nil, err
		}
		// Network interface for guest subnet
		if err := w.ensureNamedSection("network", "guest", "interface"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("network", "guest", "proto", "static"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("network", "guest", "ipaddr", guestIPAddr); err != nil {
			return nil, err
		}
		if err := w.uci.Set("network", "guest", "netmask", guestNetmask); err != nil {
			return nil, err
		}
		if err := w.uci.Commit("network"); err != nil {
			return nil, err
		}
		// DHCP for guest network
		if err := w.ensureNamedSection("dhcp", "guest", "dhcp"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("dhcp", "guest", "interface", "guest"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("dhcp", "guest", "start", "100"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("dhcp", "guest", "limit", "50"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("dhcp", "guest", "leasetime", "2h"); err != nil {
			return nil, err
		}
		if err := w.uci.Commit("dhcp"); err != nil {
			return nil, err
		}
		// Wireless interface for guest AP
		if err := w.ensureNamedSection("wireless", "guest", "wifi-iface"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("wireless", "guest", "device", guestRadio); err != nil {
			return nil, err
		}
		if err := w.uci.Set("wireless", "guest", "mode", "ap"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("wireless", "guest", "network", "guest"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("wireless", "guest", "ssid", cfg.SSID); err != nil {
			return nil, err
		}
		if err := w.uci.Set("wireless", "guest", "encryption", cfg.Encryption); err != nil {
			return nil, err
		}
		if err := w.uci.Set("wireless", "guest", "key", cfg.Key); err != nil {
			return nil, err
		}
		if err := w.uci.Set("wireless", "guest", "isolate", "1"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("wireless", "guest", "disabled", "0"); err != nil {
			return nil, err
		}
		if err := w.ensureSectionRadioEnabled("guest"); err != nil {
			return nil, err
		}
		if err := w.uci.Commit("wireless"); err != nil {
			return nil, err
		}
		// Firewall zone for guest
		if err := w.ensureNamedSection("firewall", "guest_zone", "zone"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("firewall", "guest_zone", "name", "guest"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("firewall", "guest_zone", "network", "guest"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("firewall", "guest_zone", "input", "REJECT"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("firewall", "guest_zone", "output", "ACCEPT"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("firewall", "guest_zone", "forward", "REJECT"); err != nil {
			return nil, err
		}
		// Forwarding: guest -> wan
		if err := w.ensureNamedSection("firewall", "guest_fwd", "forwarding"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("firewall", "guest_fwd", "src", "guest"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("firewall", "guest_fwd", "dest", "wan"); err != nil {
			return nil, err
		}
		// Allow DNS from guest
		if err := w.ensureNamedSection("firewall", "guest_dns", "rule"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("firewall", "guest_dns", "name", "Allow-Guest-DNS"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("firewall", "guest_dns", "src", "guest"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("firewall", "guest_dns", "dest_port", "53"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("firewall", "guest_dns", "target", "ACCEPT"); err != nil {
			return nil, err
		}
		// Allow DHCP from guest
		if err := w.ensureNamedSection("firewall", "guest_dhcp", "rule"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("firewall", "guest_dhcp", "name", "Allow-Guest-DHCP"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("firewall", "guest_dhcp", "src", "guest"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("firewall", "guest_dhcp", "dest_port", "67-68"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("firewall", "guest_dhcp", "target", "ACCEPT"); err != nil {
			return nil, err
		}
		if err := w.uci.Commit("firewall"); err != nil {
			return nil, err
		}
		return w.stageWirelessApply()
	})
}

// rejectGuestSubnetOverlap refuses the fixed guest subnet when an existing
// network interface already claims it. Two interfaces on one subnet is not a
// cosmetic problem: the guest DHCP scope hands out addresses the LAN already
// routes, and the guest firewall zone then applies to LAN traffic as well.
func (w *WifiService) rejectGuestSubnetOverlap() error {
	sections, err := w.uci.GetSections("network")
	if err != nil {
		return fmt.Errorf("reading network sections: %w", err)
	}
	guest := ipv4Net(guestIPAddr, guestNetmask)
	if guest == nil {
		return fmt.Errorf("guest subnet %s/%s is not a valid IPv4 network", guestIPAddr, guestNetmask)
	}
	names := make([]string, 0, len(sections))
	for name := range sections {
		names = append(names, name)
	}
	sort.Strings(names) // the error must name the same section every time
	for _, name := range names {
		if name == guestNetwork {
			continue // re-enabling the guest network itself is not an overlap
		}
		opts := sections[name]
		existing := ipv4Net(opts["ipaddr"], opts["netmask"])
		if existing == nil || !subnetsOverlap(guest, existing) {
			continue
		}
		return fmt.Errorf("%w: network.%s is %s/%s",
			ErrGuestSubnetOverlap, name, opts["ipaddr"], opts["netmask"])
	}
	return nil
}

// ipv4Net turns an address/netmask pair into its network, or nil when either is
// absent or not IPv4. Interfaces without both (DHCP clients, wireguard peers)
// are skipped: they have no static subnet to collide with.
func ipv4Net(ip, mask string) *net.IPNet {
	addr := net.ParseIP(strings.TrimSpace(ip)).To4()
	m := net.ParseIP(strings.TrimSpace(mask)).To4()
	if addr == nil || m == nil {
		return nil
	}
	netmask := net.IPMask(m)
	return &net.IPNet{IP: addr.Mask(netmask), Mask: netmask}
}

// subnetsOverlap reports whether two IPv4 networks share any address.
func subnetsOverlap(a, b *net.IPNet) bool {
	return a.Contains(b.IP) || b.Contains(a.IP)
}

// teardownGuestWifi disables the guest AP and removes the guest subnet it
// owned. Setting wireless.guest.disabled=1 alone leaves network.guest,
// dhcp.guest and the four firewall.guest_* sections in place, so the
// 192.168.2.0/24 interface, its DHCP scope and the guest zone stay up and
// continue to route and answer on a network the operator switched off.
func (w *WifiService) teardownGuestWifi() (*WirelessApplyResult, error) {
	abort := func(err error) (*WirelessApplyResult, error) {
		revertUCIConfig(w.uci, "wireless", "network", "dhcp", "firewall")
		return nil, err
	}
	if _, err := w.uci.GetAll("wireless", "guest"); err != nil {
		// Guest WiFi was never configured: nothing to tear down.
		return nil, nil
	}

	// 1. Take the AP down first so no client is left on a subnet that is about
	//    to disappear.
	if err := w.uci.Set("wireless", "guest", "disabled", "1"); err != nil {
		return abort(err)
	}
	if err := w.uci.Commit("wireless"); err != nil {
		return abort(err)
	}

	// 2. Remove the DHCP scope before the interface it is bound to.
	if err := w.deleteSectionIfPresent("dhcp", "guest"); err != nil {
		return abort(err)
	}
	if err := w.uci.Commit("dhcp"); err != nil {
		return abort(err)
	}

	// 3. Remove the guest network interface (192.168.2.0/24).
	if err := w.deleteSectionIfPresent("network", "guest"); err != nil {
		return abort(err)
	}
	if err := w.uci.Commit("network"); err != nil {
		return abort(err)
	}

	// 4. Remove the guest firewall zone and its rules/forwarding.
	for _, section := range []string{"guest_dhcp", "guest_dns", "guest_fwd", "guest_zone"} {
		if err := w.deleteSectionIfPresent("firewall", section); err != nil {
			return abort(err)
		}
	}
	if err := w.uci.Commit("firewall"); err != nil {
		return abort(err)
	}

	return w.stageWirelessApply()
}

// deleteSectionIfPresent removes config/section when it exists. A section that
// was never created is not an error: teardown must be idempotent.
func (w *WifiService) deleteSectionIfPresent(config, section string) error {
	if _, err := w.uci.GetAll(config, section); err != nil {
		return nil
	}
	if err := w.uci.DeleteSection(config, section); err != nil {
		return fmt.Errorf("removing %s.%s: %w", config, section, err)
	}
	return nil
}
