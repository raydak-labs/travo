package uplink

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// vpnReceiverType is the receiver type whose method calls the lock walk
// follows. The walk matches the receiver TYPE rather than a receiver name, so
// renaming a receiver cannot silently change which edges it follows.
const vpnReceiverType = "VpnService"

// A grep-style test, because the failure it guards against cannot be expressed
// as a unit test: the config locks are NOT reentrant, so a VPN flow that took
// the transaction and then called a helper that took it again would self-deadlock
// and take the whole API down with it — every other endpoint that touches a
// config queues on the same per-config locks.
//
// The refactor that made the VPN flows safe split the four firewall helpers into
// a thin locking wrapper plus a lock-free core (setupWireGuardFirewallLocked and
// friends), and pointed every internal caller at the core. This test walks the
// call graph from the five VPN entry points and fails if any function they can
// reach takes a config lock itself.
//
// The walk covers the whole package rather than one file. A helper that lives
// in the same file as its caller is just as capable of taking a lock twice as
// one that lives in another file, so scoping the walk to vpn_service.go would
// let a file split hide a nested lock rather than reveal one.
func TestVPNFlowsDoNotNestConfigLocks(t *testing.T) {
	// Method name -> the names it calls as <recv>.foo(...).
	calls := map[string][]string{}
	// Method name -> true when it takes a config lock.
	locks := map[string]bool{}

	for _, file := range packageFiles(t) {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Body == nil {
				continue
			}
			recvType, recvName := receiverOf(fn)
			if recvType != vpnReceiverType {
				continue
			}
			name := fn.Name.Name
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				x, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				// A direct call to a lock helper.
				if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "mutateWireless" {
					locks[name] = true
				}
				if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
					if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "platform" &&
						(sel.Sel.Name == "MutateUCI" || sel.Sel.Name == "LockUCIConfigs" || sel.Sel.Name == "WithConfigLocks") {
						locks[name] = true
					}
				}
				// A method call on the service: recv.helper(...)
				if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
					if id, ok := sel.X.(*ast.Ident); ok && id.Name == recvName {
						calls[name] = append(calls[name], sel.Sel.Name)
					}
				}
				return true
			})
		}
	}

	entries := []string{
		"ToggleWireguard", "SetWireguardConfig", "SetSplitTunnel",
		"ImportWireguardConfig", "ActivateProfile",
	}
	// The four helpers that legitimately take a lock when entered cold. They
	// must never be reachable from a transaction.
	// Sanity: the walk has to actually see the graph, or this test is vacuous.
	// An earlier version of it returned early from the type switch on a
	// SelectorExpr call, so `calls` was always empty and it passed no matter what.
	// Two properties, not one. "Some functions have calls" is satisfied by a
	// walker that only records a handful of edges; what the assertion needs is
	// that the graph is CONNECTED (a real edge count) and DEEP (the entry point
	// reaches the firewall helpers several hops down). A walker that silently
	// records nothing satisfies neither.
	if len(calls) < 20 {
		t.Fatalf("call graph has only %d functions with calls; the walker is probably broken and this test would pass vacuously", len(calls))
	}
	totalEdges := 0
	for _, c := range calls {
		totalEdges += len(c)
	}
	if totalEdges < 60 {
		t.Fatalf("call graph has only %d edges; the walker is probably broken and this test would pass vacuously", totalEdges)
	}
	if !locks["ToggleWireguard"] {
		t.Fatal("ToggleWireguard is not detected as taking a config lock; the test would pass vacuously")
	}
	// Depth is the real check: the entry points must actually reach the
	// firewall helpers, which is the whole point of the assertion.
	// Depth is checked on the union, not per entry: SetSplitTunnel legitimately
	// never reaches enableWireguard, so a per-entry depth requirement would be
	// wrong. What must hold is that the walk from the entry points actually gets
	// down into the firewall helpers — that is what makes the nesting assertion
	// below meaningful rather than vacuous.
	union := map[string]bool{}
	for _, entry := range entries {
		var walk func(string)
		walk = func(name string) {
			if union[name] {
				return
			}
			union[name] = true
			for _, c := range calls[name] {
				walk(c)
			}
		}
		walk(entry)
	}
	for _, deep := range []string{"enableWireguard", "disableWireguard", "setupWireGuardFirewall", "teardownWireGuardFirewall"} {
		if !union[deep] {
			t.Fatalf("the walk from the entry points never reaches %s "+
				"(%d functions reached); the walker is probably not following %s.* calls", deep, len(union), recvNameOf(t, vpnReceiverType))
		}
	}

	for _, entry := range entries {
		reached := map[string]bool{}
		var walk func(string)
		walk = func(name string) {
			if reached[name] {
				return
			}
			reached[name] = true
			for _, c := range calls[name] {
				walk(c)
			}
		}
		walk(entry)

		for fn := range reached {
			if fn == entry || !locks[fn] {
				continue
			}
			t.Errorf("%s can reach %s, which takes a UCI config lock itself.\n"+
				"The config locks are not reentrant: the transaction already holds "+
				"dhcp/firewall/network, so this self-deadlocks and hangs every "+
				"endpoint that touches a config. Call the ...Locked core instead.",
				entry, fn)
		}
	}
}

