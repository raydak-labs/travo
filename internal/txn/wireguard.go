package txn

import (
	"context"
	"errors"

	"github.com/blackdark-org/travo-greenfield/internal/atomicfile"
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
	return atomicfile.Write(m.path(id, "restore-wg0"), []byte(baseline), 0o600)
}
