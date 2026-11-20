package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/blackdark-org/travo-greenfield/internal/app"
	"github.com/blackdark-org/travo-greenfield/internal/dnslayer"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/vpndns"
	"github.com/blackdark-org/travo-greenfield/internal/vpnprofiles"
	"github.com/blackdark-org/travo-greenfield/internal/vpntunnel"
)

// newVPNDNS wires the `vpn` DNS layer reconciler to the shared layer stack.
func newVPNDNS(deps app.Deps, layers vpndns.Layers, tunnel vpndns.Tunnel, profiles *vpnprofiles.Service) *vpndns.Service {
	lookup := vpndns.LocalLookup("127.0.0.1:53")
	if deps.Mock {
		lookup = func(context.Context, string) error { return nil }
	}
	return &vpndns.Service{
		Tunnel: tunnel, Layers: layers, Lookup: lookup,
		Resolvers: func(ctx context.Context, settled bool) (vpndns.Read, error) {
			got, err := settledRead(deps, settled)(ctx)
			if err != nil {
				return vpndns.Read{}, err
			}
			dns, err := vpntunnel.TunnelResolvers(got.Sections, profiles)
			return vpndns.Read{DNS: dns, NoTransactionDuringRead: got.NoTransactionDuringRead, NoTransactionSince: got.NoTransactionSince}, err
		},
		ReadDNS: func(ctx context.Context) (dnslayer.State, error) {
			_, st, err := dnslayer.ReadState(ctx, deps.Device)
			return st, err
		},
		Nonce: func() string {
			b := make([]byte, 8)
			_, _ = rand.Read(b) // crypto/rand does not fail on supported platforms
			return "travo-" + hex.EncodeToString(b)
		},
	}
}

func guardHeld(deps app.Deps, name string) (bool, error) {
	stale, err := deps.Guards.Stale()
	for _, g := range stale {
		if g.Name == name {
			return true, err
		}
	}
	return false, err
}

// settledRead is txn.Manager.ShowSettled for Reconcile; Status, Verify and a daemon without
// a transaction manager read plainly and trust only an absent guard.
func settledRead(deps app.Deps, settled bool) func(context.Context) (txn.Settled, error) {
	if settled && deps.Txn != nil {
		return func(ctx context.Context) (txn.Settled, error) { return deps.Txn.ShowSettled(ctx, "network") }
	}
	return func(ctx context.Context) (txn.Settled, error) {
		sections, err := deps.Device.Show(ctx, "network")
		if err != nil {
			return txn.Settled{}, err
		}
		held, err := guardHeld(deps, txn.GuardName)
		since := func() (bool, error) {
			held, err := guardHeld(deps, txn.GuardName)
			return !held, err
		}
		return txn.Settled{Sections: sections, NoTransactionDuringRead: !held, NoTransactionSince: since}, err
	}
}
