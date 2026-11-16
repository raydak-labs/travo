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
)

// newVPNDNS wires the `vpn` DNS layer reconciler to the shared layer stack.
func newVPNDNS(deps app.Deps, layers vpndns.Layers, tunnel vpndns.Tunnel, profiles *vpnprofiles.Service) *vpndns.Service {
	lookup := vpndns.LocalLookup("127.0.0.1:53")
	if deps.Mock {
		lookup = func(context.Context, string) error { return nil }
	}
	return &vpndns.Service{
		Tunnel: tunnel, Profiles: profiles, Layers: layers, Lookup: lookup,
		ReadDNS: func(ctx context.Context) (dnslayer.State, error) {
			_, st, err := dnslayer.ReadState(ctx, deps.Device)
			return st, err
		},
		TxnOpen: func() (bool, error) { return guardHeld(deps, txn.GuardName) },
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
