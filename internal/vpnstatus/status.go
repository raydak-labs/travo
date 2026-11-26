// Package vpnstatus classifies WireGuard tunnel state from read-only device evidence.
package vpnstatus

import (
	"errors"
	"fmt"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

const Interface = "wg0"
const HandshakeFresh = 180 * time.Second // WireGuard REJECT_AFTER_TIME; see invariants §7.
const MaxStale = 10 * time.Minute

type State string

const (
	Disabled     State = "disabled"
	Configured   State = "configured"
	EnabledNotUp State = "enabled_not_up"
	NoHandshake  State = "no_handshake"
	Connected    State = "connected"
)

type TunnelStatus struct {
	State               State     `json:"state" enum:"disabled,configured,enabled_not_up,no_handshake,connected"`
	Stale               bool      `json:"stale" doc:"true: last good observation, the latest read failed"`
	ObservedAt          time.Time `json:"observed_at" doc:"When State was read (router clock)"`
	HandshakeAgeSeconds *int64    `json:"handshake_age_seconds" nullable:"true"`
}

var ErrUnavailable = errors.New("VPN status unavailable")

// Observation contains device evidence and the errors from obtaining it.
type Observation struct {
	Selected     bool
	Section      *device.Section
	NetifdUp     *bool
	Handshakes   []device.PeerHandshake
	SelectionErr error
	ConfigErr    error
	NetifdErr    error
	HandshakeErr error
}

// Classify is pure: observed inputs and now in, TunnelStatus out. It has no I/O.
func Classify(in Observation, now time.Time) (TunnelStatus, error) {
	if in.SelectionErr != nil {
		return TunnelStatus{}, fmt.Errorf("read profile selection: %w", in.SelectionErr)
	}
	if in.ConfigErr != nil && !errors.Is(in.ConfigErr, device.ErrNotFound) {
		return TunnelStatus{}, fmt.Errorf("read network config: %w", in.ConfigErr)
	}
	section := presentSection(in.Section)
	if errors.Is(in.ConfigErr, device.ErrNotFound) {
		section = nil
	}
	on, err := tunnelOn(section)
	if err != nil {
		return TunnelStatus{}, err
	}
	if !on {
		state := Disabled
		if in.Selected {
			state = Configured
		}
		return observed(state, now, nil), nil
	}
	if in.NetifdErr != nil {
		if errors.Is(in.NetifdErr, device.ErrNotFound) {
			return observed(EnabledNotUp, now, nil), nil
		}
		return TunnelStatus{}, fmt.Errorf("read netifd status: %w", in.NetifdErr)
	}
	if in.NetifdUp == nil {
		return TunnelStatus{}, errors.New("netifd status has no up field")
	}
	if !*in.NetifdUp {
		return observed(EnabledNotUp, now, nil), nil
	}
	return classifyHandshakes(in, now)
}

func classifyHandshakes(in Observation, now time.Time) (TunnelStatus, error) {
	if in.HandshakeErr != nil {
		if errors.Is(in.HandshakeErr, device.ErrNotFound) {
			return observed(EnabledNotUp, now, nil), nil
		}
		return TunnelStatus{}, fmt.Errorf("read WireGuard handshakes: %w", in.HandshakeErr)
	}
	var newest time.Time
	for _, peer := range in.Handshakes {
		if peer.At.After(newest) {
			newest = peer.At
		}
	}
	if newest.IsZero() {
		return observed(NoHandshake, now, nil), nil
	}
	age := now.Sub(newest)
	if age < 0 {
		return TunnelStatus{}, errors.New("newest WireGuard handshake is after now")
	}
	seconds := int64(age / time.Second)
	if age >= HandshakeFresh {
		return observed(NoHandshake, now, &seconds), nil
	}
	return observed(Connected, now, &seconds), nil
}

func presentSection(section *device.Section) *device.Section {
	if section == nil || section.Name != Interface || section.Type != "interface" || section.Options["proto"] != "wireguard" {
		return nil
	}
	return section
}

func tunnelOn(section *device.Section) (bool, error) {
	if presentSection(section) == nil {
		return false, nil
	}
	switch section.Options["disabled"] {
	case "1":
		return false, nil
	case "", "0":
		return true, nil
	default:
		return false, fmt.Errorf("network.wg0.disabled has unknown value %q", section.Options["disabled"])
	}
}

func observed(state State, now time.Time, age *int64) TunnelStatus {
	return TunnelStatus{State: state, ObservedAt: now.UTC(), HandshakeAgeSeconds: age}
}
