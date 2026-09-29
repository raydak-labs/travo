package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Crash guards were historically split across two directories — /etc/trafo and
// /etc/travo — so a guard written to one was invisible to a check that only
// looked at the other, and a redeploy could not clear all of them. /etc/trafo is
// now the single directory (AGENTS.md, ADR 0003 §2).
//
// This test walks the non-test Go sources and fails if a crash-guard path is
// still declared under /etc/travo. Non-guard state (aliases, WiFi priorities,
// repeater options, the bbolt store) legitimately lives there.
//
// Scope note: a guard path is often composed across statements — for example
// `dir = "/etc/travo"` in one line and `filepath.Join(dir, "pkg-install-in-progress")`
// in another — so the scan works per top-level function: if the function is about
// guards, any /etc/travo reference inside it fails. A whole-line token match
// alone silently missed exactly that case.
func TestCrashGuardsAllLiveUnderEtcTrafo(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read services dir: %v", err)
	}

	isGuardName := func(name string) bool {
		return strings.HasSuffix(name, "-in-progress") ||
			strings.HasSuffix(name, "-crash-guard") ||
			strings.HasSuffix(name, "-failcount")
	}
	aboutGuards := func(text string) bool {
		return strings.Contains(strings.ToLower(text), "guard")
	}
	// Strip quotes/commas/concatenation so "/etc/trafo/" + "x-in-progress" and
	// filepath.Join(dir, "x") can be inspected as plain text.
	clean := func(line string) string {
		return strings.NewReplacer(`"`, "", "'", "", ",", " ", "+", " ").Replace(line)
	}

	legacy, current, files, guardFuncs := 0, 0, 0, 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		files++

		lines := strings.Split(string(data), "\n")
		// Split into top-level func blocks so a guard's directory and its file
		// name can be judged together.
		var start int
		flush := func(end int) {
			block := lines[start:end]
			var body strings.Builder
			for _, l := range block {
				body.WriteString(l)
				body.WriteString("\n")
			}
			text := body.String()
			if !aboutGuards(text) {
				return
			}
			guardFuncs++
			for i, l := range block {
				trimmed := strings.TrimSpace(l)
				if strings.HasPrefix(trimmed, "//") {
					continue
				}
				c := clean(l)
				// Match the bare directory form ("/etc/travo") as well as the
				// directory-with-slash form: a guard dir is usually assigned as a
				// bare string and joined with the file name later.
				for _, dir := range []string{"/etc/trafo", "/etc/travo"} {
					idx := strings.Index(c, dir)
					if idx < 0 {
						continue
					}
					rest := c[idx+len(dir):]
					rest = strings.TrimPrefix(rest, "/")
					if end := strings.IndexAny(rest, " \t"); end >= 0 {
						rest = rest[:end]
					}
					// A guard reference is either the bare guard directory or a
					// guard-named file. Anything else inside a guard function is
					// state that legitimately stays in /etc/travo (config JSON,
					// templates, flags) and must not be reported.
					// Report only a bare guard directory or a guard-named file.
					// A named non-guard file (config JSON, template, flag) is state
					// that legitimately stays in /etc/travo.
					if rest != "" && !isGuardName(rest) {
						continue
					}
					if dir == "/etc/trafo" {
						current++
						continue
					}
					t.Errorf("%s:%d declares crash guard %q under /etc/travo/; guards belong in /etc/trafo/",
						name, start+i+1, filepath.Join("/etc/travo", rest))
					legacy++
				}
			}
		}
		for i, l := range lines {
			if strings.HasPrefix(l, "func ") {
				if i > start {
					flush(i)
				}
				start = i
			}
		}
		flush(len(lines))

		// Also catch a bare guard path on a const/var line outside any func.
		for i, l := range lines {
			trimmed := strings.TrimSpace(l)
			if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "func ") {
				continue
			}
			c := clean(l)
			if !strings.Contains(c, "/etc/travo/") {
				continue
			}
			rest := c[strings.Index(c, "/etc/travo/")+len("/etc/travo/"):]
			if end := strings.IndexAny(rest, " \t"); end > 0 {
				rest = rest[:end]
			}
			if isGuardName(rest) {
				t.Errorf("%s:%d declares crash guard %q under /etc/travo/; guards belong in /etc/trafo/",
					name, i+1, filepath.Base(rest))
				legacy++
			}
		}
	}
	if files == 0 {
		t.Fatal("no Go sources inspected — the scan proves nothing")
	}
	if guardFuncs == 0 {
		t.Fatal("no guard functions found — the scan is broken, so it proves nothing")
	}
	if current == 0 {
		t.Fatal("no /etc/trafo guard paths found — the scan is broken, so it proves nothing")
	}
	t.Logf("scanned %d files, %d guard functions, %d guard paths under /etc/trafo, %d legacy under /etc/travo",
		files, guardFuncs, current, legacy)
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
	// Point the fallback at a private directory instead of redirecting TMPDIR:
	// this package runs many t.Parallel() tests, which would inherit a temp dir
	// that is removed when this test ends.
	w.guardFallbackDir = t.TempDir()
	if err := w.writeCrashGuard("probe"); err != nil {
		t.Fatalf("writeCrashGuard: %v", err)
	}

	resolved := w.resolveGuardDir()
	if resolved == blocker {
		t.Fatal("expected the guard dir resolution to fall back to a writable dir")
	}
	guardFile := filepath.Join(resolved, "probe-in-progress")
	if _, err := os.Stat(guardFile); err != nil {
		t.Fatalf("expected the guard at the resolved path %s: %v", guardFile, err)
	}

	w.clearCrashGuard("probe")

	if _, err := os.Stat(guardFile); !os.IsNotExist(err) {
		t.Errorf("stale crash guard left at %s after a successful operation (stat err: %v)", guardFile, err)
	}
}
