package clients

import (
	"context"
	"errors"
	"fmt"
	"net/netip"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// KickGuard is held while hostapd deauthenticates a station. del_client is atomic in hostapd
// and leaves no config behind, so the guard is released on every return (invariants §1,
// named exception).
const KickGuard = "client-kick-in-progress"

// ListReservations reads every dhcp host MAC.
func (s *Service) ListReservations(ctx context.Context) ([]Reservation, error) {
	dhcp, err := s.UCI.Show(ctx, "dhcp")
	if err != nil {
		return nil, ErrUnavailable
	}
	return Reservations(dhcp), nil
}

// Reserve stores a DHCP reservation through the direct dhcp path (txn.Apply): dhcp cannot cut
// the caller off at once; the new address applies on the client's next lease renewal.
// It returns the stored reservation; an unchanged one writes nothing.
func (s *Service) Reserve(ctx context.Context, req Reservation) (Reservation, error) {
	network, err := s.UCI.Show(ctx, "network")
	if err != nil {
		return Reservation{}, ErrUnavailable
	}
	lan, err := LANPrefix(network)
	if err != nil {
		return Reservation{}, err
	}
	var stored Reservation
	err = s.Txns.Apply(ctx, []string{"dhcp"}, func(ctx context.Context) error {
		dhcp, err := s.UCI.Show(ctx, "dhcp")
		if err != nil {
			return ErrUnavailable
		}
		r, err := ValidateReservation(req, lan, Reservations(dhcp))
		if err != nil {
			return err
		}
		changed, err := StageReservation(ctx, s.UCI, dhcp, r)
		if err != nil {
			return err
		}
		stored = r
		if !changed {
			return errUnchanged
		}
		return nil
	})
	if errors.Is(err, errUnchanged) {
		err = nil
	}
	if err != nil {
		return Reservation{}, err
	}
	return s.find(ctx, stored.MAC)
}

// errUnchanged aborts Apply before commit when nothing differs (no reload, no dnsmasq restart).
var errUnchanged = errors.New("reservation unchanged")

func (s *Service) find(ctx context.Context, mac string) (Reservation, error) {
	all, err := s.ListReservations(ctx)
	if err != nil {
		return Reservation{}, err
	}
	for _, r := range all {
		if r.MAC == mac {
			return r, nil
		}
	}
	return Reservation{}, ErrReservationNotFound
}

// Unreserve removes mac from its host section (txn.Apply).
func (s *Service) Unreserve(ctx context.Context, mac string) error {
	m, ok := NormalizeMAC(mac)
	if !ok {
		return &FieldError{"mac", "not a unicast MAC address"}
	}
	return s.Txns.Apply(ctx, []string{"dhcp"}, func(ctx context.Context) error {
		dhcp, err := s.UCI.Show(ctx, "dhcp")
		if err != nil {
			return ErrUnavailable
		}
		return StageUnreserve(ctx, s.UCI, dhcp, m)
	})
}

// SetBlocked adds or removes the block rule in a firewall transaction the user confirms.
// Blocking the caller's own MAC is refused before any write.
func (s *Service) SetBlocked(ctx context.Context, mac string, blocked bool, caller netip.Addr) (txn.Status, error) {
	m, ok := NormalizeMAC(mac)
	if !ok {
		return txn.Status{}, &FieldError{"mac", "not a unicast MAC address"}
	}
	summary := "Unblock client " + m
	if blocked {
		if err := s.refuseCaller(ctx, m, caller); err != nil {
			return txn.Status{}, err
		}
		summary = "Block client " + m
	}
	st, err := s.Txns.Begin(ctx, summary, []string{"firewall"}, func(ctx context.Context) error {
		fw, err := s.UCI.Show(ctx, "firewall")
		if err != nil {
			return ErrUnavailable
		}
		stage := StageUnblock
		if blocked {
			stage = StageBlock
		}
		changed, err := stage(ctx, s.UCI, fw, m)
		if err == nil && !changed {
			return txn.ErrNoChanges
		}
		return err
	}, nil)
	if errors.Is(err, txn.ErrActive) {
		return st, ErrTransactionPending
	}
	return st, err
}

// Kick deauthenticates mac from every enabled AP that lists it. It changes no config, so it
// is not a transaction; it runs under KickGuard and never kicks the caller.
func (s *Service) Kick(ctx context.Context, mac string, caller netip.Addr) error {
	m, ok := NormalizeMAC(mac)
	if !ok {
		return &FieldError{"mac", "not a unicast MAC address"}
	}
	if err := s.refuseCaller(ctx, m, caller); err != nil {
		return err
	}
	aps, err := s.stations(ctx)
	if err != nil && len(aps) == 0 {
		return ErrUnavailable
	}
	var targets []string
	for _, ap := range aps {
		for _, st := range ap.Stations {
			if st.MAC == m {
				targets = append(targets, ap.Ifname)
			}
		}
	}
	if len(targets) == 0 {
		return ErrNotAssociated
	}
	if err := s.acquireKickGuard(); err != nil {
		return err
	}
	defer func() { _ = s.Guards.Release(KickGuard) }()
	kicked := 0
	for _, ifname := range targets {
		err := s.Kicker.KickStation(ctx, ifname, m)
		if errors.Is(err, device.ErrNotFound) {
			continue // left between the read and the call
		}
		if err != nil {
			return err
		}
		kicked++
	}
	if kicked == 0 {
		return ErrNotAssociated
	}
	return nil
}

// acquireKickGuard writes the guard first; a write failure aborts the kick (invariants §1).
// A guard left by a crash is cleared by this explicit kick, its retry path.
func (s *Service) acquireKickGuard() error {
	err := s.Guards.Acquire(KickGuard)
	if errors.Is(err, guard.ErrActive) {
		if err := s.Guards.Release(KickGuard); err != nil {
			return fmt.Errorf("kick guard: %w", err)
		}
		err = s.Guards.Acquire(KickGuard)
	}
	if err != nil {
		return fmt.Errorf("kick guard: %w", err)
	}
	return nil
}
