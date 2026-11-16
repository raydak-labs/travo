package wifi

import (
	"context"
	"errors"
	"net/netip"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/lockout"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

var (
	ErrAPNotFound = errors.New("access point not found")
	// ErrRadioInUse: enabling the AP would put it on a radio with an enabled STA
	// (ath11k crash, device-facts §3).
	ErrRadioInUse  = errors.New("radio carries the uplink")
	sectionPattern = regexp.MustCompile(`^[A-Za-z0-9_]{1,64}$`)
)

// APConfig is one configured access point (UCI), including disabled ones that
// network.wireless status omits. Keys are never returned.
type APConfig struct {
	Section  string `json:"section"`
	Radio    string `json:"radio"`
	Band     string `json:"band" enum:"2g,5g,6g,unknown"`
	SSID     string `json:"ssid"`
	Enabled  bool   `json:"enabled"`
	Guest    bool   `json:"guest" doc:"Not bridged into lan"`
	Security string `json:"security" enum:"open,protected"`
}

// APRequest edits one AP. An empty Key keeps the current key.
type APRequest struct {
	Section, SSID, Key string
	Enabled            bool
	Acknowledge        bool
	Caller             netip.Addr
}

func (r APRequest) validate() error {
	if !sectionPattern.MatchString(r.Section) {
		return ErrAPNotFound
	}
	if err := validSSID(r.SSID); err != nil {
		return err
	}
	if r.Key != "" {
		return validKey(r.Key)
	}
	return nil
}

func validSSID(ssid string) error {
	if len(ssid) < 1 || len(ssid) > 32 || strings.ContainsAny(ssid, "\x00\r\n") {
		return ValidationError{"wifi_ssid_invalid"}
	}
	return nil
}

func validKey(key string) error {
	if len(key) < 8 || len(key) > 63 || strings.IndexFunc(key, func(c rune) bool { return c < 32 || c > 126 }) >= 0 {
		return ValidationError{"wifi_key_invalid"}
	}
	return nil
}

func sectionNetworks(section device.Section) []string {
	return append(strings.Fields(section.Options["network"]), section.Lists["network"]...)
}

func isAP(section device.Section) bool {
	return section.Type == "wifi-iface" && section.Options["mode"] == "ap"
}

// apLayout is the lockout view of every configured AP.
func apLayout(wireless []device.Section) []lockout.AP {
	var aps []lockout.AP
	for _, section := range wireless {
		if isAP(section) {
			aps = append(aps, lockout.AP{Section: section.Name, Enabled: section.Options["disabled"] != "1", Guest: !slices.Contains(sectionNetworks(section), "lan")})
		}
	}
	return aps
}

// AccessPoints lists configured APs from UCI.
func (s *Service) AccessPoints(ctx context.Context) ([]APConfig, error) {
	if s.join == nil {
		return nil, ErrUnavailable
	}
	wireless, err := s.join.UCI.Show(ctx, "wireless")
	if err != nil {
		return nil, ErrUnavailable
	}
	bands := map[string]string{}
	for _, section := range wireless {
		if section.Type == "wifi-device" {
			bands[section.Name] = section.Options["band"]
		}
	}
	out := []APConfig{}
	for _, ap := range apLayout(wireless) {
		section := wireless[slices.IndexFunc(wireless, func(s device.Section) bool { return s.Name == ap.Section })]
		band := bands[section.Options["device"]]
		if band != "2g" && band != "5g" && band != "6g" {
			band = "unknown"
		}
		security := "protected"
		if enc := section.Options["encryption"]; enc == "" || enc == "none" {
			security = "open"
		}
		out = append(out, APConfig{Section: ap.Section, Radio: section.Options["device"], Band: band, SSID: section.Options["ssid"], Enabled: ap.Enabled, Guest: ap.Guest, Security: security})
	}
	return out, nil
}

// planAP stages one AP edit. before/after are the lockout layouts.
func planAP(wireless []device.Section, req APRequest) (writes []uciWrite, before, after []lockout.AP, err error) {
	at := slices.IndexFunc(wireless, func(s device.Section) bool { return s.Name == req.Section && isAP(s) })
	if at < 0 {
		return nil, nil, nil, ErrAPNotFound
	}
	section := wireless[at]
	before = apLayout(wireless)
	after = slices.Clone(before)
	for i := range after {
		if after[i].Section == req.Section {
			after[i].Enabled = req.Enabled
		}
	}
	if req.Enabled && radioHasSTA(wireless, section.Options["device"]) {
		return nil, before, after, ErrRadioInUse
	}
	set := func(option, value string) {
		if section.Options[option] != value {
			writes = append(writes, uciWrite{"set", "wireless", section.Name, option, value})
		}
	}
	set("ssid", req.SSID)
	if req.Key != "" {
		if enc := section.Options["encryption"]; enc == "" || enc == "none" {
			set("encryption", "psk2")
		}
		if section.Options["key"] != req.Key {
			writes = append(writes, uciWrite{"key", "wireless", section.Name, "key", ""})
		}
	}
	set("disabled", map[bool]string{true: "0", false: "1"}[req.Enabled])
	return writes, before, after, nil
}

// planAPMove moves an AP section to another radio, keeping SSID, key and network.
func planAPMove(section, radio string) uciWrite {
	return uciWrite{"set", "wireless", section, "device", radio}
}

func radioHasSTA(wireless []device.Section, radio string) bool {
	return slices.ContainsFunc(wireless, func(s device.Section) bool {
		return s.Type == "wifi-iface" && s.Options["mode"] == "sta" && s.Options["device"] == radio && s.Options["disabled"] != "1"
	})
}

// UpdateAP applies one AP edit through a guarded transaction. Disabling an enabled AP is
// lockout-checked against the real caller (invariants §2); SSID and key changes are not.
func (s *Service) UpdateAP(ctx context.Context, req APRequest) (txn.Status, error) {
	if err := req.validate(); err != nil {
		return txn.Status{}, err
	}
	if s.join == nil {
		return txn.Status{}, ErrUnavailable
	}
	if _, err := s.prepareAP(ctx, req); err != nil {
		return txn.Status{}, err
	}
	var expect []apExpectation
	st, err := s.join.Txns.Begin(ctx, "wifi: access point "+req.Section, []string{"wireless", "network", "firewall"}, func(ctx context.Context) error {
		writes, err := s.prepareAP(ctx, req)
		if err != nil {
			return err
		}
		wireless, err := s.join.UCI.Show(ctx, "wireless")
		if err != nil {
			return ErrUnavailable
		}
		expect = apExpectations(wireless, req.Section, req)
		return s.stageWrites(ctx, writes, req.Key)
	}, func(ctx context.Context) error { return s.probeAPs(expect)(ctx) })
	if errors.Is(err, txn.ErrActive) {
		return st, ErrTransactionPending
	}
	return st, err
}

func (s *Service) prepareAP(ctx context.Context, req APRequest) ([]uciWrite, error) {
	wireless, err := s.join.UCI.Show(ctx, "wireless")
	if err != nil {
		return nil, ErrUnavailable
	}
	writes, before, after, err := planAP(wireless, req)
	if err != nil {
		return nil, err
	}
	if disablesAP(before, after) {
		caller := classifyCaller(ctx, s.join.Ubus, s.join.Neighbours, apNetworks(wireless), req.Caller)
		before = withIfnames(ctx, s.join.Ubus, before)
		if err := lockout.Check(caller, lockout.Plan{Before: before, After: after}, req.Acknowledge); err != nil {
			return nil, err
		}
	}
	return writes, nil
}

func disablesAP(before, after []lockout.AP) bool {
	for i := range before {
		if before[i].Enabled && !after[i].Enabled {
			return true
		}
	}
	return false
}

// withIfnames fills live ifnames so Check can match the caller's AP to its section.
func withIfnames(ctx context.Context, ubus device.Ubus, aps []lockout.AP) []lockout.AP {
	raw, err := ubus.Call(ctx, "network.wireless", "status", nil)
	if err != nil {
		return aps
	}
	radios, err := facts.ParseWirelessRadios(raw)
	if err != nil {
		return aps
	}
	out := slices.Clone(aps)
	for _, radio := range radios {
		for _, iface := range radio.Ifaces {
			for i := range out {
				if out[i].Section == iface.Section {
					out[i].Ifname = iface.Ifname
				}
			}
		}
	}
	return out
}

// apExpectation is what the probe must prove for one AP section after apply.
type apExpectation struct {
	section string
	enabled bool
	ssid    string // checked on the live AP when set
}

// apExpectations lists every AP that must be up after the edit, plus the edited AP.
func apExpectations(wireless []device.Section, edited string, req APRequest) []apExpectation {
	var out []apExpectation
	for _, section := range wireless {
		if !isAP(section) {
			continue
		}
		if section.Name == edited {
			out = append(out, apExpectation{section: edited, enabled: req.Enabled, ssid: req.SSID})
		} else if section.Options["disabled"] != "1" && !radioDisabled(wireless, section.Options["device"]) {
			out = append(out, apExpectation{section: section.Name, enabled: true})
		}
	}
	return out
}

func radioDisabled(wireless []device.Section, radio string) bool {
	return slices.ContainsFunc(wireless, func(s device.Section) bool {
		return s.Type == "wifi-device" && s.Name == radio && s.Options["disabled"] == "1"
	})
}

func (s *Service) probeAPs(expect []apExpectation) txn.Probe {
	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, s.join.ProbeBudget)
		defer cancel()
		ticker := time.NewTicker(s.join.ProbeEvery)
		defer ticker.Stop()
		for {
			if ctx.Err() == nil && s.apsProven(ctx, expect) {
				return nil
			}
			select {
			case <-ctx.Done():
				return errors.New("access points not proven")
			case <-ticker.C:
			}
		}
	}
}

