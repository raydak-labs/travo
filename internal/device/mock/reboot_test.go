package mock

import (
	"errors"
	"testing"
)

func TestBootIDDefaultsAndCanBeSet(t *testing.T) {
	d := New()
	if got, err := d.BootID(ctx); err != nil || got != "mock-boot-0" {
		t.Fatalf("default = %q, %v", got, err)
	}
	d.SetBootID("chosen-boot")
	if got, err := d.BootID(ctx); err != nil || got != "chosen-boot" {
		t.Fatalf("set = %q, %v", got, err)
	}
}

func TestRebootRotatesBootIDOnlyOnSuccess(t *testing.T) {
	d := New()
	d.RotateBootOnReboot = true
	before, _ := d.BootID(ctx)
	d.RebootErr = errors.New("failed")
	if err := d.Reboot(ctx); err == nil {
		t.Fatal("expected error")
	}
	after, _ := d.BootID(ctx)
	if after != before || d.Reboots != 1 {
		t.Fatalf("failed reboot changed id: before=%q after=%q reboots=%d", before, after, d.Reboots)
	}
	d.RebootErr = nil
	if err := d.Reboot(ctx); err != nil {
		t.Fatal(err)
	}
	after, _ = d.BootID(ctx)
	if after == before || d.Reboots != 2 {
		t.Fatalf("successful reboot did not rotate id: before=%q after=%q reboots=%d", before, after, d.Reboots)
	}
}

func TestBootIDError(t *testing.T) {
	d := New()
	d.BootIDErr = errors.New("unreadable")
	if _, err := d.BootID(ctx); err == nil {
		t.Fatal("expected error")
	}
}
