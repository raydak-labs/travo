package vpntunnel

import (
	"context"
	"maps"
	"net"
	"net/netip"
	"slices"
	"strconv"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/vpnprofiles"
)

const peerType = "wireguard_wg0"

func desiredSections(profile vpnprofiles.Config) ([]device.Section, error) {
	tunnel := device.Section{Name: iface, Type: "interface", Options: map[string]string{
		"proto": "wireguard", "disabled": "0", "private_key": profile.PrivateKey,
	}, Lists: map[string][]string{"addresses": prefixes(profile.Addresses)}}
	addNumber(tunnel.Options, "listen_port", profile.ListenPort)
	addNumber(tunnel.Options, "mtu", profile.MTU)
	out := make([]device.Section, 1, 1+len(profile.Peers))
	out[0] = tunnel
	for i, peer := range profile.Peers {
		options := map[string]string{"public_key": peer.PublicKey, "route_allowed_ips": "1"}
		addNumber(options, "persistent_keepalive", peer.PersistentKeepalive)
		if peer.PresharedKey != nil {
			options["preshared_key"] = *peer.PresharedKey
		}
		if peer.Endpoint != nil {
			host, port, err := net.SplitHostPort(*peer.Endpoint)
			if err != nil {
				return nil, errUnavailable
			}
			options["endpoint_host"], options["endpoint_port"] = host, port
		}
		out = append(out, device.Section{Name: "wg0_peer" + strconv.Itoa(i), Type: peerType,
			Options: options, Lists: map[string][]string{"allowed_ips": prefixes(peer.AllowedIPs)}})
	}
	return out, nil
}

func prefixes(values []netip.Prefix) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value.String())
	}
	return out
}

func addNumber(options map[string]string, key string, value *uint16) {
	if value != nil {
		options[key] = strconv.FormatUint(uint64(*value), 10)
	}
}

func configuredEnabled(sections []device.Section) (bool, error) {
	for _, section := range sections {
		if section.Name != iface {
			continue
		}
		if section.Type != "interface" || section.Options["proto"] != "wireguard" || len(section.Lists["disabled"]) != 0 {
			return false, errUnavailable
		}
		switch section.Options["disabled"] {
		case "", "0":
			return true, nil
		case "1":
			return false, nil
		default:
			return false, errUnavailable
		}
	}
	return false, nil
}

func checkPeerNames(current, peers []device.Section) error {
	names := make(map[string]struct{}, len(peers))
	for _, peer := range peers {
		names[peer.Name] = struct{}{}
	}
	for _, section := range current {
		if _, wanted := names[section.Name]; wanted && section.Type != peerType {
			return errUnavailable
		}
	}
	return nil
}

var tunnelOptions = []string{"proto", "disabled", "private_key", "listen_port", "mtu", "addresses"}

func matches(current, desired []device.Section, enabled bool) bool {
	if !enabled {
		return false
	}
	found := 0
	byName := make(map[string]device.Section, len(desired))
	for _, section := range desired {
		byName[section.Name] = section
	}
	for _, section := range current {
		switch {
		case section.Name == iface:
			for _, key := range tunnelOptions {
				if key == "disabled" {
					continue // configuredEnabled already normalized the option.
				}
				if !sameOption(section, desired[0], key) {
					return false
				}
			}
			found++
		case section.Type == peerType:
			want, ok := byName[section.Name]
			if !ok || !maps.Equal(section.Options, want.Options) ||
				!maps.EqualFunc(section.Lists, want.Lists, slices.Equal[[]string]) {
				return false
			}
			found++
		}
	}
	return found == len(desired)
}

func sameOption(current, desired device.Section, key string) bool {
	prev, got := current.Options[key]
	next, want := desired.Options[key]
	return got == want && prev == next && slices.Equal(current.Lists[key], desired.Lists[key])
}

func (s *Service) stageEnable(ctx context.Context, current, desired []device.Section) error {
	var tunnel device.Section
	// Deleting last to first preserves anonymous UCI selectors.
	for _, section := range slices.Backward(current) {
		switch {
		case section.Name == iface:
			tunnel = section
		case section.Type == peerType:
			if err := s.uci.DeleteSection(ctx, "network", section.Name); err != nil {
				return errUnavailable
			}
		}
	}
	if tunnel.Name == "" {
		if err := s.uci.SetType(ctx, "network", iface, desired[0].Type); err != nil {
			return errUnavailable
		}
		tunnel.Name = iface
	}
	for _, key := range tunnelOptions {
		if sameOption(tunnel, desired[0], key) {
			continue
		}
		if err := s.writeOption(ctx, tunnel, desired[0], key); err != nil {
			return errUnavailable
		}
	}
	for _, peer := range desired[1:] {
		if err := s.uci.SetType(ctx, "network", peer.Name, peer.Type); err != nil {
			return errUnavailable
		}
		for _, key := range slices.Sorted(maps.Keys(peer.Options)) {
			if err := s.writeOption(ctx, device.Section{}, peer, key); err != nil {
				return errUnavailable
			}
		}
		if err := s.writeOption(ctx, device.Section{}, peer, "allowed_ips"); err != nil {
			return errUnavailable
		}
	}
	return nil
}

func (s *Service) writeOption(ctx context.Context, old, next device.Section, key string) error {
	value, hasValue := next.Options[key]
	_, hadValue := old.Options[key]
	if len(old.Lists[key]) > 0 || hadValue && !hasValue {
		if err := s.uci.DeleteOption(ctx, "network", next.Name, key); err != nil {
			return err
		}
	}
	if hasValue {
		switch key {
		case "private_key", "preshared_key":
			return s.keys.SetWireGuardKey(ctx, next.Name, key, value)
		default:
			return s.uci.Set(ctx, "network", next.Name, key, value)
		}
	}
	for _, item := range next.Lists[key] {
		if err := s.uci.AddToList(ctx, "network", next.Name, key, item); err != nil {
			return err
		}
	}
	return nil
}
