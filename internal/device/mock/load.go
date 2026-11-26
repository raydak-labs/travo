package mock

import (
	"context"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// Load seeds a committed config from parsed sections (cli.ParseExport on a captured fixture).
func (d *Device) Load(ctx context.Context, cfg string, secs []device.Section) error {
	for _, s := range secs {
		if err := d.SetType(ctx, cfg, s.Name, s.Type); err != nil {
			return err
		}
		for k, v := range s.Options {
			if err := d.Set(ctx, cfg, s.Name, k, v); err != nil {
				return err
			}
		}
		for k, vs := range s.Lists {
			for _, v := range vs {
				if err := d.AddToList(ctx, cfg, s.Name, k, v); err != nil {
					return err
				}
			}
		}
	}
	return d.Commit(ctx, cfg)
}
