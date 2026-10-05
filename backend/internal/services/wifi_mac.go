package services

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

// MAC address management (clone, randomize, policies).

// GetMACAddresses returns the MAC address info for WiFi interfaces.
func (w *WifiService) GetMACAddresses() ([]models.MACConfig, error) {
	var configs []models.MACConfig

	staSection, err := w.findSTASection()
	if err != nil {
		return configs, nil // No STA section; return empty (not an error)
	}
	staOpts, err := w.uci.GetAll("wireless", staSection)
	if err != nil {
		return configs, nil
	}
	currentMAC := ""
	// Try reading from sysfs (ifname pattern: phy<N>-sta<N>)
	if ifname, _, sysErr := w.findSTADevice(); sysErr == nil && ifname != "" {
		if data, readErr := os.ReadFile("/sys/class/net/" + ifname + "/address"); readErr == nil {
			currentMAC = strings.TrimSpace(string(data))
		}
	}
	customMAC := staOpts["macaddr"]
	isApplied := customMAC != "" && strings.EqualFold(currentMAC, customMAC)
	configs = append(configs, models.MACConfig{
		Interface:  "sta",
		CurrentMAC: currentMAC,
		CustomMAC:  customMAC,
		IsApplied:  isApplied,
	})

	return configs, nil
}

// macGuardFeature is the crash-guard name for the MAC change sequence (ADR 0003).
const macGuardFeature = "mac"

