package cli

import (
	"net"
	"net/netip"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// The adapter refuses anything but a bridge, an IPv4 broadcast and a magic packet before it
// opens a socket.
func TestSendWakeRefusesBadRequests(t *testing.T) {
	hw, _ := net.ParseMAC("aa:bb:cc:dd:ee:ff")
	good := device.MagicPacket(hw)
	bcast := netip.MustParseAddr("192.168.1.255")
	d := New()
	for name, call := range map[string]func() error{
		"uplink device": func() error { return d.SendWake("phy0-sta0", bcast, good) },
		"option bridge": func() error { return d.SendWake("br-;reboot", bcast, good) },
		"ipv6":          func() error { return d.SendWake("br-lan", netip.MustParseAddr("ff02::1"), good) },
		"payload":       func() error { return d.SendWake("br-lan", bcast, []byte("hello")) },
	} {
		if call() == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestValidWakeBridge(t *testing.T) {
	for _, ok := range []string{"br-lan", "br-guest", "br-iot_2"} {
		if !device.ValidWakeBridge(ok) {
			t.Errorf("%s refused", ok)
		}
	}
	for _, bad := range []string{"", "br-", "lan", "wg0", "br-lan.10x", "br-abcdefghijklm", "br-a b"} {
		if device.ValidWakeBridge(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
}
