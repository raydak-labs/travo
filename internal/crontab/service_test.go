package crontab

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
)

type directApply struct{ calls int }

func (a *directApply) Apply(ctx context.Context, configs []string, stage func(context.Context) error) error {
	a.calls++
	return stage(ctx)
}

func newService(t *testing.T, loglevel string) (*Service, *mock.Device, *directApply) {
	t.Helper()
	dev := mock.New()
	opts := map[string]string{"hostname": "OpenWrt"}
	if loglevel != "" {
		opts["cronloglevel"] = loglevel
	}
	if err := dev.Load(context.Background(), "system", []device.Section{{Name: "cfg01e48a", Type: "system", Options: opts}}); err != nil {
		t.Fatal(err)
	}
	dev.SetService("cron", true, false)
	apply := &directApply{}
	return &Service{File: dev, UCI: dev, Apply: apply, Services: dev}, dev, apply
}

func TestSetWritesBlockLogLevelAndRestartsCron(t *testing.T) {
	ctx := context.Background()
	s, dev, apply := newService(t, "")
	_ = dev.WriteCrontab(ctx, foreign)
	if err := s.Set(ctx, "leds", []string{"0 23 * * * a"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(ctx, "leds"); !slices.Equal(got, []string{"0 23 * * * a"}) {
		t.Fatalf("%q", got)
	}
	secs, _ := dev.Show(ctx, "system")
	if secs[0].Options["cronloglevel"] != "9" || apply.calls != 1 {
		t.Fatalf("loglevel %q calls %d", secs[0].Options["cronloglevel"], apply.calls)
	}
	if !slices.Equal(dev.ServiceActions, []string{"cron restart"}) {
		t.Fatalf("%q", dev.ServiceActions)
	}
	// same lines again: no write, no restart
	if err := s.Set(ctx, "leds", []string{"0 23 * * * a"}); err != nil {
		t.Fatal(err)
	}
	if len(dev.ServiceActions) != 1 {
		t.Fatalf("%q", dev.ServiceActions)
	}
}

func TestSetKeepsTheOperatorsLogLevel(t *testing.T) {
	s, _, apply := newService(t, "5")
	if err := s.Set(context.Background(), "leds", []string{"0 23 * * * a"}); err != nil {
		t.Fatal(err)
	}
	if apply.calls != 0 {
		t.Fatal("rewrote the operator's cronloglevel")
	}
}

func TestSetFailsOnUnreadableCrontab(t *testing.T) {
	s, dev, _ := newService(t, "9")
	dev.CrontabErr = errors.New("eio")
	if err := s.Set(context.Background(), "leds", []string{"0 23 * * * a"}); err == nil {
		t.Fatal("no error")
	}
	if len(dev.ServiceActions) != 0 {
		t.Fatal("restarted cron")
	}
}
