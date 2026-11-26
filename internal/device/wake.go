package device

import (
	"bytes"
	"net"
	"net/netip"
	"strings"
)

// WakePort is the UDP port magic packets go to (the "discard" port most NICs listen on).
const WakePort = 9

// MagicPacketLen is 6 bytes of 0xFF followed by the MAC 16 times.
const MagicPacketLen = 6 + 16*6

// WakeSender sends one Wake-on-LAN magic packet as an IPv4 UDP broadcast to WakePort out
// of one LAN bridge (internal/wol). It changes no router state. Adapters refuse a bridge
// that is not ValidWakeBridge, a non-IPv4 broadcast, or a payload that is not a magic
// packet before sending anything.
type WakeSender interface {
	SendWake(bridge string, broadcast netip.Addr, packet []byte) error
}

// ValidWakeBridge accepts a kernel bridge name: "br-" plus 1 to 12 letters, digits, '-'
// or '_' (IFNAMSIZ is 16 including the NUL).
func ValidWakeBridge(name string) bool {
	rest, ok := strings.CutPrefix(name, "br-")
	if !ok || rest == "" || len(name) > 15 {
		return false
	}
	for _, c := range rest {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// IsMagicPacket reports whether p is 6×0xFF followed by one 6-byte MAC repeated 16 times.
func IsMagicPacket(p []byte) bool {
	if len(p) != MagicPacketLen || !bytes.Equal(p[:6], bytes.Repeat([]byte{0xff}, 6)) {
		return false
	}
	mac := p[6:12]
	for i := 12; i < len(p); i += 6 {
		if !bytes.Equal(p[i:i+6], mac) {
			return false
		}
	}
	return true
}

// MagicPacket builds the payload for a 6-byte MAC.
func MagicPacket(mac net.HardwareAddr) []byte {
	p := bytes.Repeat([]byte{0xff}, 6)
	for range 16 {
		p = append(p, mac...)
	}
	return p
}
