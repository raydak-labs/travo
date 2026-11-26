package wifi

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// MACMode is the uplink STA MAC policy. Clone is a request-only mode: it stores the
// caller's MAC as custom.
type MACMode string

const (
	MACDefault MACMode = "default"
	MACRandom  MACMode = "random"
	MACCustom  MACMode = "custom"
	MACClone   MACMode = "clone"
)

var (
	ErrNoUplink            = errors.New("no enabled uplink STA")
	ErrMACCloneUnavailable = errors.New("caller MAC not in the neighbour table")
)

// MACPolicy is the configured policy and the MAC the STA uses now.
type MACPolicy struct {
	Mode       MACMode `json:"mode" enum:"default,random,custom"`
	Configured string  `json:"configured,omitempty" doc:"Custom MAC (custom mode only)"`
	Effective  *string `json:"effective" nullable:"true" doc:"Live STA MAC; null = could not read"`
}

// MACRequest sets the policy. MAC is used by custom only; Caller by clone only.
type MACRequest struct {
	Mode   MACMode
	MAC    string
	Caller netip.Addr
}

// unicastMAC accepts a lower-case colon MAC that is unicast and not all zero.
func unicastMAC(mac string) bool {
	if !macPattern.MatchString(mac) || mac == "00:00:00:00:00:00" {
		return false
	}
	first, err := strconv.ParseUint(mac[:2], 16, 8)
	return err == nil && first&1 == 0
}

func locallyAdministered(mac string) bool {
	first, err := strconv.ParseUint(mac[:2], 16, 8)
	return err == nil && first&2 != 0
}

func enabledUplink(wireless []device.Section) (device.Section, bool) {
	at := slices.IndexFunc(wireless, func(s device.Section) bool {
		return s.Type == "wifi-iface" && s.Options["mode"] == "sta" && s.Options["network"] == "wwan" && s.Options["disabled"] != "1"
	})
	if at < 0 {
		return device.Section{}, false
	}
	return wireless[at], true
}

func policyOf(sta device.Section) MACPolicy {
	switch mac := sta.Options["macaddr"]; mac {
	case "":
		return MACPolicy{Mode: MACDefault}
	case "random":
		return MACPolicy{Mode: MACRandom}
	default:
		return MACPolicy{Mode: MACCustom, Configured: mac}
	}
}

// UplinkMAC reports the policy of the enabled uplink STA.
func (s *Service) UplinkMAC(ctx context.Context) (MACPolicy, error) {
	if s.join == nil {
		return MACPolicy{}, ErrUnavailable
	}
	wireless, err := s.join.UCI.Show(ctx, "wireless")
	if err != nil {
		return MACPolicy{}, ErrUnavailable
	}
	sta, ok := enabledUplink(wireless)
	if !ok {
		return MACPolicy{}, ErrNoUplink
	}
	out := policyOf(sta)
	if ifname := s.staIfname(ctx, sta.Name); ifname != "" {
		if link, ok := s.link(ctx, ifname); ok && link.MAC != "" {
			out.Effective = &link.MAC
		}
	}
	return out, nil
}

// planMAC returns the write and what the probe must see. before is the live MAC.
func planMAC(sta device.Section, req MACRequest, neighbours []facts.Neighbour) ([]uciWrite, func(string) bool, error) {
	var value string
	switch req.Mode {
	case MACDefault:
	case MACRandom:
		value = "random"
	case MACCustom:
		value = strings.ToLower(req.MAC)
		if !unicastMAC(value) {
			return nil, nil, ValidationError{"wifi_mac_invalid"}
		}
	case MACClone:
		at := slices.IndexFunc(neighbours, func(n facts.Neighbour) bool { return req.Caller.IsValid() && n.IP == req.Caller.Unmap() })
		if at < 0 || !unicastMAC(neighbours[at].MAC) {
			return nil, nil, ErrMACCloneUnavailable
		}
		value = neighbours[at].MAC
	default:
		return nil, nil, ValidationError{"wifi_mac_mode_invalid"}
	}
	var writes []uciWrite
	current, set := sta.Options["macaddr"]
	switch {
	case value == "" && set:
		writes = append(writes, uciWrite{"delete", "wireless", sta.Name, "macaddr", ""})
	case value != "" && current != value:
		writes = append(writes, uciWrite{"set", "wireless", sta.Name, "macaddr", value})
	}
	return writes, macCheck(value), nil
}

