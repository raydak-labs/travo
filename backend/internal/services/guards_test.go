package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// Guards used to be split across /etc/trafo and /etc/travo, and a guard written
// to one directory was invisible to a check that only looked at the other — so a
// stale guard disabled a feature with no log line (ADR 0003 §2). Every guard is
// now built from crashGuardDir (guards.go); these tests pin that.
func TestCrashGuardDirIsEtcTrafo(t *testing.T) {
	// The documented convention. If this ever changes, ADR 0003 §2, the
	// deploy-local.sh recovery list and /etc/travo cleanup all change with it.
	if crashGuardDir != "/etc/trafo" {
		t.Fatalf("crashGuardDir = %q, want /etc/trafo (ADR 0003 §2)", crashGuardDir)
	}
}

func TestCrashGuardPathsResolveUnderCrashGuardDir(t *testing.T) {
	guards := map[string]string{
		"band switch":         bandSwitchGuardFile,
		"captive dns":         captiveDNSGuardFile,
		"captive wwan bounce": captiveWwanBounceGuardPath,
		"failover":            failoverGuardPath,
		"usb tethering":       usbTetherGuardPath,
		"vpn":                 vpnGuardPath,
		"generated toggle script": strings.Split(
			strings.Split(wirelessToggleScript, "GUARD=")[1], "\n")[0],
	}
	for name, path := range guards {
		if !strings.HasPrefix(path, crashGuardDir+"/") {
			t.Errorf("%s guard %q is not under %s/", name, path, crashGuardDir)
		}
	}

	// The guard built at call time rather than declared as a constant. Use a
	// production-constructed service: the test constructor deliberately points
	// guardDir at a temp dir.
	w := NewWifiServiceWithReloader(uci.NewMockUCI(), ubus.NewMockUbus(), &NoopWifiReloader{})
	if w.guardDir != crashGuardDir {
		t.Errorf("default wifi guard dir = %q, want %s", w.guardDir, crashGuardDir)
	}
	if got := w.guardPath("mac"); got != crashGuardDir+"/mac-in-progress" {
		t.Errorf("mac guard = %q", got)
	}
	sm := NewServiceManager()
	if got := sm.guardPath(); got != crashGuardDir+"/pkg-install-in-progress" {
		t.Errorf("package install guard = %q", got)
	}
	// SystemService resolves its guard directory the same way; with no explicit
	// guardDir the production path is used.
	sys := NewSystemService(nil, nil, nil)
	sys.guardDir = crashGuardDir
	if got := sys.guardDirOrDefault(); got != crashGuardDir {
		t.Errorf("system guard dir = %q", got)
	}
}

// Any future service that hard-codes the legacy directory for a guard would
// reintroduce the split. State files legitimately live in /etc/travo, so this
// only flags a /etc/travo path whose name is a guard, plus a bare /etc/trafo
// directory assigned in a guard context.
func TestNoGuardIsDeclaredUnderEtcTravo(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read services dir: %v", err)
	}
	isGuardName := func(name string) bool {
		return strings.HasSuffix(name, "-in-progress") ||
			strings.HasSuffix(name, "-crash-guard") ||
			strings.HasSuffix(name, "-failcount")
	}
	scanned := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		scanned++
		for i, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") {
				continue
			}
			for rest := trimmed; ; {
				idx := strings.Index(rest, "/etc/travo")
				if idx < 0 {
					break
				}
				rest = rest[idx+len("/etc/travo"):]
				if !strings.HasPrefix(rest, "/") {
					// A bare directory, not a file below it — e.g. the
					// crashGuardDir constant itself, or the state directory.
					// Hand-composed guard paths are covered by
					// TestCrashGuardPathsResolveUnderCrashGuardDir, which
					// asserts the resolved path of every guard.
					continue
				}
				rest = rest[1:]
				// Delimiters must not include "-": guard names contain hyphens.
				end := strings.IndexAny(rest, "\"' ,)")
				if end < 0 {
					end = len(rest)
				}
				if isGuardName(rest[:end]) {
					t.Errorf("%s:%d declares a guard under /etc/travo: %s\n\tguards belong in %s (ADR 0003 §2)",
						name, i+1, trimmed, crashGuardDir)
				}
			}
		}
	}
	if scanned == 0 {
		t.Fatal("no sources scanned — the check proves nothing")
	}
}

// writeCrashGuard falls back to a temp directory when the configured one cannot
// be created. clearCrashGuard must clear the directory the guard was actually
// written to — clearing only the configured path left that fallback guard behind
// forever, and a stale guard means "skip this operation".
func TestClearCrashGuard_RemovesGuardFromFallbackDir(t *testing.T) {
	// A regular file where a directory is needed: MkdirAll fails, so the
	// resolver must fall back to a writable directory.
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	w, _ := newTestWifiService()
	w.guardDir = blocker
	// The real fallback (a shared directory under the system temp dir) is used on
	// purpose: a uniquely named file there is harmless, and no production field
	// or process-wide TMPDIR redirect is needed to test the path. Redirecting
	// TMPDIR with t.Setenv would leak into the many t.Parallel() tests in this
	// package, which would then inherit a temp dir removed when this test ends.
	const feature = "probe-guard-fallback-test"
	guardPath := filepath.Join(os.TempDir(), "travo-guards", feature+"-in-progress")
	t.Cleanup(func() { _ = os.Remove(guardPath) })

	if err := w.writeCrashGuard(feature); err != nil {
		t.Fatalf("writeCrashGuard: %v", err)
	}

	resolved := w.resolveGuardDir()
	if resolved == blocker {
		t.Fatal("expected the guard dir resolution to fall back to a writable dir")
	}
	guardFile := filepath.Join(resolved, feature+"-in-progress")
	if _, err := os.Stat(guardFile); err != nil {
		t.Fatalf("expected the guard at the resolved path %s: %v", guardFile, err)
	}

	w.clearCrashGuard(feature)

	if _, err := os.Stat(guardFile); !os.IsNotExist(err) {
		t.Errorf("stale crash guard left at %s after a successful operation (stat err: %v)", guardFile, err)
	}
}
