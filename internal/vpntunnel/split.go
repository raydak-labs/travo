package vpntunnel

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/vpnprofiles"
)

// Split tunnel (invariants §11): the transaction owns `network.travo_split_<n>` route
// sections. With any present, peers get route_allowed_ips=0, so only those prefixes and the
// applied resolvers (/32, network.wg0.travo_dns) go through wg0; the rest keeps the uplink default.
// Without them the tunnel is full (route_allowed_ips=1, kernel default via wg0).
const (
	splitPrefix = "travo_split_"
	MaxSplit    = 32
)

var ErrInvalidSplit = errors.New("split routes must be up to 32 distinct IPv4 prefixes, none of them a default route")

// Split is the configured split-tunnel selection; empty Routes means full tunnel.
type Split struct {
	Routes []netip.Prefix
}

// ValidateSplit normalises user input: IPv4 only (invariants: no IPv6 policy until
// device-validated), masked, sorted, no duplicates, no 0.0.0.0/0.
func ValidateSplit(in []netip.Prefix) ([]netip.Prefix, error) {
	if len(in) > MaxSplit {
		return nil, ErrInvalidSplit
	}
	out := make([]netip.Prefix, 0, len(in))
	for _, p := range in {
		if !p.IsValid() || !p.Addr().Is4() || p.Bits() == 0 {
			return nil, ErrInvalidSplit
		}
		out = append(out, p.Masked())
	}
	slices.SortFunc(out, func(a, b netip.Prefix) int {
		if c := a.Addr().Compare(b.Addr()); c != 0 {
			return c
		}
		return a.Bits() - b.Bits()
	})
	if len(slices.Compact(slices.Clone(out))) != len(out) {
		return nil, ErrInvalidSplit
	}
	return out, nil
}

// resolverOption tells, on every split route Travo writes, whether Travo added it for a
// resolver ("1") or the user selected it ("0"), so a user /32 equal to a resolver is never
// mistaken for one. netifd ignores it (procd container).
const resolverOption = "travo_resolver"

// splitRoute is one owned route section: a user prefix, or a resolver /32 Travo added.
type splitRoute struct {
	Prefix   netip.Prefix
	Resolver bool
}

// splitRoutes is pure: the user's prefixes plus a /32 for each IPv4 resolver they do not cover,
// so the vpn DNS layer stays reachable through the tunnel.
func splitRoutes(user []netip.Prefix, dns []netip.Addr) []splitRoute {
	if len(user) == 0 {
		return nil
	}
	out := make([]splitRoute, 0, len(user)+len(dns))
	for _, p := range user {
		out = append(out, splitRoute{Prefix: p})
	}
	for _, r := range dns {
		if !r.Is4() || slices.ContainsFunc(user, func(p netip.Prefix) bool { return p.Contains(r) }) ||
			slices.Contains(out, splitRoute{Prefix: netip.PrefixFrom(r, 32), Resolver: true}) {
			continue
		}
		out = append(out, splitRoute{Prefix: netip.PrefixFrom(r, 32), Resolver: true})
	}
	return out
}

func splitSections(routes []splitRoute) []device.Section {
	out := make([]device.Section, 0, len(routes))
	for i, r := range routes {
		opts := map[string]string{"interface": iface, "target": r.Prefix.String(), resolverOption: "0"}
		if r.Resolver {
			opts[resolverOption] = "1"
		}
		out = append(out, device.Section{Name: splitPrefix + strconv.Itoa(i), Type: "route",
			Options: opts, Lists: map[string][]string{}})
	}
	return out
}

// currentSplit reads the configured routes from the owned sections, in section order.
// A section without resolverOption was written before it existed: a /32 equal to one of
// legacyDNS is then taken as a resolver route, the only guess left.
func currentSplit(sections []device.Section, legacyDNS []netip.Addr) ([]splitRoute, error) {
	var out []splitRoute
	for _, s := range sections {
		if !strings.HasPrefix(s.Name, splitPrefix) {
			continue
		}
		if s.Type != "route" || s.Options["interface"] != iface {
			return nil, errUnavailable // a foreign section on an owned name
		}
		p, err := netip.ParsePrefix(s.Options["target"])
		if err != nil {
			return nil, errUnavailable
		}
		var resolver bool
		switch marker, marked := s.Options[resolverOption]; {
		case !marked:
			resolver = p.Bits() == 32 && slices.Contains(legacyDNS, p.Addr())
		case marker == "1":
			resolver = true
		case marker != "0":
			return nil, errUnavailable
		}
		out = append(out, splitRoute{Prefix: p, Resolver: resolver})
	}
	return out, nil
}

