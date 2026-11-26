package cli

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// DevicetreeKeys is the devicetree `gpio-keys` node (device-facts §8).
const DevicetreeKeys = "/sys/firmware/devicetree/base/keys"

// evKey is the input type a gpio-keys node has without `linux,input-type`.
const evKey = 1

// Buttons reads the devicetree keys node (or SysKeys when set). A board without the node
// has no buttons; devicetree reads are in-memory, so no timeout tier applies.
func (d *Device) Buttons(ctx context.Context) ([]device.Button, error) {
	root := d.SysKeys
	if root == "" {
		root = DevicetreeKeys
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return []device.Button{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []device.Button{}
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !e.IsDir() {
			continue // compatible, name, ...: properties of the keys node itself
		}
		b, ok, err := readButton(filepath.Join(root, e.Name()))
		if err != nil {
			return nil, err
		}
		if ok {
			b.Node = e.Name()
			out = append(out, b)
		}
	}
	slices.SortFunc(out, func(a, b device.Button) int { return strings.Compare(a.Node, b.Node) })
	return out, nil
}

// readButton reads one key node; ok is false for a child without `linux,code`.
func readButton(dir string) (device.Button, bool, error) {
	code, err := readCell(filepath.Join(dir, "linux,code"))
	if errors.Is(err, fs.ErrNotExist) {
		return device.Button{}, false, nil
	}
	if err != nil {
		return device.Button{}, false, err
	}
	typ, err := readCell(filepath.Join(dir, "linux,input-type"))
	if errors.Is(err, fs.ErrNotExist) {
		typ, err = evKey, nil
	}
	if err != nil {
		return device.Button{}, false, err
	}
	label, err := os.ReadFile(filepath.Join(dir, "label"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return device.Button{}, false, err
	}
	return device.Button{Label: strings.TrimRight(string(label), "\x00\n"), Code: code, InputType: typ}, true, nil
}

// readCell reads one big-endian u32 devicetree cell.
func readCell(path string) (uint32, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if len(b) != 4 {
		return 0, fmt.Errorf("%s: want one 32-bit cell, got %d bytes", path, len(b))
	}
	return binary.BigEndian.Uint32(b), nil
}

var _ device.ButtonReader = (*Device)(nil)
