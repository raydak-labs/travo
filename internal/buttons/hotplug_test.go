package buttons

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const hotplugScript = "../../packaging/files/etc/hotplug.d/button/50-travo-greenfield"

// runHotplug sources the script the way hotplug-call does, with a fake daemon client and
// logger on PATH, and returns what the fake client was asked (empty when not called).
func runHotplug(t *testing.T, button, action string) string {
	t.Helper()
	bin := t.TempDir()
	calls := filepath.Join(bin, "calls")
	logged := filepath.Join(bin, "logged")
	fakes := map[string]string{
		"travo-greenfield": "#!/bin/sh\necho \"$*\" >> " + calls + "\necho 'error unknown-action'\n",
		"logger":           "#!/bin/sh\necho \"$*: $(cat)\" >> " + logged + "\n",
	}
	for name, body := range fakes {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	script, err := filepath.Abs(hotplugScript)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-c", ". "+script+"; wait")
	cmd.Env = []string{"PATH=" + bin + ":/usr/bin:/bin", "BUTTON=" + button, "ACTION=" + action}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %s: %v %s", button, action, err, out)
	}
	got, _ := os.ReadFile(calls)
	if len(got) > 0 {
		deadline := time.Now().Add(2 * time.Second)
		for {
			l, _ := os.ReadFile(logged)
			if strings.Contains(string(l), "-t travo-greenfield-hook: error unknown-action") {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("daemon reply not logged: %q", l)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	return strings.TrimSpace(string(got))
}

func TestHotplugScriptCallsTheDaemon(t *testing.T) {
	for _, pos := range []string{"pressed", "released"} {
		if got, want := runHotplug(t, "BTN_0", pos), "--hook "+ActionName("BTN_0", pos); got != want {
			t.Errorf("%s: got %q, want %q", pos, got, want)
		}
	}
}

// Reset, WPS and the like keep their stock /etc/rc.button behaviour; a held-button
// timeout is not a switch position.
func TestHotplugScriptIgnoresOtherButtons(t *testing.T) {
	for _, ev := range [][2]string{{"reset", "pressed"}, {"reset", "released"}, {"wps", "pressed"}, {"BTN_0", "timeout"}, {"BTN_10", "pressed"}} {
		if got := runHotplug(t, ev[0], ev[1]); got != "" {
			t.Errorf("%v: daemon called with %q", ev, got)
		}
	}
}
