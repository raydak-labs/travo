//go:build integration

package updateapply

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
)

func realService(version string, stage Stage) *Service {
	return New(guard.Dir{Path: "/etc/travo-greenfield-guards", Now: time.Now}, cli.New(), stage, version)
}

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "--update-worker" {
		if err := realService("", nil).Work(len(os.Args) > 2 && os.Args[2] == "--update-await-ack"); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type integrationStage string

func (s integrationStage) WithVerified(_ string, fn func(string) error) error { return fn(string(s)) }

func TestRealApply(t *testing.T) {
	next := os.Getenv("TRAVO_APPLY_FIXTURE")
	before := os.Getenv("TRAVO_APPLY_PREVIOUS")
	if next == "" || before == "" {
		t.Skip("requires booted package suite")
	}
	s := realService(before, integrationStage(next))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	accepted, err := s.Begin(ctx, "1.2.345")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Begin(ctx, "1.2.345"); !errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
	if status := s.Status(); status.State != "applying" || status.ID != accepted.Status.ID {
		t.Fatal(status)
	}
	accepted.Finish(true)
	await(t, s, "succeeded")
	if err := s.waitHealthy("http://127.0.0.1:8097/api/health", "1.2.345"); err != nil {
		t.Fatal(err)
	}
	if _, err := regular(s.path(".old", "binary")); err != nil {
		t.Fatal(err)
	}
	previous, err := os.ReadFile(s.path(".old", "manifest"))
	if err != nil {
		t.Fatal(err)
	}

	s.version = "1.2.345"
	accepted, err = s.Begin(ctx, "1.2.346") // Same executable, deliberately different expected version.
	if err != nil {
		t.Fatal(err)
	}
	accepted.Finish(true)
	await(t, s, "rolled_back")
	if err := s.waitHealthy("http://127.0.0.1:8097/api/health", "1.2.345"); err != nil {
		t.Fatal(err)
	}
	if retained, err := os.ReadFile(s.path(".old", "manifest")); err != nil || string(retained) != string(previous) {
		t.Fatal("previous generation changed on rollback", err)
	}

	if err := s.prepare(next, os.Args[0]); err != nil {
		t.Fatal(err)
	}
	j := journal{ApplyStatus: ApplyStatus{ID: strings.Repeat("b", 32), Version: "1.2.346", State: "applying"}, Phase: "switching", PreviousVersion: s.version, Endpoint: "http://127.0.0.1:8097/api/health"}
	if err := s.save(j); err != nil {
		t.Fatal(err)
	}
	if err := s.guards.Acquire(GuardName); err != nil {
		t.Fatal(err)
	}
	if err := s.quiesce(ctx, &j); err != nil {
		t.Fatal(err)
	}
	if err := installFile(s.path("next", "binary"), s.binary, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Recover(); err != nil {
		t.Fatal(err)
	}
	await(t, s, "rolled_back")
	if err := s.waitHealthy(j.Endpoint, j.PreviousVersion); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.ui, "index.html")); err != nil {
		t.Fatal(err)
	}
}

func await(t *testing.T, s *Service, want string) {
	t.Helper()
	deadline := time.Now().Add(75 * time.Second)
	for time.Now().Before(deadline) {
		state := s.Status().State
		guarded, err := s.guarded()
		if err != nil {
			t.Fatal(err)
		}
		if state == "recovery_required" {
			t.Fatal(state)
		}
		if state == want && !guarded {
			lock, err := s.acquire()
			if err == nil {
				lock.Close()
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("operation did not finish", s.Status())
}
