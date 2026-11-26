package pkgmgr

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestParseAPKFixture(t *testing.T) {
	b, err := os.ReadFile("testdata/apk-query-installed.json")
	if err != nil {
		t.Fatal(err)
	}
	pkgs, err := ParseAPKQuery(string(b))
	if err != nil || len(pkgs) < 50 {
		t.Fatalf("n=%d err=%v", len(pkgs), err)
	}
	v, err := find(pkgs, "dnsmasq")
	if err != nil || v == "" {
		t.Fatalf("dnsmasq: %q %v", v, err)
	}
	if _, err := find(pkgs, "nosuch"); !errors.Is(err, ErrNotInstalled) {
		t.Fatal(err)
	}
}

func TestParseOPKGFixture(t *testing.T) {
	b, err := os.ReadFile("testdata/opkg-list-installed.txt")
	if err != nil {
		t.Fatal(err)
	}
	pkgs, err := ParseOPKGList(string(b))
	if err != nil || len(pkgs) < 50 {
		t.Fatalf("n=%d err=%v", len(pkgs), err)
	}
	if v, err := find(pkgs, "dnsmasq"); err != nil || v != "2.93-r1" {
		t.Fatalf("dnsmasq: %q %v", v, err)
	}
}

func TestParsersFailClosedOnGarbage(t *testing.T) {
	if _, err := ParseAPKQuery("not json"); err == nil {
		t.Error("apk accepted garbage")
	}
	if _, err := ParseAPKQuery(`[{"version":"1"}]`); err == nil {
		t.Error("apk accepted nameless entry")
	}
	if _, err := ParseOPKGList("busybox 1.0"); err == nil {
		t.Error("opkg accepted malformed line")
	}
}

func TestFailedQueryIsNotAbsent(t *testing.T) {
	r := func(context.Context, string, ...string) (string, int, error) { return "", 1, errors.New("boom") }
	_, err := (&APK{Run: r}).Version(context.Background(), "dnsmasq")
	if err == nil || errors.Is(err, ErrNotInstalled) {
		t.Fatalf("query failure read as absent: %v", err)
	}
}

func TestInvalidNamesNeverReachTheShell(t *testing.T) {
	called := false
	r := func(context.Context, string, ...string) (string, int, error) { called = true; return "", 0, nil }
	for _, bad := range []string{"", "-f", "a b", "a;rm", "$(x)", "a/b"} {
		if err := (&APK{Run: r}).Install(context.Background(), bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
		if err := (&OPKG{Run: r}).Remove(context.Background(), bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
	if called {
		t.Fatal("runner called for invalid name")
	}
}

func TestDetectPrefersAPKThenOPKGThenFails(t *testing.T) {
	has := func(names ...string) func(string) (string, error) {
		return func(n string) (string, error) {
			for _, x := range names {
				if x == n {
					return "/bin/" + n, nil
				}
			}
			return "", errors.New("missing")
		}
	}
	cases := []struct {
		have []string
		want string
	}{{[]string{"apk", "opkg"}, "apk"}, {[]string{"opkg"}, "opkg"}}
	for _, c := range cases {
		pm, err := Detect(has(c.have...), nil)
		if err != nil || pm.Name() != c.want {
			t.Errorf("%v: %v %v", c.have, pm, err)
		}
	}
	if _, err := Detect(has(), nil); !errors.Is(err, ErrNoManager) {
		t.Fatalf("got %v", err)
	}
}

func TestInstallUsesInstallTier(t *testing.T) {
	var args string
	r := func(ctx context.Context, n string, a ...string) (string, int, error) {
		if _, ok := ctx.Deadline(); !ok {
			t.Error("no deadline")
		}
		args = n + " " + strings.Join(a, " ")
		return "", 0, nil
	}
	_ = (&APK{Run: r}).Install(context.Background(), "adguardhome")
	if args != "apk add adguardhome" {
		t.Fatal(args)
	}
	_ = (&OPKG{Run: r}).Install(context.Background(), "adguardhome")
	if args != "opkg install adguardhome" {
		t.Fatal(args)
	}
}
