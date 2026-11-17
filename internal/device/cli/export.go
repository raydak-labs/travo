package cli

import (
	"fmt"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// ParseExport parses `uci export <config>`. Unlike `uci show`, it keeps the option/list
// distinction: a one-element list prints identically to an option in `uci show`
// (testdata/device/axt1800-25.12.6/uci-show-dhcp-list-single.txt), and dnsmasq's init
// script ignores `option server` entirely. Anonymous sections get the `@type[n]` name
// that `uci set` accepts.
func ParseExport(config, out string) ([]device.Section, error) {
	var secs []device.Section
	counts := map[string]int{}
	seenPackage := false
	for n, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		kw, rest, _ := strings.Cut(line, " ")
		switch kw {
		case "package":
			vals, err := splitQuoted(rest)
			if err != nil || len(vals) != 1 || vals[0] != config {
				return nil, fmt.Errorf("uci export: line %d: unexpected package %q", n+1, rest)
			}
			seenPackage = true
		case "config":
			vals, err := splitQuoted(rest)
			if err != nil || len(vals) < 1 || len(vals) > 2 {
				return nil, fmt.Errorf("uci export: line %d: bad section header %q", n+1, line)
			}
			typ := vals[0]
			name := fmt.Sprintf("@%s[%d]", typ, counts[typ])
			if len(vals) == 2 {
				name = vals[1]
			}
			counts[typ]++
			secs = append(secs, device.Section{Name: name, Type: typ, Options: map[string]string{}, Lists: map[string][]string{}})
		case "option", "list":
			if len(secs) == 0 {
				return nil, fmt.Errorf("uci export: line %d: %s outside a section", n+1, kw)
			}
			vals, err := splitQuoted(rest)
			if err != nil || len(vals) != 2 {
				return nil, fmt.Errorf("uci export: line %d: bad %s %q", n+1, kw, line)
			}
			s := &secs[len(secs)-1]
			if _, opt := s.Options[vals[0]]; !opt && s.Lists[vals[0]] == nil {
				s.Order = append(s.Order, vals[0])
			}
			if kw == "option" {
				s.Options[vals[0]] = vals[1]
			} else {
				s.Lists[vals[0]] = append(s.Lists[vals[0]], vals[1])
			}
		default:
			return nil, fmt.Errorf("uci export: line %d: unknown keyword %q", n+1, kw)
		}
	}
	if !seenPackage {
		return nil, fmt.Errorf("uci export: no package line for %s", config)
	}
	return secs, nil
}
