package dhcpserver

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"

	"github.com/blackdark-org/travo-greenfield/internal/clients"
	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
)

// ErrUnavailable: the dhcp config cannot be read.
var ErrUnavailable = errors.New("dhcp settings are unavailable")

// Applier is the slice of txn.Manager the service uses: dhcp is not connectivity-critical
// for the caller (leases run on until expiry), so it takes the direct path.
type Applier interface {
	Apply(ctx context.Context, configs []string, stage func(context.Context) error) error
}

// Service reads and changes dhcp.lan and the local DNS names.
type Service struct {
	UCI device.UCI
	// Leases is optional: without it CallerLease stays false.
	Leases device.DHCPLeaseReader
	// Reload finishes a dhcp commit whose reload failed (idempotent reload_config).
	Reload device.Reloader
	Txns   Applier
}

// errUnchanged aborts Apply before commit when nothing differs.
var errUnchanged = errors.New("dhcp unchanged")

// apply runs stage in txn.Apply. An unchanged request commits nothing but still runs the
// idempotent reload, so a retry finishes an earlier commit whose reload failed.
func (s *Service) apply(ctx context.Context, stage func(context.Context, []device.Section) (bool, error)) error {
	err := s.Txns.Apply(ctx, []string{"dhcp"}, func(ctx context.Context) error {
		dhcp, err := s.UCI.Show(ctx, "dhcp")
		if err != nil {
			return ErrUnavailable
		}
		changed, err := stage(ctx, dhcp)
		if err == nil && !changed {
			return errUnchanged
		}
		return err
	})
	if errors.Is(err, errUnchanged) && s.Reload != nil {
		if rerr := s.Reload.Reload(ctx); rerr != nil {
			return fmt.Errorf("reload: %w", rerr)
		}
		return nil
	}
	return err
}

// Settings reads dhcp.lan; the subnet and pool are left out when the lan prefix is unknown
// or network has staged changes. GET never mutates.
func (s *Service) Settings(ctx context.Context, caller netip.Addr) (DHCPView, error) {
	dhcp, err := s.UCI.Show(ctx, "dhcp")
	if err != nil {
		return DHCPView{}, ErrUnavailable
	}
	set, err := Read(dhcp)
	if err != nil {
		return DHCPView{}, err
	}
	v := DHCPView{DHCPSettings: set, CallerLease: s.callerLeased(ctx, caller)}
	if lan, err := clients.CommittedLAN(ctx, s.UCI); err == nil {
		first, last := Pool(set, lan)
		v.Subnet, v.First, v.Last = lan.Masked().String(), first.String(), last.String()
	}
	return v, nil
}

func (s *Service) callerLeased(ctx context.Context, caller netip.Addr) bool {
	if s.Leases == nil || !caller.IsValid() {
		return false
	}
	raw, err := s.Leases.DHCPLeases(ctx)
	if err != nil {
		return false
	}
	leases, err := facts.ParseLeases(string(raw))
	return err == nil && slices.ContainsFunc(leases, func(l facts.Lease) bool { return l.IP == caller.Unmap() })
}

// SetSettings validates against the committed lan prefix and writes dhcp.lan through
// txn.Apply. It returns the stored settings.
func (s *Service) SetSettings(ctx context.Context, in DHCPSettings, caller netip.Addr) (DHCPView, error) {
	err := s.apply(ctx, func(ctx context.Context, dhcp []device.Section) (bool, error) {
		lan, err := clients.CommittedLAN(ctx, s.UCI)
		if err != nil {
			return false, err
		}
		v, err := Validate(in, lan)
		if err != nil {
			return false, err
		}
		return Stage(ctx, s.UCI, dhcp, v)
	})
	if err != nil {
		return DHCPView{}, err
	}
	return s.Settings(ctx, caller)
}

// DNSNames lists every domain section.
func (s *Service) DNSNames(ctx context.Context) ([]DNSName, error) {
	dhcp, err := s.UCI.Show(ctx, "dhcp")
	if err != nil {
		return nil, ErrUnavailable
	}
	return Names(dhcp), nil
}

// AddDNSName stores name -> ip as a new travo_dns_<n> section.
func (s *Service) AddDNSName(ctx context.Context, name, ip string) (DNSName, error) {
	n, addr, err := ValidateName(name, ip)
	if err != nil {
		return DNSName{}, err
	}
	var id string
	err = s.apply(ctx, func(ctx context.Context, dhcp []device.Section) (bool, error) {
		got, serr := StageName(ctx, s.UCI, dhcp, n, addr)
		id = got
		return serr == nil, serr
	})
	if err != nil {
		return DNSName{}, err
	}
	return DNSName{ID: id, Names: []string{n}, IP: addr.String(), Owned: true}, nil
}

// RemoveDNSName deletes Travo's section id. An absent one still runs the idempotent reload
// before the not-found error, so a retry finishes a removal whose reload failed.
func (s *Service) RemoveDNSName(ctx context.Context, id string) error {
	missing := false
	err := s.apply(ctx, func(ctx context.Context, dhcp []device.Section) (bool, error) {
		err := StageRemoveName(ctx, s.UCI, dhcp, id)
		if errors.Is(err, ErrNameNotFound) {
			missing = true
			return false, nil
		}
		return err == nil, err
	})
	if err == nil && missing {
		return ErrNameNotFound
	}
	return err
}
