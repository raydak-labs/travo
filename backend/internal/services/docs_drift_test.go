package services

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The /etc/trafo versus /etc/travo split is a live footgun, not a style rule:
// a guard written to the wrong directory is silently invisible to every
// "check the filesystem" instruction, and ADR 0003 §2 exists because a feature
// that fails open with no log line is undiagnosable. TestCrashGuardDirIsEtcTrafo
// and TestNoGuardIsDeclaredUnderEtcTravo already enforce this for Go source, and
// they cannot see markdown — which is where the drift kept coming back. The
// 2026-10-04 review found AGENTS.md (the file agents are told is
// authoritative), ADR 0001, ADR 0005, architecture.md and the on-device testing
// playbook all naming the pre-unification directory, while the code was already
// correct.
//
// These tests close that gap. They read the documentation, not the code.

const (
	docsDir   = "../../../docs"
	agentsMD  = "../../../AGENTS.md"
	claudeMD  = "../../../CLAUDE.md"
	reviewDoc = "../../../2026-10-04-deep-code-review.md"
)

// guardNameRe matches a crash-guard path as written in prose, e.g.
// "/etc/trafo/failover-in-progress".
var guardNameRe = regexp.MustCompile(`(/etc/trav[ao]/[a-z0-9-]*(?:-in-progress|-crash-guard|-failcount))`)

// docsToScan walks the documentation files an agent or operator is likely to
// read before touching a guard or the persistent store.
func docsToScan(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, root := range []string{agentsMD, claudeMD} {
		if _, err := os.Stat(root); err != nil {
			t.Fatalf("stat %s: %v", root, err)
		}
		files = append(files, root)
	}
	// The review report is deliberately NOT scanned: it quotes wrong paths on
	// purpose, because reporting "seven documents say /etc/trafo/travo.db" is the
	// whole point of it. Scanning it would mean every future mention of a fixed
	// bug re-fails this gate. It is still required to exist.
	if _, err := os.Stat(reviewDoc); err != nil {
		t.Fatalf("the review report is missing; docs tests assume it exists: %v", err)
	}

	err := filepath.Walk(docsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".obsidian" || info.Name() == "_archive" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".md") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk docs: %v", err)
	}
	if len(files) < 30 {
		t.Fatalf("only %d documentation files scanned — the walk is probably broken", len(files))
	}
	return files
}

// TestDocsNameCrashGuardsInEtcTrafo asserts that every crash-guard path written
// in documentation lives under /etc/trafo. The pre-unification /etc/travo spelling
// reads as authoritative and sends an agent to the wrong directory, where ADR
// 0003's recovery path and every "check the filesystem" instruction will not see
// the guard.
func TestDocsNameCrashGuardsInEtcTrafo(t *testing.T) {
	files := docsToScan(t)

	// ADR 0003 is the authoritative catalog and is allowed to name the legacy
	// directory once, to explain the upgrade.
	allowed := map[string]bool{
		filepath.Join(docsDir, "adr", "0003-crash-guards-and-live-state.md"): true,
	}

	var problems []string
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		for _, m := range guardNameRe.FindAllString(string(body), -1) {
			if !strings.HasPrefix(m, "/etc/trafo/") && !allowed[f] {
				problems = append(problems, f+": "+m)
			}
		}
	}
	for _, p := range problems {
		t.Errorf("crash guard named under /etc/travo in %s — guards live in "+
			"/etc/trafo (ADR 0003 section 2); /etc/travo holds ordinary state", p)
	}
}

// TestDocsNameThePersistentStoreInEtcTravo covers the same find/replace damage
// in the other direction. The store deliberately lives beside auth.json in
// /etc/travo; seven documentation sites claimed /etc/trafo, which would send an
// operator debugging "sessions come back after a restart" to a file that does not
// exist. ADR 0009 is the owning document and must say /etc/travo.
func TestDocsNameThePersistentStoreInEtcTravo(t *testing.T) {
	files := docsToScan(t)

	var problems []string
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if strings.Contains(string(body), "/etc/trafo/travo.db") {
			problems = append(problems, f)
		}
	}
	for _, p := range problems {
		t.Errorf("%s documents the store at /etc/trafo/travo.db, but the code opens "+
			"it beside auth.json in /etc/travo (ADR 0009)", p)
	}
}

// TestOwningAdrIsReferenced guards the other failure: the Obsidian hub and the
// ADR index are the prescribed retrieval path, and both had fallen behind the
// set. ADR 0010 and ADR 0011 were missing from the hub while being Accepted,
// which hid exactly the two documents that govern the classes of bug this repo
// keeps producing.
func TestOwningAdrIsReferenced(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join(docsDir, "adr"))
	if err != nil {
		t.Fatalf("read adr dir: %v", err)
	}

	index, err := os.ReadFile(filepath.Join(docsDir, "adr", "README.md"))
	if err != nil {
		t.Fatalf("read adr index: %v", err)
	}
	hub, err := os.ReadFile(filepath.Join(docsDir, "+ Start here.md"))
	if err != nil {
		t.Fatalf("read obsidian hub: %v", err)
	}

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, "0") || !strings.HasSuffix(name, ".md") {
			continue
		}
		stem := strings.TrimSuffix(name, ".md")
		if !strings.Contains(string(index), stem) {
			t.Errorf("ADR %s is not listed in docs/adr/README.md", stem)
		}
		if !strings.Contains(string(hub), stem) {
			t.Errorf("ADR %s is not listed in docs/+ Start here.md, which AGENTS.md "+
				"names as the default retrieval path", stem)
		}
	}
}
