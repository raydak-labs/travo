package wifi

import (
	"context"
	"net/netip"
	"slices"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/lockout"
)

// CheckSTARestart applies the lockout rule (invariants §2) to `ifup wwan`: netifd rebuilds
// the STA vif, and an AP on the STA's radio drops with it while APs on other radios keep
// carrier (device-facts §2). No AP shares the STA's radio → nothing to check.
func (s *Service) CheckSTARestart(ctx context.Context, ip netip.Addr, acknowledge bool) error {
	if s.join == nil {
		return ErrUnavailable
	}
	wireless, err := s.join.UCI.Show(ctx, "wireless")
	if err != nil {
		return ErrUnavailable
	}
	raw, err := s.join.Ubus.Call(ctx, "network.wireless", "status", nil)
	if err != nil {
		return ErrUnavailable
	}
	radios, err := facts.ParseWirelessRadios(raw)
	if err != nil {
		return ErrUnavailable
	}
	before, after := staRestartPlan(wireless, radios)
	if slices.Equal(before, after) {
		return nil
	}
	caller := classifyCaller(ctx, s.join.Ubus, s.join.Neighbours, apNetworks(wireless), ip)
	return lockout.Check(caller, lockout.Plan{Before: before, After: after}, acknowledge)
}

// staRestartPlan is the AP layout before and during a STA restart: every AP on a radio
// that carries a STA reads as disabled.
func staRestartPlan(wireless []device.Section, radios []facts.Radio) (before, after []lockout.AP) {
	var dropped []string
	for _, r := range radios {
		if !slices.ContainsFunc(r.Ifaces, func(i facts.WirelessIface) bool { return i.Mode == "sta" }) {
			continue
		}
		for _, i := range r.Ifaces {
			dropped = append(dropped, i.Section)
		}
	}
	before = apLayout(wireless)
	for i := range before {
		for _, r := range radios {
			for _, iface := range r.Ifaces {
				if iface.Section == before[i].Section {
					before[i].Ifname = iface.Ifname
				}
			}
		}
	}
	after = slices.Clone(before)
	for i := range after {
		if slices.Contains(dropped, after[i].Section) {
			after[i].Enabled = false
		}
	}
	return before, after
}
