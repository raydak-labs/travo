package app

import (
	"context"
	"os"
	"testing"
)

func TestMockModeNeedsNoDevice(t *testing.T) {
	d, err := New(Options{Mock: true})
	t.Cleanup(func() { _ = d.Close() })
	if err != nil || !d.Mock || d.Device == nil || d.Packages == nil || d.Accounts == nil || d.Txn == nil {
		t.Fatalf("%+v %v", d, err)
	}
}

func TestMockIsNotOpenUnlessFirstRun(t *testing.T) {
	closed, _ := New(Options{Mock: true})
	t.Cleanup(func() { _ = closed.Close() })
	if set, _ := closed.Accounts.RootPasswordSet(context.Background()); !set {
		t.Fatal("plain mock mode must have a root password")
	}
	open, _ := New(Options{Mock: true, FirstRun: true})
	t.Cleanup(func() { _ = open.Close() })
	if set, _ := open.Accounts.RootPasswordSet(context.Background()); set {
		t.Fatal("--mock-firstrun must start without a password")
	}
}

func TestCloseRemovesMockStateDir(t *testing.T) {
	d, err := New(Options{Mock: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(d.Paths.State); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(d.Paths.State); !os.IsNotExist(err) {
		t.Fatalf("mock dir left behind: %v", err)
	}
}

func TestRebootFileAndMockBootRotation(t *testing.T) {
	if got := RealPaths().RebootFile(); got != "/etc/travo-greenfield/reboot.json" {
		t.Fatalf("reboot file = %q", got)
	}
	d, err := New(Options{Mock: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	router, ok := d.Device.(interface {
		Reboot(context.Context) error
		BootID(context.Context) (string, error)
	})
	if !ok {
		t.Fatal("mock device does not implement reboot ports")
	}
	before, _ := router.BootID(context.Background())
	if err := router.Reboot(context.Background()); err != nil {
		t.Fatal(err)
	}
	after, _ := router.BootID(context.Background())
	if before == after {
		t.Fatalf("mock boot id did not rotate: %q", before)
	}
}