// userRoutes returns the user's prefixes, without the resolver /32s Travo added.
func userRoutes(routes []splitRoute) []netip.Prefix {
	var out []netip.Prefix
	for _, r := range routes {
		if !r.Resolver {
			out = append(out, r.Prefix)
		}
	}
	return out
}

// Split reads the configured split selection (GET never mutates).
func (s *Service) Split(ctx context.Context) (Split, error) {
	if s.uci == nil {
		return Split{}, errUnavailable
	}
	current, err := s.uci.Show(ctx, "network")
	if err != nil {
		return Split{}, errUnavailable
	}
	var dns []netip.Addr
	if s.profiles != nil {
		if dns, err = TunnelResolvers(current, s.profiles); err != nil && !errors.Is(err, vpnprofiles.ErrNotFound) {
			return Split{}, err
		}
	}
	routes, err := currentSplit(current, dns)
	if err != nil {
		return Split{}, err
	}
	out := userRoutes(routes)
	if out == nil {
		out = []netip.Prefix{}
	}
	return Split{Routes: out}, nil
}

// SetSplit stages the split selection in a WireGuard transaction. An enabled tunnel is
// restarted and must prove a fresh handshake and the expected default route before confirm.
func (s *Service) SetSplit(ctx context.Context, user []netip.Prefix) (txn.Status, error) {
	if s.txns == nil || s.uci == nil || s.control == nil || s.ubus == nil || s.routes == nil ||
		s.keys == nil || s.profiles == nil || s.handshakes == nil || s.now == nil {
		return txn.Status{}, errUnavailable
	}
	user, err := ValidateSplit(user)
	if err != nil {
		return txn.Status{}, err
	}
	profile, err := s.profiles.SelectedConfig()
	if err != nil {
		return txn.Status{}, err
	}
	var routes []splitRoute
	var proof enableProof
	return s.txns.BeginWireGuard(ctx, "Configure split tunnel", stageBounded(func(ctx context.Context) error {
		current, err := s.uci.Show(ctx, "network")
		if err != nil {
			return errUnavailable
		}
		enabled, err := configuredEnabled(current)
		if err != nil {
			return err
		}
		if !slices.ContainsFunc(current, func(x device.Section) bool { return x.Name == iface }) {
			return vpnprofiles.ErrNotFound // nothing to split before a profile was applied
		}
		// the running tunnel's resolvers need their /32s, not a later selection's (#289)
		dns, err := TunnelResolvers(current, s.profiles)
		if err != nil {
			return err
		}
		routes = splitRoutes(user, dns)
		have, err := currentSplit(current, dns)
		if err != nil {
			return err
		}
		if slices.Equal(have, routes) && peersRouteAll(current) == (len(routes) == 0) {
			return txn.ErrNoChanges
		}
		proof.wasEnabled = enabled
		if enabled {
			if err := s.recordHandshakes(ctx, &proof, profile); err != nil {
				return err
			}
		}
		return s.stageSplit(ctx, current, routes)
	}), func(ctx context.Context) error {
		if !proof.wasEnabled {
			return nil // nothing live changed: the routes apply on the next enable
		}
		return s.proveSplit(ctx, proof, len(routes) > 0)
	})
}

// profileSplit is the split selection before and after applying a profile: the user's
// prefixes stay, the resolver /32s follow the profile's resolvers (#296).
type profileSplit struct{ current, next []splitRoute }

func (p profileSplit) isSplit() bool { return len(p.next) > 0 }
func (p profileSplit) changed() bool { return !slices.Equal(p.current, p.next) }

