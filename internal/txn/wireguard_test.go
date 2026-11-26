package txn

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type checkingSpawner struct {
	Spawner
	check func(string) error
}

func (s checkingSpawner) Start(script string, args ...string) (int, error) {
	if err := s.check(filepath.Dir(script)); err != nil {
		return 0, err
	}
	return s.Spawner.Start(script, args...)
}

func TestBeginWireGuardRecordsBaselineBeforeWatchdog(t *testing.T) {
	for _, tt := range []struct {
		name, disabled, marker string
	}{
		{"missing", "", "absent\n"},
		{"enabled", "0", "enabled\n"},
		{"disabled", "1", "absent\n"},
		{"invalid", "yes", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m, d := setup(t, 60*time.Second)
			if tt.name != "missing" {
				for _, err := range []error{
					d.SetType(ctx, "network", "wg0", "interface"),
					d.Set(ctx, "network", "wg0", "proto", "wireguard"),
					d.Set(ctx, "network", "wg0", "disabled", tt.disabled),
					d.Device.Commit(ctx, "network"),
				} {
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			m.Spawner = checkingSpawner{Spawner: m.Spawner, check: func(dir string) error {
				path := filepath.Join(dir, "restore-wg0")
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				info, err := os.Stat(path)
				if err != nil {
					return err
				}
				if string(data) != tt.marker || info.Mode().Perm() != 0o600 {
					return fmt.Errorf("unexpected marker %q, mode %o", data, info.Mode().Perm())
				}
				return nil
			}}
			staged := false
			st, err := m.BeginWireGuard(ctx, "profile", func(ctx context.Context) error {
				staged = true
				return stageSet(d, "network")(ctx)
			}, nil)
			if tt.marker == "" {
				if err == nil || staged || guarded(t, m) {
					t.Fatalf("invalid baseline: staged=%v guard=%v error=%v", staged, guarded(t, m), err)
				}
				return
			}
			if err != nil || st.State != Pending || len(st.Configs) != 1 || st.Configs[0] != "network" {
				t.Fatalf("begin: %+v %v", st, err)
			}
			if _, err := m.Confirm(st.ID); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// isolatedWireGuardWatchdog checks a missing test link rather than the host's wg0.
func isolatedWireGuardWatchdog(root string) string {
	link := filepath.Join(root, "missing-wg0")
	quoted := "'" + strings.ReplaceAll(link, "'", "'\\''") + "'"
	return strings.ReplaceAll(string(WatchdogScript), "/sys/class/net/wg0", quoted)
}

func TestWatchdogWireGuardMarker(t *testing.T) {
	for _, tt := range []struct {
		name, marker, status, calls, result string
	}{
		{"ordinary", "", "4", "reload\n", "restored\n"},
		{"absent", "absent\n", "4", "reload\nbarrier\nstatus\nstatus\nstatus\n", "restored\n"},
		{"extra newline", "absent\n\n", "4", "reload\n", "failed\n"},
		{"failed status", "absent\n", "1", "reload\nbarrier\n" + strings.Repeat("status\n", 8), "failed\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "txn")
			config := filepath.Join(root, "config")
			for _, path := range []string{config, filepath.Join(dir, "snap")} {
				if err := os.MkdirAll(path, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			write := func(path, value string, mode os.FileMode) {
				t.Helper()
				if err := os.WriteFile(path, []byte(value), mode); err != nil {
					t.Fatal(err)
				}
			}
			write(filepath.Join(dir, "configs"), "network\n", 0o600)
			write(filepath.Join(dir, "snap", "network"), "baseline\n", 0o600)
			write(filepath.Join(config, "network"), "changed\n", 0o600)
			if tt.marker != "" {
				write(filepath.Join(dir, "restore-wg0"), tt.marker, 0o600)
			}
			write(filepath.Join(root, "watchdog.sh"), isolatedWireGuardWatchdog(root), 0o700)
			write(filepath.Join(root, "reload"), "#!/bin/sh\nread -r value < \"$TEST_NETWORK\" && [ \"$value\" = baseline ] || exit 1\necho reload >> \"$TEST_CALLS\"\n", 0o700)
			write(filepath.Join(root, "ubus"), `#!/bin/sh
case "$*" in
	'-t 3 call network reload') echo barrier >> "$TEST_CALLS" ;;
	'-t 1 call network.interface.wg0 status') echo status >> "$TEST_CALLS"; exit "$TEST_STATUS" ;;
	*) echo unexpected >> "$TEST_CALLS"; exit 1 ;;
esac
`, 0o700)
			for _, bin := range []string{"sleep", "sync"} {
				write(filepath.Join(root, bin), "#!/bin/sh\nexit 0\n", 0o700)
			}
			t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("TEST_CALLS", filepath.Join(root, "calls"))
			t.Setenv("TEST_NETWORK", filepath.Join(config, "network"))
			t.Setenv("TEST_STATUS", tt.status)
			spawner := ShellSpawner{Shell: "/bin/sh", RunTimeout: 5 * time.Second}
			err := spawner.Run(filepath.Join(root, "watchdog.sh"), dir, config, "0", "0", "true", filepath.Join(root, "reload"))
			if (err == nil) != (tt.result == "restored\n") {
				t.Errorf("watchdog error: %v", err)
			}
			for path, want := range map[string]string{
				filepath.Join(dir, "result"): tt.result,
				filepath.Join(root, "calls"): tt.calls,
			} {
				if got, err := os.ReadFile(path); err != nil || string(got) != want {
					t.Errorf("%s got %q (%v), want %q", filepath.Base(path), got, err, want)
				}
			}
		})
	}
}

func TestWireGuardRollbackFailureKeepsGuard(t *testing.T) {
	m, d := setup(t, 60*time.Second)
	root := t.TempDir()
	for _, bin := range []string{"ubus", "sync"} {
		code := "#!/bin/sh\nexit 1\n"
		if bin == "sync" {
			code = "#!/bin/sh\nexit 0\n"
		}
		if err := os.WriteFile(filepath.Join(root, bin), []byte(code), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	m.Spawner = checkingSpawner{Spawner: m.Spawner, check: func(dir string) error {
		return os.WriteFile(filepath.Join(dir, "watchdog.sh"), []byte(isolatedWireGuardWatchdog(root)), 0o700)
	}}
	st, err := m.BeginWireGuard(ctx, "profile", stageSet(d, "network"), nil)
	if err != nil {
		t.Fatal(err)
	}
	st, err = m.Rollback(st.ID)
	if err == nil || st.State != RollbackFailed || !guarded(t, m) {
		t.Fatalf("rollback: %+v error=%v guard=%v", st, err, guarded(t, m))
	}
	if got := content(t, m, "network"); got != "original network\n" {
		t.Fatalf("network snapshot not restored: %q", got)
	}
}
