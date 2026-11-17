package txn

import (
	"context"
	"slices"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/atomicfile"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
)

// recordSettle writes the watchdog's settle markers: the uplink that carries the default
// route and the radios that are up before the change. The watchdog then reports a
// restore only once that live state is back, not when reload_config returns (#158).
// Best effort: a marker that cannot be read is skipped, the configs restore either way.
func (m *Manager) recordSettle(ctx context.Context, id string, configs []string) {
	if !slices.Contains(configs, "network") && !slices.Contains(configs, "wireless") {
		return
	}
	if marker, err := m.uplinkMarker(ctx); err == nil && marker != "" {
		_ = atomicfile.Write(m.path(id, "settle-uplink"), []byte(marker), 0o600)
	}
	if !slices.Contains(configs, "wireless") {
		return
	}
	raw, err := m.Device.Call(ctx, "network.wireless", "status", nil)
	if err != nil {
		return
	}
	radios, err := facts.ParseWirelessRadios(raw)
	if err != nil {
		return
	}
	var up []string
	for _, r := range radios {
		if r.Up && !r.Disabled && uplinkName.MatchString(r.Name) {
			up = append(up, r.Name)
		}
	}
	if len(up) > 0 {
		_ = atomicfile.Write(m.path(id, "settle-radios"), []byte(strings.Join(up, "\n")+"\n"), 0o600)
	}
}
