package main

import (
	"context"
	"path/filepath"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/app"
	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/lifecycle"
	"github.com/blackdark-org/travo-greenfield/internal/statshistory"
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