// apsProven checks each AP interface individually (device-facts §2): a radio flag
// proves nothing per interface, and an unreadable reply is "cannot verify".
func (s *Service) apsProven(ctx context.Context, expect []apExpectation) bool {
	radios, err := s.joinRadios(ctx)
	if err != nil {
		return false
	}
	ifnames := map[string]string{}
	for _, radio := range radios {
		for _, iface := range radio.Ifaces {
			if iface.Mode == "ap" {
				ifnames[iface.Section] = iface.Ifname
			}
		}
	}
	for _, e := range expect {
		ifname, listed := ifnames[e.section]
		if !e.enabled {
			// netifd lists only enabled interfaces; still listed = not applied yet.
			if listed {
				return false
			}
			continue
		}
		if ifname == "" || !s.apUp(ctx, ifname) {
			return false
		}
		if e.ssid != "" && !s.apBroadcasts(ctx, ifname, e.ssid) {
			return false
		}
	}
	return true
}

func (s *Service) apUp(ctx context.Context, ifname string) bool {
	raw, err := s.join.Ubus.Call(ctx, "network.device", "status", map[string]string{"name": ifname})
	if err != nil {
		return false
	}
	link, err := facts.ParseDeviceStatus(raw)
	return err == nil && link.Present && link.Up
}

func (s *Service) apBroadcasts(ctx context.Context, ifname, ssid string) bool {
	raw, err := s.join.Ubus.Call(ctx, "iwinfo", "info", map[string]string{"device": ifname})
	if err != nil {
		return false
	}
	live, err := facts.ParseIwinfoAPSSID(raw)
	return err == nil && live == ssid
}
