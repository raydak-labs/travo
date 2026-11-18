package main

import (
	"path/filepath"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/api"
	"github.com/blackdark-org/travo-greenfield/internal/app"
	"github.com/blackdark-org/travo-greenfield/internal/clients"
	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// newClients wires the clients page; nil when the device cannot read leases, neighbours or
// kick stations (the API then answers 503 clients_unavailable).
func newClients(deps app.Deps) api.Clients {
	leases, hasLeases := deps.Device.(device.DHCPLeaseReader)
	neigh, hasNeigh := deps.Device.(device.NeighbourReader)
	kicker, hasKicker := deps.Device.(device.StationKicker)
	if !hasLeases || !hasNeigh || !hasKicker || deps.Txn == nil {
		return nil
	}
	return &clients.Service{UCI: deps.Device, Ubus: deps.Device, Neigh: neigh, Leases: leases, Kicker: kicker,
		Txns: deps.Txn, Guards: deps.Guards, Now: time.Now,
		Aliases: &clients.AliasStore{Path: filepath.Join(deps.Paths.State, "clients.json")}}
}
