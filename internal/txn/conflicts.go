package txn

import (
	"context"
	"fmt"

	"github.com/blackdark-org/travo-greenfield/internal/guard"
)

var (
	// ErrGuarded: a terminal or package-op guard is held (ADR 0005). It wraps ErrActive, so
	// every route answers it with its existing 409.
	ErrGuarded = fmt.Errorf("%w: a firmware upgrade, factory reset, reboot, update or package operation is in progress", ErrActive)
	// ErrGuardHeld: a caller that asked for RefuseAnyGuard found another guard held.
	ErrGuardHeld = guard.ErrHeld
)

type refuseAnyKey struct{}

// RefuseAnyGuard marks ctx so Begin refuses while any other guard is held. Automated
// actions (WiFi schedule, slide switch) use it: they skip while anything else runs, and
// the check is atomic with writing the transaction guard.
func RefuseAnyGuard(ctx context.Context) context.Context {
	return context.WithValue(ctx, refuseAnyKey{}, true)
}

// conflicts is the transaction's guard policy: never while the router is about to reboot,
// flash, wipe or replace the daemon, or while a package operation runs.
func conflicts(ctx context.Context) func([]guard.Info) error {
	if refuse, _ := ctx.Value(refuseAnyKey{}).(bool); refuse {
		return guard.RefuseAny
	}
	return func(held []guard.Info) error {
		for _, g := range held {
			if guard.IsTerminal(g.Name) || guard.IsPackageOp(g.Name) {
				return fmt.Errorf("%w (%s)", ErrGuarded, g.Name)
			}
		}
		return nil
	}
}
