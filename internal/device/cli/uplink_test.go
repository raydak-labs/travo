package cli

import (
	"context"
	"testing"
)

func TestReconnectUplinkRunsIfupWWANOnly(t *testing.T) {
	var calls []call
	d := &Device{Run: fake(&calls, nil, "")}
	if err := d.ReconnectUplink(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0].name != "ifup" || calls[0].args != "wwan" {
		t.Fatalf("calls = %+v", calls)
	}
}

func TestReconnectUplinkPropagatesRunnerError(t *testing.T) {
	var calls []call
	d := &Device{Run: fake(&calls, nil, "wwan")}
	if err := d.ReconnectUplink(context.Background()); err == nil {
		t.Fatal("expected runner error")
	}
}
