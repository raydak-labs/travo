package services

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// A link that points at a document which no longer exists is how a deleted doc
// keeps being referred to: AGENTS.md prescribes "a link check for anything you
// moved", but nothing resolved those links, so removing CLAUDE.md reached main
// and the only mechanical signal was a docs gate dying on a missing file.
//
// docs/_archive is deliberately excluded: it holds frozen point-in-time reports
// whose references describe the tree as it was, and 50 of its 61 links are
// broken for that reason alone (docs/README.md says so).
var (
	mdLinkRe    = regexp.MustCompile(`\]\(([^)]+)\)`)
	wikiLinkRe  = regexp.MustCompile(`\[\[([^\]|]+?)(?:\|[^\]]*)?\]\]`)
	bareURISkip = regexp.MustCompile(`^(?:[a-z][a-z0-9+.-]*:|//|#)`)
)

// TestDocsLinksResolve fails on any link or wikilink in the repo-root docs and
// the docs vault whose target does not exist.
func TestDocsLinksResolve(t *testing.T) {
	files := docsToScan(t)

	// Guards against a resolver that silently matches nothing, which would make
	// this gate pass vacuously.
	var found int
	for _, f := range files {
		found += resolveLinksIn(t, f)
	}
	if found < 100 {
		t.Fatalf("only %d links resolved across %d documents — the extractor is "+
			"probably broken, so this gate proved nothing", found, len(files))
	}
	t.Logf("%d links resolved across %d documents", found, len(files))
}

func resolveLinksIn(t *testing.T, path string) int {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	text := string(body)
	resolved := 0

	for _, m := range mdLinkRe.FindAllStringSubmatch(text, -1) {
		target := linkTarget(m[1])
		if target == "" {
			continue
		}
		resolved++
		if p, ok := resolveRelative(target, path); !ok {
			t.Errorf("%s links to %s, which does not exist", path, m[1])
		} else if p == "" {
			t.Errorf("%s links to %s, which is not a file", path, m[1])
		}
	}

	for _, m := range wikiLinkRe.FindAllStringSubmatch(text, -1) {
		target := strings.TrimSpace(m[1])
		if target == "" || strings.Contains(target, "#") && !strings.Contains(target, "/") {
			continue // heading-only link inside the note itself
		}
		resolved++
		if _, ok := resolveWiki(target, path); !ok {
			t.Errorf("%s links to [[%s]], which does not exist", path, target)
		}
	}
	return resolved
}

// linkTarget strips the decorations a markdown link may carry and returns ""
// for links this gate does not resolve: absolute URLs, mailto, protocol-relative
// and in-page anchors.
func linkTarget(raw string) string {
	// A URL may contain spaces (docs/+ Start here.md), so only trim the edges.
	t := strings.TrimSpace(raw)
	if t == "" || bareURISkip.MatchString(t) {
		return ""
	}
	if i := strings.IndexAny(t, "?#"); i >= 0 {
		t = t[:i]
	}
	if unescaped := strings.ReplaceAll(t, "%20", " "); unescaped != t {
		t = unescaped
	}
	return t
}

// resolveRelative resolves a link target against the document holding it. A
// directory target is fine — docs/README.md links to ./guides/ as a section —
// so the second result is false only when nothing is there.
func resolveRelative(target, from string) (string, bool) {
	p := target
	if !filepath.IsAbs(target) {
		p = filepath.Join(filepath.Dir(from), filepath.FromSlash(target))
	}
	if _, err := os.Stat(p); err != nil {
		return "", false
	}
	return p, true
}

// resolveWiki resolves an Obsidian wikilink. The vault root is docs/, but a note
// may also be linked from the repo root or by bare name, so each form the vault
// allows is tried: relative to the linking note, docs/-relative, then
// repo-root-relative. The .md suffix is optional, which is the Obsidian default.
func resolveWiki(target, from string) (string, bool) {
	if i := strings.Index(target, "#"); i >= 0 {
		target = target[:i]
	}
	if target == "" {
		return from, true
	}
	bases := []string{filepath.Dir(from), docsDir, repoRoot}
	names := []string{target, target + ".md"}
	for _, b := range bases {
		for _, n := range names {
			p := filepath.Join(b, filepath.FromSlash(n))
			if info, err := os.Stat(p); err == nil && !info.IsDir() {
				return p, true
			}
		}
	}
	return "", false
}

// TestWikiLinkResolverFindsAKnownLink is the self-test for resolveWiki: without
// it, a resolver that matched nothing would leave TestDocsLinksResolve green
// while proving nothing.
func TestWikiLinkResolverFindsAKnownLink(t *testing.T) {
	from := filepath.Join(docsDir, "+ Start here.md")
	for _, target := range []string{"adr/README", "adr/README.md", "architecture/overview"} {
		if _, ok := resolveWiki(target, from); !ok {
			t.Errorf("resolveWiki(%q) from %s failed on a link that exists", target, from)
		}
	}
	if _, ok := resolveWiki("adr/9999-does-not-exist", from); ok {
		t.Error("resolveWiki resolved a target that does not exist")
	}
}
