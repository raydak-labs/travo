//go:build integration

package cli

import (
	"context"
	"os"
	"slices"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// Real uci: a staged reorder moves the section in `uci export` order, which is the order
// mwan3 evaluates rules in. Container fixture: format and semantics only.
func TestRealUCIReorder(t *testing.T) {
	ctx := context.Background()
	const config = "/etc/config/travoreorder"
	if err := os.WriteFile(config, []byte("config rule 'https'\n\toption proto 'tcp'\n\nconfig rule 'default_rule_v4'\n\toption family 'ipv4'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(config) })
	d := New()
	if err := d.SetType(ctx, "travoreorder", "travo_v4", "rule"); err != nil {
		t.Fatal(err)
	}
	if err := d.Reorder(ctx, "travoreorder", "travo_v4", 0); err != nil {
		t.Fatal(err)
	}
	if err := d.Commit(ctx, "travoreorder"); err != nil {
		t.Fatal(err)
	}
	secs, err := d.Show(ctx, "travoreorder")
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(secs))
	for _, s := range secs {
		names = append(names, s.Name)
	}
	if !slices.Equal(names, []string{"travo_v4", "https", "default_rule_v4"}) {
		t.Fatalf("order after reorder = %v", names)
	}
	var _ device.UCIReorderer = d
}
