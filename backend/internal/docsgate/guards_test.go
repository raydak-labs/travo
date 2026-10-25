package docsgate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/guard"
)

// A guard declared under /etc/travo would be invisible to the startup checks
// and to deploy-local.sh, which only look in the guard dir. State files
// legitimately live in /etc/travo, so this only flags a /etc/travo path whose
// name is a guard.
func TestNoGuardIsDeclaredUnderEtcTravo(t *testing.T) {
	var files []string
	err := filepath.WalkDir("..", func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return err
	})
	if err != nil {
		t.Fatalf("walk sources: %v", err)
	}
	isGuardName := func(name string) bool {
		return strings.HasSuffix(name, "-in-progress") ||
			strings.HasSuffix(name, "-crash-guard") ||
			strings.HasSuffix(name, "-failcount")
	}
	scanned := 0
	for _, name := range files {
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
