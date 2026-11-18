package main

import (
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
	return &usbtether.Service{UCI: deps.Device, Ubus: deps.Device, Links: links, Packages: deps.Packages}
}
