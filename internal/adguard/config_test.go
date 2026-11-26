package adguard

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
)

const fixture = "../../testdata/device/axt1800-25.12.6/adguard/adguardhome.yaml"

func readFixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseListenDeviceFixture(t *testing.T) {
	l, err := ParseListen(readFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if l.HTTP != "0.0.0.0:3000" || len(l.BindHosts) != 1 || l.BindHosts[0] != "0.0.0.0" || l.DNSPort != "5353" || l.IsLoopback() {
		t.Fatalf("listen = %+v", l)
	}
}

func TestLoopbackRewritesOnlyListenLines(t *testing.T) {
	in := readFixture(t)
	out, changed, err := Loopback(in)
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	l, err := ParseListen(out)
	if err != nil || !l.IsLoopback() {
		t.Fatalf("after migration: %+v %v", l, err)
	}
	a, b := strings.Split(string(in), "\n"), strings.Split(string(out), "\n")
	if len(a) != len(b) {
		t.Fatalf("line count %d -> %d", len(a), len(b))
	}
	var diff []string
	for i := range a {
		if a[i] != b[i] {
			diff = append(diff, b[i])
		}
	}
	if strings.Join(diff, "|") != "  address: 127.0.0.1:3000|    - 127.0.0.1" {
		t.Fatalf("changed lines: %q", diff)
	}
	again, changed, err := Loopback(out)
	if err != nil || changed || !bytes.Equal(again, out) {
		t.Fatalf("second run must be a no-op: changed=%v err=%v", changed, err)
	}
}

func TestSeedIsTheMigratedDeviceConfig(t *testing.T) {
	out, _, err := Loopback(readFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	out, _ = Limits(out)
	if !bytes.Equal(out, Seed) {
		t.Fatal("seed.yaml drifted from Limits(Loopback(device fixture))")
	}
}

func TestLoopbackCollapsesSeveralBindHostsAndFixesPort(t *testing.T) {
	in := strings.Replace(string(readFixture(t)), "    - 0.0.0.0\n", "    - 192.168.1.1\n    - ::1\n", 1)
	in = strings.Replace(in, "  port: 5353\n", "  port: 53\n", 1)
	out, changed, err := Loopback([]byte(in))
	if err != nil || !changed {
		t.Fatal(changed, err)
	}
	l, _ := ParseListen(out)
	if !l.IsLoopback() || strings.Contains(string(out), "::1\n  port") {
		t.Fatalf("%+v", l)
	}
}

func TestLoopbackRefusesUnknownShapes(t *testing.T) {
	base := string(readFixture(t))
	for name, in := range map[string]string{
		"flow list":      strings.Replace(base, "  bind_hosts:\n    - 0.0.0.0\n", "  bind_hosts: [0.0.0.0]\n", 1),
		"quoted address": strings.Replace(base, "  address: 0.0.0.0:3000", `  address: "0.0.0.0:3000"`, 1),
		"no http":        strings.Replace(base, "http:\n", "web:\n", 1),
		"crlf":           strings.ReplaceAll(base, "\n", "\r\n"),
		"tabs":           strings.Replace(base, "  address:", "\taddress:", 1),
		"no dns port":    strings.Replace(base, "  port: 5353\n", "", 1),
		"duplicate dns":  base + "dns:\n  port: 53\n",
		"empty":          "",
	} {
		if _, _, err := Loopback([]byte(in)); !errors.Is(err, ErrUnknownShape) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}
