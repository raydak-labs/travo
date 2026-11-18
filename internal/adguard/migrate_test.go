package adguard

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// Seen on the AXT1800 (#164): with 127.0.0.1:3000 taken, every Reconcile tried again and
// restarted AGH twice, a DNS gap each time while AGH is the default DNS.
func TestFailedMigrationBacksOffWhileTheConfigIsUnchanged(t *testing.T) {
	r := newRig(t)
	r.installed(t, readFixture(t))
	now := time.Unix(1_800_000_000, 0)
	r.svc.Now = func() time.Time { return now }
	r.svc.Ready = func(context.Context) error { return errors.New("connection refused") }
	ctx := context.Background()

	if err := r.svc.Reconcile(ctx); err == nil {
		t.Fatal("first try must fail")
	}
	restarts := len(r.dev.ServiceActions)
	if restarts != 2 {
		t.Fatalf("actions = %v, want swap and restore restarts", r.dev.ServiceActions)
	}
	if st, err := r.svc.Status(ctx); err != nil || !st.MigrationFailed {
		t.Fatalf("status must surface the failure: %+v %v", st, err)
	}

	now = now.Add(MigrationBackoff - time.Second)
	if err := r.svc.Reconcile(ctx); err != nil {
		t.Fatalf("held-off reconcile: %v", err)
	}
	if len(r.dev.ServiceActions) != restarts {
		t.Fatalf("held-off reconcile restarted AGH: %v", r.dev.ServiceActions)
	}

	now = now.Add(time.Second)
	r.svc.Ready = func(context.Context) error { return nil }
	if err := r.svc.Reconcile(ctx); err != nil {
		t.Fatalf("retry after the back-off: %v", err)
	}
	if st, _ := r.svc.Status(ctx); st.MigrationFailed || !st.Loopback {
		t.Fatalf("success must clear the failure: %+v", st)
	}
}

// A config fixed by hand after a failed try must not keep reporting the failure.
func TestHandFixedLoopbackConfigClearsTheFailure(t *testing.T) {
	r := newRig(t)
	r.installed(t, readFixture(t))
	r.svc.Ready = func(context.Context) error { return errors.New("connection refused") }
	ctx := context.Background()
	if err := r.svc.Migrate(ctx); err == nil {
		t.Fatal("want failure")
	}
	if err := os.WriteFile(r.svc.Config, Seed, 0o600); err != nil {
		t.Fatal(err)
	}
	if st, _ := r.svc.Status(ctx); st.MigrationFailed {
		t.Fatalf("loopback config still reports a failure: %+v", st)
	}
	if err := r.svc.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if r.svc.migrationFailed() {
		t.Fatal("reconcile must forget the failure once the config is loopback")
	}
}

// Only a completed rollback counts as "failed and undone"; a kept guard is not.
func TestUnfinishedRestoreIsNotReportedAsUndone(t *testing.T) {
	r := newRig(t)
	r.installed(t, readFixture(t))
	r.dev.ServiceErr = errors.New("procd: timeout")
	if err := r.svc.Migrate(context.Background()); err == nil {
		t.Fatal("want error")
	}
	if r.svc.migrationFailed() {
		t.Fatal("a restore that kept the guard must not be recorded as rolled back")
	}
}

func TestEditedConfigIsMigratedAgainAtOnce(t *testing.T) {
	r := newRig(t)
	r.installed(t, readFixture(t))
	r.svc.Ready = func(context.Context) error { return errors.New("connection refused") }
	ctx := context.Background()
	if err := r.svc.Migrate(ctx); err == nil {
		t.Fatal("want failure")
	}
	edited := append(readFixture(t), []byte("# operator edit\n")...)
	if err := os.WriteFile(r.svc.Config, edited, 0o600); err != nil {
		t.Fatal(err)
	}
	r.svc.Ready = func(context.Context) error { return nil }
	if err := r.svc.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if st, _ := r.svc.Status(ctx); st.MigrationFailed || !st.Loopback {
		t.Fatalf("status = %+v", st)
	}
}
