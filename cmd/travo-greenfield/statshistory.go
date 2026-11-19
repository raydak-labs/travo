package main

import (
	"context"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/app"
	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/lifecycle"
	"github.com/blackdark-org/travo-greenfield/internal/statshistory"
	"github.com/blackdark-org/travo-greenfield/internal/syssettings"
	"github.com/blackdark-org/travo-greenfield/internal/version"
)

// startStatsHistory loads the history file and starts the sampler; its last act on
// shutdown is the final flush (docs/adr/0026). nil when the device cannot read counters.
func startStatsHistory(deps app.Deps, workers *lifecycle.Group) *statshistory.Store {
	counters, ok := deps.Device.(device.InterfaceCounterReader)
	if !ok {
		return nil
	}
	floor, _ := time.Parse(time.RFC3339, version.BuildTime) // unstamped dev build: zero floor
	store := statshistory.Open(filepath.Join(deps.Paths.State, "stats-history.json"), floor, time.Now)
	neighbours, _ := deps.Device.(device.NeighbourReader)
	src := statshistory.Sources{Ubus: deps.Device, Counters: counters, Neighbours: neighbours}
	workers.Go("stats-history", func(ctx context.Context) { store.Run(ctx, src, time.Now) })
	return store
}

// followTimezone counts days in the router's timezone: read once now and again after every
// settings change. An unreadable or unusable zone counts in UTC (ADR 0026).
func followTimezone(ctx context.Context, settings *syssettings.Service, history interface{ SetLocation(*time.Location) }) {
	apply := func(s syssettings.Settings) {
		loc, err := syssettings.Location(s.Timezone)
		if err != nil {
			slog.Warn("stats history: timezone unusable, days stay UTC", "timezone", s.Timezone, "err", err)
			loc = time.UTC
		}
		history.SetLocation(loc)
	}
	settings.OnChange = apply
	cur, err := settings.Get(ctx)
	if err != nil {
		slog.Warn("stats history: timezone unreadable, days stay UTC", "err", err)
		return
	}
	apply(cur)
}
