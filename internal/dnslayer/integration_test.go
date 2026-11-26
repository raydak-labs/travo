//go:build integration

package dnslayer

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/uciwrite"
)

// Real uci in an OpenWrt container: anonymous @dnsmasq[0], list rewrite, absent-option restore.
func TestRealStackRoundTrip(t *testing.T) {
	ctx := context.Background()
	d := cli.New()
	if p, err := d.Pending(ctx, "dhcp"); err != nil || p {
		t.Fatalf("precondition: dhcp must have no staged changes (pending=%v err=%v)", p, err)
	}
	_, before, err := ReadState(ctx, d)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	s := &Service{Device: d, Guards: guard.Dir{Path: filepath.Join(dir, "g"), Now: time.Now},
		RecordPath: filepath.Join(dir, "r.json"), Apply: &txn.Manager{Device: d, Locks: &uciwrite.Locks{}}}
	for _, l := range []Layer{
		{Name: VPN, Servers: []string{"10.2.0.1"}, NoResolv: true},
		{Name: Captive, Servers: []string{"10.0.0.1"}, DisableRebind: true},
		{Name: Tailscale, Servers: []string{"/ts.net/100.100.100.100"}},
	} {
		if err := s.Push(ctx, l); err != nil {
			t.Fatal(err)
		}
	}
	_, mid, _ := ReadState(ctx, d)
	if !reflect.DeepEqual(mid.Server.Values, []string{"10.0.0.1", "/ts.net/100.100.100.100"}) || mid.RebindProtection.Value != "0" {
		t.Fatalf("effective on device: %+v", mid)
	}
	for _, n := range []Name{VPN, Tailscale, Captive} {
		if err := s.Pop(ctx, n); err != nil {
			t.Fatal(err)
		}
	}
	_, after, _ := ReadState(ctx, d)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("base not restored on real uci:\n%+v\n%+v", before, after)
	}
}
