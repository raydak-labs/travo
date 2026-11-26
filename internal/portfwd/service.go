package portfwd

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/clients"
	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// ErrUnavailable: the firewall config cannot be read.
var ErrUnavailable = errors.New("port forwards are unavailable")

// ErrTransactionPending: another guarded transaction waits for its confirm.
var ErrTransactionPending = errors.New("a transaction is already pending")

// Transactions is the slice of txn.Manager the service uses. The firewall is a connectivity
// config: every change is a guarded transaction the user confirms.
type Transactions interface {
	Begin(ctx context.Context, summary string, configs []string, stage func(context.Context) error, probe txn.Probe) (txn.Status, error)
}

// Service reads and changes port forwards.
type Service struct {
	UCI  device.UCI
	Txns Transactions
}

// Forwards lists every redirect section. GET never mutates.
func (s *Service) Forwards(ctx context.Context) ([]Forward, error) {
	fw, err := s.UCI.Show(ctx, "firewall")
	if err != nil {
		return nil, ErrUnavailable
	}
	return List(fw), nil
}

// Add validates in against the committed lan prefix and stages a travo_fwd_<n> section in a
// firewall transaction the user confirms.
func (s *Service) Add(ctx context.Context, in ForwardRequest) (txn.Status, error) {
	lan, err := clients.CommittedLAN(ctx, s.UCI)
	if err != nil {
		return txn.Status{}, err
	}
	v, err := Validate(in, lan)
	if err != nil {
		return txn.Status{}, err
	}
	summary := "Forward " + strings.Join(v.Protos, "/") + " " + v.External.String() + " to " + v.Target.String()
	return s.begin(ctx, summary, func(ctx context.Context, fw []device.Section) error {
		return Stage(ctx, s.UCI, fw, v)
	})
}

// Remove deletes Travo's section id in a firewall transaction the user confirms.
func (s *Service) Remove(ctx context.Context, id string) (txn.Status, error) {
	return s.begin(ctx, "Remove port forward "+id, func(ctx context.Context, fw []device.Section) error {
		return StageRemove(ctx, s.UCI, fw, id)
	})
}

// begin re-reads the firewall under the transaction lock before stage.
func (s *Service) begin(ctx context.Context, summary string, stage func(context.Context, []device.Section) error) (txn.Status, error) {
	if s.Txns == nil {
		return txn.Status{}, ErrUnavailable
	}
	st, err := s.Txns.Begin(ctx, summary, []string{"firewall"}, func(ctx context.Context) error {
		fw, err := s.UCI.Show(ctx, "firewall")
		if err != nil {
			return ErrUnavailable
		}
		return stage(ctx, fw)
	}, nil)
	if errors.Is(err, txn.ErrActive) {
		return st, ErrTransactionPending
	}
	return st, err
}

// Stage writes v as the lowest free travo_fwd_<n>.
func Stage(ctx context.Context, uci device.UCI, fw []device.Section, v Valid) error {
	if !zonesPresent(fw) {
		return ErrNoZone
	}
	if Conflict(fw, v) {
		return ErrPortInUse
	}
	if countOwned(fw) >= MaxOwned {
		return ErrFull
	}
	id := freeSection(fw)
	opts := [][2]string{
		{"name", v.Name}, {"target", "DNAT"}, {"src", WANZone}, {"dest", LANZone},
		{"proto", strings.Join(v.Protos, " ")}, {"src_dport", v.External.String()},
		{"dest_ip", v.Target.String()}, {"enabled", "1"},
	}
	if v.TargetPort != "" {
		opts = append(opts, [2]string{"dest_port", v.TargetPort})
	}
	if err := uci.SetType(ctx, "firewall", id, "redirect"); err != nil {
		return err
	}
	for _, o := range opts {
		if err := uci.Set(ctx, "firewall", id, o[0], o[1]); err != nil {
			return err
		}
	}
	return nil
}

// StageRemove deletes Travo's section id; a foreign redirect is refused.
func StageRemove(ctx context.Context, uci device.UCI, fw []device.Section, id string) error {
	at := slices.IndexFunc(fw, func(s device.Section) bool { return s.Name == id })
	if at < 0 {
		return ErrNotFound
	}
	if !owned(fw[at]) {
		return ErrForeign
	}
	return uci.DeleteSection(ctx, "firewall", id)
}

func countOwned(fw []device.Section) int {
	n := 0
	for _, s := range fw {
		if owned(s) {
			n++
		}
	}
	return n
}

// freeSection is the lowest travo_fwd_<n> no section uses (of any type).
func freeSection(fw []device.Section) string {
	for n := 1; ; n++ {
		id := Prefix + strconv.Itoa(n)
		if !slices.ContainsFunc(fw, func(s device.Section) bool { return s.Name == id }) {
			return id
		}
	}
}
