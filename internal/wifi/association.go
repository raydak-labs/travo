package wifi

import (
	"context"

	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
)

// Association is the observed identity of the managed uplink STA. The zero value
// means the STA is absent, disabled or proven disconnected.
type Association struct{ SSID, BSSID string }

// AssociatedUplink reads which network the managed wwan STA is associated with.
// The configured SSID never counts: only carrier plus an observed SSID and BSSID do.
// Ambiguous or unreadable state is ErrUnavailable, never a guess.
func (s *Service) AssociatedUplink(ctx context.Context) (Association, error) {
	if s.ubus == nil || s.join == nil || s.join.UCI == nil {
		return Association{}, ErrUnavailable
	}
	wireless, err := s.join.UCI.Show(ctx, "wireless")
	if err != nil {
		return Association{}, ErrUnavailable
	}
	var enabled []string
	for _, section := range uplinkSections(wireless) {
		if section.Options["disabled"] != "1" {
			enabled = append(enabled, section.Name)
		}
	}
	switch len(enabled) {
	case 0:
		return Association{}, nil
	case 1:
	default:
		return Association{}, ErrUnavailable
	}
	ifname, err := s.runtimeSTA(ctx, enabled[0])
	if err != nil || ifname == "" {
		return Association{}, err
	}
	raw, err := s.ubus.Call(ctx, "network.device", "status", map[string]string{"name": ifname})
	if err != nil {
		return Association{}, ErrUnavailable
	}
	link, err := facts.ParseDeviceStatus(raw)
	if err != nil {
		return Association{}, ErrUnavailable
	}
	if !link.Present || !link.Carrier {
		return Association{}, nil
	}
	raw, err = s.ubus.Call(ctx, "iwinfo", "info", map[string]string{"device": ifname})
	if err != nil {
		return Association{}, ErrUnavailable
	}
	info, err := facts.ParseIwinfoInfo(raw)
	if err != nil {
		return Association{}, ErrUnavailable
	}
	if info.SSID == "" {
		return Association{}, nil
	}
	if !macPattern.MatchString(info.BSSID) {
		return Association{}, ErrUnavailable
	}
	return Association{SSID: info.SSID, BSSID: info.BSSID}, nil
}

// runtimeSTA resolves a UCI section to its one running STA ifname. No runtime
// entry means the STA is not up, so it is empty without error.
func (s *Service) runtimeSTA(ctx context.Context, section string) (string, error) {
	raw, err := s.ubus.Call(ctx, "network.wireless", "status", nil)
	if err != nil {
		return "", ErrUnavailable
	}
	radios, err := facts.ParseWirelessRadios(raw)
	if err != nil {
		return "", ErrUnavailable
	}
	var matches []facts.WirelessIface
	for _, radio := range radios {
		for _, iface := range radio.Ifaces {
			if iface.Section == section {
				matches = append(matches, iface)
			}
		}
	}
	switch {
	case len(matches) == 0:
		return "", nil
	case len(matches) > 1 || matches[0].Mode != "sta" || matches[0].Ifname == "":
		return "", ErrUnavailable
	}
	return matches[0].Ifname, nil
}
