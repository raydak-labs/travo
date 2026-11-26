package cli

import (
	"context"
	"os"
	"slices"
	"testing"
)

func TestLinkIndices(t *testing.T) {
	raw, err := os.ReadFile("../../../testdata/device/axt1800-25.12.6/wifi-scan/connected/ip-link-indices.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, reply string
		ok          bool
	}{
		{"observed", string(raw), true},
		{"empty", "", false},
		{"missing name", "1: :", false},
		{"missing index", ": lo:", false},
		{"wrong type", "one: lo:", false},
		{"duplicate name", "1: lo:\n2: lo:", false},
		{"duplicate index", "1: lo:\n1: lan:", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := Device{Run: func(_ context.Context, name string, args ...string) (string, int, error) {
				if name != "ip" || !slices.Equal(args, []string{"-o", "link", "show"}) {
					t.Fatal("expected kernel link listing")
				}
				return tc.reply, 0, nil
			}}
			got, err := d.LinkIndices(context.Background())
			if tc.ok {
				if err != nil || got["phy0-sta0"] <= 0 || len(got) != 8 {
					t.Fatalf("link indices %v: %v", got, err)
				}
			} else if got != nil || err != errLinkIndices {
				t.Fatalf("expected untrusted listing to be rejected: %v %v", got, err)
			}
		})
	}
}

func TestLinkIndicesStripsPeerDecoration(t *testing.T) {
	got, err := parseLinkIndices("2: eth0@if281: <BROADCAST> mtu 1500")
	if err != nil || got["eth0"] != 2 || len(got) != 1 {
		t.Fatalf("link indices %v: %v", got, err)
	}
}
