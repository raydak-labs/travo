package uplink

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/platform"
)

// adguardDNSInfoResponse matches the JSON from /control/dns_info.
type adguardDNSInfoResponse struct {
	Port int `json:"port"`
}

// getDNSPort returns the DNS port AdGuard listens on, or the default.
func (s *AdGuardService) getDNSPort() int {
	s.refreshEndpointsFromYAML()
	s.mu.RLock()
	port := s.yamlDNSPort
	s.mu.RUnlock()
	if port > 0 {
		return port
	}
	body, err := s.checker.HTTPGet(s.apiBase() + "/control/dns_info")
	if err != nil {
		return adguardDefaultDNSPort
	}
	var info adguardDNSInfoResponse
	if err := json.Unmarshal(body, &info); err != nil || info.Port == 0 {
		return adguardDefaultDNSPort
	}
	return info.Port
}

// dnsmasqServerEntry returns the dnsmasq server value for a given port.
func dnsmasqServerEntry(port int) string {
	return fmt.Sprintf("127.0.0.1#%d", port)
}

// probeAdGuardDNSListener returns true if the AdGuard DNS listener is accepting
// TCP connections on the given port.
func (s *AdGuardService) probeAdGuardDNSListener(port int) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	return s.checker.TCPProbe(addr, 2*time.Second)
}

// probeResolver tries to resolve "example.com" via the local resolver and returns true on success.
func (s *AdGuardService) probeResolver(_ int) bool {
	_, err := s.checker.RunCommand("nslookup", "example.com", "127.0.0.1")
	return err == nil
}

// GetDNSStatus checks whether dnsmasq is configured to forward to AdGuard,
// and includes health checks for the DNS listener path.
func (s *AdGuardService) GetDNSStatus() (models.AdGuardDNSStatus, error) {
	port := s.getDNSPort()
	entry := dnsmasqServerEntry(port)

	out, err := s.checker.RunCommand("uci", "get", "dhcp.@dnsmasq[0].server")

	var forwardTarget string
	var enabled bool
	if err == nil {
		enabled = strings.Contains(out, entry)
		if enabled {
			forwardTarget = entry
		}
	}

	status := models.AdGuardDNSStatus{
		Enabled:              enabled,
		DNSPort:              port,
		DnsmasqForwardTarget: forwardTarget,
		AdguardListenerReady: s.probeAdGuardDNSListener(port),
	}
	if status.AdguardListenerReady {
		status.ResolverProbeOk = s.probeResolver(port)
	}
	return status, nil
}

// SetDNS stacks or unstacks AdGuard on the shared dnsmasq resolver layer stack
// (dnsmasqLayerStackFile, ADR 0001 §3). Enabling first verifies that AdGuard is
// running and its DNS listener answers; on failure nothing is changed. The stack
// keeps the pre-any-layer resolvers, so split-DNS entries added in LuCI survive.
//
// It takes the `dhcp` lock: the VPN and captive paths write the same
// dhcp.@dnsmasq[0] options. withConfigLocks rather than mutateUCI, because this
// shells out to `uci` and has no uci.UCI handle to revert through.
func (s *AdGuardService) SetDNS(enabled bool) error {
	return platform.WithConfigLocks([]string{"dhcp"}, func() error { return s.setDNSLocked(enabled) })
}

// checkerDNS drives the shared dnsmasq stack through AdGuardService's
// AdGuardChecker, so the two features share one record and one set of rules
// while keeping their own file and process abstractions.
type checkerDNS struct{ checker AdGuardChecker }

func (k checkerDNS) getServers() ([]string, error) {
	out, err := k.checker.RunCommand("uci", "get", "dhcp.@dnsmasq[0].server")
	if err != nil {
		return nil, err
	}
	return strings.Fields(strings.TrimSpace(out)), nil
}

func (k checkerDNS) getNoResolv() (string, error) {
	out, err := k.checker.RunCommand("uci", "get", "dhcp.@dnsmasq[0].noresolv")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (k checkerDNS) setServers(servers []string) error {
	// Best-effort clear: `uci delete` exits 1 with "Entry not found" when the
	// list is already empty, which is the normal state on a router that has
	// never had forwarding configured.
	_, _ = k.checker.RunCommand("uci", "delete", "dhcp.@dnsmasq[0].server")
	for _, srv := range servers {
		if _, err := k.checker.RunCommand("uci", "add_list", "dhcp.@dnsmasq[0].server="+srv); err != nil {
			return fmt.Errorf("failed to add dnsmasq server %q: %w", srv, err)
		}
	}
	return nil
}

func (k checkerDNS) setNoResolv(value string) error {
	if _, err := k.checker.RunCommand("uci", "set", "dhcp.@dnsmasq[0].noresolv="+value); err != nil {
		return fmt.Errorf("failed to set noresolv=%s: %w", value, err)
	}
	return nil
}

func (k checkerDNS) commitAndRestart() error {
	if _, err := k.checker.RunCommand("uci", "commit", "dhcp"); err != nil {
		return fmt.Errorf("failed to commit dhcp: %w", err)
	}
	if _, err := k.checker.RunCommand("/etc/init.d/dnsmasq", "restart"); err != nil {
		return fmt.Errorf("failed to restart dnsmasq: %w", err)
	}
	return nil
}

func (k checkerDNS) readFile(path string) ([]byte, error) { return k.checker.ReadFile(path) }

func (k checkerDNS) writeFile(path string, data []byte, perm os.FileMode) error {
	return k.checker.WriteFile(path, data, perm)
}

func (k checkerDNS) removeFile(path string) error { return k.checker.RemoveFile(path) }

// dnsmasqLayers returns this service's view of the shared stack.
func (s *AdGuardService) dnsmasqLayers() *dnsmasqLayerStackFile {
	return &dnsmasqLayerStackFile{
		dns:  checkerDNS{checker: s.checker},
		path: s.dnsStackPath,
	}
}

func (s *AdGuardService) setDNSLocked(enabled bool) error {
	if enabled {
		port := s.getDNSPort()
		entry := dnsmasqServerEntry(port)

		// Pre-flight: AdGuard must be running.
		if !s.IsRunning() {
			return fmt.Errorf("AdGuard Home is not running — start it before enabling DNS forwarding")
		}
		// Pre-flight: DNS listener must be reachable.
		if !s.probeAdGuardDNSListener(port) {
			return fmt.Errorf("AdGuard Home DNS listener is not ready on 127.0.0.1:%d — verify AdGuard config", port)
		}
		// The stack records the pre-any-layer state before dnsmasq is touched.
		// Without it there would be nothing to restore, so a failure here aborts
		// the enable instead of deleting the operator's resolver entries.
		return s.dnsmasqLayers().EnableLayer(dnsLayerAdGuard, []string{entry})
	}

	// Disable: pop the AdGuard layer. When another layer is still stacked the
	// stack leaves dnsmasq pointing at it untouched; only the last layer to go
	// restores the pre-any-layer state and drops the record.
	applied, err := s.dnsmasqLayers().RemoveLayer(dnsLayerAdGuard, false)
	if err != nil {
		return err
	}
	if applied {
		return nil
	}
	// No record: this disable was never preceded by an enable through this
	// service (or it predates the stack). The current server list is left
	// untouched — it may be entirely the operator's own split-DNS entries, and
	// deleting it is exactly the data loss the record exists to prevent.
	// noresolv is still cleared so dnsmasq falls back to resolv.conf.
	return s.dnsmasqLayers().ClearNoResolv()
}
