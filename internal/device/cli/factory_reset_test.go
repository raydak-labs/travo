package cli

import (
	"context"
	"testing"
)

// The fake runner only records argv: nothing is executed.
func TestFactoryResetSchedulesDetachedWipeThenReboot(t *testing.T) {
	var calls []call
	d := &Device{Run: fake(&calls, nil, "")}
	if err := d.FactoryReset(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := "sh -c (sleep 2; firstboot -y && reboot) </dev/null >/dev/null 2>&1 &"
	if len(calls) != 1 || calls[0].name != "setsid" || calls[0].args != want {
		t.Fatalf("calls = %+v", calls)
	}
}

func TestFactoryResetPropagatesRunnerError(t *testing.T) {
	var calls []call
	d := &Device{Run: fake(&calls, nil, "sleep 2")}
	if err := d.FactoryReset(context.Background()); err == nil {
		t.Fatal("expected runner error")
	}
}
