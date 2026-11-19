package failover

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// mwan3Prefix names every chain mwan3 creates in mangle (mwan3_hook, mwan3_rules,
// mwan3_policy_<name>, ...; testdata/container/openwrt-25.12.6/mwan3/mangle-online.txt).
const mwan3Prefix = "mwan3_"

// MangleFamilies are the iptables-nft tables mwan3 writes (iptables and ip6tables).
var MangleFamilies = []string{"ip", "ip6"}

// hookChains are the base chains mwan3 hooks into with `-j mwan3_hook`.
var hookChains = []string{"PREROUTING", "OUTPUT", "POSTROUTING"}

// ErrForeignJump: a chain mwan3 did not create jumps into a mwan3 chain; deleting the
// chain would break it, so nothing is deleted.
var ErrForeignJump = errors.New("a foreign mangle rule jumps into a mwan3 chain")

// MangleLeftovers is what mwan3 left in one family's mangle table.
type MangleLeftovers struct {
	Rules  []device.NftRule
	Chains []string
}

func (l MangleLeftovers) Empty() bool { return len(l.Rules) == 0 && len(l.Chains) == 0 }

type nftObject struct {
	Chain *struct {
		Family string `json:"family"`
		Table  string `json:"table"`
		Name   string `json:"name"`
	} `json:"chain"`
	Rule *struct {
		Family string            `json:"family"`
		Table  string            `json:"table"`
		Chain  string            `json:"chain"`
		Handle int               `json:"handle"`
		Expr   []json.RawMessage `json:"expr"`
	} `json:"rule"`
}

type nftVerdict struct {
	Jump *struct {
		Target string `json:"target"`
	} `json:"jump"`
	Goto *struct {
		Target string `json:"target"`
	} `json:"goto"`
}

// PlanMangleCleanup reads `nft -j list ruleset <family>` and returns the mwan3 chains of
// <family> mangle plus the rules in PREROUTING/OUTPUT/POSTROUTING that jump into them.
// Every other chain and rule stays (Tailscale's connmark chains among them); a foreign
// rule elsewhere that jumps into a mwan3 chain refuses the whole cleanup.
func PlanMangleCleanup(family, ruleset string) (MangleLeftovers, error) {
	var doc struct {
		Nftables []nftObject `json:"nftables"`
	}
	if err := json.Unmarshal([]byte(ruleset), &doc); err != nil {
		return MangleLeftovers{}, fmt.Errorf("nft %s ruleset: %w", family, err)
	}
	var out MangleLeftovers
	for _, o := range doc.Nftables {
		if c := o.Chain; c != nil && c.Family == family && c.Table == "mangle" && strings.HasPrefix(c.Name, mwan3Prefix) {
			out.Chains = append(out.Chains, c.Name)
		}
		r := o.Rule
		if r == nil || r.Family != family || r.Table != "mangle" || strings.HasPrefix(r.Chain, mwan3Prefix) || !jumpsToMWAN3(r.Expr) {
			continue
		}
		if !slices.Contains(hookChains, r.Chain) {
			return MangleLeftovers{}, fmt.Errorf("%w: %s mangle %s handle %d", ErrForeignJump, family, r.Chain, r.Handle)
		}
		out.Rules = append(out.Rules, device.NftRule{Chain: r.Chain, Handle: r.Handle})
	}
	return out, nil
}

func jumpsToMWAN3(expr []json.RawMessage) bool {
	for _, e := range expr {
		var v nftVerdict
		if json.Unmarshal(e, &v) != nil {
			continue
		}
		for _, t := range []*struct {
			Target string `json:"target"`
		}{v.Jump, v.Goto} {
			if t != nil && strings.HasPrefix(t.Target, mwan3Prefix) {
				return true
			}
		}
	}
	return false
}

// cleanMangle removes mwan3's leftovers from ip and ip6 mangle once mwan3 is stopped: its
// own stop finds what to delete with `iptables -S`, which fails while a native nft rule
// sits in the table (ADR 0018). Idempotent; the re-read afterwards must find nothing.
func (s *Service) cleanMangle(ctx context.Context) error {
	for _, family := range MangleFamilies {
		left, err := s.mangleLeftovers(ctx, family)
		if err != nil {
			return err
		}
		if left.Empty() {
			continue
		}
		if err := s.Mangle.DeleteMangle(ctx, family, left.Rules, left.Chains); err != nil {
			return fmt.Errorf("delete mwan3 leftovers in %s mangle: %w", family, err)
		}
		if after, err := s.mangleLeftovers(ctx, family); err != nil || !after.Empty() {
			return errors.Join(fmt.Errorf("mwan3 leftovers remain in %s mangle: %+v", family, after), err)
		}
	}
	return nil
}

func (s *Service) mangleLeftovers(ctx context.Context, family string) (MangleLeftovers, error) {
	raw, err := s.Mangle.NftRuleset(ctx, family)
	if err != nil {
		return MangleLeftovers{}, fmt.Errorf("nft list ruleset %s: %w", family, err)
	}
	return PlanMangleCleanup(family, raw)
}
