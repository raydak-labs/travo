package txn

import (
	"context"
	"errors"
	"regexp"

	"github.com/blackdark-org/travo-greenfield/internal/atomicfile"
	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
)

func (m *Manager) recordWireGuardBaseline(ctx context.Context, id string) error {
	sections, err := m.Device.Show(ctx, "network")
	if err != nil {
		return err
	}
	baseline := "absent\n"
	for _, section := range sections {
		if section.Name != "wg0" {
			continue
		}
		if section.Type != "interface" || section.Options["proto"] != "wireguard" || len(section.Lists["disabled"]) != 0 {
			return errors.New("invalid wg0 section")
		}
		switch section.Options["disabled"] {
		case "", "0":
			baseline = "enabled\n"
		case "1":
		default:
			return errors.New("invalid wg0 disabled option")
		}
		break
	}
	if err := atomicfile.Write(m.path(id, "restore-wg0"), []byte(baseline), 0o600); err != nil {
		return err
	}
	return m.recordWireGuardUplink(ctx, id)
}

var uplinkName = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_-]*$`)

func (m *Manager) recordWireGuardUplink(ctx context.Context, id string) error {
	reader, ok := m.Device.(device.DefaultRouteReader)
	if !ok {
		return errors.New("default route reader unavailable")
	}
	route, found, err := reader.DefaultIPv4Route(ctx)
	if err != nil {
		return errors.New("cannot read the original default route")
	}
	if !found || route.Device == "wg0" {
		return nil
	}
	if len(route.Device) > 15 || !uplinkName.MatchString(route.Device) {
		return errors.New("invalid underlying route device")
	}
	raw, err := m.Device.Call(ctx, "network.interface", "dump", nil)
	if err != nil {
		return errors.New("cannot read underlying uplink mapping")
	}
	interfaces, err := facts.ParseInterfaceDump(raw)
	if err != nil {
		return errors.New("cannot parse underlying uplink mapping")
	}
	logical := ""
	for _, candidate := range interfaces {
		if !candidate.Up || candidate.L3Device != route.Device || len(candidate.IPv4) == 0 {
			continue
		}
		if logical != "" || !uplinkName.MatchString(candidate.Name) || candidate.Name == "wg0" {
			return errors.New("ambiguous or invalid underlying uplink")
		}
		logical = candidate.Name
	}
	if logical == "" {
		return errors.New("underlying uplink mapping unavailable")
	}
	return atomicfile.Write(m.path(id, "restore-uplink"), []byte(logical+"\n"+route.Device+"\n"), 0o600)
}
