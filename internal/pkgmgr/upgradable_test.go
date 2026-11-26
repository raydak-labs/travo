package pkgmgr

import (
	"os"
	"strings"
	"testing"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestParseAPKUpgradableCapture(t *testing.T) {
	out := readFixture(t, "apk-list-upgradable.txt")
	ups, err := ParseAPKUpgradable(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(ups) != strings.Count(out, "\n") {
		t.Fatalf("%d upgrades for %d lines", len(ups), strings.Count(out, "\n"))
	}
	want := map[string]Upgrade{
		"dnsmasq":                 {"dnsmasq", "2.93-r1", "2.93-r2"},
		"luci":                    {"luci", "26.288.56989~75a306d", "26.317.52279~1c75ea4"},
		"libblobmsg-json20260213": {"libblobmsg-json20260213", "2026.06.19~7dd12784-r1", "2026.11.02~ff13b89e-r1"},
		"hostapd-common":          {"hostapd-common", "2025.08.26~ca266cc2-r6", "2025.08.26~ca266cc2-r7"},
	}
	for _, u := range ups {
		if w, ok := want[u.Name]; ok && u != w {
			t.Errorf("%+v, want %+v", u, w)
		}
		delete(want, u.Name)
	}
	if len(want) != 0 {
		t.Fatalf("missing %v", want)
	}
}

func TestParseOPKGUpgradableCapture(t *testing.T) {
	ups, err := ParseOPKGUpgradable(readFixture(t, "opkg-list-upgradable.txt"))
	if err != nil || len(ups) == 0 {
		t.Fatalf("%v %v", ups, err)
	}
	if ups[0] != (Upgrade{"luci-mod-system", "26.205.85413~cac97ed", "26.301.70438~aa4865e"}) {
		t.Fatalf("%+v", ups[0])
	}
}

func TestParseUpgradableRejectsMalformed(t *testing.T) {
	if _, err := ParseAPKUpgradable("dnsmasq 2.93 aarch64\n"); err == nil {
		t.Fatal("apk accepted")
	}
	if _, err := ParseAPKUpgradable("a-1 x {o} (l) [upgradable from: b-0]\n"); err == nil {
		t.Fatal("apk accepted name mismatch")
	}
	if _, err := ParseOPKGUpgradable("a - 1\n"); err == nil {
		t.Fatal("opkg accepted")
	}
	if ups, err := ParseAPKUpgradable(""); err != nil || ups == nil || len(ups) != 0 {
		t.Fatal("empty")
	}
}
