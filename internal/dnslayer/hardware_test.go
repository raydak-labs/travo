//go:build hardware

package dnslayer

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

	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/uciwrite"
)

// Spike S5 on the router: the stack against real dnsmasq (+ AdGuard as installed).

func emit(k string, v any) { b, _ := json.Marshal(map[string]any{k: v}); fmt.Println("HW", string(b)) }

// fresh names defeat dnsmasq's cache, which hid a dead AdGuard for minutes (device-facts §11).
var fresh = []string{"openwrt.org", "kernel.org", "debian.org", "golang.org", "mozilla.org", "wikipedia.org", "python.org", "rust-lang.org", "gnu.org", "apache.org", "nginx.org", "postgresql.org", "sqlite.org", "freebsd.org", "archlinux.org", "ubuntu.com", "fedoraproject.org", "gentoo.org", "alpinelinux.org", "nixos.org"}
var next int

// resolve asks the server for a name not asked before in this run.
func resolve(t *testing.T, server string) (string, bool) {
	name := fresh[next%len(fresh)]
	next++
	out, err := exec.Command("nslookup", name, server).CombinedOutput()
	s := string(out)
	ok := err == nil && strings.Contains(s, "Address") && !strings.Contains(s, "can't resolve") && !strings.Contains(s, "SERVFAIL")
	return s, ok
}

func dnsmasqServers(t *testing.T) []string {
	_, st, err := ReadState(context.Background(), cli.New())
	if err != nil {
		t.Fatal(err)
	}
	return st.Server.Values
}

func svc(dir string) *Service {
	d := cli.New()
	return &Service{Device: d, Guards: guard.Dir{Path: filepath.Join(dir, "g"), Now: time.Now},
		RecordPath: filepath.Join(dir, "r.json"), Apply: &txn.Manager{Device: d, Locks: &uciwrite.Locks{}}}
}

// Captive layer over the existing AdGuard forwarding, then pop: base (scalar server,
// noresolv=1) must come back byte-for-byte and DNS must keep working throughout.
func TestHWCaptiveOverAdGuardRestoresBase(t *testing.T) {
	ctx := context.Background()
	d := cli.New()
	_, before, err := ReadState(ctx, d)
	if err != nil {
		t.Fatal(err)
	}
	emit("base", before)
	s := svc(t.TempDir())
	if err := s.Push(ctx, Layer{Name: Captive, Servers: []string{"192.168.90.2"}, DisableRebind: true}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Second) // dnsmasq restart via procd trigger
	emit("captive_servers", dnsmasqServers(t))
	out, ok := resolve(t, "127.0.0.1")
	emit("resolve_via_captive_ok", ok)
	if !ok {
		t.Log(out)
	}
	if err := s.Pop(ctx, Captive); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Second)
	_, after, _ := ReadState(ctx, d)
	emit("after", after)
	emit("base_restored_exact", reflect.DeepEqual(before, after))
	_, ok = resolve(t, "127.0.0.1")
	emit("resolve_after_pop_ok", ok)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("base not restored")
	}
}

// AdGuard down while dnsmasq forwards to it: LAN DNS dies (device-facts §11). Popping the
// adguard layer to the uplink resolver must make DNS work again (fail open).
func TestHWAdGuardDownFailOpen(t *testing.T) {
	ctx := context.Background()
	d := cli.New()
	_, before, _ := ReadState(ctx, d)
	if err := exec.Command("/etc/init.d/adguardhome", "stop").Run(); err != nil {
		t.Fatal(err)
	}
	defer exec.Command("/etc/init.d/adguardhome", "start").Run()
	time.Sleep(2 * time.Second)
	_, ok := resolve(t, "127.0.0.1")
	emit("resolve_with_agh_down_ok", ok)

	// Base on this device IS the adguard forwarding (old Travo set it without a record).
	// Simulate the fail-open: a layer pointing at the uplink resolver wins over base.
	s := svc(t.TempDir())
	t0 := time.Now()
	if err := s.Push(ctx, Layer{Name: Captive, Servers: []string{"192.168.90.2"}}); err != nil {
		t.Fatal(err)
	}
	var okAfter bool
	for time.Since(t0) < 15*time.Second {
		if _, okAfter = resolve(t, "127.0.0.1"); okAfter {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	emit("fail_open_resolve_ok", okAfter)
	emit("fail_open_after_s", time.Since(t0).Seconds())
	if err := s.Pop(ctx, Captive); err != nil {
		t.Fatal(err)
	}
	_, after, _ := ReadState(ctx, d)
	emit("base_restored_exact", reflect.DeepEqual(before, after))
	if !okAfter || !reflect.DeepEqual(before, after) {
		t.Fatal("fail-open or restore failed")
	}
}
