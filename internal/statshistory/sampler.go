package statshistory

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/lifecycle"
)

// clientNetworks are the LAN-side logical interfaces whose neighbours count as clients.
var clientNetworks = []string{"lan", "guest"}

// Sources are the read-only device ports one observation needs. Neighbours may be nil.
type Sources struct {
	Ubus       device.Ubus
	Counters   device.InterfaceCounterReader
	Neighbours device.NeighbourReader
}

// Observe reads one observation. Uptime, interfaces and counters are required; an
// unreadable neighbour table only leaves Clients nil.
func Observe(ctx context.Context, src Sources, at time.Time) (Observation, error) {
	raw, err := src.Ubus.Call(ctx, "system", "info", nil)
	if err != nil {
		return Observation{}, err
	}
	info, err := facts.ParseSystemInfo(raw)
	if err != nil {
		return Observation{}, err
	}
	raw, err = src.Ubus.Call(ctx, "network.interface", "dump", nil)
	if err != nil {
		return Observation{}, err
	}
	ifaces, err := facts.ParseInterfaceDump(raw)
	if err != nil {
		return Observation{}, err
	}
	counters, err := src.Counters.ReadInterfaceCounters(ctx)
	if err != nil {
		return Observation{}, err
	}
	o := Observation{At: at, Uptime: info.Uptime, Uplinks: uplinkCounters(ifaces, counters)}
	if src.Neighbours != nil {
		if raw, err := src.Neighbours.Neighbours(ctx); err == nil {
			if neigh, err := facts.ParseNeighJSON(raw); err == nil {
				n := countClients(ifaces, neigh)
				o.Clients = &n
			}
		}
	}
	return o, nil
}

// uplinkCounters maps each up candidate uplink to its l3 device's counters.
func uplinkCounters(ifaces []facts.Interface, counters map[string]device.InterfaceCounters) map[string]Counter {
	out := map[string]Counter{}
	for _, it := range ifaces {
		if !it.Up || it.L3Device == "" || !slices.Contains(Uplinks, it.Name) {
			continue
		}
		if c, ok := counters[it.L3Device]; ok {
			out[it.Name] = Counter{Device: it.L3Device, RX: c.RXBytes, TX: c.TXBytes}
		}
	}
	return out
}

// countClients counts distinct MACs in the neighbour table on LAN-side devices.
func countClients(ifaces []facts.Interface, neigh []facts.Neighbour) int {
	devices := map[string]bool{}
	for _, it := range ifaces {
		if slices.Contains(clientNetworks, it.Name) && it.L3Device != "" {
			devices[it.L3Device] = true
		}
	}
	macs := map[string]bool{}
	for _, n := range neigh {
		if devices[n.Dev] {
			macs[n.MAC] = true
		}
	}
	return len(macs)
}

// Run samples every Interval until ctx ends, then flushes once (graceful shutdown).
// wall is the router's wall clock; Store.now drives only the flush cadence.
func (s *Store) Run(ctx context.Context, src Sources, wall func() time.Time) {
	lifecycle.Every(Interval, func(ctx context.Context) {
		if ctx.Err() != nil {
			return
		}
		o, err := Observe(ctx, src, wall())
		if err != nil {
			if ctx.Err() == nil {
				slog.Debug("stats history sample skipped", "err", err)
			}
			s.Skip()
		} else {
			s.Record(o)
		}
		s.MaybeFlush()
	})(ctx)
	s.Flush()
}
