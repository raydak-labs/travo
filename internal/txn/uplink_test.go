package txn

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

func TestWireGuardRecordsUplinkBeforeSpawning(t *testing.T) {
	m, d := setup(t, 60*time.Second)
	d.DefaultRoute = true
	dump, err := os.ReadFile("../../testdata/device/axt1800-25.12.6/wireguard/connected/interface-dump.json")
	if err != nil {
		t.Fatal(err)
	}
	d.SetUbusReply("network.interface", "dump", dump)
	m.Spawner = checkingSpawner{Spawner: m.Spawner, check: func(dir string) error {
		marker := filepath.Join(dir, "restore-uplink")
		raw, err := os.ReadFile(marker)
		if err != nil {
			return err
		}
		info, err := os.Stat(marker)
		if err != nil {
			return err
		}
		if string(raw) != "wwan\nphy0-sta0\n" || info.Mode().Perm() != 0600 {
			return fmt.Errorf("unexpected uplink marker %q, mode %o", raw, info.Mode().Perm())
		}
		return nil
	}}
	st, err := m.BeginWireGuard(ctx, "profile", stageSet(d, "network"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Confirm(st.ID); err != nil {
		t.Fatal(err)
	}
}

func TestWatchdogProvesUnderlyingDefault(t *testing.T) {
	const marker = "wwan\nphy0-sta0\n"
	const expected = "default via 192.0.2.1 dev phy0-sta0 metric 10"
	for _, tc := range []struct {
		name, marker, routes, after, calls string
		success                            bool
	}{
		{"already restored", marker, expected, "", "", true},
		{"higher metric ignored", marker, expected + "\ndefault dev other metric 20", "", "", true},
		{"equal best needs repair", marker, expected + "\ndefault dev other metric 10", expected, "status down up ", true},
		{"missing default restarts", marker, "", expected, "status down up ", true},
		{"restart cannot prove route", marker, "", "default dev other", "status down up ", false},
		{"invalid gateway skips restart", marker, "default via invalid dev phy0-sta0", expected, "", false},
		{"duplicate device skips restart", marker, "default dev other dev phy0-sta0", expected, "", false},
		{"equal metric distinct records", marker, expected + " proto static\n" + expected + " proto dhcp", expected, "status down up ", true},
		{"missing protocol value skips restart", marker, "default dev other proto", expected, "", false},
		{"unknown route token skips restart", marker, expected + " garbage", expected, "", false},
		{"non-unicast needs repair", marker, "blackhole default proto static", expected, "status down up ", true},
		{"multipath needs repair", marker, "default proto static metric 10\n nexthop via 192.0.2.1 dev phy0-sta0 weight 1\n nexthop via 192.0.2.2 dev other weight 1", expected, "status down up ", true},
		{"bad route read skips restart", marker, "default metric bogus", expected, "", false},
		{"bad marker skips restart", marker + "\n", "", expected, "", false},
		{"linkdown needs repair", marker, expected + " linkdown", expected, "status down up ", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "txn")
			config := filepath.Join(root, "config")
			for _, path := range []string{config, filepath.Join(dir, "snap")} {
				if err := os.MkdirAll(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			for name, contents := range map[string]string{
				"txn/configs": "network\n", "txn/snap/network": "original\n",
				"txn/restore-wg0": "absent\n", "txn/restore-uplink": tc.marker,
				"watchdog": isolatedWireGuardWatchdog(root),
				"sleep":    "#!/bin/sh\nexit 0\n", "sync": "#!/bin/sh\nexit 0\n",
				"ip": `#!/bin/sh
if [ -e "$TEST_UP" ]; then printf '%s\n' "$TEST_AFTER"; else printf '%s\n' "$TEST_ROUTES"; fi
`,
				"jsonfilter": "#!/bin/sh\ncat\n",
				"ubus": `#!/bin/sh
case "$4:$5" in
	network:reload) exit 0 ;;
	network.interface.wg0:status) exit 4 ;;
	network.interface.wwan:status) printf 'status ' >> "$TEST_CALLS"; echo phy0-sta0 ;;
	network.interface.wwan:down) printf 'down ' >> "$TEST_CALLS" ;;
	network.interface.wwan:up) printf 'up ' >> "$TEST_CALLS"; touch "$TEST_UP" ;;
	*) exit 1 ;;
esac
`,
			} {
				if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("TEST_UP", filepath.Join(root, "up"))
			t.Setenv("TEST_CALLS", filepath.Join(root, "calls"))
			t.Setenv("TEST_ROUTES", tc.routes)
			t.Setenv("TEST_AFTER", tc.after)
			spawner := ShellSpawner{Shell: "/bin/sh", RunTimeout: 10 * time.Second}
			err := spawner.Run(filepath.Join(root, "watchdog"), dir, config, "0", "0", "true", "true")
			got, readErr := os.ReadFile(filepath.Join(dir, "result"))
			calls, _ := os.ReadFile(filepath.Join(root, "calls"))
			want := "failed"
			if tc.success {
				want = "restored"
			}
			if (err == nil) != tc.success || readErr != nil || strings.TrimSpace(string(got)) != want || string(calls) != tc.calls {
				t.Fatalf("restore %v, result %q (%v), calls %q", err, got, readErr, calls)
			}
		})
	}
}

type uplinkTestDevice struct {
	*fileDevice
	route string
}

func (d uplinkTestDevice) DefaultIPv4Route(context.Context) (device.IPv4DefaultRoute, bool, error) {
	return device.IPv4DefaultRoute{Device: d.route}, d.route != "", nil
}

func TestWireGuardUplinkBaselineSelection(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/device/axt1800-25.12.6/wireguard/connected/interface-dump.json")
	if err != nil {
		t.Fatal(err)
	}
	var dump struct {
		Interface []json.RawMessage `json:"interface"`
	}
	if err := json.Unmarshal(raw, &dump); err != nil {
		t.Fatal(err)
	}
	for _, iface := range dump.Interface {
		if strings.Contains(string(iface), `"wwan"`) {
			duplicate := strings.Replace(string(iface), `"wwan"`, `"duplicate_wwan"`, 1)
			dump.Interface = append(dump.Interface, json.RawMessage(duplicate))
			break
		}
	}
	ambiguous, err := json.Marshal(dump)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, route string
		dump        []byte
		refuse      bool
	}{
		{"no default", "", nil, false},
		{"wg0 default", "wg0", nil, false},
		{"no mapping", "unmapped", raw, true},
		{"multiple mappings", "phy0-sta0", ambiguous, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, d := setup(t, time.Minute)
			m.Device = uplinkTestDevice{d, tc.route}
			if tc.dump != nil {
				d.SetUbusReply("network.interface", "dump", tc.dump)
			}
			spawned, staged := false, false
			m.Spawner = checkingSpawner{Spawner: m.Spawner, check: func(dir string) error {
				spawned = true
				if _, err := os.Stat(filepath.Join(dir, "restore-uplink")); !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("expected no uplink marker: %v", err)
				}
				return nil
			}}
			st, err := m.BeginWireGuard(ctx, "test", func(ctx context.Context) error {
				staged = true
				return stageSet(d, "network")(ctx)
			}, nil)
			if tc.refuse {
				if err == nil || spawned || staged || guarded(t, m) {
					t.Fatalf("expected refusal before staging, got spawned=%t staged=%t err=%v", spawned, staged, err)
				}
				return
			}
			if err != nil || !spawned || !staged {
				t.Fatalf("expected transaction without route marker: %+v %v", st, err)
			}
			if _, err := m.Confirm(st.ID); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestWireGuardMarkedUplinkRollbackPaths(t *testing.T) {
	dump, err := os.ReadFile("../../testdata/device/axt1800-25.12.6/wireguard/connected/interface-dump.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"explicit", "failed probe", "timeout", "startup recovery"} {
		t.Run(scenario, func(t *testing.T) {
			window := time.Minute
			if scenario == "timeout" {
				window = time.Second
			}
			m, d := setup(t, window)
			d.DefaultRoute = true
			d.SetUbusReply("network.interface", "dump", dump)
			root := t.TempDir()
			setUplinkRecoveryCommands(t, root)
			m.Spawner = checkingSpawner{Spawner: m.Spawner, check: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "watchdog.sh"), []byte(isolatedWireGuardWatchdog(root)), 0700)
			}}
			st, err := m.BeginWireGuard(ctx, "test", stageSet(d, "network"), func(context.Context) error {
				if scenario == "failed probe" {
					return errors.New("peer unreachable")
				}
				return nil
			})
			if scenario == "failed probe" {
				if !errors.Is(err, ErrProbeFailed) {
					t.Fatalf("expected failed probe: %v", err)
				}
			} else if err != nil || st.State != Pending {
				t.Fatalf("begin: %+v %v", st, err)
			}
			st = finishMarkedUplinkRollback(t, m, scenario, st)
			calls, err := os.ReadFile(filepath.Join(root, "calls"))
			if st.State != RolledBack || err != nil || string(calls) != "down\nup\n" ||
				content(t, m, "network") != "original network\n" || guarded(t, m) {
				t.Fatalf("restore: %+v, restart calls %q (%v)", st, calls, err)
			}
		})
	}
}

func setUplinkRecoveryCommands(t *testing.T, root string) {
	t.Helper()
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_CALLS", filepath.Join(root, "calls"))
	for name, script := range map[string]string{
		"sync":       "exit 0",
		"jsonfilter": "cat",
		"ip": `if grep -q '^up$' "$TEST_CALLS" 2>/dev/null; then
	echo 'default via 192.0.2.1 dev phy0-sta0'
fi`,
		"ubus": `case "$4:$5" in
	network:reload) exit 0 ;;
	network.interface.wg0:status) exit 4 ;;
	network.interface.wwan:status) echo phy0-sta0 ;;
	network.interface.wwan:down|network.interface.wwan:up) echo "$5" >> "$TEST_CALLS" ;;
	*) exit 1 ;;
esac`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("#!/bin/sh\n"+script+"\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
}

func finishMarkedUplinkRollback(t *testing.T, m *Manager, scenario string, st Status) Status {
	t.Helper()
	switch scenario {
	case "explicit":
		got, err := m.Rollback(st.ID)
		if err != nil {
			t.Fatal(err)
		}
		return got
	case "startup recovery":
		pidRaw, err := os.ReadFile(m.path(st.ID, "pid"))
		if err != nil {
			t.Fatal(err)
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(pidRaw)))
		if err != nil {
			t.Fatal(err)
		}
		process, err := os.FindProcess(pid)
		if err != nil {
			t.Fatal(err)
		}
		if err := process.Kill(); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(m.path(st.ID, "claim")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("watchdog claimed restore before recovery: %v", err)
		}
		recovered, _ := setup(t, time.Minute)
		recovered.Dir, recovered.ConfigDir, recovered.Guards = m.Dir, m.ConfigDir, m.Guards
		out, err := recovered.Recover(ctx, func(int, string) bool { return false })
		if err != nil || len(out) != 1 || out[0].ID != st.ID {
			t.Fatalf("recovery: %+v %v", out, err)
		}
		return out[0]
	case "timeout":
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			got, err := m.Get(st.ID)
			if err != nil {
				t.Fatal(err)
			}
			st = got
			if st.State != Pending && st.State != Restoring {
				return st
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatal("detached rollback did not finish")
	}
	return st
}
