package adguard

import (
	"fmt"
	"slices"
	"strings"
)

// Expert topology (ADR 0021): AGH answers on port 53 of loopback and the lan (and guest)
// addresses and forwards everything to dnsmasq, which moved to 127.0.0.1:5354 and keeps the
// DNS layer stack.
const (
	ExpertDNSPort  = "53"
	DnsmasqPort    = "5354"
	ExpertUpstream = LoopbackDNS + ":" + DnsmasqPort
)

// expertKeys are the dns: keys the expert swap rewrites, in file order of the seed. Only
// bind_hosts, port and upstream_dns must exist; the others are rewritten when present.
var expertKeys = []string{"bind_hosts", "port", "upstream_dns", "fallback_dns", "use_private_ptr_resolvers", "local_ptr_upstreams"}

var requiredExpertKeys = []string{"bind_hosts", "port", "upstream_dns"}

// SavedDNS holds the verbatim lines of every rewritten key as the default topology had them,
// so switching back restores exactly those lines and keeps every other edit made meanwhile.
type SavedDNS map[string][]string

// expertLines is what each key becomes. fallback_dns is emptied: a fallback would answer
// past dnsmasq and bypass the layer stack (a VPN DNS leak); PTR for private ranges goes to
// dnsmasq, which knows the DHCP names.
func expertLines(key string, hosts []string) []string {
	switch key {
	case "bind_hosts":
		out := []string{"  bind_hosts:"}
		for _, h := range hosts {
			out = append(out, "    - "+h)
		}
		return out
	case "port":
		return []string{"  port: " + ExpertDNSPort}
	case "upstream_dns":
		return []string{"  upstream_dns:", "    - " + ExpertUpstream}
	case "fallback_dns":
		return []string{"  fallback_dns: []"}
	case "use_private_ptr_resolvers":
		return []string{"  use_private_ptr_resolvers: true"}
	default: // local_ptr_upstreams
		return []string{"  local_ptr_upstreams:", "    - " + ExpertUpstream}
	}
}

// ExpertHosts is loopback first, then the served addresses.
func ExpertHosts(lan, guest string) []string {
	hosts := []string{LoopbackDNS, lan}
	if guest != "" {
		hosts = append(hosts, guest)
	}
	return hosts
}

// ToExpert rewrites a loopback (ADR 0006) config to the expert topology and returns the
// lines it replaced. A config that is not loopback is refused: the saved lines must be the
// default topology's, or switching back would restore something else.
func ToExpert(yaml []byte, hosts []string) ([]byte, SavedDNS, error) {
	l, err := ParseListen(yaml)
	if err != nil {
		return nil, nil, err
	}
	if !l.IsLoopback() {
		return nil, nil, fmt.Errorf("%w: not the loopback topology", ErrUnknownShape)
	}
	lines := strings.Split(string(yaml), "\n")
	spans, err := dnsSpans(lines)
	if err != nil {
		return nil, nil, err
	}
	saved := SavedDNS{}
	repl := map[string][]string{}
	for key, s := range spans {
		saved[key] = slices.Clone(lines[s.start:s.end])
		repl[key] = expertLines(key, hosts)
	}
	return []byte(strings.Join(replaceSpans(lines, spans, repl), "\n")), saved, nil
}

// ToDefault puts the saved lines back. Keys the saved set lacks were absent before and
// were not added, so they are left alone.
func ToDefault(yaml []byte, saved SavedDNS) ([]byte, error) {
	for _, k := range requiredExpertKeys {
		if len(saved[k]) == 0 {
			return nil, fmt.Errorf("%w: no saved %s", ErrUnknownShape, k)
		}
	}
	lines := strings.Split(string(yaml), "\n")
	spans, err := dnsSpans(lines)
	if err != nil {
		return nil, err
	}
	repl := map[string][]string{}
	for key := range spans {
		if v, ok := saved[key]; ok {
			repl[key] = v
		}
	}
	return []byte(strings.Join(replaceSpans(lines, spans, repl), "\n")), nil
}

// IsExpert reports whether the config is the expert topology for hosts.
func IsExpert(yaml []byte, hosts []string) bool {
	l, err := ParseListen(yaml)
	if err != nil || l.HTTP != LoopbackHTTP || l.DNSPort != ExpertDNSPort || !slices.Equal(l.BindHosts, hosts) {
		return false
	}
	lines := strings.Split(string(yaml), "\n")
	spans, err := dnsSpans(lines)
	if err != nil {
		return false
	}
	s := spans["upstream_dns"]
	return slices.Equal(lines[s.start:s.end], expertLines("upstream_dns", nil))
}

// dnsSpans locates each expert key directly inside dns:, with its indented continuation.
func dnsSpans(lines []string) (map[string]span, error) {
	dns, err := block(lines, "dns:")
	if err != nil {
		return nil, err
	}
	out := map[string]span{}
	for _, key := range expertKeys {
		found := -1
		for i := dns.start; i < dns.end; i++ {
			if lines[i] == "  "+key+":" || strings.HasPrefix(lines[i], "  "+key+": ") {
				if found >= 0 {
					return nil, fmt.Errorf("%w: duplicate %s", ErrUnknownShape, key)
				}
				found = i
			}
		}
		if found < 0 {
			if slices.Contains(requiredExpertKeys, key) {
				return nil, fmt.Errorf("%w: no %s", ErrUnknownShape, key)
			}
			continue
		}
		end := found + 1
		for end < dns.end && strings.HasPrefix(lines[end], "    ") {
			end++
		}
		out[key] = span{found, end}
	}
	return out, nil
}

// replaceSpans swaps each span for its replacement, back to front so indexes stay valid.
func replaceSpans(lines []string, spans map[string]span, repl map[string][]string) []string {
	keys := make([]string, 0, len(repl))
	for k := range repl {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b string) int { return spans[b].start - spans[a].start })
	out := slices.Clone(lines)
	for _, k := range keys {
		s := spans[k]
		out = slices.Replace(out, s.start, s.end, repl[k]...)
	}
	return out
}
