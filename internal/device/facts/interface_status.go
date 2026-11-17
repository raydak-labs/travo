package facts

import (
	"encoding/json"
	"fmt"
	"net/netip"
)

// InterfaceStatus is one netifd logical interface.
type InterfaceStatus struct {
	Up   bool
	IPv4 []netip.Prefix
}

// ParseInterfaceStatus parses `ubus call network.interface.<name> status`. A down
// interface omits "ipv4-address" entirely (wwan-down capture); "up" is always present.
func ParseInterfaceStatus(b []byte) (InterfaceStatus, error) {
	var raw struct {
		Up   *bool `json:"up"`
		IPv4 []struct {
			Address string `json:"address"`
			Mask    *int   `json:"mask"`
		} `json:"ipv4-address"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return InterfaceStatus{}, fmt.Errorf("interface status: %w", err)
	}
	if raw.Up == nil {
		return InterfaceStatus{}, fmt.Errorf("interface status: %w: no up field", ErrShape)
	}
	out := InterfaceStatus{Up: *raw.Up}
	for _, a := range raw.IPv4 {
		ip, err := netip.ParseAddr(a.Address)
		if err != nil || a.Mask == nil {
			return InterfaceStatus{}, fmt.Errorf("interface status: %w: bad address", ErrShape)
		}
		pfx := netip.PrefixFrom(ip, *a.Mask)
		if !pfx.IsValid() {
			return InterfaceStatus{}, fmt.Errorf("interface status: %w: bad mask %d", ErrShape, *a.Mask)
		}
		out.IPv4 = append(out.IPv4, pfx)
	}
	return out, nil
}
