package main

import (
	"github.com/blackdark-org/travo-greenfield/internal/adguard"
	"github.com/blackdark-org/travo-greenfield/internal/api"
	"github.com/blackdark-org/travo-greenfield/internal/app"
	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/usbtether"
)

// newUSBTether wires the USB tethering uplink; nil when the device cannot list netdevs.
func newUSBTether(deps app.Deps) api.USBTether {
	links, ok := deps.Device.(device.NetLinkReader)
	if !ok {
		return nil
	}
	s := &usbtether.Service{UCI: deps.Device, Ubus: deps.Device, Links: links, Packages: deps.Packages, Guards: deps.Guards}
	if services, ok := deps.Device.(device.Services); ok {
		s.Services = services
	}
	s.FreeBytes = adguard.FreeBytes("/overlay", "/")
	if deps.Mock {
		s.FreeBytes = func() (uint64, error) { return 64 << 20, nil }
	}
	if deps.Txn != nil {
		s.Txns = deps.Txn
	}
	return s
}
