package syssettings

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
)

const capture = "../../testdata/device/axt1800-25.12.6/uci-export-system.txt"

// applier stands in for txn.Manager.Apply: stage, then commit, counting calls.
type applier struct {
	d     *mock.Device
	calls int
	err   error
}

func (a *applier) Apply(ctx context.Context, configs []string, stage func(context.Context) error) error {
	a.calls++
	if a.err != nil {
		return a.err
	}
	if err := stage(ctx); err != nil {
		return errors.Join(err, a.d.Revert(ctx, configs[0]))
	}
	return a.d.Commit(ctx, configs[0])
}

func seeded(t *testing.T) (*Service, *applier) {
	t.Helper()
	raw, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	secs, err := cli.ParseExport("system", string(raw))
	if err != nil {
		t.Fatal(err)
	}
	d := mock.New()
	if err := d.Load(context.Background(), "system", secs); err != nil {
		t.Fatal(err)
	}
	a := &applier{d: d}
	return &Service{UCI: d, Apply: a}, a
}

func TestGetReadsTheDeviceCapture(t *testing.T) {
	s, _ := seeded(t)
	got, err := s.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := Settings{
		Hostname: "OpenWrt", Zonename: "UTC", Timezone: "GMT0",
		NTP: NTP{Enabled: true, Servers: []string{"0.openwrt.pool.ntp.org", "1.openwrt.pool.ntp.org", "2.openwrt.pool.ntp.org", "3.openwrt.pool.ntp.org"}},
	}
	if !equal(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestPutBackTheGetBodyWritesNothing(t *testing.T) {
	s, a := seeded(t)
	cur, _ := s.Get(context.Background())
	s.OnChange = func(Settings) { t.Fatal("notified without a change") }
	if _, err := s.Set(context.Background(), cur); err != nil {
		t.Fatal(err)
	}
	if a.calls != 0 {
		t.Fatalf("apply ran %d times for an unchanged body", a.calls)
	}
}

func TestSetWritesHostnameZoneAndServers(t *testing.T) {
	s, a := seeded(t)
	var notified []string
	s.OnChange = func(st Settings) { notified = append(notified, st.Timezone) }
	got, err := s.Set(context.Background(), Settings{
		Hostname: "travo", Zonename: "Europe/Berlin",
		NTP: NTP{Enabled: false, Servers: []string{"192.168.1.10", "time.example.org"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if a.calls != 1 {
		t.Fatalf("apply calls = %d", a.calls)
	}
	if got.Hostname != "travo" || got.Zonename != "Europe/Berlin" || got.Timezone != "CET-1CEST,M3.5.0,M10.5.0/3" {
		t.Fatalf("got %+v", got)
	}
	if got.NTP.Enabled || !slices.Equal(got.NTP.Servers, []string{"192.168.1.10", "time.example.org"}) {
		t.Fatalf("ntp %+v", got.NTP)
	}
	if !slices.Equal(notified, []string{"CET-1CEST,M3.5.0,M10.5.0/3"}) {
		t.Fatalf("OnChange got %v", notified)
	}
}

func TestSetCreatesAMissingNTPSection(t *testing.T) {
	d := mock.New()
	ctx := context.Background()
	if err := d.Load(ctx, "system", []device.Section{{Name: "@system[0]", Type: "system", Options: map[string]string{"hostname": "OpenWrt", "zonename": "UTC", "timezone": "GMT0"}}}); err != nil {
		t.Fatal(err)
	}
	s := &Service{UCI: d, Apply: &applier{d: d}}
	got, err := s.Set(ctx, Settings{Hostname: "OpenWrt", Zonename: "UTC", NTP: NTP{Enabled: true, Servers: []string{"pool.ntp.org"}}})
	if err != nil {
		t.Fatal(err)
	}
	if !got.NTP.Enabled || !slices.Equal(got.NTP.Servers, []string{"pool.ntp.org"}) {
		t.Fatalf("ntp %+v", got.NTP)
	}
}

func TestSetReturnsApplyFailure(t *testing.T) {
	s, a := seeded(t)
	a.err = errors.New("foreign staged changes")
	_, err := s.Set(context.Background(), Settings{Hostname: "x", Zonename: "UTC", NTP: NTP{Servers: []string{}}})
	if err == nil {
		t.Fatal("want the apply error")
	}
}

func TestValidate(t *testing.T) {
	cur := Settings{Hostname: "OpenWrt", Zonename: "UTC", Timezone: "GMT0"}
	ok := Settings{Hostname: "OpenWrt", Zonename: "UTC", NTP: NTP{Enabled: true, Servers: []string{"0.openwrt.pool.ntp.org"}}}
	for _, tc := range []struct {
		name  string
		edit  func(*Settings)
		field string
	}{
		{"ok", func(*Settings) {}, ""},
		{"empty hostname", func(s *Settings) { s.Hostname = "" }, "hostname"},
		{"dotted hostname", func(s *Settings) { s.Hostname = "a.b" }, "hostname"},
		{"leading dash", func(s *Settings) { s.Hostname = "-a" }, "hostname"},
		{"64 chars", func(s *Settings) { s.Hostname = string(make([]byte, 64)) }, "hostname"},
		{"unknown zone", func(s *Settings) { s.Zonename = "Mars/Olympus" }, "zonename"},
		{"legacy alias zone", func(s *Settings) { s.Zonename = "US/Eastern" }, "zonename"},
		{"mismatched timezone", func(s *Settings) { s.Timezone = "CET-1" }, "timezone"},
		{"matching timezone", func(s *Settings) { s.Timezone = "GMT0" }, ""},
		{"enabled without servers", func(s *Settings) { s.NTP.Servers = nil }, "ntp.servers"},
		{"disabled without servers", func(s *Settings) { s.NTP = NTP{} }, ""},
		{"bad server", func(s *Settings) { s.NTP.Servers = []string{"bad host"} }, "ntp.servers"},
		{"numeric name", func(s *Settings) { s.NTP.Servers = []string{"1.2.3.256"} }, "ntp.servers"},
		{"ipv6 server", func(s *Settings) { s.NTP.Servers = []string{"2001:db8::1"} }, ""},
		{"duplicate server", func(s *Settings) { s.NTP.Servers = []string{"a.org", "A.org"} }, "ntp.servers"},
		{"nine servers", func(s *Settings) {
			s.NTP.Servers = []string{"a1", "a2", "a3", "a4", "a5", "a6", "a7", "a8", "a9"}
		}, "ntp.servers"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := ok
			s.NTP.Servers = slices.Clone(ok.NTP.Servers)
			tc.edit(&s)
			_, err := Validate(s, cur)
			var fe *FieldError
			switch {
			case tc.field == "" && err != nil:
				t.Fatalf("unexpected %v", err)
			case tc.field != "" && (!errors.As(err, &fe) || fe.Field != tc.field):
				t.Fatalf("err %v, want field %s", err, tc.field)
			}
		})
	}
}

func TestUnchangedUnlistedZoneKeepsItsTimezone(t *testing.T) {
	cur := Settings{Zonename: "Etc/GMT+5", Timezone: "<-05>5"}
	tz, err := Validate(Settings{Hostname: "r", Zonename: "Etc/GMT+5"}, cur)
	if err != nil || tz != "<-05>5" {
		t.Fatalf("tz %q err %v", tz, err)
	}
}

func TestZonenames(t *testing.T) {
	names := Zonenames()
	if names[0] != UTC || len(names) < 300 {
		t.Fatalf("first %q, %d names", names[0], len(names))
	}
	if tz, _ := POSIXTimezone("UTC"); tz != "GMT0" {
		t.Fatalf("UTC -> %q, want the OpenWrt default GMT0", tz)
	}
}
