package api

import (
	"encoding/base64"
	"net"
	"regexp"
	"strconv"
	"strings"
)

var validHostnameRe = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-\.]{0,61}[a-zA-Z0-9])?$`)

// isValidIPv4 returns true if s is a valid IPv4 address (no CIDR, no port).
func isValidIPv4(s string) bool {
	ip := net.ParseIP(s)
	if ip == nil {
		return false
	}
	// Ensure it's IPv4 (net.ParseIP also accepts IPv6)
	return ip.To4() != nil
}

// isValidCIDR returns true if s is valid CIDR notation (e.g. "10.0.0.0/24").
func isValidCIDR(s string) bool {
	_, _, err := net.ParseCIDR(s)
	return err == nil
}

// isValidPort returns true if s is a valid port number (1-65535).
func isValidPort(s string) bool {
	p, err := strconv.Atoi(s)
	if err != nil {
		return false
	}
	return p >= 1 && p <= 65535
}

// isValidMTU returns true if mtu is in the allowed range 68-9000.
func isValidMTU(mtu int) bool {
	return mtu >= 68 && mtu <= 9000
}

// isValidBase64Key returns true if s is a 44-character base64-encoded key
// (standard WireGuard key format: 32 bytes → 44 base64 chars with = padding).
func isValidBase64Key(s string) bool {
	if len(s) != 44 {
		return false
	}
	_, err := base64.StdEncoding.DecodeString(s)
	return err == nil
}

// isValidEndpoint returns true if s is in host:port format with a valid port.
func isValidEndpoint(s string) bool {
	if s == "" {
		return false
	}
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		return false
	}
	if host == "" {
		return false
	}
	return isValidPort(port)
}

// isValidNetmask returns true if s is a valid IPv4 netmask (e.g. "255.255.255.0").
func isValidNetmask(s string) bool {
	ip := net.ParseIP(s)
	if ip == nil {
		return false
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	mask := net.IPv4Mask(ip4[0], ip4[1], ip4[2], ip4[3])
	// A valid netmask must have contiguous 1-bits followed by contiguous 0-bits.
	ones, bits := mask.Size()
	return bits == 32 && ones > 0
}

// isValidWanType returns true if t is one of the allowed WAN types.
func isValidWanType(t string) bool {
	switch strings.ToLower(t) {
	case "dhcp", "static", "pppoe":
		return true
	}
	return false
}

// isValidMAC checks if a string is a valid EUI-48 MAC address in colon-separated
// form (XX:XX:XX:XX:XX:XX). Other separators (dashes, none) are rejected because
// OpenWRT UCI only accepts the colon format.
func isValidMAC(mac string) bool {
	hw, err := net.ParseMAC(mac)
	// Ensure it parsed as EUI-48 AND uses colon separators (not dash or no separator).
	return err == nil && len(hw) == 6 && len(mac) == 17 && mac[2] == ':'
}

// isValidHostname checks if a string is a valid DNS hostname.
func isValidHostname(name string) bool {
	if len(name) == 0 || len(name) > 63 {
		return false
	}
	return validHostnameRe.MatchString(name)
}
