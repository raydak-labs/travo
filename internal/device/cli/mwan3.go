package cli

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

func (d *Device) MWAN3Interfaces(ctx context.Context) (string, error) {
	return d.run(ctx, TierRead, "mwan3", "interfaces")
}

func (d *Device) MangleRules(ctx context.Context) (string, error) {
	return d.run(ctx, TierRead, "iptables", "-t", "mangle", "-w", "-S")
}

var _ device.MWAN3Reader = (*Device)(nil)

// nftName: chain names go into an nft script; nft's own identifier characters only.
var nftName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)

func (d *Device) NftRuleset(ctx context.Context, family string) (string, error) {
	if family != "ip" && family != "ip6" {
		return "", fmt.Errorf("nft family %q", family)
	}
	return d.run(ctx, TierRead, "nft", "-j", "list", "ruleset", family)
}

// DeleteMangle feeds one script to `nft -f -`, which nft applies as one transaction.
func (d *Device) DeleteMangle(ctx context.Context, family string, rules []device.NftRule, chains []string) error {
	script, err := MangleDeleteScript(family, rules, chains)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, TierWrite)
	defer cancel()
	_, _, err = d.RunInput(ctx, script, "nft", "-f", "-")
	return err
}

// MangleDeleteScript: rules first (they may jump into the chains), then every chain is
// flushed before any is deleted (mwan3 chains jump into each other). Verified in the
// container (testdata/container/openwrt-25.12.6/mwan3/PROVENANCE.md).
func MangleDeleteScript(family string, rules []device.NftRule, chains []string) (string, error) {
	if family != "ip" && family != "ip6" {
		return "", fmt.Errorf("nft family %q", family)
	}
	var b strings.Builder
	for _, r := range rules {
		if !nftName.MatchString(r.Chain) || r.Handle <= 0 {
			return "", fmt.Errorf("nft rule %s handle %d", r.Chain, r.Handle)
		}
		fmt.Fprintf(&b, "delete rule %s mangle %s handle %d\n", family, r.Chain, r.Handle)
	}
	for _, verb := range []string{"flush", "delete"} {
		for _, c := range chains {
			if !nftName.MatchString(c) {
				return "", fmt.Errorf("nft chain %q", c)
			}
			fmt.Fprintf(&b, "%s chain %s mangle %s\n", verb, family, c)
		}
	}
	return b.String(), nil
}

func (d *Device) DeleteEmptyMangleChains(ctx context.Context, family string, chains []string) error {
	script, err := EmptyChainDeleteScript(family, chains)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, TierWrite)
	defer cancel()
	_, _, err = d.RunInput(ctx, script, "nft", "-f", "-")
	return err
}

// EmptyChainDeleteScript is `delete chain` only, never `flush`: nft fails a delete of a
// non-empty chain ("Device or resource busy"), and the whole `nft -f` transaction with it.
func EmptyChainDeleteScript(family string, chains []string) (string, error) {
	if family != "ip" && family != "ip6" {
		return "", fmt.Errorf("nft family %q", family)
	}
	var b strings.Builder
	for _, c := range chains {
		if !nftName.MatchString(c) {
			return "", fmt.Errorf("nft chain %q", c)
		}
		fmt.Fprintf(&b, "delete chain %s mangle %s\n", family, c)
	}
	return b.String(), nil
}

var _ device.MangleCleaner = (*Device)(nil)