// receiverOf returns the receiver type name and the receiver variable name of
// a method, e.g. ("VpnService", "v") for `func (v *VpnService) Foo()`.
func receiverOf(fn *ast.FuncDecl) (typeName, varName string) {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return "", ""
	}
	field := fn.Recv.List[0]
	if field.Type == nil {
		return "", ""
	}
	switch t := field.Type.(type) {
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			typeName = id.Name
		}
	case *ast.Ident:
		typeName = t.Name
	}
	if len(field.Names) > 0 {
		varName = field.Names[0].Name
	}
	return typeName, varName
}

// recvNameOf returns the receiver variable name used by the methods of a
// receiver type. The package uses one name per type, so the first method found
// for the type is representative.
func recvNameOf(t *testing.T, typeName string) string {
	t.Helper()
	for _, file := range packageFiles(t) {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if recvType, recvName := receiverOf(fn); recvType == typeName {
				return recvName
			}
		}
	}
	return "?"
}

// The VPN transaction has to name every config the flow can reach — `dhcp`
// included, which is the one that is easy to miss because
// enableVpnDNSForwarding / disableVpnDNSForwarding shell out to `uci` directly
// instead of going through the UCI interface, so no audit of "v.uci.Set(...)"
// ever sees it.
//
// This parses vpnFlowConfigs out of the source and asserts set membership. The
// first version grepped the whole file for a quoted "dhcp", which any stray
// occurrence satisfied: dropping "dhcp" from the transaction entirely left the
// suite green, which is precisely the bug this test exists to catch.
func TestVPNFlowConfigSetCoversShelledOutUCI(t *testing.T) {
	declared := map[string]bool{}
	found := false
	forEachPackageFile(t, func(file *ast.File) {
		ast.Inspect(file, func(n ast.Node) bool {
			vs, ok := n.(*ast.ValueSpec)
			if !ok || len(vs.Names) != 1 || vs.Names[0].Name != "vpnFlowConfigs" {
				return true
			}
			found = true
			for _, elt := range vs.Values {
				cl, ok := elt.(*ast.CompositeLit)
				if !ok {
					continue
				}
				for _, e := range cl.Elts {
					if bl, ok := e.(*ast.BasicLit); ok {
						declared[strings.Trim(bl.Value, `"`)] = true
					}
				}
			}
			return true
		})
	})
	if !found {
		t.Fatal("vpnFlowConfigs not found; this test would be vacuous")
	}

	// Every config a VPN entry point can reach must be locked. `dhcp` is the one
	// the shelled-out DNS-forwarding helpers mutate behind the UCI interface.
	for _, cfg := range []string{"dhcp", "firewall", "network"} {
		if !declared[cfg] {
			t.Errorf("vpnFlowConfigs does not lock %q (locks %v), but the flow can reach it", cfg, sortedKeys(declared))
		}
	}
}

// The shelled-out writes are the reason dhcp has to be in that set. If someone
// converts them to the UCI interface this test should tell them to drop the
// comment, and if they add another one it should make them think about the set.
//
// The scan covers every non-test file in the package: the shelled-out write is
// a package-wide contract, not a property of one file.
func TestShelledOutUCIWritesAreOnTheRecord(t *testing.T) {
	src := readPackageSource(t)
	// Written as plain literals. An earlier version of this had "fmt.Sprintf("
	// inside a raw string, so it was dead text rather than code, and the
	// surrounding test still passed.
	for _, needle := range []string{
		"\"uci\", \"set\", \"dhcp.",
		"\"uci\", \"add_list\", fmt.Sprintf(\"dhcp.",
		"\"uci\", \"delete\", \"dhcp.",
	} {
		if !strings.Contains(src, needle) {
			t.Errorf("expected to find the shelled-out dnsmasq write %s", needle)
		}
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// packageFiles parses every non-test file in the services package.
//
// The files are read one by one rather than through parser.ParseDir: the
// services package has no build-tagged files, and ParseDir is deprecated
// precisely because it does not reason about build tags.
func packageFiles(t *testing.T) []*ast.File {
	t.Helper()
	fset := token.NewFileSet()
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	var files []*ast.File
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		files = append(files, file)
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name.Name < files[j].Name.Name
	})
	return files
}

// forEachPackageFile parses every non-test file in the services package and
// calls fn with it. A file split must not change what these package-wide
// checks can see.
func forEachPackageFile(t *testing.T, fn func(*ast.File)) {
	t.Helper()
	for _, file := range packageFiles(t) {
		fn(file)
	}
}

// readPackageSource concatenates the source of every non-test file in the
// services package.
func readPackageSource(t *testing.T) string {
	t.Helper()
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	var b strings.Builder
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		b.Write(data)
		b.WriteString("\n")
	}
	return b.String()
}
