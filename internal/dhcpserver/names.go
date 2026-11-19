package dhcpserver

import (
	"context"
	"errors"
	"net/netip"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// NamePrefix names the dhcp domain sections Travo creates: travo_dns_<n>.
const NamePrefix = "travo_dns_"

// MaxNames bounds Travo's own entries; each is one line of dnsmasq's hosts file.
const MaxNames = 64

var (
	ErrNameExists   = errors.New("the name already has a local DNS entry")
	ErrNameNotFound = errors.New("no local DNS name with that id")
	// ErrNameForeign: the section was not created by Travo; LuCI owns it.
	ErrNameForeign = errors.New("the entry is not managed by Travo")
	ErrNamesFull   = errors.New("too many local DNS names")
)

// DNSName is one dhcp `domain` section: dnsmasq's init script writes `<ip> <name...>` into
// its hosts file (testdata dhcp/init-dnsmasq-domain-add.txt).
type DNSName struct {
	ID    string   `json:"id" doc:"UCI section"`
	Names []string `json:"names" nullable:"false" doc:"Host names answered with ip"`
	IP    string   `json:"ip"`
	Owned bool     `json:"owned" doc:"Created by Travo (travo_dns_<n>); others are read-only"`
}

// domainNames reads a domain section's names: an option holding space-separated names (the
// init script splits it) or a list.
func domainNames(s device.Section) []string {
	out := slices.Clone(s.Lists["name"])
	return append(out, strings.Fields(s.Options["name"])...)
}

// Names lists every domain section with a name and an ip, in section order: the init script
// skips the others.
func Names(dhcp []device.Section) []DNSName {
	out := []DNSName{}
	for _, s := range dhcp {
		names := domainNames(s)
		if s.Type != "domain" || len(names) == 0 || s.Options["ip"] == "" {
			continue
		}
		out = append(out, DNSName{ID: s.Name, Names: names, IP: s.Options["ip"], Owned: owned(s)})
	}
	return out
}

func owned(s device.Section) bool {
	n, ok := strings.CutPrefix(s.Name, NamePrefix)
	if !ok || s.Type != "domain" {
		return false
	}
	_, err := strconv.Atoi(n)
	return err == nil
}

var hostLabel = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// ValidateName checks one new entry: a host name (dot-separated labels, at most 253
// characters) and an IPv4 or IPv6 literal without zone. It returns the normalised entry.
func ValidateName(name, ip string) (string, netip.Addr, error) {
	n := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(name)), ".")
	labels := strings.Split(n, ".")
	if n == "" || len(n) > 253 || slices.ContainsFunc(labels, func(l string) bool { return !hostLabel.MatchString(l) }) {
		return "", netip.Addr{}, &FieldError{"name", "must be a host name (letters, digits, hyphen, dots)"}
	}
	if last := labels[len(labels)-1]; strings.Trim(last, "0123456789") == "" {
		return "", netip.Addr{}, &FieldError{"name", "must not end in a numeric label"}
	}
	addr, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil || addr.Zone() != "" {
		return "", netip.Addr{}, &FieldError{"ip", "must be a unicast IPv4 or IPv6 address"}
	}
	if addr = addr.Unmap(); addr.IsUnspecified() || addr.IsMulticast() {
		return "", netip.Addr{}, &FieldError{"ip", "must be a unicast IPv4 or IPv6 address"}
	}
	return n, addr, nil
}

// StageName adds name -> ip as the next free travo_dns_<n> and reports whether it staged.
// Travo's own entry with exactly that name and ip is an unchanged retry (its id, false), so a
// retry finishes a commit whose reload failed. A name any other domain section answers is
// refused: dnsmasq would answer both addresses.
func StageName(ctx context.Context, uci device.UCI, dhcp []device.Section, name string, ip netip.Addr) (string, bool, error) {
	all := Names(dhcp)
	at := slices.IndexFunc(all, func(d DNSName) bool {
		return slices.ContainsFunc(d.Names, func(n string) bool { return strings.EqualFold(n, name) })
	})
	if at >= 0 {
		if d := all[at]; d.Owned && len(d.Names) == 1 && d.IP == ip.String() {
			return d.ID, false, nil
		}
		return "", false, ErrNameExists
	}
	if countOwned(dhcp) >= MaxNames {
		return "", false, ErrNamesFull
	}
	id := freeSection(dhcp)
	if err := uci.SetType(ctx, "dhcp", id, "domain"); err != nil {
		return "", false, err
	}
	if err := uci.Set(ctx, "dhcp", id, "name", name); err != nil {
		return "", false, err
	}
	return id, true, uci.Set(ctx, "dhcp", id, "ip", ip.String())
}

func countOwned(dhcp []device.Section) int {
	n := 0
	for _, s := range dhcp {
		if owned(s) {
			n++
		}
	}
	return n
}

// freeSection is the lowest travo_dns_<n> no section uses (of any type).
func freeSection(dhcp []device.Section) string {
	for n := 1; ; n++ {
		id := NamePrefix + strconv.Itoa(n)
		if !slices.ContainsFunc(dhcp, func(s device.Section) bool { return s.Name == id }) {
			return id
		}
	}
}

// StageRemoveName deletes Travo's section id; a user section is refused.
func StageRemoveName(ctx context.Context, uci device.UCI, dhcp []device.Section, id string) error {
	at := slices.IndexFunc(dhcp, func(s device.Section) bool { return s.Name == id })
	if at < 0 {
		return ErrNameNotFound
	}
	if !owned(dhcp[at]) {
		return ErrNameForeign
	}
	return uci.DeleteSection(ctx, "dhcp", id)
}
