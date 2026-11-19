package wansettings

import (
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
)

const captures = "../../testdata/device/axt1800-25.12.6/"

func capture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(captures + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// network is the captured config: wan dhcp on device wan, lan 192.168.1.1/24 (list),
// guest 192.168.2.1/24.
func network(t *testing.T) []device.Section {
	t.Helper()
	secs, err := cli.ParseExport("network", string(capture(t, "wan/uci-export-network.txt")))
	if err != nil {
		t.Fatal(err)
	}
	return secs
}

// withWAN replaces network.wan's options and lists.
func withWAN(t *testing.T, opts map[string]string, lists map[string][]string) []device.Section {
	t.Helper()
	secs := network(t)
	i := slices.IndexFunc(secs, func(s device.Section) bool { return s.Name == Interface })
	secs[i].Options, secs[i].Lists = opts, lists
	if secs[i].Lists == nil {
		secs[i].Lists = map[string][]string{}
	}
	return secs
}

func TestReadCapturedDHCPWAN(t *testing.T) {
	v, err := Read(network(t))
	if err != nil {
		t.Fatal(err)
	}
	if v.Proto != ProtoDHCP || v.Device != "wan" || v.IPAddr != "" || v.DNS == nil || len(v.DNS) != 0 || v.PasswordSet {
		t.Fatalf("view = %+v", v)
	}
}

func TestReadStaticWithNetmaskAndPPPoE(t *testing.T) {
	v, err := Read(withWAN(t, map[string]string{"device": "wan", "proto": "static", "ipaddr": "203.0.113.5",
		"netmask": "255.255.255.0", "gateway": "203.0.113.1", "dns": "1.1.1.1"}, map[string][]string{}))
	if err != nil || v.IPAddr != "203.0.113.5/24" || v.Gateway != "203.0.113.1" || !slices.Equal(v.DNS, []string{"1.1.1.1"}) {
		t.Fatalf("static = %+v, %v", v, err)
	}
	v, err = Read(withWAN(t, map[string]string{"device": "wan", "proto": "pppoe", "username": "u@isp", "password": "secret"}, nil))
	if err != nil || v.Username != "u@isp" || !v.PasswordSet {
		t.Fatalf("pppoe = %+v, %v", v, err)
	}
}

func TestReadRefusesForeignLayouts(t *testing.T) {
	for name, opts := range map[string]map[string]string{
		"proto":   {"device": "wan", "proto": "qmi"},
		"device":  {"proto": "dhcp"},
		"netmask": {"device": "wan", "proto": "static", "ipaddr": "203.0.113.5", "netmask": "255.0.255.0"},
	} {
		if _, err := Read(withWAN(t, opts, nil)); !errors.Is(err, ErrForeign) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	two := withWAN(t, map[string]string{"device": "wan", "proto": "static"},
		map[string][]string{"ipaddr": {"203.0.113.5/24", "203.0.113.6/24"}})
	if _, err := Read(two); !errors.Is(err, ErrForeign) {
		t.Errorf("two addresses: err = %v", err)
	}
}

func TestValidateStatic(t *testing.T) {
	net := network(t)
	good := Request{Proto: ProtoStatic, IPAddr: "203.0.113.5/24", Gateway: "203.0.113.1", DNS: []string{"1.1.1.1", " 9.9.9.9", "1.1.1.1"}}
	v, err := Validate(good, net, false)
	if err != nil || !slices.Equal(v.DNS, []string{"1.1.1.1", "9.9.9.9"}) {
		t.Fatalf("good = %+v, %v", v, err)
	}
	for _, c := range []struct {
		field string
		in    Request
	}{
		{"ipaddr", Request{Proto: ProtoStatic, IPAddr: "203.0.113.5", Gateway: "203.0.113.1"}},
		{"ipaddr", Request{Proto: ProtoStatic, IPAddr: "203.0.113.0/24", Gateway: "203.0.113.1"}},
		{"ipaddr", Request{Proto: ProtoStatic, IPAddr: "203.0.113.5/31", Gateway: "203.0.113.4"}},
		{"gateway", Request{Proto: ProtoStatic, IPAddr: "203.0.113.5/24", Gateway: "198.51.100.1"}},
		{"gateway", Request{Proto: ProtoStatic, IPAddr: "203.0.113.5/24", Gateway: "203.0.113.255"}},
		{"gateway", Request{Proto: ProtoStatic, IPAddr: "203.0.113.5/24", Gateway: "203.0.113.5"}},
		{"dns", Request{Proto: ProtoStatic, IPAddr: "203.0.113.5/24", Gateway: "203.0.113.1", DNS: []string{"2606:4700::1111"}}},
		{"dns", Request{Proto: ProtoStatic, IPAddr: "203.0.113.5/24", Gateway: "203.0.113.1", DNS: []string{"1.1.1.1", "1.0.0.1", "8.8.8.8", "8.8.4.4", "9.9.9.9"}}},
		{"username", Request{Proto: ProtoStatic, IPAddr: "203.0.113.5/24", Gateway: "203.0.113.1", Username: "x"}},
	} {
		var fe *FieldError
		if _, err := Validate(c.in, net, false); !errors.As(err, &fe) || fe.Field != c.field {
			t.Errorf("%+v: err = %v", c.in, err)
		}
	}
	for _, lan := range []string{"192.168.1.20/24", "192.168.0.2/16", "192.168.2.9/24"} {
		in := Request{Proto: ProtoStatic, IPAddr: lan, Gateway: "192.168.1.1"}
		if lan == "192.168.2.9/24" {
			in.Gateway = "192.168.2.2"
		}
		if _, err := Validate(in, net, false); !errors.Is(err, ErrOverlapsLAN) {
			t.Errorf("%s: err = %v", lan, err)
		}
	}
}

func TestValidatePPPoEAndDHCP(t *testing.T) {
	net := network(t)
	if _, err := Validate(Request{Proto: ProtoPPPoE, Username: "u@isp", Password: "p w"}, net, false); err != nil {
		t.Fatal(err)
	}
	if _, err := Validate(Request{Proto: ProtoPPPoE, Username: "u@isp"}, net, true); err != nil {
		t.Fatalf("kept password: %v", err)
	}
	for field, c := range map[string]struct {
		in  Request
		set bool
	}{
		"password": {Request{Proto: ProtoPPPoE, Username: "u@isp"}, false},
		"username": {Request{Proto: ProtoPPPoE, Username: "has space", Password: "x"}, false},
		"ipaddr":   {Request{Proto: ProtoDHCP, IPAddr: "203.0.113.5/24"}, false},
		"proto":    {Request{Proto: "qmi"}, false},
	} {
		var fe *FieldError
		if _, err := Validate(c.in, net, c.set); !errors.As(err, &fe) || fe.Field != field {
			t.Errorf("%s: err = %v", field, err)
		}
	}
	var fe *FieldError
	if _, err := Validate(Request{Proto: ProtoPPPoE, Username: "u", Password: "a\nb"}, net, false); !errors.As(err, &fe) || fe.Field != "password" {
		t.Errorf("newline password: %v", err)
	}
}