// validMAC matches an EUI-48 address in the colon- or dash-separated notation
// used by `ip link` and netifd.
var validMAC = regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]){5}[0-9A-Fa-f]{2}$`)

// normalizeMAC canonicalizes a user-supplied MAC to lowercase colon notation,
// which is what netifd/mac80211.sh expect in wireless.<sta>.macaddr.
func normalizeMAC(mac string) (string, error) {
	mac = strings.TrimSpace(mac)
	if !validMAC.MatchString(mac) {
		return "", fmt.Errorf("invalid MAC address %q: expected six hex octets like 02:11:22:33:44:55", mac)
	}
	return strings.ToLower(strings.ReplaceAll(mac, "-", ":")), nil
}

// SetMACAddress sets a custom MAC address on the STA WiFi interface.
// The value is validated first, then written to the wireless.<sta>.macaddr UCI
// option, verified, and applied. Only after the staged apply is started does it
// touch the live link with "ip link": doing that before would take the interface
// down with a possibly invalid address and outside any crash guard.
//
// The guard is cleared only when the WHOLE synchronous sequence succeeded,
// including the live `ip link` change. A failed link step returns an error and
// keeps the guard (ADR 0003 §1.3: the marker is removed only after the operation
// completes end-to-end).
//
// This path self-confirms as far as the guard is concerned, unlike a
// browser-driven apply: stageWirelessApply calls StartApply AFTER the commit, so
// rpcd's rollback snapshot already contains the new wireless.<sta>.macaddr —
// rolling the apply back restores the same committed value, and the only thing
// the pending confirm decides is whether that value is kept. The browser is
// never the witness that the live interface actually took the MAC; the link
// step below is, and it is now error-checked.
func (w *WifiService) SetMACAddress(mac string) (*WirelessApplyResult, error) {
	return w.mutateWireless([]string{"wireless"}, func() (*WirelessApplyResult, error) {
		targetMAC := ""
		if strings.TrimSpace(mac) != "" {
			// netifd replays wireless.<sta>.macaddr on every wifi up, so an invalid
			// value committed here would keep breaking the interface across reboots.
			normalized, err := normalizeMAC(mac)
			if err != nil {
				return nil, err
			}
			targetMAC = normalized
		}
		staSection, err := w.findSTASection()
		if err != nil {
			return nil, fmt.Errorf("STA interface not found")
		}
		// Crash guard first: from here on the sequence changes live state, and a
		// crash mid-way must be recoverable on the next boot (ADR 0003).
		if err := w.writeCrashGuard(macGuardFeature); err != nil {
			return nil, err
		}
		guardStays := true
		defer func() {
			if guardStays {
				// The apply did not complete; leave the guard for recovery.
				log.Printf("WARNING: %s MAC change incomplete, crash guard kept at %s", macGuardFeature, w.guardPath(macGuardFeature))
			}
		}()
		if err := w.uci.Set("wireless", staSection, "macaddr", targetMAC); err != nil {
			return nil, fmt.Errorf("setting MAC: %w", err)
		}
		// Verify what was actually staged before committing it.
		staged, err := w.uci.Get("wireless", staSection, "macaddr")
		if err != nil {
			return nil, fmt.Errorf("verifying MAC: %w", err)
		}
		if staged != targetMAC {
			return nil, fmt.Errorf("MAC verification failed: wanted %q, wireless config holds %q", targetMAC, staged)
		}
		if err := w.uci.Commit("wireless"); err != nil {
			return nil, fmt.Errorf("committing wireless: %w", err)
		}
		apply, err := w.stageWirelessApply()
		if err != nil {
			// The config is committed but not applied: keep the guard so the next
			// boot / recovery path can settle it, and do not touch the link.
			return nil, err
		}
		// The apply is in flight; only now is it safe to change the live link.
		// The error is returned rather than logged and dropped: a caller that got
		// 200 with the STA link still down has no way to learn that it lost the
		// interface it was connected through.
		if err := w.applyMACImmediate(targetMAC); err != nil {
			return nil, err
		}
		guardStays = false
		w.clearCrashGuard(macGuardFeature)
		return apply, nil
	})
}

// applyMACImmediate applies (or restores) the MAC address on the live STA
// interface right now using ip link, without requiring a wifi restart.
//
// The sequence is down -> set address -> up, so a failure in the middle leaves
// the interface DOWN. That is the worst possible outcome here: the STA is the
// router's own upstream link, so it silently disappears behind a successful HTTP
// response and the operator loses connectivity with nothing to act on. Every
// failure path therefore restores the link and returns the error.
func (w *WifiService) applyMACImmediate(mac string) error {
	if w.cmd == nil {
		return nil
	}
	ifname, _, err := w.findSTADevice()
	if err != nil || ifname == "" {
		return nil
	}
	if mac == "" {
		// Restore hardware MAC from the phy's permanent address list.
		hwMAC := w.readPhyHardwareMAC(ifname)
		if hwMAC == "" {
			return nil // can't restore without knowing the permanent MAC
		}
		mac = hwMAC
	}
	if _, err := w.cmd.Run("ip", "link", "set", ifname, "down"); err != nil {
		return fmt.Errorf("ip link set %s down: %w", ifname, err)
	}
	if _, err := w.cmd.Run("ip", "link", "set", ifname, "address", mac); err != nil {
		// The link is down right now and would stay down if we stopped here.
		w.bringLinkUp(ifname)
		return fmt.Errorf("applying MAC %s to %s (link brought back up): %w", mac, ifname, err)
	}
	if _, err := w.cmd.Run("ip", "link", "set", ifname, "up"); err != nil {
		w.bringLinkUp(ifname)
		return fmt.Errorf("ip link set %s up: %w", ifname, err)
	}
	return nil
}

// bringLinkUp makes a best-effort attempt to return an interface to the up
// state after a failed MAC change, so a rejected address cannot strand the STA
// offline. Its own failure is only logged: the caller is already returning the
// error that caused the interface to be down.
func (w *WifiService) bringLinkUp(ifname string) {
	if w.cmd == nil {
		return
	}
	if _, err := w.cmd.Run("ip", "link", "set", ifname, "up"); err != nil {
		log.Printf("WARNING: could not bring %s back up after a failed MAC change: %v", ifname, err)
	}
}

// readPhyHardwareMAC reads the permanent/hardware MAC address for a wireless
// interface from its parent phy's sysfs address list.
func (w *WifiService) readPhyHardwareMAC(ifname string) string {
	// Resolve the phy name from the interface: /sys/class/net/<ifname>/phy80211/name
	phyNameBytes, err := os.ReadFile("/sys/class/net/" + ifname + "/phy80211/name")
	if err != nil {
		return ""
	}
	phyName := strings.TrimSpace(string(phyNameBytes))
	// Read the first address from the phy (permanent hardware MAC).
	addrsBytes, err := os.ReadFile("/sys/class/ieee80211/" + phyName + "/addresses")
	if err != nil {
		return ""
	}
	lines := strings.Fields(string(addrsBytes))
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return ""
}

// RandomizeMAC generates a random locally-administered unicast MAC address
// and applies it to the STA WiFi interface. It returns the new MAC.
func (w *WifiService) RandomizeMAC() (string, *WirelessApplyResult, error) {
	mac, err := generateRandomMAC()
	if err != nil {
		return "", nil, fmt.Errorf("generating random MAC: %w", err)
	}
	apply, err := w.SetMACAddress(mac)
	if err != nil {
		return "", nil, err
	}
	return mac, apply, nil
}

// generateRandomMAC creates a random locally-administered unicast MAC address.
// Locally-administered: bit 1 of first octet set. Unicast: bit 0 of first octet cleared.
func generateRandomMAC() (string, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	// Set locally-administered bit (bit 1) and clear unicast/multicast bit (bit 0)
	buf[0] = (buf[0] | 0x02) & 0xFE
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", buf[0], buf[1], buf[2], buf[3], buf[4], buf[5]), nil
}

// generateRandomWPAKey creates a 20-character WPA2 passphrase from an
// unambiguous alphabet. It is used for passphrases the service has to invent
// (a default AP the operator never configured), so the credential must not be
// predictable across devices.
func generateRandomWPAKey() (string, error) {
	const alphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	const length = 20
	out := make([]byte, 0, length)
	buf := make([]byte, length)
	for len(out) < length {
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		for _, b := range buf {
			// Reject values above the largest whole multiple of the alphabet
			// size, otherwise short strings would be biased by modulo.
			if int(b) >= (256/len(alphabet))*len(alphabet) {
				continue
			}
			out = append(out, alphabet[int(b)%len(alphabet)])
			if len(out) == length {
				break
			}
		}
	}
	return string(out), nil
}

const macPoliciesPath = "/etc/travo/mac-policies.json"

// GetMACPolicies returns the saved per-network MAC policies.
func (w *WifiService) GetMACPolicies() (models.MACPolicies, error) {
	data, err := os.ReadFile(macPoliciesPath)
	if err != nil {
		return models.MACPolicies{Policies: []models.MACPolicy{}}, nil
	}
	var p models.MACPolicies
	if err := json.Unmarshal(data, &p); err != nil {
		return models.MACPolicies{Policies: []models.MACPolicy{}}, nil
	}
	return p, nil
}

// SetMACPolicies saves the per-network MAC policies.
func (w *WifiService) SetMACPolicies(policies models.MACPolicies) error {
	data, err := json.Marshal(policies)
	if err != nil {
		return err
	}
	if err := os.MkdirAll("/etc/travo", 0o755); err != nil {
		return err
	}
	return os.WriteFile(macPoliciesPath, data, 0o644)
}
