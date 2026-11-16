package mock

import (
	"context"
	"fmt"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// serviceState is one scripted procd service.
type serviceState struct{ enabled, running bool }

// SetService scripts a procd service; absent services read as not running.
func (d *Device) SetService(name string, enabled, running bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.services == nil {
		d.services = map[string]serviceState{}
	}
	d.services[name] = serviceState{enabled: enabled, running: running}
}

func (d *Device) ServiceRunning(_ context.Context, name string) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.services[name].running, nil
}

// ServiceAction mimics the init script; actions on an unknown service fail like a missing script.
func (d *Device) ServiceAction(_ context.Context, name, action string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !device.ServiceActions[action] {
		return fmt.Errorf("invalid service action %q", action)
	}
	d.ServiceActions = append(d.ServiceActions, name+" "+action)
	if d.ServiceErr != nil {
		return d.ServiceErr
	}
	st, ok := d.services[name]
	if !ok {
		return fmt.Errorf("/etc/init.d/%s: %w", name, device.ErrNotFound)
	}
	switch action {
	case "start", "restart":
		st.running = true
	case "stop":
		st.running = false
	case "enable":
		st.enabled = true
	case "disable":
		st.enabled = false
	}
	d.services[name] = st
	return nil
}

var _ device.Services = (*Device)(nil)
