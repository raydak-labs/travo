package connectivity

import (
	"context"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
)

func TestConnectivityMediumOfActiveInterface(t *testing.T) {
	dev := mock.New()
	if err := mock.SeedNetLinks(dev); err != nil {
		t.Fatal(err)
	}
	route := &routeSource{route: device.IPv4DefaultRoute{Device: "phy0-sta0"}}
	service := New(t.Context(), route, dev, func(context.Context) Result { return Result{Internet: Online} }, time.Now)
	if got := service.Status(t.Context()); got.Medium != nil {
		t.Fatalf("no link reader: medium = %v, want null", *got.Medium)
	}
	service.Links = dev
	if got := service.Status(t.Context()); got.Medium == nil || *got.Medium != "wifi" {
		t.Fatalf("phy0-sta0 medium = %v, want wifi", got.Medium)
	}
	// No phone capture yet: the link carries the kernel driver name of an Android RNDIS phone.
	dev.AddNetLink(device.NetLink{Name: "usb0", Bus: "usb", Driver: "rndis_host"})
	route.route.Device = "usb0"
	if got := service.Status(t.Context()); got.Medium == nil || *got.Medium != "usb" {
		t.Fatalf("usb0 medium = %v, want usb", got.Medium)
	}
	route.route.Device = "gone0"
	if got := service.Status(t.Context()); got.Medium != nil {
		t.Fatalf("unknown link medium = %v, want null", *got.Medium)
	}
}
