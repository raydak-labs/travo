// Package uplinkview is the dashboard's uplink snapshot for the topology diagram. It runs
// on every dashboard stream tick, so it reads only cheap sources: one interface dump, one
// wireless status, sysfs, the failover state file and `uci export mwan3`. No package
// manager query and no per-interface ubus fan-out (docs/invariants.md §4).
package uplinkview

import (
	"context"
	"errors"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/failover"
)

// ErrUnavailable means the interface dump could not be read.
var ErrUnavailable = errors.New("uplink data is unavailable")

// Names is the topology order; wan6 is wan's IPv6 side and is not drawn.
var Names = []string{"wan", "wwan", "usbtether"}

// Uplinks is GET /api/v1/dashboard/uplinks.
type Uplinks struct {
	Uplinks  []Uplink  `json:"uplinks" nullable:"false"`
	Failover *Failover `json:"failover" doc:"null when the failover settings or mwan3's config cannot be read (mwan3 not installed)"`
}

// Uplink is one uplink as the topology diagram draws it.
type Uplink struct {
	Name       string  `json:"name" enum:"wan,wwan,usbtether"`
	Configured bool    `json:"configured" doc:"The netifd interface exists"`
	Up         bool    `json:"up" doc:"netifd reports it up and it has an IPv4 address"`
	Device     *string `json:"device" nullable:"true" doc:"Kernel device carrying layer 3; null when absent"`
	IPv4       *string `json:"ipv4" nullable:"true" doc:"First IPv4 address in CIDR form; null when none"`
	SSID       *string `json:"ssid" nullable:"true" doc:"wwan only: SSID of the uplink STA; null when unknown or not wwan"`
	Kind       *string `json:"kind" nullable:"true" doc:"usbtether only: tether protocol of the bound link (rndis, ncm, ecm, ipheth); null when unknown or not usbtether"`
}

// Failover is the failover part the diagram labels.
type Failover struct {
	// Huma marks object fields nullable only from the struct itself.
	_       struct{} `nullable:"true"`
	Enabled bool     `json:"enabled" doc:"The saved failover choice"`
	Active  bool     `json:"active" doc:"The committed mwan3 config carries Travo's rules"`
	Order   []string `json:"order" nullable:"false" doc:"Uplinks in priority order"`
}

// Inputs are the reads Build joins; a nil optional input leaves its fields null.
type Inputs struct {
	Interfaces []facts.Interface
	Radios     []facts.Radio    // nil: wireless status unreadable
	Links      []device.NetLink // nil: sysfs unreadable
	Failover   *Failover
}

// Build is the pure join of the reads.
func Build(in Inputs) Uplinks {
	out := Uplinks{Uplinks: make([]Uplink, 0, len(Names)), Failover: in.Failover}
	for _, name := range Names {
		u := uplink(name, in.Interfaces)
		switch name {
		case "wwan":
			u.SSID = staSSID(in.Radios, u.Device)
		case "usbtether":
			u.Kind = tetherKind(in.Links, u.Device)
		}
		out.Uplinks = append(out.Uplinks, u)
	}
	return out
}

func uplink(name string, ifaces []facts.Interface) Uplink {
	out := Uplink{Name: name}
	for _, iface := range ifaces {
		if iface.Name != name {
			continue
		}
		out.Configured = true
		out.Up = iface.Up && len(iface.IPv4) > 0
		out.Device = nonEmpty(iface.L3Device)
		if len(iface.IPv4) > 0 {
			out.IPv4 = nonEmpty(iface.IPv4[0].String())
		}
		return out
	}
	return out
}

// staSSID: the STA on the wwan device, else the first STA (wwan is down), else null.
func staSSID(radios []facts.Radio, device *string) *string {
	var first *string
	for _, radio := range radios {
		for _, iface := range radio.Ifaces {
			if iface.Mode != "sta" {
				continue
			}
			if device != nil && iface.Ifname == *device {
				return nonEmpty(iface.SSID)
			}
			if first == nil {
				first = nonEmpty(iface.SSID)
			}
		}
	}
	return first
}

func tetherKind(links []device.NetLink, device *string) *string {
	if device == nil {
		return nil
	}
	for _, link := range links {
		if link.Name != *device {
			continue
		}
		if kind, ok := facts.Tether(link); ok {
			return nonEmpty(string(kind))
		}
		return nil
	}
	return nil
}

func nonEmpty(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

// FailoverSettings reads failover.json (failover.Service).
type FailoverSettings interface {
	LoadSettings() (failover.Settings, error)
}

// Service reads the inputs. Links and Failover are optional.
type Service struct {
	Ubus     device.Ubus
	UCI      device.UCI
	Links    device.NetLinkReader
	Failover FailoverSettings
}

// Uplinks never mutates; only the interface dump is required.
func (s *Service) Uplinks(ctx context.Context) (Uplinks, error) {
	if s.Ubus == nil {
		return Uplinks{}, ErrUnavailable
	}
	raw, err := s.Ubus.Call(ctx, "network.interface", "dump", nil)
	if err != nil {
		return Uplinks{}, errors.Join(ErrUnavailable, err)
	}
	in := Inputs{}
	if in.Interfaces, err = facts.ParseInterfaceDump(raw); err != nil {
		return Uplinks{}, errors.Join(ErrUnavailable, err)
	}
	if raw, err := s.Ubus.Call(ctx, "network.wireless", "status", nil); err == nil {
		in.Radios, _ = facts.ParseWirelessRadios(raw)
	}
	if s.Links != nil {
		in.Links, _ = s.Links.NetLinks(ctx)
	}
	in.Failover = s.failover(ctx)
	return Build(in), nil
}

func (s *Service) failover(ctx context.Context) *Failover {
	if s.Failover == nil || s.UCI == nil {
		return nil
	}
	set, err := s.Failover.LoadSettings()
	if err != nil {
		return nil
	}
	current, err := s.UCI.Show(ctx, failover.Config)
	if err != nil {
		return nil
	}
	order := set.Uplinks
	if order == nil {
		order = []string{}
	}
	return &Failover{Enabled: set.Enabled, Active: failover.Active(current), Order: order}
}
