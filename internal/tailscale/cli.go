package tailscale

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/runner"
)

// Timeout tiers for the tailscale CLI (invariants §1: every shell-out has one).
const (
	TierRead  = 5 * time.Second
	TierWrite = 15 * time.Second
	// UpWait is what `tailscale up --timeout` waits for Running; TierUp leaves room around it.
	UpWait = 10 * time.Second
	TierUp = UpWait + 10*time.Second
)

// ErrUpTimeout means `tailscale up` did not reach Running within UpWait. Before login this is
// the normal outcome: the backend keeps the auth URL after the CLI exits
// (testdata/device/axt1800-25.12.6/tailscale/up-timeout.err, status-authurl.json).
var ErrUpTimeout = errors.New("tailscale up: not running yet")

// upTimeoutText is the CLI's message for ErrUpTimeout (up-timeout.err).
const upTimeoutText = "timeout waiting for Tailscale service to enter a Running state"

// CLI is the port to the tailscale command line. Status and Prefs return raw JSON for Parse.
type CLI interface {
	Status(ctx context.Context) ([]byte, error)
	Prefs(ctx context.Context) ([]byte, error)
	// Up runs `tailscale up` with args; it returns ErrUpTimeout when the backend is not Running yet.
	Up(ctx context.Context, args []string) error
	Set(ctx context.Context, args []string) error
	Logout(ctx context.Context) error
}

// Command runs the real `tailscale` binary through a runner.
type Command struct {
	Run  runner.Runner
	Path string // empty = "tailscale" from PATH
}

func (c Command) bin() string {
	if c.Path == "" {
		return "tailscale"
	}
	return c.Path
}

func (c Command) run(ctx context.Context, tier time.Duration, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, tier)
	defer cancel()
	out, _, err := c.Run(ctx, c.bin(), args...)
	return out, err
}

func (c Command) Status(ctx context.Context) ([]byte, error) {
	out, err := c.run(ctx, TierRead, "status", "--json")
	return []byte(out), err
}

func (c Command) Prefs(ctx context.Context) ([]byte, error) {
	out, err := c.run(ctx, TierRead, "debug", "prefs")
	return []byte(out), err
}

func (c Command) Up(ctx context.Context, args []string) error {
	all := append([]string{"up"}, args...)
	all = append(all, fmt.Sprintf("--timeout=%s", UpWait))
	_, err := c.run(ctx, TierUp, all...)
	if stderr, ok := runner.Stderr(err); ok && strings.Contains(stderr, upTimeoutText) {
		return ErrUpTimeout
	}
	return err
}

func (c Command) Set(ctx context.Context, args []string) error {
	_, err := c.run(ctx, TierWrite, append([]string{"set"}, args...)...)
	return err
}

func (c Command) Logout(ctx context.Context) error {
	_, err := c.run(ctx, TierWrite, "logout")
	return err
}

var _ CLI = Command{}
