package vpnprofiles

import (
	"encoding/base64"
	"errors"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	maxConfig = 64 << 10
	maxPeers  = 32
)

var ErrInvalid = errors.New("invalid WireGuard profile")

type Config struct {
	PrivateKey string
	Addresses  []netip.Prefix
	DNS        []netip.Addr
	MTU        *uint16
	ListenPort *uint16
	Peers      []Peer
}
type Peer struct {
	PublicKey           string
	PresharedKey        *string
	AllowedIPs          []netip.Prefix
	Endpoint            *string
	PersistentKeepalive *uint16
}

func Parse(raw string) (Config, error) {
	if len(raw) > maxConfig || !utf8.ValidString(raw) {
		return Config{}, ErrInvalid
	}
	var sections []map[string]string
	for _, line := range strings.Split(raw, "\n") {
		line, _, _ = strings.Cut(line, "#")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch asciiLower(line) {
		case "[interface]":
			if len(sections) != 0 {
				return Config{}, ErrInvalid
			}
			sections = append(sections, map[string]string{})
			continue
		case "[peer]":
			if len(sections) == 0 || len(sections) > maxPeers {
				return Config{}, ErrInvalid
			}
			sections = append(sections, map[string]string{})
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		k, v = asciiLower(strings.TrimSpace(k)), strings.TrimSpace(v)
		if !ok || len(sections) == 0 || v == "" {
			return Config{}, ErrInvalid
		}
		current := sections[len(sections)-1]
		if _, duplicate := current[k]; duplicate {
			return Config{}, ErrInvalid
		}
		current[k] = v
	}
	if len(sections) < 2 {
		return Config{}, ErrInvalid
	}
	c := Config{Peers: make([]Peer, 0, len(sections)-1)}
	if err := parseInterface(&c, sections[0]); err != nil {
		return Config{}, err
	}
	for _, fields := range sections[1:] {
		p, err := parsePeer(fields)
		if err != nil {
			return Config{}, err
		}
		c.Peers = append(c.Peers, p)
	}
	if !c.valid() {
		return Config{}, ErrInvalid
	}
	return c, nil
}

func asciiLower(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return r
	}, s)
}

func parseInterface(c *Config, fields map[string]string) error {
	for k, v := range fields {
		var err error
		switch k {
		case "privatekey":
			c.PrivateKey = v
		case "address":
			c.Addresses, err = parseList(v, netip.ParsePrefix)
		case "dns":
			c.DNS, err = parseList(v, netip.ParseAddr)
		case "mtu":
			c.MTU, err = parseNumber(v)
		case "listenport":
			c.ListenPort, err = parseNumber(v)
		default:
			return ErrInvalid
		}
		if err != nil {
			return ErrInvalid
		}
	}
	return nil
}

func parsePeer(fields map[string]string) (Peer, error) {
	var p Peer
	for k, v := range fields {
		var err error
		switch k {
		case "publickey":
			p.PublicKey = v
		case "presharedkey":
			p.PresharedKey = &v
		case "allowedips":
			p.AllowedIPs, err = parseList(v, netip.ParsePrefix)
			for i, ip := range p.AllowedIPs {
				p.AllowedIPs[i] = ip.Masked()
			}
		case "endpoint":
			p.Endpoint = &v
		case "persistentkeepalive":
			p.PersistentKeepalive, err = parseNumber(v)
		default:
			return Peer{}, ErrInvalid
		}
		if err != nil {
			return Peer{}, ErrInvalid
		}
	}
	return p, nil
}

func parseList[T any](v string, parse func(string) (T, error)) ([]T, error) {
	var values []T
	for _, field := range strings.Split(v, ",") {
		value, err := parse(strings.TrimSpace(field))
		if err != nil {
			return nil, ErrInvalid
		}
		values = append(values, value)
	}
	return values, nil
}
func parseNumber(s string) (*uint16, error) {
	n, e := strconv.ParseUint(s, 10, 16)
	if e != nil {
		return nil, ErrInvalid
	}
	v := uint16(n)
	return &v, nil
}

func validKey(s string) bool {
	b, e := base64.StdEncoding.Strict().DecodeString(s)
	return e == nil && len(b) == 32 && base64.StdEncoding.EncodeToString(b) == s && string(b) != string(make([]byte, 32))
}
func validEndpoint(s string) bool {
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		return false
	}
	n, err := parseNumber(port)
	if err != nil || *n == 0 {
		return false
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		return ip.Zone() == "" && strings.HasPrefix(s, "[") == ip.Is6()
	}
	if strings.HasPrefix(s, "[") || len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(strings.TrimSuffix(host, "."), ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range asciiLower(label) {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
				return false
			}
		}
	}
	return true
}

func (c Config) valid() bool {
	if !validKey(c.PrivateKey) || len(c.Addresses) == 0 || len(c.Peers) == 0 || len(c.Peers) > maxPeers ||
		c.MTU != nil && *c.MTU < 576 || c.ListenPort != nil && *c.ListenPort == 0 {
		return false
	}
	for _, a := range c.Addresses {
		if !a.IsValid() {
			return false
		}
	}
	for _, a := range c.DNS {
		if !a.IsValid() || a.Zone() != "" {
			return false
		}
	}
	seen := map[string]bool{}
	for _, p := range c.Peers {
		if seen[p.PublicKey] || !p.valid() {
			return false
		}
		seen[p.PublicKey] = true
	}
	return true
}
func (p Peer) valid() bool {
	if !validKey(p.PublicKey) || p.PresharedKey != nil && !validKey(*p.PresharedKey) ||
		p.Endpoint != nil && !validEndpoint(*p.Endpoint) || len(p.AllowedIPs) == 0 {
		return false
	}
	for _, a := range p.AllowedIPs {
		if !a.IsValid() || a.Masked() != a {
			return false
		}
	}
	return true
}
