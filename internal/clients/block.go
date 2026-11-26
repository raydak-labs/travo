package clients

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// BlockPrefix names the firewall rules Travo owns: travo_block_<mac without colons>.
const BlockPrefix = "travo_block_"

// ErrBlockConflict: a section that is not Travo's block rule uses the owned name.
var ErrBlockConflict = errors.New("a foreign firewall section uses the block rule's name")

// BlockSection is the firewall rule section for mac.
func BlockSection(mac string) string { return BlockPrefix + strings.ReplaceAll(mac, ":", "") }

// BlockRule rejects every forwarded packet from mac, both families. `src '*'` and `dest '*'`
// put it in fw4's global forward chain before the zone dispatch (testdata
// clients/fw4-print-block.txt), so no user forwarding is edited and input (the admin UI on
// br-lan) stays open: a blocked device can still reach the router.
func BlockRule(mac string) device.Section {
	return device.Section{Name: BlockSection(mac), Type: "rule",
		Options: map[string]string{"name": "Travo block " + mac, "src": "*", "dest": "*", "proto": "all", "target": "REJECT"},
		Lists:   map[string][]string{"src_mac": {mac}}}
}

// Blocked lists the MACs of Travo's block rules whose content is exactly BlockRule.
func Blocked(firewall []device.Section) []string {
	out := []string{}
	for _, s := range firewall {
		if s.Type != "rule" || !strings.HasPrefix(s.Name, BlockPrefix) || len(s.Lists["src_mac"]) != 1 {
			continue
		}
		mac := s.Lists["src_mac"][0]
		if s.Name == BlockSection(mac) && sameRule(s, BlockRule(mac)) {
			out = append(out, mac)
		}
	}
	return out
}

func sameRule(a, b device.Section) bool {
	return a.Type == b.Type && maps.Equal(a.Options, b.Options) &&
		maps.EqualFunc(a.Lists, b.Lists, func(x, y []string) bool { return slices.Equal(x, y) })
}

// StageBlock writes the block rule; it reports false when the exact rule already exists.
func StageBlock(ctx context.Context, uci device.UCI, firewall []device.Section, mac string) (bool, error) {
	want := BlockRule(mac)
	if at := slices.IndexFunc(firewall, func(s device.Section) bool { return s.Name == want.Name }); at >= 0 {
		if sameRule(firewall[at], want) {
			return false, nil
		}
		if firewall[at].Type != "rule" {
			return false, ErrBlockConflict
		}
		// Our name with drifted content (an operator edit): rewrite it whole.
		if err := uci.DeleteSection(ctx, "firewall", want.Name); err != nil {
			return false, err
		}
	}
	if err := uci.SetType(ctx, "firewall", want.Name, want.Type); err != nil {
		return false, err
	}
	for _, k := range slices.Sorted(maps.Keys(want.Options)) {
		if err := uci.Set(ctx, "firewall", want.Name, k, want.Options[k]); err != nil {
			return false, err
		}
	}
	return true, uci.AddToList(ctx, "firewall", want.Name, "src_mac", mac)
}

// StageUnblock deletes the block rule; it reports false when there is none.
func StageUnblock(ctx context.Context, uci device.UCI, firewall []device.Section, mac string) (bool, error) {
	name := BlockSection(mac)
	at := slices.IndexFunc(firewall, func(s device.Section) bool { return s.Name == name })
	if at < 0 {
		return false, nil
	}
	if firewall[at].Type != "rule" {
		return false, ErrBlockConflict
	}
	return true, uci.DeleteSection(ctx, "firewall", name)
}
