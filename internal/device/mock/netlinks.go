package mock

import (
	"context"
	_ "embed"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

//go:embed testdata/sysfs-net-links.tsv
var sysfsNetLinks []byte

// ParseLinksTSV reads the sysfs capture format of testdata/device/*/sysfs-net/links.tsv:
// name, device link, driver link, subsystem link, DEVTYPE, tab-separated.
func ParseLinksTSV(b []byte) ([]device.NetLink, error) {
	var out []device.NetLink
	for n, line := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
		f := strings.Split(line, "\t")
		if len(f) != 5 || f[0] == "" {
			return nil, fmt.Errorf("links.tsv line %d: want 5 fields", n+1)
		}
		out = append(out, device.NetLink{Name: f[0], Driver: linkBase(f[2]), Bus: linkBase(f[3]), DevType: f[4]})
	}
	return out, nil
}

func linkBase(link string) string {
	if link == "" {
		return ""
	}
	return path.Base(link)
}

// SeedNetLinks lists the AXT1800's captured netdevs.
func SeedNetLinks(d *Device) error {
	links, err := ParseLinksTSV(sysfsNetLinks)
	if err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.netLinks = links
	return nil
}

// AddNetLink adds or replaces one netdev (a tether link plugged in).
func (d *Device) AddNetLink(l device.NetLink) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.netLinks = slices.DeleteFunc(d.netLinks, func(x device.NetLink) bool { return x.Name == l.Name })
	d.netLinks = append(d.netLinks, l)
}

// RemoveNetLink drops one netdev (the phone was unplugged).
func (d *Device) RemoveNetLink(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.netLinks = slices.DeleteFunc(d.netLinks, func(x device.NetLink) bool { return x.Name == name })
}

func (d *Device) NetLinks(ctx context.Context) ([]device.NetLink, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.netLinks), nil
}

var _ device.NetLinkReader = (*Device)(nil)
