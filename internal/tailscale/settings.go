package tailscale

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/vpnkill"
	"github.com/blackdark-org/travo-greenfield/internal/vpnstatus"
)

var (
	ErrExitNodeUnknown  = errors.New("the exit node is not offered by any peer")
	ErrExitNodeConflict = errors.New("the WireGuard tunnel is enabled; it and a Tailscale exit node both claim the default route")
	ErrExitNodeOffline  = errors.New("no Internet through the exit node; the previous settings are back")
	ErrUnavailable      = errors.New("Tailscale settings are unavailable")
)

// Settings is the user-facing slice of the prefs; GET and PUT share this shape.
type Settings struct {
	ExitNode     string `json:"exit_node" maxLength:"128" doc:"Stable ID of a peer offered as exit node; empty = none"`
	AdvertiseLAN bool   `json:"advertise_lan" doc:"Advertise the lan IPv4 prefix to the tailnet (subnet router)"`
	SSH          bool   `json:"ssh" doc:"Tailscale SSH on this router"`
}

// Tunnel reads the classified WireGuard state.
type Tunnel interface {
	Status(ctx context.Context) (vpnstatus.TunnelStatus, error)
}

// Transactions begins a guarded UCI transaction the user confirms (txn.Manager).
type Transactions interface {
	Begin(ctx context.Context, summary string, configs []string, stage func(context.Context) error, probe txn.Probe) (txn.Status, error)
}

// restored marks a failure after which the previous prefs are live again: the change
// guard is released, unlike any other failure.
type restored struct{ error }

func (r restored) Unwrap() error { return r.error }

// SettingsOf is pure: the parsed backend and the lan prefix in, the user settings out.
// advertise_lan means the lan prefix is advertised; other routes do not count.
func SettingsOf(b Backend, lan string) Settings {
	return Settings{ExitNode: b.Prefs.ExitNode, SSH: b.Prefs.SSH,
		AdvertiseLAN: lan != "" && slices.Contains(b.Prefs.AdvertiseRoutes, lan)}
}

// SetSettings applies one complete settings value with `tailscale set`. A newly selected
// exit node must prove Internet within ProbeWait, else the previous prefs are restored.
func (s *Service) SetSettings(ctx context.Context, in Settings) (Status, error) {
	var routes []string
	if in.AdvertiseLAN {
		lan, err := s.lanPrefix(ctx)
		if err != nil {
			return Status{}, err
		}
		routes = []string{lan}
	}
	var want Desired
	check := func(ctx context.Context, b Backend) error {
		want = Desired{AdvertiseRoutes: routes, SSH: in.SSH}
		if in.ExitNode == "" {
			return nil
		}
		i := slices.IndexFunc(b.ExitNodes, func(n ExitNode) bool { return n.ID == in.ExitNode })
		ip, ok := "", false
		if i >= 0 {
			ip, ok = b.ExitNodes[i].IPv4()
		}
		if !ok {
			return ErrExitNodeUnknown
		}
		want.ExitNodeIP = ip
		return s.checkTunnel(ctx)
	}
	apply := func(ctx context.Context, b Backend) error {
		old := CurrentDesired(b)
		if err := s.CLI.Set(ctx, Flags(want)); err != nil {
			return err
		}
		if want.ExitNodeIP == "" || want.ExitNodeIP == old.ExitNodeIP || s.waitOnline(ctx) {
			return nil
		}
		if err := s.CLI.Set(ctx, Flags(old)); err != nil {
			return errors.Join(ErrExitNodeOffline, fmt.Errorf("restore previous prefs: %w", err))
		}
		return restored{ErrExitNodeOffline}
	}
	if err := s.changeChecked(ctx, check, apply); err != nil {
		return Status{}, err
	}
	return s.Status(ctx)
}

// checkTunnel refuses an exit node while wg0 is enabled: tailscale's rule 5270 (table 52)
// would silently take the default route from the tunnel (ADR 0017). Unreadable fails closed.
func (s *Service) checkTunnel(ctx context.Context) error {
	if s.Tunnel == nil {
		return nil
	}
	st, err := s.Tunnel.Status(ctx)
	if err != nil {
		return fmt.Errorf("%w: WireGuard status: %v", ErrUnavailable, err)
	}
	if st.State != vpnstatus.Disabled && st.State != vpnstatus.Configured {
		return ErrExitNodeConflict
	}
	return nil
}

func (s *Service) waitOnline(ctx context.Context) bool {
	wait := s.ProbeWait
	if wait <= 0 {
		wait = 20 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	for {
		if s.Online(ctx) {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(s.every()):
		}
	}
}

func (s *Service) lanPrefix(ctx context.Context) (string, error) {
	if s.UCI == nil {
		return "", ErrUnavailable
	}
	network, err := s.UCI.Show(ctx, "network")
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	p, err := LANPrefix(network)
	if err != nil {
		return "", err
	}
	return p.String(), nil
}

// firewall reports whether the owned sections match the settings. Reads only.
func (s *Service) firewallInSync(ctx context.Context, advertiseLAN bool) (bool, error) {
	if s.UCI == nil {
		return false, ErrUnavailable
	}
	current, err := s.UCI.Show(ctx, "firewall")
	if err != nil {
		return false, err
	}
	return FirewallInSync(current, FirewallSections(advertiseLAN)), nil
}

// ApplyFirewall stages the Tailscale zone and forwardings in a firewall transaction the
// user confirms (invariants §1, §7). A foreign layout is refused before any write.
func (s *Service) ApplyFirewall(ctx context.Context) (txn.Status, error) {
	if s.UCI == nil || s.Txns == nil {
		return txn.Status{}, ErrUnavailable
	}
	st, err := s.Status(ctx)
	if err != nil {
		return txn.Status{}, err
	}
	if !st.Installed {
		return txn.Status{}, errNotInstalled()
	}
	want := FirewallSections(st.Settings.AdvertiseLAN)
	current, err := s.UCI.Show(ctx, "firewall")
	if err != nil {
		return txn.Status{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	if err := CheckFirewall(current); err != nil {
		return txn.Status{}, err
	}
	stage := func(ctx context.Context) error {
		current, err := s.UCI.Show(ctx, "firewall")
		if err != nil {
			return err
		}
		if err := CheckFirewall(current); err != nil {
			return err
		}
		if FirewallInSync(current, want) {
			return txn.ErrNoChanges
		}
		return stageFirewall(ctx, s.UCI, current, want)
	}
	return s.Txns.Begin(ctx, "Tailscale firewall zone", []string{"firewall"}, stage, nil)
}

func stageFirewall(ctx context.Context, uci device.UCI, current, want []device.Section) error {
	for _, x := range slices.Backward(current) {
		if owned(x.Name) {
			if err := uci.DeleteSection(ctx, "firewall", x.Name); err != nil {
				return err
			}
		}
	}
	for _, w := range want {
		if err := vpnkill.WriteSection(ctx, uci, w); err != nil {
			return err
		}
	}
	return nil
}
