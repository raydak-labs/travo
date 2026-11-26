package cli

import (
	"bufio"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// SysClassNet is where the kernel lists its netdevs.
const SysClassNet = "/sys/class/net"

// NetLinks reads /sys/class/net (or SysNet when set). sysfs reads are in-memory and
// do not block, so no timeout tier applies; ctx is checked between links.
func (d *Device) NetLinks(ctx context.Context) ([]device.NetLink, error) {
	root := d.SysNet
	if root == "" {
		root = SysClassNet
	}
	return readNetLinks(ctx, root)
}

func readNetLinks(ctx context.Context, root string) ([]device.NetLink, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	links := make([]device.NetLink, 0, len(entries))
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		link, err := readNetLink(filepath.Join(root, e.Name()))
		if errors.Is(err, fs.ErrNotExist) {
			continue // the netdev vanished between ReadDir and the read
		}
		if err != nil {
			return nil, err
		}
		link.Name = e.Name()
		links = append(links, link)
	}
	slices.SortFunc(links, func(a, b device.NetLink) int { return strings.Compare(a.Name, b.Name) })
	return links, nil
}

func readNetLink(dir string) (device.NetLink, error) {
	var link device.NetLink
	devType, err := ueventDevType(filepath.Join(dir, "uevent"))
	if err != nil {
		return link, err
	}
	link.DevType = devType
	if link.Bus, err = linkBase(filepath.Join(dir, "device", "subsystem")); err != nil {
		return link, err
	}
	if link.Driver, err = linkBase(filepath.Join(dir, "device", "driver")); err != nil {
		return link, err
	}
	return link, nil
}

// linkBase is the basename of a symlink's target; empty when the link does not exist
// (a netdev without a backing device has no `device` directory).
func linkBase(path string) (string, error) {
	target, err := os.Readlink(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return filepath.Base(target), nil
}

func ueventDevType(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "DEVTYPE="); ok {
			return v, nil
		}
	}
	return "", sc.Err()
}

var _ device.NetLinkReader = (*Device)(nil)