func (s *Service) splitForProfile(network []device.Section, profile vpnprofiles.Config) (profileSplit, error) {
	appliedDNS, err := TunnelResolvers(network, s.profiles)
	if err != nil && !errors.Is(err, vpnprofiles.ErrNotFound) {
		return profileSplit{}, errUnavailable
	}
	current, err := currentSplit(network, appliedDNS)
	if err != nil {
		return profileSplit{}, err
	}
	return profileSplit{current: current, next: splitRoutes(userRoutes(current), profile.DNS)}, nil
}

// withoutPeers drops the peer sections, which stageEnable has already rewritten with the
// right route_allowed_ips, so stageSplit replaces only the route sections.
func withoutPeers(sections []device.Section) []device.Section {
	return slices.DeleteFunc(slices.Clone(sections), func(x device.Section) bool { return x.Type == peerType })
}

func peersRouteAll(sections []device.Section) bool {
	for _, x := range sections {
		if x.Type == peerType && x.Options["route_allowed_ips"] == "0" {
			return false
		}
	}
	return true
}

func (s *Service) stageSplit(ctx context.Context, current []device.Section, routes []splitRoute) error {
	for _, x := range slices.Backward(current) {
		if strings.HasPrefix(x.Name, splitPrefix) {
			if err := s.uci.DeleteSection(ctx, "network", x.Name); err != nil {
				return errUnavailable
			}
		}
	}
	routeAll := "1"
	if len(routes) > 0 {
		routeAll = "0"
	}
	for _, x := range current {
		if x.Type == peerType && x.Options["route_allowed_ips"] != routeAll {
			if err := s.uci.Set(ctx, "network", x.Name, "route_allowed_ips", routeAll); err != nil {
				return errUnavailable
			}
		}
	}
	for _, x := range splitSections(routes) {
		if err := s.uci.SetType(ctx, "network", x.Name, x.Type); err != nil {
			return errUnavailable
		}
		for _, k := range []string{"interface", "target", resolverOption} {
			v, ok := x.Options[k]
			if !ok {
				continue
			}
			if err := s.uci.Set(ctx, "network", x.Name, k, v); err != nil {
				return errUnavailable
			}
		}
	}
	return nil
}

func (s *Service) recordHandshakes(ctx context.Context, proof *enableProof, profile vpnprofiles.Config) error {
	proof.started = s.now().Unix()
	previous, err := s.handshakes.LatestHandshakes(ctx, iface)
	if err != nil && !errors.Is(err, device.ErrNotFound) {
		return errUnavailable
	}
	proof.previous = make(map[string]time.Time, len(previous))
	for _, peer := range previous {
		proof.previous[peer.PublicKey] = peer.At
	}
	proof.configured = make(map[string]struct{}, len(profile.Peers))
	for _, peer := range profile.Peers {
		proof.configured[peer.PublicKey] = struct{}{}
	}
	return nil
}

// proveSplit restarts wg0 (a peer edit needs it, device-facts §1), proves a new handshake,
// then that the kernel default route matches the mode: via wg0 for full, not via wg0 for split.
func (s *Service) proveSplit(ctx context.Context, proof enableProof, split bool) error {
	if err := s.proveEnabled(ctx, proof); err != nil {
		return err
	}
	if !split {
		return poll(ctx, func(ctx context.Context) (bool, error) {
			route, found, err := s.routes.DefaultIPv4Route(ctx)
			return err == nil && found && route.Device == iface, nil
		})
	}
	// Leaving full mode can drop the uplink default (device-facts §1): wait, then bounce it.
	wait, cancel := context.WithTimeout(ctx, 8*time.Second)
	err := poll(wait, s.hasUplinkDefault)
	cancel()
	if err == nil {
		return nil
	}
	logical, err := s.recoveryInterface(ctx)
	if err != nil {
		return err
	}
	if err := s.control.SetInterfaceUp(ctx, logical, false); err != nil {
		return errProof
	}
	if err := s.control.SetInterfaceUp(ctx, logical, true); err != nil {
		return errProof
	}
	return poll(ctx, s.hasUplinkDefault)
}
