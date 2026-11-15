package cli

import (
	"context"
	"errors"
	"net/netip"
	"regexp"
	"strconv"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

var nextHopContinuation = regexp.MustCompile(`\n[ \t]+nexthop\b`)

var errDefaultRoute = errors.New("cannot select IPv4 default route")

func (d *Device) DefaultIPv4Route(ctx context.Context) (device.IPv4DefaultRoute, bool, error) {
	out, err := d.run(ctx, TierRead, "ip", "-4", "route", "show", "default")
	if err != nil {
		return device.IPv4DefaultRoute{}, false, err
	}
	return parseDefaultRoutes(out)
}

func parseDefaultRoutes(out string) (best device.IPv4DefaultRoute, found bool, err error) {
	out = nextHopContinuation.ReplaceAllString(out, " nexthop")
	ambiguous, valid := false, false
	for line := range strings.SplitSeq(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		route, usable, ranked := parseDefaultCandidate(fields)
		if !ranked {
			return best, false, errDefaultRoute
		}
		switch {
		case !found || route.Metric < best.Metric:
			best, found, ambiguous, valid = route, true, false, usable
		case route.Metric == best.Metric && (route != best || !usable || !valid):
			ambiguous = true
		}
	}
	if found && (ambiguous || !valid) {
		return best, false, errDefaultRoute
	}
	return best, found, nil
}

func parseDefaultCandidate(fields []string) (route device.IPv4DefaultRoute, usable, ranked bool) {
	usable = fields[0] == "default"
	if fields[0] == "unicast" && len(fields) > 1 && fields[1] == "default" {
		usable = true
	}
	metricSeen := false
	for i := 1; i < len(fields); i++ {
		key := fields[i]
		switch key {
		case "nexthop", "nhid", "linkdown":
			usable = false
		case "dev", "via", "src", "metric":
			if i+1 == len(fields) {
				return route, false, key != "metric"
			}
			i++
			val := fields[i]
			switch key {
			case "dev":
				route.Device = val
			case "metric":
				metric, err := strconv.ParseUint(val, 10, 32)
				if err != nil || metricSeen {
					return route, false, false
				}
				route.Metric, metricSeen = metric, true
			case "via", "src":
				addr, err := netip.ParseAddr(val)
				if err != nil || !addr.Is4() {
					usable = false
				}
				if key == "via" {
					route.Gateway = val
				} else {
					route.Source = val
				}
			}
		}
	}
	return route, usable && route.Device != "", true
}
