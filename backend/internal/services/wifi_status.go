package services

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
)

func (w *WifiService) appliedWirelessUp(
	sections map[string]map[string]string, wantAPs, wantSTAs []string,
) error {
	resp, err := w.ubus.Call("network.wireless", "status", nil)
	if err != nil {
		return fmt.Errorf("%w: %w: %w", ErrWirelessUnverifiable, errWirelessStatusUnread, err)
	}
	radios, unreadable, shapeErr := wirelessStatusRadios(resp)
	// section name -> what the config expects it to be, so one pass over the
	// status answers both kinds.
	pending := make(map[string]string, len(wantAPs)+len(wantSTAs))
	for _, name := range wantAPs {
		pending[name] = expectedAPKind
	}
	for _, name := range wantSTAs {
		pending[name] = expectedSTAKind
	}
	want := make([]string, 0, len(pending))
	for name := range pending {
		want = append(want, name)
	}
	sort.Strings(want)
	for _, radioName := range slices.Sorted(maps.Keys(radios)) {
		radio := radios[radioName]
		// Collect first, prove second: whether the radio flag counts as proof
		// depends on how many EXPECTED interfaces it carries, and that is only
		// known once the whole radio has been read.
		var onRadio []struct {
			name  string
			kind  string
			iface map[string]any
		}
		for _, iface := range radio.interfaces {
			name := wirelessStatusSection(iface, sections, want)
			kind, expected := pending[name]
			if !expected {
				continue
			}
			onRadio = append(onRadio, struct {
				name  string
				kind  string
				iface map[string]any
			}{name, kind, iface})
		}
		// One expected interface on the radio: the radio flag is proof, and no
		// ubus round-trip is spent. More than one: the flag says the RADIO is
		// up, not which of its interfaces are, so each one has to answer for
		// itself.
		radioIsProof := radio.settled && len(onRadio) < 2
		for _, entry := range onRadio {
			// An uplink STA is never proven by the radio flag. The flag says the
			// radio is up, which a station that has configured itself and never
			// associated also satisfies: on the device a non-associated STA reads
			// up:true, carrier:false while its radio is settled. Taking the flag
			// there confirmed a client-mode apply whose uplink cannot work and
			// cancelled the rollback that would have restored the working one —
			// the operator was left with no internet and no way back.
			//
			// carrier is the only signal that says the station associated, so the
			// per-device answer is the authority for an uplink STA. An access
			// point keeps the radio shortcut: a settled radio has already created
			// its interface, and skipping the round-trip keeps the probe inside
			// wirelessProbeMaxUbusCalls for the common single-AP layout.
			up := radioIsProof && entry.kind != expectedSTAKind
			if !up {
				var err error
				if up, err = w.wirelessDeviceUp(entry.iface); err != nil {
					return err
				}
			}
			if up {
				delete(pending, entry.name)
			}
		}
	}
	if len(pending) == 0 {
		return nil
	}
	// The interfaces that read fine were proven; anything still missing may be
	// sitting behind a radio this build cannot read. When one of the radios the
	// still-missing sections belong to is the unreadable one, the honest answer
	// is "I cannot read this", not "your access point did not come up". Either
	// way the proof is not made and the rollback stays armed.
	for _, name := range want {
		if _, still := pending[name]; !still {
			continue
		}
		if _, hidden := unreadable[sections[name]["device"]]; hidden && shapeErr != nil {
			return shapeErr
		}
	}
	missing := make([]string, 0, len(pending))
	for _, name := range want {
		if kind, still := pending[name]; still {
			missing = append(missing, fmt.Sprintf("%s (%s)", name, kind))
		}
	}
	return fmt.Errorf("%w: sections %s did not come up",
		ErrWirelessNotUp, strings.Join(missing, ", "))
}

// wirelessRadio is one entry of a `network.wireless status` payload: the radio's
// liveness and the interfaces netifd created on it.
type wirelessRadio struct {
	settled    bool
	interfaces []map[string]any
}

