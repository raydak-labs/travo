package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/blackdark-org/travo-greenfield/internal/app"
	"github.com/blackdark-org/travo-greenfield/internal/hook"
	"github.com/blackdark-org/travo-greenfield/internal/lifecycle"
)

// runHook is the client side of `--hook <action>`: cron jobs and button scripts call it
// (ADR 0024). Exit status 1 on any failure; the message goes to stderr for logger.
func runHook(socket, action string) int {
	if err := hook.Call(socket, action, hook.ClientTimeout); err != nil {
		fmt.Fprintln(os.Stderr, "travo-greenfield hook:", err)
		return 1
	}
	return 0
}

// hookSocket is the router path, or one in the mock state dir.
func hookSocket(deps app.Deps) string {
	if deps.Mock {
		return filepath.Join(deps.Paths.State, "hook.sock")
	}
	return hook.SocketPath
}

// startHooks registers the actions helpers may call and serves them until shutdown.
func startHooks(deps app.Deps, workers *lifecycle.Group, register func(*hook.Server)) {
	s := &hook.Server{Path: hookSocket(deps)}
	s.Register("ping", func(context.Context) error { return nil })
	register(s)
	l, err := s.Listen()
	if err != nil {
		slog.Warn("hook socket unavailable: schedules and buttons will fail", "err", err)
		return
	}
	workers.Go("hook-socket", func(ctx context.Context) { s.Serve(ctx, l) })
}
