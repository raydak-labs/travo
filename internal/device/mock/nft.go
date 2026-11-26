package mock

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// nftEmpty is `nft -j list ruleset` of a family without tables
// (testdata/container/openwrt-25.12.6/mwan3/nft-ruleset-empty.json).
const nftEmpty = `{"nftables": [{"metainfo": {"version": "1.1.6", "release_name": "Commodore Bullmoose #7", "json_schema_version": 1}}]}`

// NftRuleset returns NftRulesets[family] (set it from a capture), the empty ruleset otherwise.
func (d *Device) NftRuleset(_ context.Context, family string) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if r, ok := d.NftRulesets[family]; ok {
		return r, nil
	}
	return nftEmpty, nil
}

// DeleteMangle removes the rules and chains from NftRulesets[family] like one nft
// transaction: a missing rule or chain, or a chain another kept rule still jumps to,
// fails it and changes nothing. DeleteMangleErr replaces it.
func (d *Device) DeleteMangle(_ context.Context, family string, rules []device.NftRule, chains []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.MangleDeletes++
	if d.DeleteMangleErr != nil {
		return d.DeleteMangleErr
	}
	var doc struct {
		Nftables []map[string]map[string]any `json:"nftables"`
	}
	if err := json.Unmarshal([]byte(d.NftRulesets[family]), &doc); err != nil {
		return fmt.Errorf("nft: %s mangle: %w", family, err)
	}
	inMangle := func(o map[string]any) bool { return o != nil && o["family"] == family && o["table"] == "mangle" }
	var kept []map[string]map[string]any
	found := 0
	for _, o := range doc.Nftables {
		if r := o["rule"]; inMangle(r) {
			h, _ := r["handle"].(float64)
			if slices.Contains(rules, device.NftRule{Chain: fmt.Sprint(r["chain"]), Handle: int(h)}) {
				found++
				continue
			}
			if slices.Contains(chains, fmt.Sprint(r["chain"])) {
				continue // flushed with its chain
			}
		}
		if c := o["chain"]; inMangle(c) && slices.Contains(chains, fmt.Sprint(c["name"])) {
			found++
			continue
		}
		kept = append(kept, o)
	}
	if found != len(rules)+len(chains) {
		return fmt.Errorf("nft: Error: Could not process rule: No such file or directory")
	}
	out, _ := json.Marshal(map[string]any{"nftables": kept})
	if s := string(out); slices.ContainsFunc(chains, func(c string) bool { return strings.Contains(s, `"target":"`+c+`"`) }) {
		return fmt.Errorf("nft: Error: Could not process rule: Device or resource busy")
	}
	d.NftRulesets[family] = string(out)
	return nil
}

// DeleteEmptyMangleChains is DeleteMangle without the flush: a chain that still holds a
// rule fails the whole delete like nft ("Device or resource busy") and nothing changes.
func (d *Device) DeleteEmptyMangleChains(ctx context.Context, family string, chains []string) error {
	d.mu.Lock()
	var doc struct {
		Nftables []map[string]map[string]any `json:"nftables"`
	}
	err := json.Unmarshal([]byte(d.NftRulesets[family]), &doc)
	d.mu.Unlock()
	if err != nil {
		return fmt.Errorf("nft: %s mangle: %w", family, err)
	}
	for _, o := range doc.Nftables {
		if r := o["rule"]; r != nil && r["family"] == family && r["table"] == "mangle" && slices.Contains(chains, fmt.Sprint(r["chain"])) {
			return fmt.Errorf("nft: Error: Could not process rule: Device or resource busy")
		}
	}
	return d.DeleteMangle(ctx, family, nil, chains)
}

var _ device.MangleCleaner = (*Device)(nil)
