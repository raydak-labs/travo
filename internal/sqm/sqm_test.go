package sqm

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/raydak-labs/travo/internal/device/cli"
	"github.com/raydak-labs/travo/internal/device/mock"
	"github.com/raydak-labs/travo/internal/pkgmgr"
)

type directApply struct {
	dev     *mock.Device
	configs []string
	err     error
}

func (a *directApply) Apply(ctx context.Context, configs []string, stage func(context.Context) error) error {
	if a.err != nil {
		return a.err
	}
	a.configs = configs
	if err := stage(ctx); err != nil {
		_ = a.dev.Revert(ctx, configs[0])
		return err
	}
	for _, c := range configs {
		if err := a.dev.Commit(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

func newTestService(t *testing.T, installed bool) (*Service, *mock.Device, *directApply) {
	t.Helper()
	dev := mock.New()
	dev.SetService(InitScript, true, false)
	raw, err := os.ReadFile("../../testdata/device/axt1800-25.12.6/sqm/uci-export-sqm.txt")
	if err != nil {
		t.Fatal(err)
	}
	secs, err := cli.ParseExport(Config, string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err := dev.Load(context.Background(), Config, secs); err != nil {
		t.Fatal(err)
	}
	pm := pkgmgr.NewMock()
	if installed {
		if err := pm.Install(context.Background(), Package); err != nil {
			t.Fatal(err)
		}
	}
	ap := &directApply{dev: dev}
	return &Service{UCI: dev, Packages: pm, Services: dev, Apply: ap}, dev, ap
}

func TestGetFromStockFixture(t *testing.T) {
	svc, _, _ := newTestService(t, true)
	got, err := svc.Get(context.Background())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	want := Settings{
		Enabled:   false,
		Interface: "eth1",
		Download:  85000,
		Upload:    10000,
		Qdisc:     "cake",
		Script:    "piece_of_cake.qos",
		Linklayer: "none",
		Overhead:  0,
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestMissingPackageReturnsNotInstalled(t *testing.T) {
	svc, _, _ := newTestService(t, false)
	if _, err := svc.Get(context.Background()); !errors.Is(err, pkgmgr.ErrNotInstalled) {
		t.Fatalf("Get err = %v, want ErrNotInstalled", err)
	}
	want := Settings{
		Enabled: true, Interface: "eth1", Download: 50000, Upload: 10000,
		Qdisc: "cake", Script: "piece_of_cake.qos", Linklayer: "ethernet", Overhead: 44,
	}
	if _, err := svc.Set(context.Background(), want); !errors.Is(err, pkgmgr.ErrNotInstalled) {
		t.Fatalf("Set err = %v, want ErrNotInstalled", err)
	}
}

func TestSetPreservesSectionNameAndUnknownOptions(t *testing.T) {
	svc, dev, ap := newTestService(t, true)
	want := Settings{
		Enabled:   true,
		Interface: "phy0-sta0",
		Download:  45000,
		Upload:    12000,
		Qdisc:     "fq_codel",
		Script:    "simple.qos",
		Linklayer: "ethernet",
		Overhead:  44,
	}
	got, err := svc.Set(context.Background(), want)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if len(ap.configs) != 1 || ap.configs[0] != Config {
		t.Fatalf("Apply configs = %v, want [sqm]", ap.configs)
	}
	if len(dev.ServiceActions) != 1 || dev.ServiceActions[0] != "sqm restart" {
		t.Fatalf("ServiceActions = %v, want [sqm restart]", dev.ServiceActions)
	}
	secs, err := dev.Show(context.Background(), Config)
	if err != nil {
		t.Fatal(err)
	}
	if len(secs) != 1 || secs[0].Name != "eth1" {
		t.Fatalf("section name changed: %+v", secs)
	}
	for k, v := range map[string]string{
		"qdisc_advanced":               "0",
		"ingress_ecn":                  "ECN",
		"egress_ecn":                   "ECN",
		"qdisc_really_really_advanced": "0",
		"itarget":                      "auto",
		"etarget":                      "auto",
		"use_mq":                       "0",
	} {
		if secs[0].Options[k] != v {
			t.Errorf("unknown option %s = %q, want %q", k, secs[0].Options[k], v)
		}
	}
}

func TestSetNoOpSkipsApply(t *testing.T) {
	svc, _, ap := newTestService(t, true)
	cur, err := svc.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Set(context.Background(), cur)
	if err != nil {
		t.Fatalf("Set no-op: %v", err)
	}
	if got != cur {
		t.Fatalf("got %+v, want %+v", got, cur)
	}
	if len(ap.configs) != 0 {
		t.Fatalf("expected no Apply on no-op, got %v", ap.configs)
	}
}

func TestSetCreatesQueueSectionWhenConfigMissing(t *testing.T) {
	dev := mock.New()
	dev.SetService(InitScript, true, false)
	pm := pkgmgr.NewMock()
	_ = pm.Install(context.Background(), Package)
	ap := &directApply{dev: dev}
	svc := &Service{UCI: dev, Packages: pm, Services: dev, Apply: ap}

	want := Settings{
		Enabled: true, Interface: "eth1", Download: 30000, Upload: 5000,
		Qdisc: "cake", Script: "layer_cake.qos", Linklayer: "atm", Overhead: 44,
	}
	got, err := svc.Set(context.Background(), want)
	if err != nil {
		t.Fatalf("Set on empty config: %v", err)
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestValidateRejectsBadValues(t *testing.T) {
	base := Settings{
		Enabled: true, Interface: "eth1", Download: 50000, Upload: 10000,
		Qdisc: "cake", Script: "piece_of_cake.qos", Linklayer: "ethernet", Overhead: 44,
	}
	cases := []struct {
		field string
		mod   func(*Settings)
	}{
		{"interface", func(s *Settings) { s.Interface = "" }},
		{"interface", func(s *Settings) { s.Interface = "-eth1" }},
		{"interface", func(s *Settings) { s.Interface = "eth1;rm" }},
		{"interface", func(s *Settings) { s.Interface = "a_very_long_ifname" }},
		{"download", func(s *Settings) { s.Download = -1 }},
		{"download", func(s *Settings) { s.Download = MaxBandwidth + 1 }},
		{"upload", func(s *Settings) { s.Upload = -5 }},
		{"qdisc", func(s *Settings) { s.Qdisc = "pfifo" }},
		{"script", func(s *Settings) { s.Script = "custom.qos" }},
		{"linklayer", func(s *Settings) { s.Linklayer = "dsl" }},
		{"overhead", func(s *Settings) { s.Overhead = -1 }},
		{"overhead", func(s *Settings) { s.Overhead = 257 }},
	}
	for _, c := range cases {
		s := base
		c.mod(&s)
		err := Validate(s)
		var fe *FieldError
		if !errors.As(err, &fe) || fe.Field != c.field {
			t.Errorf("field %s: err = %v", c.field, err)
		}
	}
}
