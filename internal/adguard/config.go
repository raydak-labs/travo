package adguard

import (
	_ "embed"
	"errors"
	"fmt"
	"strings"
)

// Loopback targets (ADR 0006): the admin UI only on loopback (the gated proxy forwards to
// it), DNS only on loopback where dnsmasq forwards (`127.0.0.1#5353`).
const (
	LoopbackHTTP = "127.0.0.1:3000"
	LoopbackDNS  = "127.0.0.1"
	DNSPort      = "5353"
)

// ErrUnknownShape means the YAML is not the canonical block style AdGuard Home writes.
// The migration refuses rather than guess: a wrong edit could expose the admin API or kill DNS.
var ErrUnknownShape = errors.New("adguardhome.yaml: unrecognised shape")

// Seed is the config written before a fresh install, so AGH never starts its first-run
// wizard on the LAN. It is the captured 0.107.79 config (testdata/device/axt1800-25.12.6/
// adguard/adguardhome.yaml) passed through Loopback; a test keeps the two in step.
//
//go:embed seed.yaml
var Seed []byte

// Listen is what the YAML says AGH binds.
type Listen struct {
	HTTP      string   // http.address
	BindHosts []string // dns.bind_hosts
	DNSPort   string   // dns.port
}

// IsLoopback reports whether the config already matches the ADR 0006 topology.
func (l Listen) IsLoopback() bool {
	return l.HTTP == LoopbackHTTP && len(l.BindHosts) == 1 && l.BindHosts[0] == LoopbackDNS && l.DNSPort == DNSPort
}

// span locates the lines one key occupies: [start, end) with end-start > 1 for a list.
type span struct{ start, end int }

type parsed struct {
	lines      []string
	http, port span
	bindHosts  span
	listen     Listen
}

// ParseListen reads the listen addresses from AGH's canonical YAML (two-space block style).
func ParseListen(yaml []byte) (Listen, error) {
	p, err := parse(yaml)
	return p.listen, err
}

// Loopback rewrites http.address and dns.bind_hosts/port to the loopback topology. Every
// other byte stays as it was. changed is false when the config was already loopback.
func Loopback(yaml []byte) (out []byte, changed bool, err error) {
	p, err := parse(yaml)
	if err != nil {
		return nil, false, err
	}
	if p.listen.IsLoopback() {
		return yaml, false, nil
	}
	lines := p.lines
	var b []string
	b = append(b, lines[:p.http.start]...)
	b = append(b, "  address: "+LoopbackHTTP)
	b = append(b, lines[p.http.end:minInt(p.bindHosts.start, p.port.start)]...)
	// bind_hosts and port are both inside dns:, in either order
	first, second := p.bindHosts, p.port
	if p.port.start < p.bindHosts.start {
		first, second = p.port, p.bindHosts
	}
	b = append(b, rewrite(first, p)...)
	b = append(b, lines[first.end:second.start]...)
	b = append(b, rewrite(second, p)...)
	b = append(b, lines[second.end:]...)
	return []byte(strings.Join(b, "\n")), true, nil
}

func rewrite(s span, p parsed) []string {
	if s == p.port {
		return []string{"  port: " + DNSPort}
	}
	return []string{"  bind_hosts:", "    - " + LoopbackDNS}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func parse(yaml []byte) (parsed, error) {
	s := string(yaml)
	if strings.ContainsRune(s, '\r') || strings.ContainsRune(s, '\t') {
		return parsed{}, fmt.Errorf("%w: CR or tab", ErrUnknownShape)
	}
	p := parsed{lines: strings.Split(s, "\n")}
	http, err := block(p.lines, "http:")
	if err != nil {
		return parsed{}, err
	}
	dns, err := block(p.lines, "dns:")
	if err != nil {
		return parsed{}, err
	}
	if http.start > dns.start {
		return parsed{}, fmt.Errorf("%w: http: after dns:", ErrUnknownShape)
	}
	if p.http, p.listen.HTTP, err = scalar(p.lines, http, "address"); err != nil {
		return parsed{}, err
	}
	if p.port, p.listen.DNSPort, err = scalar(p.lines, dns, "port"); err != nil {
		return parsed{}, err
	}
	if p.bindHosts, p.listen.BindHosts, err = list(p.lines, dns, "bind_hosts"); err != nil {
		return parsed{}, err
	}
	return p, nil
}

// block finds a top-level key and returns the span of its indented body.
func block(lines []string, key string) (span, error) {
	start := -1
	for i, l := range lines {
		if l == key {
			if start >= 0 {
				return span{}, fmt.Errorf("%w: duplicate %s", ErrUnknownShape, key)
			}
			start = i + 1
		}
	}
	if start < 0 {
		return span{}, fmt.Errorf("%w: no top-level %s", ErrUnknownShape, key)
	}
	end := start
	for end < len(lines) && (lines[end] == "" || strings.HasPrefix(lines[end], " ")) {
		end++
	}
	return span{start, end}, nil
}

// scalar finds exactly one `  key: value` directly inside b.
func scalar(lines []string, b span, key string) (span, string, error) {
	found, val := -1, ""
	for i := b.start; i < b.end; i++ {
		v, ok := strings.CutPrefix(lines[i], "  "+key+":")
		if !ok {
			continue
		}
		if found >= 0 {
			return span{}, "", fmt.Errorf("%w: duplicate %s", ErrUnknownShape, key)
		}
		val = strings.TrimSpace(v)
		if val == "" || strings.ContainsAny(val, `"'[{#`) {
			return span{}, "", fmt.Errorf("%w: %s value %q", ErrUnknownShape, key, val)
		}
		found = i
	}
	if found < 0 {
		return span{}, "", fmt.Errorf("%w: no %s", ErrUnknownShape, key)
	}
	return span{found, found + 1}, val, nil
}

// list finds exactly one `  key:` followed by `    - item` lines directly inside b.
func list(lines []string, b span, key string) (span, []string, error) {
	found := -1
	for i := b.start; i < b.end; i++ {
		if lines[i] == "  "+key+":" {
			if found >= 0 {
				return span{}, nil, fmt.Errorf("%w: duplicate %s", ErrUnknownShape, key)
			}
			found = i
		}
	}
	if found < 0 {
		return span{}, nil, fmt.Errorf("%w: no block list %s", ErrUnknownShape, key)
	}
	var items []string
	end := found + 1
	for end < b.end {
		v, ok := strings.CutPrefix(lines[end], "    - ")
		if !ok {
			break
		}
		v = strings.TrimSpace(v)
		if v == "" || strings.ContainsAny(v, `"'[{#`) {
			return span{}, nil, fmt.Errorf("%w: %s item %q", ErrUnknownShape, key, v)
		}
		items = append(items, v)
		end++
	}
	if len(items) == 0 {
		return span{}, nil, fmt.Errorf("%w: empty %s", ErrUnknownShape, key)
	}
	return span{found, end}, items, nil
}
