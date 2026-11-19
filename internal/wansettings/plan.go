package wansettings

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// write is one staged change to network.wan. Values never carry the PPPoE password.
type write struct {
	Op     string // set, delete, add (list entry)
	Option string
	Value  string
}

// Plan is the set-if-different delta from network.wan to a validated request.
type Plan struct {
	Writes []write
	// Password is staged through the key writer, never argv; empty keeps the stored one.
	Password string
}

// Empty: the request equals the stored config, so nothing is staged.
func (p Plan) Empty() bool { return len(p.Writes) == 0 && p.Password == "" }

// staticOnly and pppoeOnly are the options another protocol must not keep.
var (
	staticOnly = []string{"ipaddr", "netmask", "gateway"}
	pppoeOnly  = []string{"username", "password"}
)

// Diff plans the writes for a validated request. Leaving static drops its DNS list; a dhcp
// wan's own `dns`/`peerdns` (custom resolvers) is kept, since Travo does not edit it.
func Diff(network []device.Section, in Request) (Plan, error) {
	s, ok := wanSection(network)
	if !ok {
		return Plan{}, ErrForeign
	}
	cur, err := read(network)
	if err != nil {
		return Plan{}, err
	}
	var p Plan
	has := func(opt string) bool {
		_, o := s.Options[opt]
		_, l := s.Lists[opt]
		return o || l
	}
	del := func(opts ...string) {
		for _, o := range opts {
			if has(o) {
				p.Writes = append(p.Writes, write{"delete", o, ""})
			}
		}
	}
	set := func(opt, v string) {
		if s.Options[opt] != v || s.Lists[opt] != nil {
			p.Writes = append(p.Writes, write{"set", opt, v})
		}
	}
	set("proto", in.Proto)
	switch in.Proto {
	case ProtoDHCP, ProtoPPPoE:
		del(staticOnly...)
		if cur.Proto == ProtoStatic {
			del("dns")
		}
		if in.Proto == ProtoDHCP {
			del(pppoeOnly...)
			break
		}
		set("username", in.Username)
		if in.Password != "" && in.Password != cur.password {
			p.Password = in.Password
		}
	case ProtoStatic:
		del(pppoeOnly...)
		if cur.Proto != ProtoStatic || cur.IPAddr != in.IPAddr || has("netmask") {
			p.Writes = append(p.Writes, write{"set", "ipaddr", in.IPAddr})
			del("netmask")
		}
		set("gateway", in.Gateway)
		if !slices.Equal(append(strings.Fields(s.Options["dns"]), s.Lists["dns"]...), in.DNS) {
			del("dns")
			for _, d := range in.DNS {
				p.Writes = append(p.Writes, write{"add", "dns", d})
			}
		}
	}
	return p, nil
}

// Stage applies a plan to network.wan; the password goes through keys, out of argv.
func Stage(ctx context.Context, uci device.UCI, keys device.WANPasswordWriter, p Plan) error {
	for _, w := range p.Writes {
		var err error
		switch w.Op {
		case "set":
			err = uci.Set(ctx, "network", Interface, w.Option, w.Value)
		case "add":
			err = uci.AddToList(ctx, "network", Interface, w.Option, w.Value)
		default:
			err = uci.DeleteOption(ctx, "network", Interface, w.Option)
			if errors.Is(err, device.ErrNotFound) {
				err = nil
			}
		}
		if err != nil {
			return err
		}
	}
	if p.Password == "" {
		return nil
	}
	if keys == nil {
		return errors.New("wan: no password writer")
	}
	return keys.SetWANPassword(ctx, p.Password)
}
