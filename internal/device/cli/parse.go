// Package cli is the real Device adapter: it shells out to uci, ubus and ip.
package cli

import (
	"fmt"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// ParseShow parses `uci show <config>` output. Fixture use only: `uci show` cannot tell a
// one-element list from an option (see ParseExport); the adapter reads with ParseExport.
func ParseShow(config, out string) ([]device.Section, error) {
	var order []string
	byName := map[string]*device.Section{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("uci show: malformed line %q", line)
		}
		parts := strings.SplitN(key, ".", 3)
		if len(parts) < 2 || parts[0] != config {
			return nil, fmt.Errorf("uci show: unexpected key %q", key)
		}
		name := parts[1]
		sec, exists := byName[name]
		if !exists {
			sec = &device.Section{Name: name, Options: map[string]string{}, Lists: map[string][]string{}}
			byName[name] = sec
			order = append(order, name)
		}
		if len(parts) == 2 { // section header: dhcp.lan=dhcp
			sec.Type = val
			continue
		}
		values, err := splitQuoted(val)
		if err != nil {
			return nil, fmt.Errorf("uci show %s: %w", key, err)
		}
		if len(values) == 1 && !strings.Contains(val, "' '") {
			sec.Options[parts[2]] = values[0]
		} else {
			sec.Lists[parts[2]] = values
		}
	}
	out2 := make([]device.Section, 0, len(order))
	for _, n := range order {
		out2 = append(out2, *byName[n])
	}
	return out2, nil
}

// splitQuoted splits `'a' 'b'` honouring the shell `'\”` escape uci uses.
func splitQuoted(s string) ([]string, error) {
	var vals []string
	var cur strings.Builder
	i := 0
	inQuote := false
	started := false
	for i < len(s) {
		c := s[i]
		switch {
		case inQuote && c == '\'':
			inQuote = false
		case inQuote:
			cur.WriteByte(c)
		case c == '\'':
			inQuote, started = true, true
		case c == '\\' && i+1 < len(s):
			i++
			cur.WriteByte(s[i])
			started = true
		case c == ' ':
			if started {
				vals = append(vals, cur.String())
				cur.Reset()
				started = false
			}
		default:
			cur.WriteByte(c)
			started = true
		}
		i++
	}
	if inQuote {
		return nil, fmt.Errorf("unterminated quote in %q", s)
	}
	if started {
		vals = append(vals, cur.String())
	}
	return vals, nil
}
