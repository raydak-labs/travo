package adguard

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestToExpertAndBackIsExact(t *testing.T) {
	hosts := ExpertHosts("192.168.1.1", "192.168.2.1")
	out, saved, err := ToExpert(Seed, hosts)
	if err != nil {
		t.Fatal(err)
	}
	l, err := ParseListen(out)
	if err != nil || l.HTTP != LoopbackHTTP || l.DNSPort != "53" || !slices.Equal(l.BindHosts, hosts) {
		t.Fatalf("listen %+v %v", l, err)
	}
	if !IsExpert(out, hosts) || IsExpert(out, hosts[:2]) || IsExpert(Seed, hosts) {
		t.Fatal("IsExpert must match only the expert config for these hosts")
	}
	for _, want := range []string{"  upstream_dns:\n    - 127.0.0.1:5354\n", "  fallback_dns: []\n", "  local_ptr_upstreams:\n    - 127.0.0.1:5354\n", "  use_private_ptr_resolvers: true\n"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("expert config lacks %q", want)
		}
	}
	// the admin UI and every other key stay as they were
	if !strings.Contains(string(out), "  address: 127.0.0.1:3000\n") || !strings.Contains(string(out), "  bootstrap_dns:\n    - 9.9.9.10\n") {
		t.Error("unrelated keys changed")
	}
	back, err := ToDefault(out, saved)
	if err != nil || !bytes.Equal(back, Seed) {
		t.Fatalf("switching back must restore the seed byte for byte: %v", err)
	}
	again, err := ToDefault(back, saved)
	if err != nil || !bytes.Equal(again, Seed) {
		t.Fatalf("ToDefault is idempotent (a retried revert): %v", err)
	}
}

func TestToExpertKeepsEditsMadeMeanwhile(t *testing.T) {
	out, saved, err := ToExpert(Seed, ExpertHosts("192.168.1.1", ""))
	if err != nil {
		t.Fatal(err)
	}
	edited := bytes.Replace(out, []byte("  ratelimit: 20\n"), []byte("  ratelimit: 50\n"), 1)
	back, err := ToDefault(edited, saved)
	if err != nil || !bytes.Equal(back, bytes.Replace(Seed, []byte("  ratelimit: 20\n"), []byte("  ratelimit: 50\n"), 1)) {
		t.Fatalf("an edit made in AGH's UI while expert must survive the way back: %v", err)
	}
}

func TestToExpertRefusesWhatItCannotRestore(t *testing.T) {
	lan := bytes.Replace(Seed, []byte("  bind_hosts:\n    - 127.0.0.1\n"), []byte("  bind_hosts:\n    - 0.0.0.0\n"), 1)
	if _, _, err := ToExpert(lan, ExpertHosts("192.168.1.1", "")); !errors.Is(err, ErrUnknownShape) {
		t.Fatalf("a LAN-bound config is migrated first, never saved as the default: %v", err)
	}
	noUpstream := bytes.Replace(Seed, []byte("  upstream_dns:\n"), []byte("  upstreams:\n"), 1)
	if _, _, err := ToExpert(noUpstream, ExpertHosts("192.168.1.1", "")); !errors.Is(err, ErrUnknownShape) {
		t.Fatalf("missing upstream_dns: %v", err)
	}
	if _, err := ToDefault(Seed, SavedDNS{"port": {"  port: 5353"}}); !errors.Is(err, ErrUnknownShape) {
		t.Fatalf("an incomplete saved set must not be applied: %v", err)
	}
}
