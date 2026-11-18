// Package wol sends Wake-on-LAN magic packets to LAN clients. It changes no router state:
// no UCI, no guard, no transaction. The packet goes as an IPv4 UDP broadcast to port 9 on
// each LAN bridge (lan, guest) that is up, or on the one the caller names.
package wol

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"slices"
	"strings"
	"sync"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
)

var (
	ErrInvalidMAC       = errors.New("mac must be a unicast Ethernet address")
	ErrInvalidInterface = errors.New("interface is not an up LAN bridge")
	ErrBusy             = errors.New("another wake request is running")
	ErrUnavailable      = errors.New("wake-on-lan is unavailable")
)

// uplinkInterfaces never carry a wake packet (GLOSSARY "Uplink"), even when bridged.
var uplinkInterfaces = []string{"wan", "wan6", "wwan", "usbtether"}

// Target is one broadcast domain: a netifd interface, its bridge and an IPv4 broadcast.
type Target struct {
	Interface string
	Bridge    string
	Broadcast netip.Addr
}

// Wake is one finished request: the normalised MAC and the interfaces the packet left on.
type Wake struct {
	MAC        string   `json:"mac" doc:"Target MAC, lower case with colons"`
	Interfaces []string `json:"interfaces" nullable:"false" doc:"netifd interfaces the magic packet was broadcast on"`
}

// Service serialises requests: one at a time across all callers.
type Service struct {
	Ubus   device.Ubus
	Sender device.WakeSender
	mu     sync.Mutex
}

// Wake validates the MAC and the optional interface, then broadcasts the magic packet on
// every matching target. It fails only when no packet left the router.
func (s *Service) Wake(ctx context.Context, mac, iface string) (Wake, error) {
	hw, err := ParseMAC(mac)
	if err != nil {
		return Wake{}, err
	}
	if !s.mu.TryLock() {
		return Wake{}, ErrBusy
	}
	defer s.mu.Unlock()
	raw, err := s.Ubus.Call(ctx, "network.interface", "dump", nil)
	if err != nil {
		return Wake{}, errors.Join(ErrUnavailable, err)
	}
	dump, err := facts.ParseInterfaceDump(raw)
	if err != nil {
		return Wake{}, errors.Join(ErrUnavailable, err)
	}
	targets := Targets(dump, iface)
	if len(targets) == 0 {
		if iface != "" {
			return Wake{}, ErrInvalidInterface
		}
		return Wake{}, ErrUnavailable
	}
	return s.send(hw, targets)
}

func (s *Service) send(hw net.HardwareAddr, targets []Target) (Wake, error) {
	packet := device.MagicPacket(hw)
	res := Wake{MAC: hw.String(), Interfaces: []string{}}
	var errs []error
	for _, t := range targets {
		if err := s.Sender.SendWake(t.Bridge, t.Broadcast, packet); err != nil {
			errs = append(errs, err)
			continue
		}
		if !slices.Contains(res.Interfaces, t.Interface) {
			res.Interfaces = append(res.Interfaces, t.Interface)
		}
	}
	if len(res.Interfaces) == 0 {
		return Wake{}, errors.Join(append([]error{ErrUnavailable}, errs...)...)
	}
	return res, nil
}

// ParseMAC accepts a 6-byte unicast MAC with ':' or '-' separators; zero is refused.
func ParseMAC(s string) (net.HardwareAddr, error) {
	if len(s) != 17 || strings.Count(s, ":")+strings.Count(s, "-") != 5 {
		return nil, ErrInvalidMAC
	}
	hw, err := net.ParseMAC(s)
	if err != nil || len(hw) != 6 || hw[0]&1 == 1 || slices.Equal(hw, make(net.HardwareAddr, 6)) {
		return nil, ErrInvalidMAC
	}
	return hw, nil
}

// Targets keeps up, non-uplink interfaces on a bridge (br-lan, br-guest) with the broadcast
// of each IPv4 prefix shorter than /31; only iface when it is set.
func Targets(dump []facts.Interface, iface string) []Target {
	var out []Target
	for _, it := range dump {
		if !it.Up || slices.Contains(uplinkInterfaces, it.Name) || !device.ValidWakeBridge(it.L3Device) {
			continue
		}
		if iface != "" && it.Name != iface {
			continue
		}
		for _, p := range it.IPv4 {
			if b, ok := Broadcast(p); ok {
				out = append(out, Target{Interface: it.Name, Bridge: it.L3Device, Broadcast: b})
			}
		}
	}
	return out
}

// Broadcast is the last address of an IPv4 prefix; /31 and /32 have none.
func Broadcast(p netip.Prefix) (netip.Addr, bool) {
	if !p.Addr().Is4() || p.Bits() < 0 || p.Bits() > 30 {
		return netip.Addr{}, false
	}
	a := p.Addr().As4()
	host := uint32(1)<<(32-p.Bits()) - 1
	v := uint32(a[0])<<24 | uint32(a[1])<<16 | uint32(a[2])<<8 | uint32(a[3]) | host
	return netip.AddrFrom4([4]byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}), true
}
