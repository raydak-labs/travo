package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// ledSysfsFromCapture rebuilds /sys/class/leds from the verbatim listing: name,
// brightness, max_brightness, trigger, tab-separated.
func ledSysfsFromCapture(t *testing.T, tsv string) string {
	t.Helper()
	root := t.TempDir()
	for line := range strings.SplitSeq(strings.TrimRight(tsv, "\n"), "\n") {
		f := strings.Split(line, "\t")
		if len(f) != 4 {
			t.Fatalf("bad capture line %q", line)
		}
		dir := filepath.Join(root, f[0])
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, v := range map[string]string{"brightness": f[1], "max_brightness": f[2], "trigger": f[3]} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(v+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return root
}

func TestLEDsFromAXT1800Capture(t *testing.T) {
	b, err := os.ReadFile("../../../testdata/device/axt1800-25.12.6/leds/sysfs-default.tsv")
	if err != nil {
		t.Fatal(err)
	}
	d := &Device{SysLEDs: ledSysfsFromCapture(t, string(b))}
	got, err := d.LEDs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].Name != "blue:run" || got[1].Name != "mmc0::" || got[2].Name != "white:system" {
		t.Fatalf("got %+v", got)
	}
	want := device.LED{Name: "blue:run", Brightness: 1, MaxBrightness: 1,
		Trigger: "[none] timer heartbeat default-on netdev mmc0 phy0rx phy0tx phy0assoc phy0radio phy1rx phy1tx phy1assoc phy1radio"}
	if got[0] != want {
		t.Fatalf("got %+v", got[0])
	}
	if got[1].MaxBrightness != 255 {
		t.Fatalf("mmc0 %+v", got[1])
	}
}

func TestLEDsMissingRootIsAnError(t *testing.T) {
	d := &Device{SysLEDs: filepath.Join(t.TempDir(), "absent")}
	if _, err := d.LEDs(context.Background()); err == nil {
		t.Fatal("want error")
	}
}
