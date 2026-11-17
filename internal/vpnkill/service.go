package vpnkill

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/vpnstatus"
)

var (
	ErrUnavailable = errors.New("VPN kill switch is unavailable")
	// ErrNotConnected: enabling needs a proven tunnel, or LAN clients lose internet at once.
	ErrNotConnected = errors.New("the tunnel must be connected to enable the kill switch")
)

type Tunnel interface {
	Status(ctx context.Context) (vpnstatus.TunnelStatus, error)
}

type Transactions interface {
	Begin(ctx context.Context, summary string, configs []string, stage func(context.Context) error, probe txn.Probe) (txn.Status, error)
}

type Service struct {
	UCI    device.UCI
	Tunnel Tunnel
	Txns   Transactions
	// ProbeWait bounds how long the enable probe waits for a connected tunnel after reload.
	ProbeWait time.Duration
}

type KillSwitchState struct {
	Enabled bool `json:"enabled" doc:"Kill switch rules are configured"`
	// Zone: the travo_vpn zone and lan forwarding exist, so LAN clients can use the tunnel.
	Zone bool `json:"zone"`
}

// Status reads UCI only; GET never mutates.
func (s *Service) Status(ctx context.Context) (KillSwitchState, error) {
	if s.UCI == nil {
		return KillSwitchState{}, ErrUnavailable
	}
	current, err := s.UCI.Show(ctx, "firewall")
	if err != nil {
		return KillSwitchState{}, ErrUnavailable
	}
	zone := slices.ContainsFunc(current, func(x device.Section) bool { return x.Name == ZoneName }) &&
		slices.ContainsFunc(current, func(x device.Section) bool { return x.Name == LANForward })
	return KillSwitchState{Enabled: Enabled(current), Zone: zone}, nil
}

// Set enables or disables the kill switch in a firewall transaction the user confirms.
func (s *Service) Set(ctx context.Context, enabled bool) (txn.Status, error) {
	if s.UCI == nil || s.Txns == nil || s.Tunnel == nil {
		return txn.Status{}, ErrUnavailable
	}
	if !enabled {
		return s.Txns.Begin(ctx, "Disable VPN kill switch", []string{"firewall"}, s.stageDisable, nil)
	}
	st, err := s.Tunnel.Status(ctx)
	if err != nil {
		return txn.Status{}, ErrUnavailable
	}
	if st.Stale || st.State != vpnstatus.Connected {
		return txn.Status{}, ErrNotConnected
	}
	return s.Txns.Begin(ctx, "Enable VPN kill switch", []string{"firewall"}, s.stageEnable, s.proveConnected)
}

func (s *Service) stageEnable(ctx context.Context) error {
	current, err := s.UCI.Show(ctx, "firewall")
	if err != nil {
		return ErrUnavailable
	}
	want, err := Desired(current)
	if err != nil {
		return err
	}
	if Matches(current, want) {
		return txn.ErrNoChanges
	}
	if err := checkOwnedTypes(current, want); err != nil {
		return err
	}
	// Deleting last to first keeps anonymous selectors valid; owned sections are named anyway.
	for _, x := range slices.Backward(current) {
		if Owned(x.Name) {
			if err := s.UCI.DeleteSection(ctx, "firewall", x.Name); err != nil {
				return ErrUnavailable
			}
		}
	}
	for _, x := range want {
		if err := s.write(ctx, x); err != nil {
			return ErrUnavailable
		}
	}
	return nil
}

// stageDisable removes only the rules; the VPN zone keeps LAN traffic flowing into the tunnel.
func (s *Service) stageDisable(ctx context.Context) error {
	current, err := s.UCI.Show(ctx, "firewall")
	if err != nil {
		return ErrUnavailable
	}
	if !Enabled(current) {
		return txn.ErrNoChanges
	}
	for _, x := range slices.Backward(current) {
		if x.Type == "rule" && strings.HasPrefix(x.Name, RulePrefix) {
			if err := s.UCI.DeleteSection(ctx, "firewall", x.Name); err != nil {
				return ErrUnavailable
			}
		}
	}
	return nil
}

func checkOwnedTypes(current, want []device.Section) error {
	types := make(map[string]string, len(want))
	for _, x := range want {
		types[x.Name] = x.Type
	}
	for _, x := range current {
		if t, ok := types[x.Name]; ok && x.Type != t {
			return ErrUnsupported // a foreign section squats on an owned name
		}
	}
	return nil
}

func (s *Service) write(ctx context.Context, x device.Section) error {
	return writeSection(ctx, s.UCI, x)
}

// ZoneMissing reports whether the VPN zone or its lan forwarding is absent or differs.
// A foreign section on one of their names is ErrUnsupported.
func ZoneMissing(current []device.Section) (bool, error) {
	want := VPNZone()
	if err := checkOwnedTypes(current, want); err != nil {
		return false, err
	}
	for _, w := range want {
		i := slices.IndexFunc(current, func(x device.Section) bool { return x.Name == w.Name })
		if i < 0 || !Matches([]device.Section{current[i]}, []device.Section{w}) {
			return true, nil
		}
	}
	return false, nil
}

// StageZone (re)writes the VPN zone and lan forwarding, nothing else. The caller holds the
// firewall lock inside its transaction (wg0 enable stages it with the tunnel).
func StageZone(ctx context.Context, uci device.UCI, current []device.Section) error {
	for _, w := range VPNZone() {
		if slices.ContainsFunc(current, func(x device.Section) bool { return x.Name == w.Name }) {
			if err := uci.DeleteSection(ctx, "firewall", w.Name); err != nil {
				return err
			}
		}
		if err := writeSection(ctx, uci, w); err != nil {
			return err
		}
	}
	return nil
}

func writeSection(ctx context.Context, uci device.UCI, x device.Section) error {
	if err := uci.SetType(ctx, "firewall", x.Name, x.Type); err != nil {
		return err
	}
	for _, k := range slices.Sorted(maps.Keys(x.Options)) {
		if err := uci.Set(ctx, "firewall", x.Name, k, x.Options[k]); err != nil {
			return err
		}
	}
	for _, k := range slices.Sorted(maps.Keys(x.Lists)) {
		for _, v := range x.Lists[k] {
			if err := uci.AddToList(ctx, "firewall", x.Name, k, v); err != nil {
				return err
			}
		}
	}
	return nil
}

// proveConnected: a firewall reload must not break the tunnel the kill switch now depends on.
func (s *Service) proveConnected(ctx context.Context) error {
	wait := s.ProbeWait
	if wait <= 0 {
		wait = 20 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if st, err := s.Tunnel.Status(ctx); err == nil && !st.Stale && st.State == vpnstatus.Connected {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.Join(ErrNotConnected, ctx.Err())
		case <-ticker.C:
		}
	}
}
