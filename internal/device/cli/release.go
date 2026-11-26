package cli

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

func (d *Device) VerifyRelease(ctx context.Context, manifest, signature, key string) error {
	_, err := d.run(ctx, TierRead, "usign", "-V", "-q", "-m", manifest, "-x", signature, "-p", key)
	return err
}

// UpdateSettings uses the same defaults as the packaged procd init script.
func (d *Device) UpdateSettings(ctx context.Context) (listen, staticDir string, err error) {
	sections, err := d.Show(ctx, "travo-greenfield")
	if err != nil {
		return "", "", err
	}
	for _, section := range sections {
		if section.Name != "main" {
			continue
		}
		if section.Options["enabled"] == "0" {
			break
		}
		listen = section.Options["listen"]
		staticDir = section.Options["static_dir"]
		if listen == "" {
			listen = "0.0.0.0:8090"
		}
		if staticDir == "" {
			staticDir = "/www/travo-greenfield"
		}
		return listen, staticDir, nil
	}
	return "", "", errors.New("enabled Travo service not configured")
}

func (d *Device) StopUpdateService(ctx context.Context) error {
	_, err := d.run(ctx, 15*time.Second, "/etc/init.d/travo-greenfield", "stop")
	if err == nil {
		return nil
	}
	// Recovery can start after an interrupted stop, when procd has no instance.
	pid, readErr := d.UpdatePID(ctx)
	if readErr == nil && pid == 0 {
		return nil
	}
	return err
}

func (d *Device) StartUpdateService(ctx context.Context) error {
	_, err := d.run(ctx, 15*time.Second, "/etc/init.d/travo-greenfield", "restart")
	return err
}

func (d *Device) UpdatePID(ctx context.Context) (int, error) {
	raw, err := d.Call(ctx, "service", "list", map[string]string{"name": "travo-greenfield"})
	if err != nil {
		return 0, err
	}
	var services map[string]struct {
		Instances map[string]struct {
			Running bool
			PID     int
		}
	}
	if err := json.Unmarshal(raw, &services); err != nil {
		return 0, err
	}
	instances := services["travo-greenfield"].Instances
	if len(instances) > 1 {
		return 0, errors.New("ambiguous Travo procd instance")
	}
	for _, instance := range instances {
		if instance.Running && instance.PID > 0 {
			return instance.PID, nil
		}
	}
	return 0, nil
}
