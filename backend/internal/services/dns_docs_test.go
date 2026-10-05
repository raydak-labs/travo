package services

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Finding 7 of the 2026-10-04 review: ADR 0001 said the opposite of what the
// branch implemented. It claimed "a unified stack of snapshots is NOT IMPLEMENTED
// today", named /etc/trafo/vpn-dns-snapshot.json as the file the code writes
// (the constant is /etc/travo/vpn-dns-snapshot.json), promised that disable
// DELETES the snapshot, and promised one restore entrypoint per feature while
// there are two. The file the code actually writes,
// /etc/trafo/dnsmasq-layers.json, appeared nowhere in docs/ — while roughly ten
// comments in the services package cite "ADR 0001 section 3" as its authority.
//
// The existing docs_drift_test.go cannot see this: it matches only paths ending
// in -in-progress / -crash-guard / -failcount, and its other gate greps for one
// literal (travo.db). Both are guards_test-style literal lists that nobody has
// to remember to extend.
//
// The gate below derives its inputs from the code instead. It parses every
// non-test file of this package, collects the /etc/travo//etc/trafo/ path
// literals each file declares, and requires the ADR that owns the feature to
// name them. Adding a state file is then a one-line code change that cannot
// land undocumented.

const (
	adrDir = "../../../docs/adr"
	// adr0001 is the owning ADR for dnsmasq resolver state: it is the document
	// the VPN and AdGuard services cite for the shared layer stack.
	adr0001 = adrDir + "/0001-dns-vpn-captive-portal-architecture.md"
	// adr0003 owns the crash guards (its section 2 is the authoritative list).
	adr0003 = adrDir + "/0003-crash-guards-and-live-state.md"
)

// statePathRe matches a full path literal under /etc/trafo/ or /etc/travo/ that
// names a FILE. A bare directory literal ("/etc/trafo") is the crashGuardDir
// root itself and needs no ADR row.
//
// The two directories are spelled out in full on purpose: `trav[ao]` never
// matches "traf", and a guard regex written that way silently passes.
//
// Guard paths built from crashGuardDir never match: the constant in the source
// is "/captive-dns-in-progress", not the full path. guardSuffixes still covers
// a future guard written as a full literal.
var statePathRe = regexp.MustCompile(`^/etc/tra(?:vo|fo)(/[\w.-]+)+$`)

var guardSuffixes = []string{"-in-progress", "-crash-guard", "-failcount"}

// resolverOptionMarker is the dnsmasq option both DNS-forwarding features write.
// A file that names it writes dhcp.@dnsmasq[0]'s resolver state, so it is a DNS
// resolver-state owner and ADR 0001 owns whatever paths it declares. Deriving
// the file set from this marker means a new DNS service, or a new path added to
// an existing one, is covered without editing this test.
const resolverOptionMarker = "noresolv"

// statePathRef is one declared /etc/travo|/etc/trafo path and the identifier
// that declares it, so a failure can name the constant to document.
type statePathRef struct {
	file       string // package file that declares it
	owner      string // constant, var or function it is declared in
	path       string // the full path literal
	guard      bool   // crash guard rather than state
	inGuardDir bool   // under /etc/trafo, the guard directory
}

// String names the path and the declaration to document, so a failure tells the
// next person exactly which constant to cite.
func (r statePathRef) String() string {
	return r.path + " (declared in " + r.file + ", " + r.owner + ")"
}

// statePathRefsInSource parses one Go source and returns the state paths it
// declares, each tagged with the declaration it sits in.
func statePathRefsInSource(filename, src string) ([]statePathRef, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		return nil, err
	}

	var refs []statePathRef
	var add func(n ast.Node, owner string)
	addLit := func(bl *ast.BasicLit, owner string) {
		if bl.Kind != token.STRING {
			return
		}
		val, err := strconv.Unquote(bl.Value)
		if err != nil || !statePathRe.MatchString(val) {
			return
		}
		ref := statePathRef{
			file:       filepath.Base(filename),
			owner:      owner,
			path:       val,
			inGuardDir: strings.HasPrefix(val, "/etc/trafo/"),
		}
		for _, s := range guardSuffixes {
			if strings.HasSuffix(val, s) {
				ref.guard = true
			}
		}
		refs = append(refs, ref)
	}

	add = func(n ast.Node, owner string) {
		switch d := n.(type) {
		case *ast.BasicLit:
			addLit(d, owner)
			return
		case *ast.FuncDecl:
			owner = d.Name.Name
		case *ast.GenDecl:
			// A const/var spec is attributed to its own names so the failure
			// message names the constant rather than the file.
			inner := owner
			for _, spec := range d.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				names := make([]string, 0, len(vs.Names))
				for _, id := range vs.Names {
					names = append(names, id.Name)
				}
				if len(names) > 0 {
					inner = strings.Join(names, " / ")
				}
				add(spec, inner)
			}
			return
		}
		for _, child := range childrenOf(n) {
			add(child, owner)
		}
	}

	for _, decl := range f.Decls {
		add(decl, "")
	}
	return refs, nil
}

// childrenOf returns a node's child nodes without descending further.
func childrenOf(n ast.Node) []ast.Node {
	var out []ast.Node
	ast.Inspect(n, func(c ast.Node) bool {
		if c == nil || c == n {
			return c == n
		}
		out = append(out, c)
		return false
	})
	return out
}

