package cli

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"syscall"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// wakeTimeout bounds one send; a local UDP write does not block in practice.
const wakeTimeout = 2 * time.Second

// SendWake writes the magic packet from Go: the 25.12.6 image ships no etherwake or
// wakeonlan (testdata wol/tools.txt), so no package is needed. The socket gets
// SO_BROADCAST and, on Linux, SO_BINDTODEVICE to the bridge, so the packet leaves on the
// chosen LAN bridge even when an uplink uses the same prefix.
func (d *Device) SendWake(bridge string, broadcast netip.Addr, packet []byte) error {
	if !device.ValidWakeBridge(bridge) || !broadcast.Is4() || !device.IsMagicPacket(packet) {
		return fmt.Errorf("wake request %q %s", bridge, broadcast)
	}
	lc := net.ListenConfig{Control: func(_, _ string, c syscall.RawConn) error {
		var serr error
		if err := c.Control(func(fd uintptr) { serr = wakeSocketOptions(fd, bridge) }); err != nil {
			return err
		}
		return serr
	}}
	ctx, cancel := context.WithTimeout(context.Background(), wakeTimeout)
	defer cancel()
	pc, err := lc.ListenPacket(ctx, "udp4", ":0")
	if err != nil {
		return err
	}
	defer pc.Close()
	if err := pc.SetWriteDeadline(time.Now().Add(wakeTimeout)); err != nil {
		return err
	}
	_, err = pc.WriteTo(packet, net.UDPAddrFromAddrPort(netip.AddrPortFrom(broadcast, device.WakePort)))
	return err
}

var _ device.WakeSender = (*Device)(nil)
