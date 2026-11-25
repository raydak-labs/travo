// Package uplinkview is the dashboard's uplink snapshot for the topology diagram and the
// uplink cards. It runs on every dashboard stream tick, so it reads only cheap sources: one
// interface dump, one wireless status, sysfs, the failover state file, `uci export mwan3`, the
// kernel default route, and one `iwinfo info` on the wwan STA while it is up (for its signal).
// No package manager query and no fan-out the data does not need (docs/invariants.md §4).
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

// names is the topology order; wan6 is wan's IPv6 side and is not drawn.
var names = []string{"wan", "wwan", "usbtether"}

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
	Proto      *string `json:"proto" nullable:"true" doc:"netifd protocol (dhcp, static, pppoe, ...); null when not configured"`
	Gateway    *string `json:"gateway" nullable:"true" doc:"IPv4 gateway of the kernel default route when this uplink carries it; null otherwise"`
	Band       *string `json:"band" nullable:"true" doc:"wwan only: band of the STA's radio (2g, 5g, 6g); null when unknown or not wwan"`
	SignalDBm  *int    `json:"signal_dbm" nullable:"true" doc:"wwan only: STA signal in dBm while wwan is up; null otherwise"`
}

// Failover is the failover part the diagram labels.
type Failover struct {
	// Huma marks object fields nullable only from the struct itself.
	_       struct{} `nullable:"true"`
	Enabled bool     `json:"enabled" doc:"The saved failover choice"`
	Active  bool     `json:"active" doc:"The committed mwan3 config carries Travo's rules"`
	Order   []string `json:"order" nullable:"false" doc:"Uplinks in priority order"`
}

// inputs are the reads build joins; a nil optional input leaves its fields null.
type inputs struct {
	Interfaces []facts.Interface
	Radios     []facts.Radio    // nil: wireless status unreadable
	Links      []device.NetLink // nil: sysfs unreadable
	Failover   *Failover
	Route      *device.IPv4DefaultRoute // nil: no default route, or not read
	STAInfo    *facts.IwinfoInfo        // nil: wwan STA not read
}

// build is the pure join of the reads.
func build(in inputs) Uplinks {
	out := Uplinks{Uplinks: make([]Uplink, 0, len(names)), Failover: in.Failover}
	for _, name := range names {
		u := uplink(name, in.Interfaces)
		switch name {
		case "wwan":
			u.SSID, u.Band = sta(in.Radios, u.Device)
			if u.Up && u.Device != nil && in.STAInfo != nil {
				u.SignalDBm = in.STAInfo.Signal
			}
		case "usbtether":
			u.Kind = tetherKind(in.Links, u.Device)
		}
		if in.Route != nil && u.Device != nil && in.Route.Device == *u.Device {
			u.Gateway = nonEmpty(in.Route.Gateway)
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
		out.Proto = nonEmpty(iface.Proto)
		out.Up = iface.Up && len(iface.IPv4) > 0
		out.Device = nonEmpty(iface.L3Device)
		if len(iface.IPv4) > 0 {
			out.IPv4 = nonEmpty(iface.IPv4[0].String())
		}
		return out
	}
	return out
}

// sta: SSID and band of the STA on the wwan device, else of the first STA (wwan is down),
// else null. Both come from the same STA so they always describe one association.
func sta(radios []facts.Radio, l3 *string) (ssid, band *string) {
	found := false
	for _, radio := range radios {
		for _, iface := range radio.Ifaces {
			if iface.Mode != "sta" {
				continue
			}
			if l3 != nil && iface.Ifname == *l3 {
				return nonEmpty(iface.SSID), nonEmpty(radio.Band)
			}
			if !found {
				found = true
				ssid, band = nonEmpty(iface.SSID), nonEmpty(radio.Band)
			}
		}
	}
	return ssid, band
}

func tetherKind(links []device.NetLink, l3 *string) *string {
	if l3 == nil {
		return nil
	}
	for _, link := range links {
		if link.Name != *l3 {
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
	Routes   device.DefaultRouteReader // optional: nil leaves gateway null
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
	in := inputs{}
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
	if s.Routes != nil {
		if route, found, err := s.Routes.DefaultIPv4Route(ctx); err == nil && found {
			in.Route = &route
		}
	}
	in.STAInfo = s.staInfo(ctx, in.Interfaces)
	return build(in), nil
}

// staInfo reads the wwan STA only while wwan is up with a device: the one extra call per tick.
func (s *Service) staInfo(ctx context.Context, ifaces []facts.Interface) *facts.IwinfoInfo {
	wwan := uplink("wwan", ifaces)
	if !wwan.Up || wwan.Device == nil {
		return nil
	}
	raw, err := s.Ubus.Call(ctx, "iwinfo", "info", map[string]string{"device": *wwan.Device})
	if err != nil {
		return nil
	}
	info, err := facts.ParseIwinfoInfo(raw)
	if err != nil {
		return nil
	}
	return &info
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
