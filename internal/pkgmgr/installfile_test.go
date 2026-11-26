package pkgmgr

import (
	"context"
	"strings"
	"testing"
)

func TestInstallFileSkipsOnlyApkSignatureCheckAndRefusesRelativePaths(t *testing.T) {
	var args string
	r := func(_ context.Context, n string, a ...string) (string, int, error) {
		args = n + " " + strings.Join(a, " ")
		return "", 0, nil
	}
	const file = "/tmp/travo-greenfield-bundle/adguardhome.apk"
	if err := (&APK{Run: r}).InstallFile(context.Background(), file); err != nil || args != "apk add --allow-untrusted "+file {
		t.Fatalf("%q %v", args, err)
	}
	if err := (&OPKG{Run: r}).InstallFile(context.Background(), file); err != nil || args != "opkg install "+file {
		t.Fatalf("%q %v", args, err)
	}
	args = ""
	for _, bad := range []string{"", "-f", "adguardhome.apk", "/tmp/a\nb"} {
		if err := (&APK{Run: r}).InstallFile(context.Background(), bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
	if args != "" {
		t.Fatalf("runner called for an invalid file: %q", args)
	}
}

func TestMockInstallFileNamesThePackage(t *testing.T) {
	m := NewMock()
	if err := m.InstallFile(context.Background(), "/tmp/b/adguardhome-0.107.80-r1.apk"); err != nil {
		t.Fatal(err)
	}
	if v, err := m.Version(context.Background(), "adguardhome"); err != nil || v == "" {
		t.Fatalf("%q %v", v, err)
	}
}
