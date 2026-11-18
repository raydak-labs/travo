// Package app wires adapters into the daemon; it is the only place that picks real vs mock.
package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/pkgmgr"
	"github.com/blackdark-org/travo-greenfield/internal/runner"
	"github.com/blackdark-org/travo-greenfield/internal/tailscale"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/uciwrite"
)

// Paths are the daemon's on-disk locations. Guards never live in the state dir (invariants §1).
type Paths struct {
	State  string // session epoch
	Guards string
	Txn    string // snapshots on flash: a reboot mid-window must still restore
}

// RealPaths are the router locations.
func RealPaths() Paths {
	return Paths{State: "/etc/travo-greenfield", Guards: "/etc/travo-greenfield-guards", Txn: "/etc/travo-greenfield/txn"}
}

// EpochFile is where the session epoch is persisted.
func (p Paths) EpochFile() string { return filepath.Join(p.State, "epoch.json") }

// RebootFile is the durable record of the boot that accepted a restart.
func (p Paths) RebootFile() string { return filepath.Join(p.State, "reboot.json") }

// Options select the adapters.
type Options struct {
	Mock      bool
	FirstRun  bool // mock only: start with no root password
	StopCheck func() bool
}

// Deps are the ports every feature receives; features never import adapters.
type Deps struct {
	Device   device.Device
	Accounts device.Accounts
	Packages pkgmgr.PackageManager
	// Tailscale drives the tailscale CLI of the optional package.
	Tailscale tailscale.CLI
	Txn       *txn.Manager
	Guards    guard.Dir
	Paths     Paths
	Mock      bool
	// cleanup removes the mock temp dir; nil for the real adapters.
	cleanup func() error
}

// Close releases resources New created (the mock state dir). Safe to call on a zero Deps.
func (d Deps) Close() error {
	if d.cleanup == nil {
		return nil
	}
	return d.cleanup()
}

// New builds real adapters, or mock adapters in mock mode (develop without a device).
func New(o Options) (Deps, error) {
	var d Deps
	if o.Mock {
		dev := mock.New()
		dev.RotateBootOnReboot = true
		mock.SeedSystem(dev)
		mock.SeedConnectivity(dev)
		mock.SeedWiFi(dev)
		if err := mock.SeedNetLinks(dev); err != nil {
			return Deps{}, fmt.Errorf("mock links: %w", err)
		}
		dev.SetService("adguardhome", false, false) // absent package; the mock install adds it
		dev.SetService(tailscale.Package, false, false)
		dev.SetService("usbmuxd", false, false) // absent until the iOS tether driver install
		if secs, err := cli.ParseExport("dhcp", string(mock.DHCPExport)); err == nil {
			_ = dev.Load(context.Background(), "dhcp", secs) // the embedded capture always parses (seed test)
		}
		if secs, err := cli.ParseExport("system", string(mock.SystemExport)); err == nil {
			_ = dev.Load(context.Background(), "system", secs)
		}
		wireless, err := cli.ParseShow("wireless", string(mock.WirelessUCI))
		if err != nil {
			return Deps{}, fmt.Errorf("mock wireless: %w", err)
		}
		if err := dev.Load(context.Background(), "wireless", wireless); err != nil {
			return Deps{}, fmt.Errorf("mock wireless: %w", err)
		}
		if o.FirstRun {
			dev.SetPassword("")
		}
		tmp, err := os.MkdirTemp("", "travo-greenfield-mock-")
		if err != nil {
			return Deps{}, err
		}
		d = Deps{Device: dev, Accounts: dev, Packages: pkgmgr.NewMock(), Tailscale: tailscale.NewMock(), Mock: true,
			Paths:   Paths{State: tmp, Guards: filepath.Join(tmp, "guards"), Txn: filepath.Join(tmp, "txn")},
			cleanup: func() error { return os.RemoveAll(tmp) }}
	} else {
		pm, err := pkgmgr.DetectSystem(runner.Exec)
		if err != nil {
			return Deps{}, fmt.Errorf("package manager: %w", err)
		}
		dev := cli.New()
		d = Deps{Device: dev, Accounts: dev, Packages: pm, Tailscale: tailscale.Command{Run: runner.Exec}, Paths: RealPaths()}
	}
	d.Guards = guard.Dir{Path: d.Paths.Guards, Now: time.Now}
	d.Txn = newTxn(d, o.StopCheck)
	return d, nil
}

type wallClock struct{}

func (wallClock) Now() time.Time { return time.Now() }

func newTxn(d Deps, stopped func() bool) *txn.Manager {
	return &txn.Manager{
		Device: d.Device, Locks: &uciwrite.Locks{}, Guards: d.Guards, Dir: d.Paths.Txn,
		ConfigDir: "/etc/config", Reload: []string{"reload_config"}, UCI: "uci",
		Spawner: txn.ShellSpawner{Shell: "/bin/sh", RunTimeout: 60 * time.Second},
		Clock:   wallClock{}, Stopped: stopped,
		Window: 60 * time.Second, ArmLimit: 75 * time.Second, ApplyBudget: 65 * time.Second, Settle: 15 * time.Second,
		NewID: func() string {
			b := make([]byte, 8)
			_, _ = rand.Read(b) // crypto/rand does not fail on supported platforms
			return hex.EncodeToString(b)
		},
	}
}
