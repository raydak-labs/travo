// Package wifi provides wireless summaries, site surveys and guarded uplink joins.
package wifi

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
)

var (
	ErrUnavailable = errors.New("wifi unavailable")
	ErrScanFailed  = errors.New("wifi scan failed")
)

const defaultRadioTimeout = 20 * time.Second

// Service accesses wireless state through narrow device ports.
type Service struct {
	RadioTimeout time.Duration
	daemon       context.Context
	ubus         device.Ubus
	scanner      device.WirelessScanner
	flightMu     sync.Mutex
	flight       *scanFlight
	join         *JoinDeps
}

func New(daemon context.Context, ubus device.Ubus, scanner device.WirelessScanner) *Service {
	return &Service{RadioTimeout: defaultRadioTimeout, daemon: daemon, ubus: ubus, scanner: scanner}
}

type Summary struct {
	Radios []RadioInfo `json:"radios" nullable:"false"`
	STAs   []STA       `json:"stas" nullable:"false"`
	APs    []AP        `json:"aps" nullable:"false"`
}

type RadioInfo struct {
	Name     string `json:"name"`
	Band     string `json:"band" enum:"2g,5g,6g,unknown"`
	Disabled bool   `json:"disabled"`
	Up       bool   `json:"up"`
}

type STA struct {
	Section    string `json:"section"`
	Radio      string `json:"radio"`
	SSID       string `json:"ssid" doc:"Configured SSID"`
	Ifname     string `json:"ifname" doc:"Empty when netifd reports no device"`
	Associated *bool  `json:"associated" nullable:"true" doc:"Device carrier; null = could not read"`
}

type AP struct {
	Section string `json:"section"`
	Radio   string `json:"radio"`
	SSID    string `json:"ssid"`
	Ifname  string `json:"ifname"`
	Up      *bool  `json:"up" nullable:"true" doc:"present && up; null = could not read"`
}

func emptySummary() Summary {
	return Summary{Radios: []RadioInfo{}, STAs: []STA{}, APs: []AP{}}
}

// Summary reports configured radio state and independent per-interface liveness.
func (s *Service) Summary(ctx context.Context) (Summary, error) {
	out := emptySummary()
	reply, err := s.ubus.Call(ctx, "network.wireless", "status", nil)
	if err != nil {
		return Summary{}, ErrUnavailable
	}
	radios, err := facts.ParseWirelessRadios(reply)
	if err != nil {
		return Summary{}, ErrUnavailable
	}
	for _, radio := range radios {
		out.Radios = append(out.Radios, RadioInfo{Name: radio.Name, Band: radio.Band, Disabled: radio.Disabled, Up: radio.Up})
		for _, iface := range radio.Ifaces {
			s.appendInterface(ctx, &out, radio.Name, iface)
		}
	}
	return out, nil
}

func (s *Service) appendInterface(ctx context.Context, out *Summary, radio string, iface facts.WirelessIface) {
	if iface.Mode != "sta" && iface.Mode != "ap" {
		return
	}
	var up *bool
	if iface.Ifname == "" {
		up = new(bool)
	} else if reply, err := s.ubus.Call(ctx, "network.device", "status", map[string]string{"name": iface.Ifname}); err == nil {
		if link, err := facts.ParseDeviceStatus(reply); err == nil {
			value := link.Carrier
			if iface.Mode == "ap" {
				value = link.Present && link.Up
			} else if !link.Present {
				value = false
			}
			up = &value
		}
	}
	if iface.Mode == "sta" {
		out.STAs = append(out.STAs, STA{Section: iface.Section, Radio: radio, SSID: iface.SSID, Ifname: iface.Ifname, Associated: up})
	} else {
		out.APs = append(out.APs, AP{Section: iface.Section, Radio: radio, SSID: iface.SSID, Ifname: iface.Ifname, Up: up})
	}
}
