package services

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

// GetFirewallZones returns a summary of all UCI firewall zones.
func (n *NetworkService) GetFirewallZones() ([]models.FirewallZone, error) {
	sections, err := n.uci.GetSections("firewall")
	if err != nil {
		return nil, err
	}
	var zones []models.FirewallZone
	for _, opts := range sections {
		if opts[".type"] != "zone" {
			continue
		}
		z := models.FirewallZone{
			Name:    opts["name"],
			Input:   "DROP",
			Output:  "ACCEPT",
			Forward: "DROP",
		}
		if v := opts["input"]; v != "" {
			z.Input = v
		}
		if v := opts["output"]; v != "" {
			z.Output = v
		}
		if v := opts["forward"]; v != "" {
			z.Forward = v
		}
		if v := opts["network"]; v != "" {
			z.Network = strings.Fields(v)
		}
		if z.Network == nil {
			z.Network = []string{}
		}
		zones = append(zones, z)
	}
	if zones == nil {
		zones = []models.FirewallZone{}
	}
	return zones, nil
}

const defaultPortForwardsFile = "/etc/travo/port-forwards.json"

// portForwardsPath returns the port-forward store in use.
func (n *NetworkService) portForwardsPath() string {
	if n.portForwardsFile != "" {
		return n.portForwardsFile
	}
	return defaultPortForwardsFile
}

// GetPortForwards returns stored port-forward rules.
func (n *NetworkService) GetPortForwards() ([]models.PortForwardRule, error) {
	data, err := os.ReadFile(n.portForwardsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return []models.PortForwardRule{}, nil
		}
		return nil, err
	}
	var rules []models.PortForwardRule
	if err := json.Unmarshal(data, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}

// newPortForwardID returns an ID that cannot collide with an existing rule.
// The millisecond prefix keeps IDs roughly ordered and human-readable; the
// random suffix is what makes a same-millisecond collision impossible.
func newPortForwardID(existing []models.PortForwardRule) string {
	taken := make(map[string]bool, len(existing))
	for _, r := range existing {
		taken[r.ID] = true
	}
	for {
		var suffix [4]byte
		if _, err := rand.Read(suffix[:]); err != nil {
			// crypto/rand must not fail in practice; fall back to a counter-like
			// value rather than reusing an ID that may already be taken.
			return fmt.Sprintf("pf%d-%s", time.Now().UnixMilli(), strconv.FormatInt(portForwardFallbackID.Add(1), 36))
		}
		id := fmt.Sprintf("pf%d-%s", time.Now().UnixMilli(), hex.EncodeToString(suffix[:]))
		if !taken[id] {
			return id
		}
	}
}

// portForwardFallbackID only advances if crypto/rand is unavailable.
var portForwardFallbackID atomic.Int64

// AddPortForward adds a new port-forward rule.
func (n *NetworkService) AddPortForward(rule models.PortForwardRule) error {
	n.portForwardsMu.Lock()
	defer n.portForwardsMu.Unlock()

	rules, err := n.GetPortForwards()
	if err != nil {
		return err
	}
	// IDs must be unique per call: DeletePortForward removes every rule with
	// the ID.
	rule.ID = newPortForwardID(rules)
	rules = append(rules, rule)
	return n.savePortForwards(rules)
}

// DeletePortForward removes a port-forward rule by ID.
func (n *NetworkService) DeletePortForward(id string) error {
	n.portForwardsMu.Lock()
	defer n.portForwardsMu.Unlock()

	rules, err := n.GetPortForwards()
	if err != nil {
		return err
	}
	filtered := rules[:0]
	for _, r := range rules {
		if r.ID != id {
			filtered = append(filtered, r)
		}
	}
	return n.savePortForwards(filtered)
}

func (n *NetworkService) savePortForwards(rules []models.PortForwardRule) error {
	path := n.portForwardsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return err
	}
	data, err := json.Marshal(rules)
	if err != nil {
		return err
	}
	// Write to a sibling temp file and rename, so a reader never observes a
	// truncated file. os.WriteFile truncates first, which is what produced
	// "unexpected end of JSON input" on the device when a GET raced a POST.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
