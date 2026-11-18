package mock

import (
	"fmt"
	"net/netip"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// Wake is one recorded SendWake call.
type Wake struct {
	Bridge    string
	Broadcast netip.Addr
	Packet    []byte
}

// SendWake records the request with the adapter's checks; nothing leaves the host.
func (d *Device) SendWake(bridge string, broadcast netip.Addr, packet []byte) error {
	if !device.ValidWakeBridge(bridge) || !broadcast.Is4() || !device.IsMagicPacket(packet) {
		return fmt.Errorf("wake request %q %s", bridge, broadcast)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.WakeErr != nil {
		return d.WakeErr
	}
	d.Wakes = append(d.Wakes, Wake{Bridge: bridge, Broadcast: broadcast, Packet: append([]byte(nil), packet...)})
	return nil
}

var _ device.WakeSender = (*Device)(nil)
