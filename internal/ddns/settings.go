// Package ddns manages dynamic DNS through the optional `ddns-scripts` package (#245).
// Travo owns one service section, ddns.travo; the package's sample sections and any other
// service stay as they are. The token is write-only: staged through stdin, never returned.
package ddns

import (
	"errors"
	"fmt"
	"regexp"
	"slices"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

const (
	Package = "ddns-scripts"
	Config  = "ddns"
	Section = "travo"
	// Uplink is the hotplug interface: ddns-scripts starts the section on its ifup and stops
	// it on its ifdown (testdata ddns/hotplug-95-ddns.txt). The travel uplink is wwan.
	Uplink = "wwan"
	// CACerts is the ca-bundle file the image ships (`ls /etc/ssl/certs`, testdata
	// ddns/PROVENANCE.md); ddns-scripts passes a file to curl as --cacert.
	CACerts = "/etc/ssl/certs/ca-certificates.crt"
)

var (
	ErrForeign  = errors.New("ddns.travo is not a service section Travo wrote; use LuCI")
	ErrNoChange = errors.New("dynamic DNS already has these settings")
)

// FieldError names the request field that failed validation.
type FieldError struct{ Field, Reason string }

func (e *FieldError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Reason) }

// View is the GET body. The token is never returned.
type View struct {
	Installed    bool     `json:"installed" doc:"ddns-scripts is installed"`
	Enabled      bool     `json:"enabled"`
	Service      string   `json:"service" doc:"ddns-scripts service_name, e.g. duckdns.org"`
	Domain       string   `json:"domain"`
	Username     string   `json:"username"`
	PasswordSet  bool     `json:"password_set" doc:"A token or password is stored; it is never returned"`
	RegisteredIP string   `json:"registered_ip" doc:"Address ddns-scripts last registered or looked up; empty when unknown"`
	Services     []string `json:"services" doc:"Providers ddns-scripts ships (/usr/share/ddns/default)"`
}

// Request is the PUT body (strict). An empty password keeps the stored one.
type Request struct {
	Enabled  bool   `json:"enabled"`
	Service  string `json:"service" maxLength:"64"`
	Domain   string `json:"domain" maxLength:"253"`
	Username string `json:"username,omitempty" maxLength:"128"`
	Password string `json:"password,omitempty" maxLength:"128" doc:"Token or password; write-only, empty keeps the stored one"`
}

var (
	domainRE    = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,63}$`)
	printableRE = regexp.MustCompile(`^[ -~]{1,128}$`)
)

// Read derives the view from ddns config sections (nil when the config is absent).
func Read(secs []device.Section) (View, error) {
	v := View{}
	i := slices.IndexFunc(secs, func(s device.Section) bool { return s.Name == Section })
	if i < 0 {
		return v, nil
	}
	s := secs[i]
	if s.Type != "service" {
		return View{}, ErrForeign
	}
	v.Enabled = s.Options["enabled"] == "1"
	v.Service, v.Domain, v.Username = s.Options["service_name"], s.Options["domain"], s.Options["username"]
	v.PasswordSet = s.Options["password"] != ""
	return v, nil
}

// Validate checks a request against the provider list and whether a token is stored.
func Validate(in Request, services []string, passwordSet bool) error {
	switch {
	case !slices.Contains(services, in.Service):
		return &FieldError{"service", "not a provider ddns-scripts ships"}
	case len(in.Domain) > 253 || !domainRE.MatchString(in.Domain):
		return &FieldError{"domain", "must be a fully qualified host name"}
	case in.Username != "" && !printableRE.MatchString(in.Username):
		return &FieldError{"username", "must be printable ASCII"}
	case in.Password != "" && !printableRE.MatchString(in.Password):
		return &FieldError{"password", "must be printable ASCII"}
	case in.Password == "" && !passwordSet:
		return &FieldError{"password", "required"}
	}
	return nil
}

// Write is one staged change to ddns.travo; it never carries the token.
type Write struct{ Op, Option, Value string }

// Plan returns the set-if-different writes for a validated request; token is true
// when the token must be staged through the secret writer.
func Plan(secs []device.Section, in Request) (writes []Write, token bool) {
	var cur device.Section
	if i := slices.IndexFunc(secs, func(s device.Section) bool { return s.Name == Section }); i >= 0 {
		cur = secs[i]
	} else {
		writes = append(writes, Write{"type", "", "service"})
	}
	enabled := "0"
	if in.Enabled {
		enabled = "1"
	}
	want := [][2]string{
		{"enabled", enabled}, {"service_name", in.Service}, {"domain", in.Domain}, {"lookup_host", in.Domain},
		{"username", in.Username}, {"interface", Uplink}, {"ip_source", "web"}, {"use_https", "1"}, {"cacert", CACerts},
	}
	for _, kv := range want {
		switch got, ok := cur.Options[kv[0]]; {
		case kv[1] == "" && ok:
			writes = append(writes, Write{"delete", kv[0], ""})
		case kv[1] != "" && got != kv[1]:
			writes = append(writes, Write{"set", kv[0], kv[1]})
		}
	}
	return writes, in.Password != "" && in.Password != cur.Options["password"]
}
