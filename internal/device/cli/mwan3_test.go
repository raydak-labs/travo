package cli

import (
	"context"
	"testing"
)

func TestMWAN3ReadersRunReadOnlyCommands(t *testing.T) {
	var calls []call
	d := &Device{Run: fake(&calls, map[string]string{
		"mwan3 interfaces":         "Interface status:\n",
		"iptables -t mangle -w -S": "-P PREROUTING ACCEPT\n",
	}, "")}
	if out, err := d.MWAN3Interfaces(context.Background()); err != nil || out != "Interface status:\n" {
		t.Fatalf("interfaces = %q, %v", out, err)
	}
	if out, err := d.MangleRules(context.Background()); err != nil || out != "-P PREROUTING ACCEPT\n" {
		t.Fatalf("mangle = %q, %v", out, err)
	}
	if len(calls) != 2 {
		t.Fatalf("calls = %v", calls)
	}
}
