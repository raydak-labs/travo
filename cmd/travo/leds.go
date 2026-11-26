package main

import (
	"github.com/raydak-labs/travo/internal/app"
	"github.com/raydak-labs/travo/internal/crontab"
	"github.com/raydak-labs/travo/internal/device"
	"github.com/raydak-labs/travo/internal/hook"
	"github.com/raydak-labs/travo/internal/leds"
)

// startLEDs builds the LED service and registers the night schedule's hook actions. nil
// when the device cannot list LEDs or there is no crontab writer; the API then answers 503.
func startLEDs(deps app.Deps, cron *crontab.Service, hooks *hook.Server) *leds.Service {
	sysfs, ok := deps.Device.(device.LEDReader)
	services, hasServices := deps.Device.(device.Services)
	if !ok || !hasServices || cron == nil {
		return nil
	}
	svc := &leds.Service{UCI: deps.Device, Sysfs: sysfs, Apply: deps.Txn, Services: services, Cron: cron}
	if hooks != nil {
		for name, a := range svc.Actions() {
			hooks.Register(name, a)
		}
	}
	return svc
}