// wirelessStatusRadios splits a `network.wireless status` payload per radio and
// records whether each radio is settled. An entry the probe cannot read is
// skipped and REPORTED, never a silent skip: its interfaces are not listed, so
// anything the config expects on it stays unproven and the proof fails closed.
//
// Reading it aborts the whole payload (as this used to), so one malformed radio
// the apply has nothing to do with turned a transient netifd hiccup into a
// failure that was never retried, even when every interface the change is about
// read fine. The returned set names the radios that were skipped, so the caller
// can still tell "I cannot read this radio" from "the access point is down".
func wirelessStatusRadios(resp map[string]any) (map[string]wirelessRadio, map[string]bool, error) {
	out := make(map[string]wirelessRadio, len(resp))
	unreadable := map[string]bool{}
	var firstErr error
	note := func(radio string, err error) {
		unreadable[radio] = true
		if firstErr == nil {
			firstErr = err
		}
	}
	for name, data := range resp {
		radio, ok := data.(map[string]any)
		if !ok {
			note(name, fmt.Errorf("%w: radio %s is %T, not an object",
				ErrWirelessUnverifiable, name, data))
			continue
		}
		up, hasUp := radio["up"].(bool)
		if !hasUp {
			note(name, fmt.Errorf("%w: radio %s reports no \"up\" flag", ErrWirelessUnverifiable, name))
			continue
		}
		pending, _ := radio["pending"].(bool)
		retry, _ := radio["retry_setup_failed"].(bool)
		raw, ok := radio["interfaces"].([]any)
		if !ok {
			note(name, fmt.Errorf("%w: radio %s reports no \"interfaces\" list",
				ErrWirelessUnverifiable, name))
			continue
		}
		ifaces := make([]map[string]any, 0, len(raw))
		readable := true
		for _, entry := range raw {
			iface, ok := entry.(map[string]any)
			if !ok {
				note(name, fmt.Errorf("%w: radio %s has a non-object interface entry",
					ErrWirelessUnverifiable, name))
				readable = false
				break
			}
			if _, ok := iface["ifname"].(string); !ok {
				note(name, fmt.Errorf("%w: an interface on radio %s reports no \"ifname\"",
					ErrWirelessUnverifiable, name))
				readable = false
				break
			}
			ifaces = append(ifaces, iface)
		}
		if !readable {
			continue
		}
		out[name] = wirelessRadio{settled: up && !pending && !retry, interfaces: ifaces}
	}
	return out, unreadable, firstErr
}

// wirelessDeviceUp asks netifd whether one wireless interface is live. It is the
// second liveness signal, and the same `network.device` object the rest of the
// backend already reads (see NetworkService), so it adds no dependency.
//
// An unknown device makes the call fail and an unassociated device answers
// present+up without carrier; both count as down. A non-empty answer missing one
// of the three flags is a shape problem, not a down interface, and is reported
// as one.
func (w *WifiService) wirelessDeviceUp(iface map[string]any) (bool, error) {
	ifname, _ := iface["ifname"].(string)
	if ifname == "" {
		return false, nil
	}
	resp, err := w.ubus.Call("network.device", "status", map[string]any{"name": ifname})
	if err != nil {
		return false, nil
	}
	if len(resp) == 0 {
		return false, nil
	}
	for _, key := range []string{"present", "up", "carrier"} {
		if _, ok := resp[key].(bool); !ok {
			return false, fmt.Errorf("%w: network.device status for %s reports no %q flag",
				ErrWirelessUnverifiable, ifname, key)
		}
	}
	present, _ := resp["present"].(bool)
	up, _ := resp["up"].(bool)
	carrier, _ := resp["carrier"].(bool)
	return present && up && carrier, nil
}

// wirelessStatusSection names the wanted config section a netifd interface
// entry belongs to, or "" when it is not one of them. It falls back to mode+ssid
// so a build without "section" does not read as every interface being down.
func wirelessStatusSection(
	iface map[string]any, sections map[string]map[string]string, want []string,
) string {
	name, _ := iface["section"].(string)
	for _, candidate := range want {
		if candidate == name {
			return name
		}
	}
	cfg, _ := iface["config"].(map[string]any)
	mode, _ := cfg["mode"].(string)
	ssid, _ := cfg["ssid"].(string)
	for _, candidate := range want {
		opts := sections[candidate]
		if ssid != "" && opts["ssid"] == ssid && opts["mode"] == mode {
			return candidate
		}
	}
	return ""
}
