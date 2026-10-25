package uplink

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// readDHCPDNS parses the first nameserver line from resolv.conf.auto —
// this is the DNS server handed out by the upstream network via DHCP.
func (c *CaptiveService) readDHCPDNS() string {
	path := c.resolvConfPath()
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if after, ok := strings.CutPrefix(line, "nameserver "); ok {
			ip := strings.TrimSpace(after)
			if ip != "" {
				return ip
			}
		}
	}
	return ""
}

// dnsmasqLayers returns this service's view of the shared stack.
func (c *CaptiveService) dnsmasqLayers() *dnsmasqLayerStackFile {
	path := c.dnsStackPath
	if path == "" && c.dnsGuard.Path() != "" {
		// Next to the guard: /etc/trafo in production, a temp dir in tests.
		path = filepath.Join(filepath.Dir(c.dnsGuard.Path()), dnsmasqLayerStackFileName)
	}
	return &dnsmasqLayerStackFile{dns: commandRunnerDNS{cmd: c.cmd}, path: path}
}

// getDnsmasqOption reads a single dnsmasq option via uci get.
func (c *CaptiveService) getDnsmasqOption(option string) string {
	if c.cmd == nil {
		return ""
	}
	out, err := c.cmd.Run("uci", "get", "dhcp.@dnsmasq[0]."+option)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// setDnsmasqOption sets a dnsmasq option via uci set.
func (c *CaptiveService) setDnsmasqOption(option, value string) error {
	if c.cmd == nil {
		return nil
	}
	_, err := c.cmd.Run("uci", "set", "dhcp.@dnsmasq[0]."+option+"="+value)
	return err
}

// commitDhcp runs uci commit dhcp.
func (c *CaptiveService) commitDhcp() error {
	if c.cmd == nil {
		return nil
	}
	_, err := c.cmd.Run("uci", "commit", "dhcp")
	return err
}
