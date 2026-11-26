//go:build hardware

package captive

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/connectivity"
	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/dnslayer"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/uciwrite"
)

// Runs ON the router (docs/testing.md "Hardware layer"). It changes live dnsmasq and
// renews the wwan lease, then restores; take a backup first.

func emit(k string, v any) { b, _ := json.Marshal(map[string]any{k: v}); fmt.Println("HW", string(b)) }

func resolves(name string) bool {
	out, err := exec.Command("nslookup", name, "127.0.0.1").CombinedOutput()
	s := string(out)
	return err == nil && strings.Contains(s, "Address") && !strings.Contains(s, "can't resolve")
}

func TestHWBypassUsesUplinkDNSAndRestoresBase(t *testing.T) {
	ctx := context.Background()
	d := cli.New()
	_, before, err := dnslayer.ReadState(ctx, d)
	if err != nil {
		t.Fatal(err)
	}
	emit("base", before)
	dir := t.TempDir()
	layers := &dnslayer.Service{Device: d, Guards: guard.Dir{Path: filepath.Join(dir, "g"), Now: time.Now},
		RecordPath: filepath.Join(dir, "r.json"), Apply: &txn.Manager{Device: d, Locks: &uciwrite.Locks{}}}
	b := NewBypass(layers, d, d, time.Now)
	if err := b.Enable(ctx); err != nil {
		t.Fatal(err)
	}
	_, during, _ := dnslayer.ReadState(ctx, d)
	emit("bypass", during)
	emit("status", b.Status())
	time.Sleep(4 * time.Second) // reload_config restarts dnsmasq asynchronously (SPIKES S5)
	emit("resolve_during_ok", resolves("openwrt.org"))
	if during.RebindProtection.Value != "0" || len(during.Server.Values) == 0 {
		t.Fatalf("bypass not applied: %+v", during)
	}
	if err := b.Restore(ctx); err != nil {
		t.Fatal(err)
	}
	_, after, _ := dnslayer.ReadState(ctx, d)
	emit("after", after)
	time.Sleep(4 * time.Second)
	emit("resolve_after_ok", resolves("kernel.org"))
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("base not restored:\nbefore %+v\nafter  %+v", before, after)
	}
}

type offline struct{}

func (offline) Status(context.Context) connectivity.Status {
	return connectivity.Status{Internet: connectivity.Offline} // force applicability on a working uplink
}
func (offline) Invalidate() {}

func TestHWRenewWWANProvesRoute(t *testing.T) {
	ctx := context.Background()
	d := cli.New()
	g := guard.Dir{Path: filepath.Join(t.TempDir(), "g"), Now: time.Now}
	r := NewRenewer(g, d, d, d, offline{}, time.Now, Sleep)
	start := time.Now()
	err := r.Renew(ctx)
	emit("renew_err", fmt.Sprint(err))
	emit("renew_seconds", time.Since(start).Seconds())
	if err != nil {
		t.Fatal(err)
	}
	route, found, err := d.DefaultIPv4Route(ctx)
	emit("route_after", route)
	if err != nil || !found {
		t.Fatal("no default route after renew", err)
	}
}
