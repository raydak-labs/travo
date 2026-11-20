package failover

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// ErrMangleIncompatible: ip/ip6 mangle holds nftables chains or rules iptables-nft cannot
// read, so mwan3 cannot set itself up (ADR 0018 "Stale mangle base chains").
var ErrMangleIncompatible = errors.New("the mangle table holds nftables chains or rules mwan3 cannot manage; remove them before enabling failover")

type baseChain struct {
	Type, Hook string
	Prio       int
}

// iptablesMangle is the base chain iptables-nft creates per built-in mangle chain. A base
// chain in table mangle that differs (Tailscale leaves OUTPUT as `type filter`, iptables-nft
// wants `type route`) makes iptables-nft call the table incompatible: mwan3's setup batch
// then fails ("CHAIN_ADD failed (File exists): chain OUTPUT") and mwan3_hook never exists.
var iptablesMangle = map[string]baseChain{
	"PREROUTING":  {"filter", "prerouting", -150},
	"INPUT":       {"filter", "input", -150},
	"FORWARD":     {"filter", "forward", -150},
	"OUTPUT":      {"route", "output", -150},
	"POSTROUTING": {"filter", "postrouting", -150},
}

// PlanStaleBaseChains reads `nft -j list ruleset <family>` and returns the base chains of
// <family> mangle iptables-nft cannot use. Only empty ones with policy accept are returned:
// they filter nothing, so deleting them changes no traffic. One holding a rule or dropping
// refuses with ErrMangleIncompatible and nothing is deleted.
func PlanStaleBaseChains(family, ruleset string) ([]string, error) {
	var doc struct {
		Nftables []nftObject `json:"nftables"`
	}
	if err := json.Unmarshal([]byte(ruleset), &doc); err != nil {
		return nil, fmt.Errorf("nft %s ruleset: %w", family, err)
	}
	rules := map[string]int{}
	for _, o := range doc.Nftables {
		if r := o.Rule; r != nil && r.Family == family && r.Table == "mangle" {
			rules[r.Chain]++
		}
	}
	var stale []string
	for _, o := range doc.Nftables {
		c := o.Chain
		if c == nil || c.Family != family || c.Table != "mangle" || c.Hook == "" {
			continue
		}
		if want, ok := iptablesMangle[c.Name]; ok && want == (baseChain{c.Type, c.Hook, c.Prio}) {
			continue
		}
		if rules[c.Name] > 0 || c.Policy != "accept" {
			return nil, fmt.Errorf("%w: %s mangle %s (type %s hook %s, %d rules, policy %s)",
				ErrMangleIncompatible, family, c.Name, c.Type, c.Hook, rules[c.Name], c.Policy)
		}
		stale = append(stale, c.Name)
	}
	return stale, nil
}

// clearStaleBaseChains deletes the empty incompatible base chains of ip and ip6 mangle
// before mwan3 starts, then requires `iptables -t mangle -S` to read the table cleanly.
func (s *Service) clearStaleBaseChains(ctx context.Context) error {
	for _, family := range MangleFamilies {
		raw, err := s.Mangle.NftRuleset(ctx, family)
		if err != nil {
			return fmt.Errorf("nft list ruleset %s: %w", family, err)
		}
		stale, err := PlanStaleBaseChains(family, raw)
		if err != nil {
			return err
		}
		if len(stale) == 0 {
			continue
		}
		slog.Warn("failover: deleting empty mangle base chains iptables-nft cannot use", "family", family, "chains", stale)
		if err := s.Mangle.DeleteMangle(ctx, family, nil, stale); err != nil {
			return fmt.Errorf("delete stale %s mangle base chains: %w", family, err)
		}
	}
	out, err := s.Runtime.MangleRules(ctx)
	if err != nil {
		return errors.Join(ErrMangleIncompatible, err)
	}
	if strings.Contains(out, "incompatible") {
		return fmt.Errorf("%w: %s", ErrMangleIncompatible, strings.TrimSpace(out))
	}
	return nil
}
