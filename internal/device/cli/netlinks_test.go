package cli

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// sysfsFromCapture rebuilds /sys/class/net from the verbatim readlink capture: the device
// directory is a plain dir holding the captured driver and subsystem symlink targets.
func sysfsFromCapture(t *testing.T, tsv string) string {
	t.Helper()
	root := t.TempDir()
	for line := range strings.SplitSeq(strings.TrimRight(tsv, "\n"), "\n") {
		f := strings.Split(line, "\t")
		if len(f) != 5 {
			t.Fatalf("bad capture line %q", line)
		}
		dir := filepath.Join(root, f[0])
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		uevent := "INTERFACE=" + f[0] + "\n"
		if f[4] != "" {
			uevent = "DEVTYPE=" + f[4] + "\n" + uevent
		}
		if err := os.WriteFile(filepath.Join(dir, "uevent"), []byte(uevent), 0o644); err != nil {
			t.Fatal(err)
		}
		if f[1] == "" {
			continue
		}
		if err := os.Mkdir(filepath.Join(dir, "device"), 0o755); err != nil {
			t.Fatal(err)
		}
		for name, target := range map[string]string{"driver": f[2], "subsystem": f[3]} {
			if target != "" {
				if err := os.Symlink(target, filepath.Join(dir, "device", name)); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	return root
}

func TestNetLinksFromAXT1800Capture(t *testing.T) {
	b, err := os.ReadFile("../../../testdata/device/axt1800-25.12.6/sysfs-net/links.tsv")
	if err != nil {
		t.Fatal(err)
	}
	d := &Device{SysNet: sysfsFromCapture(t, string(b))}
	links, err := d.NetLinks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]device.NetLink{
		"br-lan":    {Name: "br-lan", DevType: "bridge"},
		"lo":        {Name: "lo"},
		"wan":       {Name: "wan", Bus: "platform", Driver: "nss-dp"},
		"lan1":      {Name: "lan1", Bus: "platform", Driver: "nss-dp"},
		"phy0-sta0": {Name: "phy0-sta0", Bus: "platform", Driver: "ath11k", DevType: "wlan"},
	}
	if len(links) != 10 {
		t.Fatalf("got %d links, want 10: %+v", len(links), links)
	}
	if !slices.IsSortedFunc(links, func(a, b device.NetLink) int { return strings.Compare(a.Name, b.Name) }) {
		t.Fatal("links not sorted by name")
	}
	for _, l := range links {
		if w, ok := want[l.Name]; ok && l != w {
			t.Errorf("%s = %+v, want %+v", l.Name, l, w)
		}
	}
}

func TestNetLinksMissingRootIsAnError(t *testing.T) {
	d := &Device{SysNet: filepath.Join(t.TempDir(), "absent")}
	if _, err := d.NetLinks(context.Background()); err == nil {
		t.Fatal("want error for unreadable /sys/class/net")
	}
}
