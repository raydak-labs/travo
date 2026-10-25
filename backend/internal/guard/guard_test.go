package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultDirIsEtcTrafo(t *testing.T) {
	// ADR 0003 §2, deploy-local.sh and install.sh all clear guards here.
	if DefaultDir != "/etc/trafo" {
		t.Fatalf("DefaultDir = %q, want /etc/trafo", DefaultDir)
	}
}

func TestPathUsesFeatureSuffix(t *testing.T) {
	g := In("/x", "vpn")
	if got := g.Path(); got != "/x/vpn-in-progress" {
		t.Fatalf("Path() = %q", got)
	}
	if got := In("", "vpn").Path(); got != filepath.Join(Dir(), "vpn-in-progress") {
		t.Fatalf("empty dir should resolve to Dir(): %q", got)
	}
}

func TestWritePresentReadClear(t *testing.T) {
	g := In(filepath.Join(t.TempDir(), "nested"), "feature")
	if g.Present() {
		t.Fatal("guard present before Write")
	}
	if err := g.Write("payload"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !g.Present() {
		t.Fatal("guard not present after Write")
	}
	data, err := g.Read()
	if err != nil || string(data) != "payload" {
		t.Fatalf("Read = %q, %v", data, err)
	}
	info, err := os.Stat(g.Path())
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, %v; want 0600", info.Mode().Perm(), err)
	}
	if err := g.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if g.Present() {
		t.Fatal("guard present after Clear")
	}
	if err := g.Clear(); err != nil {
		t.Fatalf("Clear of a missing guard must succeed: %v", err)
	}
}

func TestWriteLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	g := In(dir, "feature")
	for range 3 {
		if err := g.Write("x"); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("dir holds %d entries, want only the guard", len(entries))
	}
}

func TestCreateRefusesExistingGuard(t *testing.T) {
	g := In(t.TempDir(), "pkg-install")
	if err := g.Create("first"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	err := g.Create("second")
	if err == nil || !strings.Contains(err.Error(), g.Path()) || !strings.Contains(err.Error(), "first") {
		t.Fatalf("second Create = %v, want refusal naming the path and the interrupted run", err)
	}
	if data, _ := g.Read(); string(data) != "first" {
		t.Fatalf("refused Create overwrote the guard: %q", data)
	}
}

func TestSetDirRedirectsDefault(t *testing.T) {
	dir := t.TempDir()
	restore := SetDir(dir)
	if got := In("", "x").Path(); got != filepath.Join(dir, "x-in-progress") {
		t.Fatalf("Path after SetDir = %q", got)
	}
	restore()
	if Dir() != DefaultDir {
		t.Fatalf("restore did not reset Dir(): %q", Dir())
	}
}

func TestMarkWritesTimestamp(t *testing.T) {
	g := In(t.TempDir(), "feature")
	if err := g.Mark(); err != nil {
		t.Fatal(err)
	}
	data, _ := g.Read()
	if _, err := time.Parse(time.RFC3339Nano, string(data)); err != nil {
		t.Fatalf("payload %q is not a timestamp: %v", data, err)
	}
}
