package main

import (
	"context"
	"log/slog"

	"github.com/blackdark-org/travo-greenfield/internal/app"
	"github.com/blackdark-org/travo-greenfield/internal/firmware"
)

// startFirmware builds the firmware service and, before any guarded feature starts,
// releases the guard of an upgrade that finished (ADR 0030). Nil without the port.
func startFirmware(ctx context.Context, deps app.Deps, stopped func() bool) *firmware.Service {
	f, ok := deps.Device.(firmware.Device)
	if !ok {
		return nil
	}
	svc := &firmware.Service{Device: f, Guards: deps.Guards, Stopped: stopped}
	switch out, err := svc.Complete(ctx); {
	case err != nil:
		slog.Warn("firmware: completion check failed; the guard stays", "err", err)
	case out == firmware.OutcomeCompleted:
		slog.Info("firmware: upgrade completed; guard released", "guard", firmware.GuardName)
	case out == firmware.OutcomeInterrupted || out == firmware.OutcomeUnknown:
		slog.Warn("firmware: upgrade not confirmed; the guard stays", "guard", firmware.GuardName, "outcome", out)
	}
	return svc
}
