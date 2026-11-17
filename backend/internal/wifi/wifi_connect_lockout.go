package wifi

import (
	"errors"
	"fmt"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/network"
)

// ConnectWithLockout applies the caller-specific AP guard before Connect writes
// anything. Connect remains the background/test entrypoint with no caller identity.
func (w *Service) ConnectWithLockout(
	config models.WifiConfig, req LockoutRequest,
) (*WirelessApplyResult, error) {
	if err := w.validateNewConnectProfile(config); err != nil {
		return nil, err
	}
	if err := w.guardConnectLockout(config, req); err != nil {
		return nil, err
	}
	return w.Connect(config)
}

// validateNewConnectProfile keeps input errors ahead of the lockout guard, so a
// malformed request cannot be mistaken for an acknowledged lockout decision.
func (w *Service) validateNewConnectProfile(config models.WifiConfig) error {
	_, err := w.findSTASectionBySSID(config.SSID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, ErrNoSTASection) {
		return err
	}
	encryption := strings.TrimSpace(config.Encryption)
	if encryption == "" {
		return ErrEncryptionRequiredForNewSTA
	}
	if encryption != "none" && strings.TrimSpace(config.Password) == "" {
		return ErrPasswordRequiredForNewSTA
	}
	return nil
}

// guardConnectLockout covers the case the last-AP rule cannot: another radio's
// AP survives, but the AP carrying this caller is the one Connect disables.
func (w *Service) guardConnectLockout(config models.WifiConfig, req LockoutRequest) error {
	if req.AcknowledgeLockout || req.ClientIP == "" {
		return nil
	}
	radio, err := w.connectUplinkRadio(config)
	if err != nil {
		return err
	}
	sections, err := w.uci.GetSections("wireless")
	if err != nil {
		return fmt.Errorf("connect lockout guard could not read wireless config: %w", err)
	}
	radioCount := 0
	selectedRadioHasAP := false
	for _, opts := range sections {
		if opts["type"] != "" {
			radioCount++
		}
		if opts["mode"] == "ap" && opts["disabled"] != "1" && opts["device"] == radio {
			selectedRadioHasAP = true
		}
	}
	if radioCount <= 1 || !selectedRadioHasAP {
		return nil
	}
	options, err := w.loadRepeaterOptions()
	if err != nil {
		return fmt.Errorf("connect lockout guard could not read repeater options: %w", err)
	}
	if options.AllowAPOnSTARadio {
		return nil
	}
	// Preserve the original last-AP guard, including its acknowledgement shape.
	if err := w.guardLockoutExcluding(req, "", radio); err != nil {
		return err
	}

	apIfname, complete := network.ClientAccessPoint(w.classifyDeps(), req.ClientIP)
	if apIfname == "" {
		caller := network.ClassifyClientConnection(w.classifyDeps(), req.ClientIP)
		if caller.Method == "ethernet" && caller.Interface != "br-lan" && caller.Interface != "lan" {
			return nil
		}
		if complete && caller.Method == "ethernet" {
			return nil
		}
		// An unclassified caller may be on the AP being disabled. Refusal costs
		// one acknowledgement; allowing it can disconnect their only management path.
		return ErrLockoutRefused
	}
	callerRadio, err := w.radioForWirelessInterface(apIfname)
	if err != nil {
		return err
	}
	if callerRadio == radio {
		return ErrLockoutRefused
	}
	return nil
}

// connectUplinkRadio resolves the same radio Connect will select, without writing UCI.
func (w *Service) connectUplinkRadio(config models.WifiConfig) (string, error) {
	if config.Band != "" {
		return w.getRadioForBand(config.Band)
	}
	section, err := w.findSTASectionBySSID(config.SSID)
	if err == nil {
		radio, getErr := w.uci.Get("wireless", section, "device")
		if getErr != nil {
			return "", fmt.Errorf("reading saved uplink radio: %w", getErr)
		}
		return radio, nil
	}
	if !errors.Is(err, ErrNoSTASection) {
		return "", err
	}
	sections, err := w.uci.GetSections("wireless")
	if err != nil {
		return "", fmt.Errorf("connect lockout guard could not list radios: %w", err)
	}
	return w.radioForNewSTA(sections)
}

func (w *Service) radioForWirelessInterface(ifname string) (string, error) {
	status, err := w.ubus.Call("network.wireless", "status", nil)
	if err != nil {
		return "", fmt.Errorf("connect lockout guard could not read wireless status: %w", err)
	}
	for radioName, rawRadio := range status {
		radio, ok := rawRadio.(map[string]any)
		if !ok {
			continue
		}
		interfaces, _ := radio["interfaces"].([]any)
		for _, rawInterface := range interfaces {
			iface, ok := rawInterface.(map[string]any)
			if ok && iface["ifname"] == ifname {
				return radioName, nil
			}
		}
	}
	return "", fmt.Errorf("connect lockout guard could not map caller AP interface %s", ifname)
}
