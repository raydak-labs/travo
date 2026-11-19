package main

import (
	"context"
	"log/slog"

	"github.com/blackdark-org/travo-greenfield/internal/api"
	"github.com/blackdark-org/travo-greenfield/internal/app"
	"github.com/blackdark-org/travo-greenfield/internal/buttons"
	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/hook"
)

// startButtons builds the button service and registers the slide switch's hook actions,
// which the hotplug script calls (ADR 0024). nil when the device cannot read the
// devicetree keys or has no transaction manager; the API then answers 503.
func startButtons(ctx context.Context, deps app.Deps, hooks *hook.Server, tunnel api.VPNTunnelSetter, guest api.WiFiGuestEditor) *buttons.Service {
	keys, ok := deps.Device.(device.ButtonReader)
	if !ok || deps.Txn == nil {
		return nil
	}
	svc := &buttons.Service{UCI: deps.Device, Keys: keys, Apply: deps.Txn, Tunnel: tunnel, Guest: guest, Txns: deps.Txn, Guards: deps.Guards}
	if hooks == nil {
		return svc
	}
	actions, err := svc.Actions(ctx)
	if err != nil {
		slog.Warn("buttons: switch actions not registered", "err", err)
		return svc
	}
	for name, a := range actions {
		hooks.Register(name, a)
	}
	return svc
}
