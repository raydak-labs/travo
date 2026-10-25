package services

import (
	"github.com/openwrt-travel-gui/backend/internal/guard"

	"os"
	"strings"
	"testing"
)

// The generated shell helpers cannot call the guard package, so they spell
// the directory out; it must be the one every Go guard uses (ADR 0003 §2).
func TestShellHelpersUseTheGuardDir(t *testing.T) {
	toggle := strings.Split(strings.Split(wirelessToggleScript, "GUARD=")[1], "\n")[0]
	if !strings.HasPrefix(toggle, guard.DefaultDir+"/") {
		t.Errorf("toggle script guard %q is not under %s/", toggle, guard.DefaultDir)
	}
	if !strings.Contains(reconnectScriptContent, `GUARD_DIR="`+guard.DefaultDir+`"`) {
		t.Errorf("reconnect script does not use %s as its guard dir", guard.DefaultDir)
	}
}

// A guard declared under /etc/travo would be invisible to the startup checks
// and to deploy-local.sh, which only look in the guard dir. State files legitimately live in /etc/travo, so this
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
					// A bare directory, not a file below it: the state dir.
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
						name, i+1, trimmed, guard.DefaultDir)
				}
			}
		}
	}
	if scanned == 0 {
		t.Fatal("no sources scanned — the check proves nothing")
	}
}
