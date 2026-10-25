package services

import (
	"log/slog"
	"strings"
	"time"
)

// dnsmasqLayers returns this service's view of the shared stack.
func (v *VpnService) dnsmasqLayers() *dnsmasqLayerStackFile {
	return &dnsmasqLayerStackFile{
		dns:  commandRunnerDNS{cmd: v.cmd},
		path: v.dnsStackPath,
	}
}

// splitWireGuardDNSOption splits UCI network.wg0.dns. OpenWrt normally uses
// space-separated values; some imports (or hand-edited UCI) use commas.
func splitWireGuardDNSOption(dns string) []string {
	dns = strings.TrimSpace(dns)
	if dns == "" {
		return nil
	}
	dns = strings.ReplaceAll(dns, ",", " ")
	var out []string
	for s := range strings.FieldsSeq(dns) {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func (v *VpnService) wgConfiguredDNSServers() []string {
	dns, err := v.uci.Get("network", "wg0", "dns")
	if err != nil {
		return nil
	}
	return splitWireGuardDNSOption(dns)
}

// enableVpnDNSForwarding pushes the VPN onto the shared dnsmasq layer stack. It
// reports failure but must never fail the toggle: the tunnel is already
// verified up before it runs, so aborting the whole enable over DNS bookkeeping
// would tear down a working VPN. Returning an error lets the caller record why
// LAN DNS was left alone.
//
// Enabling is idempotent: a layer already stacked has its resolvers refreshed
// and the base state is left alone.
func (v *VpnService) enableVpnDNSForwarding() error {
	vpnDNS := v.wgConfiguredDNSServers()
	if len(vpnDNS) == 0 {
		return nil
	}
	return v.dnsmasqLayers().EnableLayer(dnsLayerVPN, vpnDNS)
}

// disableVpnDNSForwarding pops the VPN off the shared dnsmasq layer stack,
// restoring the layer below it — or the pre-any-layer state when it was the
// last one. It is best-effort and reports nothing.
func (v *VpnService) disableVpnDNSForwarding() {
	if _, err := v.dnsmasqLayers().RemoveLayer(dnsLayerVPN, false); err != nil {
		slog.Error("vpn: restoring dnsmasq from the layer stack", "err", err)
	}
}

// observeTunnelDown debounces a repeated enabled_not_up state and reports
// whether the heal may fire.
func (v *VpnService) observeTunnelDown(tunnelDetail string) bool {
	v.dnsHealMu.Lock()
	defer v.dnsHealMu.Unlock()
	if tunnelDetail != "enabled_not_up" {
		v.dnsHealStreak = 0
		v.dnsHealSince = time.Time{}
		return false
	}
	if v.dnsHealStreak == 0 {
		v.dnsHealSince = time.Now()
	}
	v.dnsHealStreak++
	if v.dnsHealStreak < vpnDNSHealStreakRequired {
		return false
	}
	return time.Since(v.dnsHealSince) >= vpnDNSHealGracePeriod
}

// maybeSelfHealVpnDNS pops the VPN layer when the tunnel that justified it can
// no longer carry DNS: wg0 is absent for good, or the tunnel is disabled in UCI.
//
// It heals only on TERMINAL states. `configured` and `up_no_handshake` mean the
// tunnel is on its way up (a reboot into a slow upstream). `enabled_not_up` is
// ambiguous, since a single failed `wg show` looks the same, so it is debounced
// (observeTunnelDown) instead of trusted.
//
// tunnelDetail is the already-computed wgRuntimeState detail, so this costs no
// extra shell-out. Returns true when a restore happened.
func (v *VpnService) maybeSelfHealVpnDNS(tunnelDetail string) bool {
	// Reset the streak on anything that is not the ambiguous down reading.
	healable := v.observeTunnelDown(tunnelDetail)
	switch tunnelDetail {
	case "connected", "up_no_handshake", "configured":
		return false
	case "enabled_not_up":
		if !healable {
			return false
		}
	default:
		// "disabled": the tunnel is off in UCI, which is terminal rather than
		// a reading that could be transient.
	}

	layers := v.dnsmasqLayers()
	if !layers.hasLayer(dnsLayerVPN) {
		// No layer stacked: dnsmasq was never pointed at the tunnel.
		return false
	}
	slog.Info("vpn: tunnel down with the VPN resolver layer stacked; restoring the layer below", "state", tunnelDetail)
	// The stack shells out to `uci` for dhcp, so it needs the same lock every
	// other dhcp writer holds (ADR 0010).
	if err := withConfigLocks([]string{"dhcp"}, func() error {
		// keepRecord: the heal must not drop the pre-any-layer state, or a
		// later explicit disable has nothing to restore.
		_, err := layers.RemoveLayer(dnsLayerVPN, true)
		return err
	}); err != nil {
		slog.Error("vpn: healing dnsmasq", "state", tunnelDetail, "err", err)
		return false
	}
	return true
}
