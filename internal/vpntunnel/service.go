// Package vpntunnel applies and verifies guarded WireGuard tunnel changes.
package vpntunnel

import (
	"context"
	"errors"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/vpnkill"
	"github.com/blackdark-org/travo-greenfield/internal/vpnprofiles"
)

const iface = "wg0"

var errUnavailable = errors.New("WireGuard tunnel changes are unavailable")
var errProof = errors.New("could not verify WireGuard tunnel change")

type ProfileStore interface {
	SelectedConfig() (vpnprofiles.Config, error)
}

type Transactions interface {
	BeginWireGuard(context.Context, string, func(context.Context) error, txn.Probe) (txn.Status, error)
	BeginWireGuardWith(context.Context, string, []string, func(context.Context) error, txn.Probe) (txn.Status, error)
}

type Service struct {
	uci        device.UCI
	keys       device.WireGuardKeyWriter
	ubus       device.Ubus
	control    device.WireGuardControl
	handshakes device.WireGuardReader
	routes     device.DefaultRouteReader
	profiles   ProfileStore
	txns       Transactions
	now        func() time.Time
}

func New(uci device.UCI, keys device.WireGuardKeyWriter, ubus device.Ubus, control device.WireGuardControl, handshakes device.WireGuardReader, routes device.DefaultRouteReader, profiles ProfileStore, txns Transactions, now func() time.Time) *Service {
	return &Service{uci, keys, ubus, control, handshakes, routes, profiles, txns, now}
}

func (s *Service) Set(ctx context.Context, enabled bool) (txn.Status, error) {
	if s.txns == nil || s.uci == nil || s.control == nil || s.ubus == nil || s.routes == nil ||
		(enabled && (s.keys == nil || s.profiles == nil || s.handshakes == nil || s.now == nil)) {
		return txn.Status{}, errUnavailable
	}
	if !enabled {
		return s.txns.BeginWireGuard(ctx, "Disable WireGuard tunnel", stageBounded(s.stageDisable), s.proveDisabled)
	}

	profile, err := s.profiles.SelectedConfig()
	if err != nil {
		return txn.Status{}, err
	}
	desired, err := desiredSections(profile)
	if err != nil {
		return txn.Status{}, errUnavailable
	}
	var proof enableProof
	networkChanged := true
	// The VPN zone lets LAN clients use wg0 (device-facts §1, #87): it is staged with the
	// tunnel in one transaction, independent of the kill switch.
	return s.txns.BeginWireGuardWith(ctx, "Enable WireGuard tunnel", []string{"firewall"}, stageBounded(func(ctx context.Context) error {
		current, err := s.uci.Show(ctx, "network")
		if err != nil {
			return errUnavailable
		}
		firewall, err := s.uci.Show(ctx, "firewall")
		if err != nil {
			return errUnavailable
		}
		zoneMissing, err := vpnkill.ZoneMissing(firewall)
		if err != nil {
			return errUnavailable
		}
		wasEnabled, err := configuredEnabled(current)
		if err != nil {
			return err
		}
		if err := checkPeerNames(current, desired[1:]); err != nil {
			return err
		}
		split, err := currentSplit(current)
		if err != nil {
			return err
		}
		if len(split) > 0 { // the split selection survives a profile (re)apply
			desired = withRouteAll(desired, "0")
		}
		networkChanged = !matches(current, desired, wasEnabled)
		if !networkChanged && !zoneMissing {
			return txn.ErrNoChanges
		}
		if zoneMissing {
			if err := vpnkill.StageZone(ctx, s.uci, firewall); err != nil {
				return errUnavailable
			}
		}
		if !networkChanged {
			return nil // tunnel already as wanted: only the zone was missing
		}

		proof.started = s.now().Unix()
		proof.wasEnabled = wasEnabled
		previous, err := s.handshakes.LatestHandshakes(ctx, iface)
		if err != nil && !errors.Is(err, device.ErrNotFound) {
			return errUnavailable
		}
		proof.previous = make(map[string]time.Time, len(previous))
		for _, peer := range previous {
			proof.previous[peer.PublicKey] = peer.At
		}
		proof.configured = make(map[string]struct{}, len(profile.Peers))
		for _, peer := range profile.Peers {
			proof.configured[peer.PublicKey] = struct{}{}
		}
		return s.stageEnable(ctx, current, desired)
	}), func(ctx context.Context) error {
		if !networkChanged {
			return nil // a firewall-only change leaves wg0 untouched
		}
		return s.proveEnabled(ctx, proof)
	})
}

func stageBounded(stage func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		if err := stage(ctx); err != nil {
			return err
		}
		return ctx.Err()
	}
}

func (s *Service) stageDisable(ctx context.Context) error {
	current, err := s.uci.Show(ctx, "network")
	if err != nil {
		return errUnavailable
	}
	enabled, err := configuredEnabled(current)
	if err != nil {
		return err
	}
	if !enabled {
		return txn.ErrNoChanges
	}
	if err := s.uci.Set(ctx, "network", iface, "disabled", "1"); err != nil {
		return errUnavailable
	}
	return nil
}
