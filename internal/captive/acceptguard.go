package captive

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/netip"
	"slices"
	"syscall"
)

// ErrTargetRefused: a portal page pointed the walker at the router itself or at a
// network behind it (ADR 0023). Every dial is checked, so redirects and DNS answers
// that change between lookups (rebinding) are covered too.
var ErrTargetRefused = errors.New("portal target is the router or a network it serves")

// IfaceAddrs is one kernel interface with its addresses as prefixes (host bits kept).
type IfaceAddrs struct {
	Name     string
	Prefixes []netip.Prefix
}

// TargetPolicy decides which addresses a walk may dial. Pure data plus an optional
// route lookup; build it with NewTargetPolicy.
type TargetPolicy struct {
	// Own holds every address on every router interface, the uplink's included.
	Own []netip.Addr
	// Served holds the subnets of every interface except the uplink: LAN, guest, VPN.
	Served []netip.Prefix
	// UplinkSources are the uplink's own addresses: the kernel must route a target
	// out of the uplink, so the source it picks must be one of them.
	UplinkSources []netip.Addr
	// RouteSource returns the source address the kernel would use for a target; nil
	// skips the route check (unit tests of the address rules).
	RouteSource func(netip.Addr) (netip.Addr, error)
	// allowLoopback lets same-package tests use httptest servers; production never sets it.
	allowLoopback bool
}

// NewTargetPolicy allows the uplink's subnet and anything routed out of the uplink
// (public addresses, the hotel's other private nets); it refuses the router's own
// addresses and the subnets of every other interface.
func NewTargetPolicy(ifaces []IfaceAddrs, uplink string, routeSource func(netip.Addr) (netip.Addr, error)) TargetPolicy {
	p := TargetPolicy{RouteSource: routeSource}
	for _, it := range ifaces {
		for _, pfx := range it.Prefixes {
			p.Own = append(p.Own, pfx.Addr().Unmap())
			if it.Name == uplink {
				p.UplinkSources = append(p.UplinkSources, pfx.Addr().Unmap())
			} else {
				p.Served = append(p.Served, pfx.Masked())
			}
		}
	}
	return p
}

// Check refuses addresses a portal page must never reach.
func (p TargetPolicy) Check(ip netip.Addr) error {
	ip = ip.Unmap()
	switch {
	case !ip.IsValid(), ip.IsUnspecified(), ip.IsLoopback() && !p.allowLoopback, ip.IsLinkLocalUnicast(), ip.IsMulticast():
		return fmt.Errorf("%w: %s", ErrTargetRefused, ip)
	case slices.Contains(p.Own, ip):
		return fmt.Errorf("%w: %s is a router address", ErrTargetRefused, ip)
	case slices.ContainsFunc(p.Served, func(s netip.Prefix) bool { return s.Contains(ip) }):
		return fmt.Errorf("%w: %s is behind the router", ErrTargetRefused, ip)
	}
	if p.RouteSource == nil {
		return nil
	}
	src, err := p.RouteSource(ip)
	if err != nil || !slices.Contains(p.UplinkSources, src.Unmap()) {
		return fmt.Errorf("%w: %s is not routed via the uplink", ErrTargetRefused, ip)
	}
	return nil
}

// control runs on every dial with the address actually dialed.
func (p TargetPolicy) control(_, address string, _ syscall.RawConn) error {
	ap, err := netip.ParseAddrPort(address)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrTargetRefused, address)
	}
	return p.Check(ap.Addr())
}

// NewPortalClient is the production HTTP edge for one walk: a fresh cookie jar,
// bounded redirects and requests, verified TLS, no proxy, no connection reuse, and the
// policy on every dial.
func NewPortalClient(p TargetPolicy) *http.Client {
	jar, _ := cookiejar.New(nil) // never fails without options
	dialer := &net.Dialer{Timeout: acceptRequest, Control: p.control}
	transport := &http.Transport{DialContext: dialer.DialContext, TLSHandshakeTimeout: acceptRequest,
		ResponseHeaderTimeout: acceptRequest, DisableKeepAlives: true}
	return &http.Client{Jar: jar, Transport: transport, Timeout: acceptRequest, CheckRedirect: p.checkRedirect}
}

// checkRedirect refuses a literal-IP redirect target before dialing; names are checked
// by the dial hook on the address they resolve to.
func (p TargetPolicy) checkRedirect(req *http.Request, via []*http.Request) error {
	if ip, err := netip.ParseAddr(req.URL.Hostname()); err == nil {
		if err := p.Check(ip); err != nil {
			return err
		}
	}
	return limitRedirects(req, via)
}

// KernelInterfaces lists the router's interfaces and addresses. The daemon runs on the
// router, so the kernel's view is the router's (netifd's is not: device-facts §1).
func KernelInterfaces() ([]IfaceAddrs, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	out := make([]IfaceAddrs, 0, len(ifaces))
	for _, it := range ifaces {
		addrs, err := it.Addrs()
		if err != nil {
			return nil, err
		}
		entry := IfaceAddrs{Name: it.Name}
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok {
				ip, _ := netip.AddrFromSlice(n.IP)
				ones, _ := n.Mask.Size()
				entry.Prefixes = append(entry.Prefixes, netip.PrefixFrom(ip.Unmap(), ones))
			}
		}
		out = append(out, entry)
	}
	return out, nil
}

// KernelRouteSource asks the kernel which source address it would use for ip: a
// connected UDP socket sends nothing but runs the route lookup (policy rules included).
func KernelRouteSource(ip netip.Addr) (netip.Addr, error) {
	conn, err := net.DialUDP("udp", nil, net.UDPAddrFromAddrPort(netip.AddrPortFrom(ip, 9)))
	if err != nil {
		return netip.Addr{}, err
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).AddrPort().Addr().Unmap(), nil
}
