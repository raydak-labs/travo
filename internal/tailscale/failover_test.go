package tailscale

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func (r *rig) failover(on bool, err error) *sync.Mutex {
	mu := new(sync.Mutex)
	r.svc.Exclusive = mu
	r.svc.FailoverEnabled = func() (bool, error) { return on, err }
	return mu
}

func TestInstallRefusedWhileFailoverIsEnabled(t *testing.T) {
	r := newRig(t)
	r.failover(true, nil)
	if err := r.svc.Install(context.Background()); !errors.Is(err, ErrFailover) {
		t.Fatalf("err = %v", err)
	}
	if v, _ := r.pkgs.Version(context.Background(), Package); v != "" || r.held(t, InstallGuard) || len(r.dev.ServiceActions) != 0 {
		t.Fatalf("refusal wrote: version %q, actions %v", v, r.dev.ServiceActions)
	}
	r.failover(false, nil)
	if err := r.svc.Install(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestLoginRefusedWhileFailoverIsEnabled(t *testing.T) {
	r := newRig(t)
	r.installed(t)
	r.failover(true, nil)
	if _, err := r.svc.Login(context.Background()); !errors.Is(err, ErrFailover) {
		t.Fatalf("err = %v", err)
	}
	if len(r.cli.Calls) != 0 || r.held(t, ChangeGuard) {
		t.Fatalf("refusal wrote: %v", r.cli.Calls)
	}
	if _, err := r.svc.Logout(context.Background()); err != nil {
		t.Fatalf("logout stays allowed: %v", err)
	}
}

func TestFailoverReadErrorRefuses(t *testing.T) {
	r := newRig(t)
	r.failover(false, errors.New("failover.json: unexpected end of JSON input"))
	if err := r.svc.Install(context.Background()); err == nil || errors.Is(err, ErrFailover) {
		t.Fatalf("err = %v", err)
	}
}

func TestInstallWaitsForAFailoverEnableHoldingTheLock(t *testing.T) {
	r := newRig(t)
	mu := r.failover(false, nil)
	on := false
	r.svc.FailoverEnabled = func() (bool, error) { return on, nil }
	mu.Lock() // a failover enable in flight
	done := make(chan error, 1)
	go func() { done <- r.svc.Install(context.Background()) }()
	select {
	case err := <-done:
		t.Fatalf("install ran past the lock: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	on = true // the enable saved its settings
	mu.Unlock()
	if err := <-done; !errors.Is(err, ErrFailover) {
		t.Fatalf("err = %v", err)
	}
}