// macCheck says which live MAC proves the policy: an exact match for custom, a locally
// administered address for random, and a universally administered (factory) one for
// default. A random MAC that happens to equal the previous one is accepted: netifd
// draws it per bring-up, and "different" would not prove more.
func macCheck(value string) func(string) bool {
	switch value {
	case "":
		return func(mac string) bool { return unicastMAC(mac) && !locallyAdministered(mac) }
	case "random":
		return func(mac string) bool { return unicastMAC(mac) && locallyAdministered(mac) }
	default:
		return func(mac string) bool { return mac == value }
	}
}

// SetUplinkMAC applies the policy through a guarded transaction and proves carrier, the
// kernel default route via the STA and the effective MAC before arming confirmation.
func (s *Service) SetUplinkMAC(ctx context.Context, req MACRequest) (txn.Status, error) {
	if s.join == nil {
		return txn.Status{}, ErrUnavailable
	}
	var section string
	var check func(string) bool
	plan := func(ctx context.Context) ([]uciWrite, error) {
		wireless, err := s.join.UCI.Show(ctx, "wireless")
		if err != nil {
			return nil, ErrUnavailable
		}
		sta, ok := enabledUplink(wireless)
		if !ok {
			return nil, ErrNoUplink
		}
		var neighbours []facts.Neighbour
		if req.Mode == MACClone {
			if neighbours, err = s.neighbours(ctx); err != nil {
				return nil, ErrMACCloneUnavailable
			}
		}
		writes, ok2, err := planMAC(sta, req, neighbours)
		section, check = sta.Name, ok2
		return writes, err
	}
	if _, err := plan(ctx); err != nil {
		return txn.Status{}, err
	}
	var oldLinks map[int]struct{}
	st, err := s.join.Txns.Begin(ctx, "wifi: uplink MAC "+string(req.Mode), []string{"wireless"}, func(ctx context.Context) error {
		writes, err := plan(ctx)
		if err != nil {
			return err
		}
		// The old STA can keep carrier, route and even an LAA MAC right after reload
		// (#115): only a new kernel link proves the new policy is live.
		if oldLinks, err = s.linkSnapshot(ctx); err != nil {
			return err
		}
		return s.stageWrites(ctx, writes, "")
	}, func(ctx context.Context) error { return s.probeMAC(section, check, oldLinks)(ctx) })
	if errors.Is(err, txn.ErrActive) {
		return st, ErrTransactionPending
	}
	return st, err
}

func (s *Service) neighbours(ctx context.Context) ([]facts.Neighbour, error) {
	if s.join.Neighbours == nil {
		return nil, ErrUnavailable
	}
	raw, err := s.join.Neighbours.Neighbours(ctx)
	if err != nil {
		return nil, err
	}
	return facts.ParseNeighJSON(raw)
}

func (s *Service) staIfname(ctx context.Context, section string) string {
	radios, err := s.joinRadios(ctx)
	if err != nil {
		return ""
	}
	for _, r := range radios {
		for _, iface := range r.Ifaces {
			if iface.Section == section && iface.Mode == "sta" {
				return iface.Ifname
			}
		}
	}
	return ""
}

func (s *Service) link(ctx context.Context, ifname string) (facts.Link, bool) {
	raw, err := s.join.Ubus.Call(ctx, "network.device", "status", map[string]string{"name": ifname})
	if err != nil {
		return facts.Link{}, false
	}
	link, err := facts.ParseDeviceStatus(raw)
	return link, err == nil
}

func (s *Service) linkSnapshot(ctx context.Context) (map[int]struct{}, error) {
	links, err := s.join.Links.LinkIndices(ctx)
	if err != nil || len(links) == 0 {
		return nil, ErrUnavailable
	}
	out := make(map[int]struct{}, len(links))
	for _, index := range links {
		out[index] = struct{}{}
	}
	return out, nil
}

func (s *Service) probeMAC(section string, check func(string) bool, oldLinks map[int]struct{}) txn.Probe {
	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, s.join.ProbeBudget)
		defer cancel()
		ticker := time.NewTicker(s.join.ProbeEvery)
		defer ticker.Stop()
		for {
			if ctx.Err() == nil && s.macProven(ctx, section, check, oldLinks) {
				return nil
			}
			select {
			case <-ctx.Done():
				return errors.New("uplink MAC, carrier and route not proven")
			case <-ticker.C:
			}
		}
	}
}

func (s *Service) macProven(ctx context.Context, section string, check func(string) bool, oldLinks map[int]struct{}) bool {
	ifname := s.staIfname(ctx, section)
	if ifname == "" {
		return false
	}
	if link, ok := s.link(ctx, ifname); !ok || !check(link.MAC) {
		return false
	}
	// Fresh link index, carrier and the kernel default route via the STA, as join.
	return s.provenFreshLink(ctx, ifname, oldLinks)
}