// packageStatePathRefs scans every non-test Go file of this package for declared
// state paths.
func packageStatePathRefs(t *testing.T, dir string) []statePathRef {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var refs []statePathRef
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		fileRefs, err := statePathRefsInSource(name, string(src))
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		refs = append(refs, fileRefs...)
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].String() < refs[j].String() })
	return dedupeRefs(refs)
}

// dedupeRefs collapses the same path declared twice in one file (a constructor
// that takes the path as an argument defaults it twice) into one finding, so the
// failure output stays readable.
func dedupeRefs(refs []statePathRef) []statePathRef {
	seen := map[string]bool{}
	out := refs[:0]
	for _, r := range refs {
		key := r.file + " " + r.path
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, r)
	}
	return out
}

// resolverStateOwners returns the package files that write dnsmasq's resolver
// options. Both DNS forwarding features touch the same two UCI options of the
// same section, so ADR 0001 owns every state path declared by these files.
func resolverStateOwners(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var owners []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if strings.Contains(string(src), resolverOptionMarker) {
			owners = append(owners, name)
		}
	}
	sort.Strings(owners)
	if len(owners) == 0 {
		t.Fatalf("no services file mentions %q; the owner scan is broken",
			resolverOptionMarker)
	}
	return owners
}

func readDoc(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}

// TestDnsResolverStatePathsAreNamedInAdr0001 is the gate for Finding 7: a path
// this code writes to persistent DNS state that the owning ADR does not name is
// drift, because the code's comments cite that ADR as the authority.
func TestDnsResolverStatePathsAreNamedInAdr0001(t *testing.T) {
	adr := readDoc(t, adr0001)
	owners := map[string]bool{}
	for _, name := range resolverStateOwners(t, ".") {
		owners[name] = true
	}

	var problems []string
	for _, ref := range packageStatePathRefs(t, ".") {
		if !owners[ref.file] || ref.guard {
			continue
		}
		if !strings.Contains(adr, ref.path) {
			problems = append(problems, ref.String())
		}
	}
	for _, p := range problems {
		t.Errorf("ADR 0001 does not name the dnsmasq resolver state path %s."+
			" Document the path and its restore semantics in %s, or, if it is a"+
			" crash guard, list it in %s section 2.", p, adr0001, adr0003)
	}
}

// TestEtcTrafoStatePathsAreNamedInAnAdr widens the same rule to the whole
// services package for /etc/trafo, the guard directory: anything persisted there
// that is not a crash guard is live state an operator has to be able to find and
// reason about, so it must appear in some ADR.
//
// The rule stops at the /etc/travo boundary on purpose. Most /etc/travo state
// files (aliases, radio policies, thresholds) are documented in a plan or not at
// all, and demanding an ADR row for each of them today would be a failing test
// that nobody can land. Widening this to /etc/travo is a follow-up: either
// document those files in the ADR that owns them, or drop them from the shipped
// image.
func TestEtcTrafoStatePathsAreNamedInAnAdr(t *testing.T) {
	entries, err := os.ReadDir(adrDir)
	if err != nil {
		t.Fatalf("read %s: %v", adrDir, err)
	}
	var all strings.Builder
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		all.WriteString(readDoc(t, filepath.Join(adrDir, e.Name())))
	}
	adrs := all.String()

	var problems []string
	for _, ref := range packageStatePathRefs(t, ".") {
		if ref.guard || !ref.inGuardDir {
			continue
		}
		if !strings.Contains(adrs, ref.path) {
			problems = append(problems, ref.String())
		}
	}
	for _, p := range problems {
		t.Errorf("the code persists %s under /etc/trafo but no ADR in %s names"+
			" it. Add the path to the ADR that owns the feature.", p, adrDir)
	}
}

// TestStatePathRefsInSourceFindsDeclaredPaths proves the scanner itself, so the
// two gates above cannot pass by scanning nothing: a declared constant, a var
// and a literal inside a function body are all found, each tagged with the
// declaration it sits in, and a bare directory or guard fragment is not.
func TestStatePathRefsInSourceFindsDeclaredPaths(t *testing.T) {
	src := `package services

const (
	crashGuardDir = "/etc/trafo"
	probeStatePath = "/etc/trafo/probe-state.json"
)

func writesStuff() {
	os.WriteFile("/etc/travo/probe-var.json", nil, 0o600)
	os.WriteFile("/etc/trafo", nil, 0o755)
	os.WriteFile(crashGuardDir+"/probe-in-progress", nil, 0o600)
}
`
	refs, err := statePathRefsInSource("probe.go", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	got := map[string]string{}
	for _, r := range refs {
		got[r.path] = r.owner
	}
	want := map[string]string{
		"/etc/trafo/probe-state.json": "probeStatePath",
		"/etc/travo/probe-var.json":   "writesStuff",
	}
	if len(refs) != len(want) {
		t.Fatalf("found %d refs %v, want exactly %v", len(refs), got, want)
	}
	for path, owner := range want {
		if got[path] != owner {
			t.Errorf("%s attributed to %q, want %q", path, got[path], owner)
		}
	}
}
