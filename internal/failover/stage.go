package failover

import (
	"context"
	"errors"
	"slices"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// Stage writes a plan under the caller's mwan3 lock (inside a transaction): owned values
// are set or removed, unowned options of an adopted section stay, stale stamped sections
// go, and the Travo rules move to the top so no stock catch-all rule matches first.
// ordered says the adapter reports `uci export` order; then rules already on top are not
// moved again.
func Stage(ctx context.Context, uci device.UCI, reorder device.UCIReorderer, current []device.Section, p Plan, ordered bool) error {
	for _, w := range p.Write {
		if err := writeOwned(ctx, uci, current, w); err != nil {
			return err
		}
	}
	for _, name := range p.Disable {
		i := slices.IndexFunc(current, func(c device.Section) bool { return c.Name == name })
		if i < 0 {
			continue
		}
		if err := writeOwned(ctx, uci, current, Disabled(current[i])); err != nil {
			return err
		}
	}
	for _, name := range p.Delete {
		if err := uci.DeleteSection(ctx, Config, name); err != nil {
			return err
		}
	}
	if ordered && rulesFirst(current) {
		return nil
	}
	for i, rule := range []string{RuleV4, RuleV6} {
		if err := reorder.Reorder(ctx, Config, rule, i); err != nil {
			return err
		}
	}
	return nil
}

func rulesFirst(current []device.Section) bool {
	var rules []string
	for _, c := range current {
		if c.Type == "rule" {
			rules = append(rules, c.Name)
		}
	}
	return len(rules) >= 2 && rules[0] == RuleV4 && rules[1] == RuleV6
}

// writeOwned is set-if-different, so re-applying unchanged settings stages nothing.
func writeOwned(ctx context.Context, uci device.UCI, current []device.Section, s device.Section) error {
	var cur device.Section
	if i := slices.IndexFunc(current, func(c device.Section) bool { return c.Name == s.Name }); i >= 0 {
		cur = current[i]
	} else if err := uci.SetType(ctx, Config, s.Name, s.Type); err != nil {
		return err
	}
	for _, k := range append(slices.Clone(owned[s.Type]), FingerprintOption) {
		want, has := s.Options[k]
		got, had := cur.Options[k]
		if has == had && want == got && slices.Equal(s.Lists[k], cur.Lists[k]) {
			continue
		}
		if had || len(cur.Lists[k]) > 0 {
			if err := deleteIfPresent(ctx, uci, s.Name, k); err != nil {
				return err
			}
		}
		if has {
			if err := uci.Set(ctx, Config, s.Name, k, want); err != nil {
				return err
			}
		}
		for _, v := range s.Lists[k] {
			if err := uci.AddToList(ctx, Config, s.Name, k, v); err != nil {
				return err
			}
		}
	}
	return nil
}

func deleteIfPresent(ctx context.Context, uci device.UCI, section, option string) error {
	if err := uci.DeleteOption(ctx, Config, section, option); err != nil && !errors.Is(err, device.ErrNotFound) {
		return err
	}
	return nil
}
