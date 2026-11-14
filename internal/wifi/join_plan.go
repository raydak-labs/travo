package wifi

import (
	"context"
	"slices"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/lockout"
)

func pickRadio(radios []facts.Radio, bands []string, wireless []device.Section) string {
	best := ""
	for _, radio := range radios {
		if radio.Disabled || !slices.Contains(bands, radio.Band) || (best != "" && best < radio.Name) {
			continue
		}
		if slices.ContainsFunc(wireless, func(section device.Section) bool {
			return (section.Type == "wifi-iface" && section.Options["mode"] == "ap" && section.Options["device"] == radio.Name && section.Options["disabled"] != "1") ||
				(section.Type == "wifi-device" && section.Name == radio.Name && section.Options["disabled"] == "1")
		}) {
			continue
		}
		best = radio.Name
	}
	return best
}

func uplinkSections(wireless []device.Section) []device.Section {
	var sta []device.Section
	for _, section := range wireless {
		if section.Type == "wifi-iface" && section.Options["mode"] == "sta" && section.Options["network"] == "wwan" {
			sta = append(sta, section)
		}
	}
	return sta
}

type uciWrite struct{ action, config, section, option, value string }

func planJoin(wireless, network, firewall []device.Section, req JoinRequest, radio string) (writes []uciWrite, before, after []lockout.AP, err error) {
	for _, section := range wireless {
		if section.Type == "wifi-iface" && section.Options["mode"] == "ap" {
			onLAN := slices.Contains(strings.Fields(section.Options["network"]), "lan") || slices.Contains(section.Lists["network"], "lan")
			before = append(before, lockout.AP{Section: section.Name, Enabled: section.Options["disabled"] != "1", Guest: !onLAN})
		}
	}
	after = slices.Clone(before)
	zone := slices.IndexFunc(firewall, func(s device.Section) bool { return s.Type == "zone" && s.Options["name"] == "wan" })
	if zone < 0 {
		return nil, before, after, ErrFirewallUnavailable
	}
	sta := uplinkSections(wireless)
	writes, err = planSTA(wireless, sta, req, radio)
	if err != nil {
		return nil, before, after, err
	}
	if !hasSection(network, "wwan") {
		writes = append(writes, uciWrite{"type", "network", "wwan", "", "interface"}, uciWrite{"set", "network", "wwan", "proto", "dhcp"})
	}
	wan := firewall[zone]
	if !slices.Contains(wan.Lists["network"], "wwan") && !slices.Contains(strings.Fields(wan.Options["network"]), "wwan") {
		writes = append(writes, uciWrite{"list", "firewall", wan.Name, "network", "wwan"})
	}
	return writes, before, after, nil
}

func hasSection(sections []device.Section, name string) bool {
	return slices.ContainsFunc(sections, func(s device.Section) bool { return s.Name == name })
}

func planSTA(wireless, sta []device.Section, req JoinRequest, radio string) ([]uciWrite, error) {
	var writes []uciWrite
	section := device.Section{Name: "wwan"}
	create := len(sta) == 0
	if create {
		// Never repurpose an unrelated interface that happens to be named wwan.
		if hasSection(wireless, section.Name) {
			return nil, ErrUnavailable
		}
		writes = append(writes, uciWrite{"type", "wireless", section.Name, "", "wifi-iface"})
	} else {
		section = sta[0]
	}
	set := func(option, value string) {
		if section.Options[option] != value {
			writes = append(writes, uciWrite{"set", "wireless", section.Name, option, value})
		}
	}
	remove := func(option string) {
		_, scalar := section.Options[option]
		_, list := section.Lists[option]
		if scalar || list {
			writes = append(writes, uciWrite{"delete", "wireless", section.Name, option, ""})
		}
	}
	set("device", radio)
	if create {
		set("mode", "sta")
		set("network", "wwan")
	}
	set("ssid", req.SSID)
	encryption := "none"
	switch req.Security {
	case SecurityWPA2:
		encryption = "psk2"
	case SecurityWPA2WPA3:
		encryption = "sae-mixed" // UNVERIFIED on hardware.
	}
	set("encryption", encryption)
	set("disabled", "0")
	if req.Security == SecurityOpen {
		remove("key")
	} else if section.Options["key"] != req.Key {
		writes = append(writes, uciWrite{"key", "wireless", section.Name, "key", ""})
	}
	if req.BSSID == "" {
		remove("bssid")
	} else {
		set("bssid", req.BSSID)
	}
	if !create {
		for _, extra := range sta[1:] {
			if extra.Options["disabled"] != "1" {
				writes = append(writes, uciWrite{"set", "wireless", extra.Name, "disabled", "1"})
			}
		}
	}
	return writes, nil
}

func (s *Service) stageJoin(ctx context.Context, writes []uciWrite, key string) error {
	d := s.join.UCI
	for _, w := range writes {
		var err error
		switch w.action {
		case "type":
			err = d.SetType(ctx, w.config, w.section, w.value)
		case "set":
			err = d.Set(ctx, w.config, w.section, w.option, w.value)
		case "list":
			err = d.AddToList(ctx, w.config, w.section, w.option, w.value)
		case "delete":
			err = d.DeleteOption(ctx, w.config, w.section, w.option)
		case "key":
			err = s.join.Keys.SetWirelessKey(ctx, w.section, key)
		}
		if err != nil {
			return ErrUnavailable
		}
	}
	return nil
}
