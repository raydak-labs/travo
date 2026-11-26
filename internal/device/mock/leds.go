package mock

import (
	"context"
	_ "embed"
	"fmt"
	"strconv"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

//go:embed testdata/sysfs-leds.tsv
var sysfsLEDs []byte

// ParseLEDsTSV reads the capture format of testdata/device/*/leds/sysfs-*.tsv: name,
// brightness, max_brightness, trigger, tab-separated.
func ParseLEDsTSV(b []byte) ([]device.LED, error) {
	var out []device.LED
	for n, line := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
		f := strings.Split(line, "\t")
		if len(f) != 4 || f[0] == "" {
			return nil, fmt.Errorf("leds tsv line %d: want 4 fields", n+1)
		}
		br, err1 := strconv.Atoi(f[1])
		maxBr, err2 := strconv.Atoi(f[2])
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("leds tsv line %d: brightness", n+1)
		}
		out = append(out, device.LED{Name: f[0], Brightness: br, MaxBrightness: maxBr, Trigger: f[3]})
	}
	return out, nil
}

// LEDs lists the AXT1800's captured LEDs. A committed `system` led section with
// `default 0` and trigger `none` shows its LED dark, as `/etc/init.d/led restart` would
// leave it.
func (d *Device) LEDs(ctx context.Context) ([]device.LED, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	leds, err := ParseLEDsTSV(sysfsLEDs)
	if err != nil {
		return nil, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	dark := map[string]bool{}
	if c := d.committed["system"]; c != nil {
		for _, s := range c.sections {
			if s.Type == "led" && s.Options["default"] == "0" && s.Options["trigger"] == "none" {
				dark[s.Options["sysfs"]] = true
			}
		}
	}
	for i, l := range leds {
		if dark[l.Name] {
			leds[i].Brightness = 0
			leds[i].Trigger = markTrigger(l.Trigger, "none")
		}
	}
	return leds, nil
}

// markTrigger moves the brackets of a sysfs trigger list to active.
func markTrigger(raw, active string) string {
	f := strings.Fields(strings.NewReplacer("[", "", "]", "").Replace(raw))
	for i, t := range f {
		if t == active {
			f[i] = "[" + t + "]"
		}
	}
	return strings.Join(f, " ")
}

var _ device.LEDReader = (*Device)(nil)
