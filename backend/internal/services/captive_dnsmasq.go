package services

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
	if path == "" && c.guardFile != "" {
		// /etc/trafo in production: the same file the VPN and AdGuard paths use,
		// derived rather than hard-coded so a test service pointed at a temp
		// guard file cannot reach the real record.
		path = filepath.Join(filepath.Dir(c.guardFile), dnsmasqLayerStackFileName)
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

// deleteDnsmasqOption deletes a dnsmasq option via uci delete.
// deleteDnsmasqOption removes a dnsmasq option. A missing option is not a
// failure: the AdGuard-only bypass path never creates a `server` list, and
// treating "Entry not found" as an error made RestoreDNS fail on every attempt,
// which kept the guard forever and left IsDNSBypassed stuck true.
func (c *CaptiveService) deleteDnsmasqOption(option string) error {
	if c.cmd == nil {
		return nil
	}
	out, err := c.cmd.Run("uci", "delete", "dhcp.@dnsmasq[0]."+option)
	if err != nil && strings.Contains(string(out)+err.Error(), "Entry not found") {
		return nil
	}
	return err
}

// addDnsmasqListItem appends a value to a dnsmasq list option.
func (c *CaptiveService) addDnsmasqListItem(option, value string) error {
	if c.cmd == nil {
		return nil
	}
	_, err := c.cmd.Run("uci", "add_list", "dhcp.@dnsmasq[0]."+option+"="+value)
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
