package pkgmgr

import (
	"context"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/runner"
)

// Runner is the shared command runner.
type Runner = runner.Runner

// Timeout tiers for package ops: lists and installs go over the network and flash.
const (
	TierQuery   = 15 * time.Second
	TierUpdate  = 2 * time.Minute
	TierInstall = 10 * time.Minute
)

func run(ctx context.Context, r Runner, tier time.Duration, name string, args ...string) (string, int, error) {
	ctx, cancel := context.WithTimeout(ctx, tier)
	defer cancel()
	return r(ctx, name, args...)
}
