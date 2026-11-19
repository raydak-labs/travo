package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
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

func TestNftRulesetListsOneFamily(t *testing.T) {
	var calls []call
	d := &Device{Run: fake(&calls, map[string]string{"nft -j list ruleset ip6": "{}"}, "")}
	if out, err := d.NftRuleset(context.Background(), "ip6"); err != nil || out != "{}" {
		t.Fatalf("ruleset = %q, %v", out, err)
	}
	if _, err := d.NftRuleset(context.Background(), "inet"); err == nil {
		t.Fatal("inet listed: only the iptables-nft families are read")
	}
	if len(calls) != 1 {
		t.Fatalf("calls = %v", calls)
	}
}

func TestDeleteMangleIsOneNftTransaction(t *testing.T) {
	var stdin, cmd string
	d := &Device{RunInput: func(ctx context.Context, in, name string, args ...string) (string, int, error) {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("no timeout tier")
		}
		stdin, cmd = in, name+" "+strings.Join(args, " ")
		return "", 0, nil
	}}
	rules := []device.NftRule{{Chain: "PREROUTING", Handle: 21}, {Chain: "OUTPUT", Handle: 23}}
	if err := d.DeleteMangle(context.Background(), "ip", rules, []string{"mwan3_hook", "mwan3_rules"}); err != nil {
		t.Fatal(err)
	}
	want := "delete rule ip mangle PREROUTING handle 21\ndelete rule ip mangle OUTPUT handle 23\n" +
		"flush chain ip mangle mwan3_hook\nflush chain ip mangle mwan3_rules\n" +
		"delete chain ip mangle mwan3_hook\ndelete chain ip mangle mwan3_rules\n"
	if cmd != "nft -f -" || stdin != want {
		t.Fatalf("%s <<\n%s", cmd, stdin)
	}
}

func TestMangleDeleteScriptRefusesUnsafeNames(t *testing.T) {
	for _, c := range []struct {
		family string
		rules  []device.NftRule
		chains []string
	}{
		{"inet", nil, []string{"mwan3_hook"}},
		{"ip", []device.NftRule{{Chain: "PREROUTING; flush ruleset", Handle: 1}}, nil},
		{"ip", []device.NftRule{{Chain: "PREROUTING", Handle: 0}}, nil},
		{"ip", nil, []string{"mwan3_hook\nflush ruleset"}},
	} {
		if s, err := MangleDeleteScript(c.family, c.rules, c.chains); err == nil {
			t.Fatalf("%+v accepted: %q", c, s)
		}
	}
}
