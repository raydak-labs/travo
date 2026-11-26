//go:build integration

package txn

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	wgBaseline = "192.0.2.10"
	wgChanged  = "192.0.2.20"
)

func TestRealWireGuardRollback(t *testing.T) {
	if os.Getenv("TRAVO_WG_ROLLBACK_CONTAINER") != "1" {
		t.Skip("requires the disposable OpenWrt WireGuard container")
	}
	runWGCommand(t, "reload_config")
	runWGCommand(t, "ubus", "-t", "3", "call", "network", "reload")
	if err := waitWireGuardEndpoint(context.Background(), wgBaseline); err != nil {
		t.Fatal(err)
	}

	for _, scenario := range []string{"explicit", "probe failure", "timeout", "startup recovery", "disabled baseline"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			m := realManager(MinWindow)
			m.Dir = filepath.Join(root, "txn")
			m.Guards.Path = filepath.Join(root, "guards")
			spawner := &wgIntegrationSpawner{ShellSpawner: m.Spawner.(ShellSpawner)}
			m.Spawner = spawner
			if scenario == "timeout" {
				spawner.window = "2"
			}
			if scenario == "disabled baseline" {
				runWGCommand(t, "uci", "set", "network.wg0.disabled=1")
				runWGCommand(t, "uci", "commit", "network")
				runWGCommand(t, "reload_config")
				runWGCommand(t, "ubus", "-t", "3", "call", "network", "reload")
				if err := waitWireGuardLinkAbsent(context.Background()); err != nil {
					t.Fatal(err)
				}
			}

			probeFailure := errors.New("synthetic probe failure after the new peer is live")
			st, err := m.BeginWireGuard(context.Background(), "test peer switch", func(ctx context.Context) error {
				if err := m.Device.Set(ctx, "network", "wg0", "disabled", "0"); err != nil {
					return err
				}
				return m.Device.Set(ctx, "network", "wg0_peer0", "endpoint_host", wgChanged)
			}, func(ctx context.Context) error {
				if err := restartWireGuard(ctx); err != nil {
					return err
				}
				if err := waitWireGuardEndpoint(ctx, wgChanged); err != nil {
					return err
				}
				if scenario == "probe failure" {
					return probeFailure
				}
				return nil
			})
			if scenario == "probe failure" {
				if !errors.Is(err, probeFailure) {
					t.Fatalf("did not reach the intended probe failure: %v", err)
				}
			} else if err != nil || st.State != Pending {
				t.Fatalf("begin: %+v %v", st, err)
			}
			if st = triggerWireGuardRestore(t, m, spawner, scenario, st); st.State != RolledBack {
				t.Fatalf("restore: %+v", st)
			}
			if got := uciGet(t, "network.wg0_peer0.endpoint_host"); got != wgBaseline {
				t.Fatalf("restored UCI endpoint: %q", got)
			}
			if scenario == "disabled baseline" {
				if got := uciGet(t, "network.wg0.disabled"); got != "1" {
					t.Fatalf("restored disabled option: %q", got)
				}
				if _, err := os.Stat("/sys/class/net/wg0"); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("disabled wg0 device remains: %v", err)
				}
			} else if err := waitWireGuardEndpoint(context.Background(), wgBaseline); err != nil {
				t.Fatal(err)
			}
			if guarded(t, m) {
				t.Fatal("successful restore kept the transaction guard")
			}
		})
	}
}

type wgIntegrationSpawner struct {
	ShellSpawner
	pid    int
	window string
}

func (s *wgIntegrationSpawner) Start(script string, args ...string) (int, error) {
	if s.window != "" {
		args[2] = s.window
	}
	pid, err := s.ShellSpawner.Start(script, args...)
	s.pid = pid
	return pid, err
}

func triggerWireGuardRestore(t *testing.T, m *Manager, spawner *wgIntegrationSpawner, scenario string, st Status) Status {
	t.Helper()
	var err error
	switch scenario {
	case "explicit", "disabled baseline":
		st, err = m.Rollback(st.ID)
	case "startup recovery":
		proc, findErr := os.FindProcess(spawner.pid)
		if findErr != nil {
			t.Fatal(findErr)
		}
		if err := proc.Kill(); err != nil {
			t.Fatal(err)
		}
		if err := waitWireGuardCondition(context.Background(), func() bool {
			return !ProcAlive(spawner.pid, filepath.Join(m.Dir, st.ID, "watchdog.sh"))
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(m.Dir, st.ID, "claim")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("watchdog already claimed restore: %v", err)
		}
		// Construct a new manager to exercise startup from the persisted state.
		recovered := realManager(MinWindow)
		recovered.Dir, recovered.Guards = m.Dir, m.Guards
		outcomes, err := recovered.Recover(context.Background(), ProcAlive)
		if err != nil || len(outcomes) != 1 {
			t.Fatalf("recovery: %+v %v", outcomes, err)
		}
		st = outcomes[0]
	}
	if err != nil && st.State != Restoring {
		t.Fatal(err)
	}
	deadline := time.Now().Add(60 * time.Second)
	for st.State == Pending || st.State == Restoring {
		if time.Now().After(deadline) {
			t.Fatalf("restore never reached a terminal result: %+v", st)
		}
		st, err = m.Get(st.ID)
		if err != nil {
			t.Fatal(err)
		}
		if st.State == Pending || st.State == Restoring {
			time.Sleep(100 * time.Millisecond)
		}
	}
	return st
}

func restartWireGuard(ctx context.Context) error {
	run := func(object, method string) error {
		out, err := exec.CommandContext(ctx, "ubus", "-t", "3", "call", object, method).CombinedOutput()
		if err != nil {
			return fmt.Errorf("ubus %s %s: %w (%s)", object, method, err, out)
		}
		return nil
	}
	if err := run("network.interface.wg0", "down"); err != nil {
		return err
	}
	if err := waitWireGuardLinkAbsent(ctx); err != nil {
		return err
	}
	if err := run("network", "reload"); err != nil {
		return err
	}
	return run("network.interface.wg0", "up")
}

func waitWireGuardLinkAbsent(ctx context.Context) error {
	return waitWireGuardCondition(ctx, func() bool {
		_, err := os.Stat("/sys/class/net/wg0")
		return errors.Is(err, os.ErrNotExist)
	})
}

func waitWireGuardEndpoint(ctx context.Context, endpoint string) error {
	err := waitWireGuardCondition(ctx, func() bool {
		out, err := exec.CommandContext(ctx, "wg", "show", "wg0", "endpoints").Output()
		fields := strings.Fields(string(out))
		return err == nil && len(fields) == 2 && fields[1] == endpoint+":51820"
	})
	if err != nil {
		return fmt.Errorf("WireGuard endpoint %s did not converge: %w", endpoint, err)
	}
	return nil
}

func waitWireGuardCondition(ctx context.Context, condition func() bool) error {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for !condition() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
	return nil
}

func runWGCommand(t *testing.T, command string, args ...string) {
	t.Helper()
	if out, err := exec.Command(command, args...).CombinedOutput(); err != nil {
		t.Fatalf("%s: %v (%s)", command, err, out)
	}
}
