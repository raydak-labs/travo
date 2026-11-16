package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

var serviceName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// ServiceRunning reads `ubus call service list {"name":...}`; procd answers {} for an
// unknown service (testdata/device/axt1800-25.12.6/adguard/service-list-absent.json).
func (d *Device) ServiceRunning(ctx context.Context, name string) (bool, error) {
	if !serviceName.MatchString(name) {
		return false, fmt.Errorf("invalid service name %q", name)
	}
	raw, err := d.Call(ctx, "service", "list", map[string]string{"name": name})
	if err != nil {
		return false, err
	}
	return ParseServiceRunning(raw, name)
}

// ParseServiceRunning is true when any procd instance of name is running.
func ParseServiceRunning(raw json.RawMessage, name string) (bool, error) {
	var services map[string]struct {
		Instances map[string]struct {
			Running bool `json:"running"`
		} `json:"instances"`
	}
	if err := json.Unmarshal(raw, &services); err != nil {
		return false, fmt.Errorf("service list: %w", err)
	}
	for _, in := range services[name].Instances {
		if in.Running {
			return true, nil
		}
	}
	return false, nil
}

// ServiceAction runs /etc/init.d/<name> <action>. A restart waits for procd, so it gets
// the apply tier.
func (d *Device) ServiceAction(ctx context.Context, name, action string) error {
	if !serviceName.MatchString(name) || name == "network" {
		return fmt.Errorf("invalid service name %q", name)
	}
	if !device.ServiceActions[action] {
		return fmt.Errorf("invalid service action %q", action)
	}
	_, err := d.run(ctx, TierApply, "/etc/init.d/"+name, action)
	return err
}

var _ device.Services = (*Device)(nil)
