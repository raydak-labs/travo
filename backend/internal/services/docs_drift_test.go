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
//
// The entry has to be a LINK, not a mention: a bare substring is satisfied by
// the sentence "0012 is not an ADR yet", so a new ADR could be filed and left
// unreachable. An unnumbered file in docs/adr is an error for the same reason —
// the numbered series is what makes "read the ADR for this area" possible.
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
	indexLink := regexp.MustCompile(`\]\(\./([^)"]+\.md)\)`)
	hubLink := regexp.MustCompile(`\[\[adr/([^\]|]+?)(?:\|[^\]]*)?\]\]`)

	linked := map[string]bool{}
	for _, m := range indexLink.FindAllStringSubmatch(string(index), -1) {
		linked["index:"+m[1]] = true
	}
	for _, m := range hubLink.FindAllStringSubmatch(string(hub), -1) {
		linked["hub:"+m[1]] = true
	}

	numbered := regexp.MustCompile(`^[0-9]{4}-[a-z0-9-]+\.md$`)
	adrs := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || name == "README.md" || !strings.HasSuffix(name, ".md") {
			continue
		}
		adrs++
		if !numbered.MatchString(name) {
			t.Errorf("docs/adr/%s is not in the numbered ADR series, so nobody knows "+
				"which subsystem owns it; rename it to NNNN-topic.md", name)
			continue
		}
		stem := strings.TrimSuffix(name, ".md")
		if !linked["index:"+name] {
			t.Errorf("ADR %s has no link in docs/adr/README.md", stem)
		}
		if !linked["hub:"+stem] {
			t.Errorf("ADR %s has no wikilink in docs/+ Start here.md, which AGENTS.md "+
				"names as the default retrieval path", stem)
		}
	}
	if adrs == 0 {
		t.Fatal("no ADRs found — the directory walk is broken, so this gate proved nothing")
	}
}

// TestVerificationPlaybooksAreLinked closes the same hole one level down: a
// playbook in docs/tests/ that nothing links to is a document that exists only
// to satisfy the review that asked for it. Any document in the vault may be the
// entry point, so the gate follows links from all of docs/ rather than from one
// nominated file that is easy to forget to update.
func TestVerificationPlaybooksAreLinked(t *testing.T) {
	playbookDir := filepath.Join(docsDir, "tests")
	entries, err := os.ReadDir(playbookDir)
	if err != nil {
		t.Skipf("no playbook directory: %v", err)
	}

	// Basenames are enough: docs/ has one file per basename by convention, and a
	// false link is caught by the resolution check below.
	linked := map[string]bool{}
	filepath.Walk(docsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if !strings.HasSuffix(path, ".md") ||
			info.Name() == ".obsidian" || strings.Contains(path, "_archive") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, re := range []*regexp.Regexp{
			regexp.MustCompile(`\[\[[^\]|]*?([a-z0-9-]+\.md|[a-z0-9-]+)(?:\|[^\]]*)?\]\]`),
			regexp.MustCompile(`\]\([^)]*?([a-z0-9-]+(?:\.md)?)\)`),
		} {
			for _, found := range re.FindAllStringSubmatch(string(body), -1) {
				linked[filepath.Base(found[1])] = true
			}
		}
		return nil
	})

	playbooks := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		playbooks++
		if !linked[e.Name()] && !linked[strings.TrimSuffix(e.Name(), ".md")] {
			t.Errorf("docs/tests/%s is not linked from any document in docs/: a "+
				"playbook nobody can reach is a document that only exists to satisfy "+
				"the review that asked for it", e.Name())
		}
	}
	if playbooks == 0 {
		t.Fatal("no playbooks found in docs/tests — the walk is broken, so this gate " +
			"proved nothing")
	}
}
