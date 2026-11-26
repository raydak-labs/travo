package cli

import (
	"context"
	"errors"
	"testing"
)

const rebootCommand = "(sleep 2; reboot) </dev/null >/dev/null 2>&1 &"

func TestRebootSchedulesDetachedCommand(t *testing.T) {
	var calls []call
	d := &Device{Run: fake(&calls, nil, "")}
	if err := d.Reboot(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 {
		t.Fatalf("calls = %v", calls)
	}
	if calls[0].name != "setsid" || calls[0].args != "sh -c "+rebootCommand {
		t.Fatalf("call = %+v", calls[0])
	}
}

func TestRebootPropagatesRunnerError(t *testing.T) {
	var calls []call
	d := &Device{Run: fake(&calls, nil, "sleep 2")}
	if err := d.Reboot(context.Background()); err == nil {
		t.Fatal("expected runner error")
	}
	if len(calls) != 1 {
		t.Fatalf("calls = %v", calls)
	}
}

func TestBootIDUsesReadFileHookAndTrims(t *testing.T) {
	d := &Device{ReadFile: func(path string) ([]byte, error) {
		if path != "/proc/sys/kernel/random/boot_id" {
			t.Fatalf("path = %q", path)
		}
		return []byte("  kernel-boot-id\n"), nil
	}}
	got, err := d.BootID(context.Background())
	if err != nil || got != "kernel-boot-id" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestBootIDRejectsEmptyAndReadFailure(t *testing.T) {
	d := &Device{ReadFile: func(string) ([]byte, error) { return []byte(" \n"), nil }}
	if _, err := d.BootID(context.Background()); err == nil {
		t.Fatal("accepted empty boot id")
	}
	d.ReadFile = func(string) ([]byte, error) { return nil, errors.New("read failed") }
	if _, err := d.BootID(context.Background()); err == nil {
		t.Fatal("accepted read failure")
	}
}
