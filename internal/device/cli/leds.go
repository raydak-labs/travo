package cli

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// SysClassLEDs is where the kernel lists its LEDs.
const SysClassLEDs = "/sys/class/leds"

// LEDs reads /sys/class/leds (or SysLEDs when set). sysfs reads are in-memory and do not
// block, so no timeout tier applies. An empty directory is no LEDs; a missing one an error.
func (d *Device) LEDs(ctx context.Context) ([]device.LED, error) {
	root := d.SysLEDs
	if root == "" {
		root = SysClassLEDs
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	out := make([]device.LED, 0, len(entries))
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		led, err := readLED(filepath.Join(root, e.Name()))
		if errors.Is(err, fs.ErrNotExist) {
			continue // the LED vanished between ReadDir and the read
		}
		if err != nil {
			return nil, err
		}
		led.Name = e.Name()
		out = append(out, led)
	}
	slices.SortFunc(out, func(a, b device.LED) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

func readLED(dir string) (device.LED, error) {
	var led device.LED
	var err error
	if led.Brightness, err = readInt(filepath.Join(dir, "brightness")); err != nil {
		return led, err
	}
	if led.MaxBrightness, err = readInt(filepath.Join(dir, "max_brightness")); err != nil {
		return led, err
	}
	b, err := os.ReadFile(filepath.Join(dir, "trigger"))
	led.Trigger = strings.TrimSpace(string(b))
	return led, err
}

func readInt(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(b)))
}

var _ device.LEDReader = (*Device)(nil)
