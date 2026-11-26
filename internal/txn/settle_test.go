package txn

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBeginRecordsSettleMarkers(t *testing.T) {
	m, d := setup(t, 60*time.Second)
	d.DefaultRoute = true
	dump, err := os.ReadFile("../../testdata/device/axt1800-25.12.6/wireguard/connected/interface-dump.json")
	if err != nil {
		t.Fatal(err)
	}
	radios, err := os.ReadFile("../../testdata/device/axt1800-25.12.6/wireless-status.json")
	if err != nil {
		t.Fatal(err)
	}
	d.SetUbusReply("network.interface", "dump", dump)
	d.SetUbusReply("network.wireless", "status", radios)
	m.Spawner = checkingSpawner{Spawner: m.Spawner, check: func(dir string) error {
		for name, want := range map[string]string{"settle-uplink": "wwan\nphy0-sta0\n", "settle-radios": "radio0\nradio1\n"} {
			got, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil || string(got) != want {
				return fmt.Errorf("%s = %q (%v), want %q", name, got, err, want)
			}
		}
		return nil
	}}
	st, err := m.Begin(ctx, "wifi", []string{"wireless", "network"}, stageSet(d, "wireless"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Confirm(st.ID); err != nil {
		t.Fatal(err)
	}
}

// TestWatchdogSettlesLiveStateBeforeResult runs the real script with fake ubus and ip:
// the result must wait for the uplink route and the radio, and a route that never comes
// back gets one interface cycle without failing the restore (#158).
func TestWatchdogSettlesLiveStateBeforeResult(t *testing.T) {
	const route = "default via 192.0.2.1 dev phy0-sta0 metric 10"
	for _, tc := range []struct {
		name             string
		routeAfter       int // ip calls before the route shows; -1 never (until ifup)
		radioAfter       int // wireless status calls before radio1 is up and not pending
		wantCalls        string
		wantIPAtLeast    int
		wantRadioAtLeast int
	}{
		{"route and radio come back", 3, 4, "", 3, 4},
		{"route needs one ifup", -1, 0, "status down up ", 18, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "txn")
			for _, p := range []string{filepath.Join(root, "config"), filepath.Join(dir, "snap")} {
				if err := os.MkdirAll(p, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			for name, contents := range map[string]string{
				"txn/configs": "wireless\n", "txn/snap/wireless": "original\n",
				"txn/settle-uplink": "wwan\nphy0-sta0\n", "txn/settle-radios": "radio1\n",
				"watchdog": string(WatchdogScript),
				"sleep":    "#!/bin/sh\nexit 0\n", "sync": "#!/bin/sh\nexit 0\n",
				"ip": `#!/bin/sh
n=$(($(cat "$TEST_IP" 2>/dev/null || echo 0) + 1)); echo $n > "$TEST_IP"
if [ -e "$TEST_UP" ] || { [ "$TEST_ROUTE_AFTER" -ge 0 ] && [ $n -ge "$TEST_ROUTE_AFTER" ]; }; then echo "` + route + `"; fi
`,
				"jsonfilter": `#!/bin/sh
read -r up pending
case "$2" in *.up) echo "$up" ;; *.pending) echo "$pending" ;; *) echo "$up" ;; esac
`,
				"ubus": `#!/bin/sh
case "$4:$5" in
	network.wireless:status)
		n=$(($(cat "$TEST_RADIO" 2>/dev/null || echo 0) + 1)); echo $n > "$TEST_RADIO"
		if [ $n -ge "$TEST_RADIO_AFTER" ]; then echo "true false"; else echo "true true"; fi ;;
	network.interface.wwan:status) printf 'status ' >> "$TEST_CALLS"; echo phy0-sta0 ;;
	network.interface.wwan:down) printf 'down ' >> "$TEST_CALLS" ;;
	network.interface.wwan:up) printf 'up ' >> "$TEST_CALLS"; touch "$TEST_UP" ;;
	*) exit 1 ;;
esac
`,
			} {
				if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("TEST_UP", filepath.Join(root, "up"))
			t.Setenv("TEST_CALLS", filepath.Join(root, "calls"))
			t.Setenv("TEST_IP", filepath.Join(root, "ipcalls"))
			t.Setenv("TEST_RADIO", filepath.Join(root, "radiocalls"))
			t.Setenv("TEST_ROUTE_AFTER", fmt.Sprint(tc.routeAfter))
			t.Setenv("TEST_RADIO_AFTER", fmt.Sprint(tc.radioAfter))
			spawner := ShellSpawner{Shell: "/bin/sh", RunTimeout: 10 * time.Second}
			if err := spawner.Run(filepath.Join(root, "watchdog"), dir, filepath.Join(root, "config"), "0", "0", "true", "true"); err != nil {
				t.Fatal(err)
			}
			read := func(name string) string {
				b, _ := os.ReadFile(filepath.Join(root, name))
				return strings.TrimSpace(string(b))
			}
			var ipCalls, radioCalls int
			fmt.Sscan(read("ipcalls"), &ipCalls)
			fmt.Sscan(read("radiocalls"), &radioCalls)
			result, _ := os.ReadFile(filepath.Join(dir, "result"))
			if string(result) != "restored\n" || read("calls") != strings.TrimSpace(tc.wantCalls) ||
				ipCalls < tc.wantIPAtLeast || radioCalls < tc.wantRadioAtLeast {
				t.Fatalf("result %q, calls %q, ip %d, radio %d", result, read("calls"), ipCalls, radioCalls)
			}
		})
	}
}
