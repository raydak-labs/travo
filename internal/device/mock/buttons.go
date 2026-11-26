package mock

import (
	"context"
	_ "embed"
	"fmt"
	"strconv"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

//go:embed testdata/devicetree-keys.tsv
var devicetreeKeys []byte

// ParseButtonsTSV reads the capture format of testdata/device/*/buttons/devicetree-keys.tsv:
// node, label, then linux,code and linux,input-type as hex (empty when absent), tab-separated.
func ParseButtonsTSV(b []byte) ([]device.Button, error) {
	var out []device.Button
	for n, line := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
		f := strings.Split(line, "\t")
		if len(f) != 4 || f[0] == "" {
			return nil, fmt.Errorf("buttons tsv line %d: want 4 fields", n+1)
		}
		code, err := strconv.ParseUint(f[2], 16, 32)
		if err != nil {
			return nil, fmt.Errorf("buttons tsv line %d: code: %w", n+1, err)
		}
		typ := uint64(1) // EV_KEY when the node has no linux,input-type
		if f[3] != "" {
			if typ, err = strconv.ParseUint(f[3], 16, 32); err != nil {
				return nil, fmt.Errorf("buttons tsv line %d: input type: %w", n+1, err)
			}
		}
		out = append(out, device.Button{Node: f[0], Label: f[1], Code: uint32(code), InputType: uint32(typ)})
	}
	return out, nil
}

// Buttons lists the AXT1800's captured devicetree keys: the reset button and the slide switch.
func (d *Device) Buttons(ctx context.Context) ([]device.Button, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return ParseButtonsTSV(devicetreeKeys)
}

var _ device.ButtonReader = (*Device)(nil)
