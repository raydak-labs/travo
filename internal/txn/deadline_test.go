package txn

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/uciwrite"
)

type deadlineDevice struct {
	device.Device
	commit func(context.Context, string) error
	reload func(context.Context) error
}

func (d deadlineDevice) Commit(ctx context.Context, cfg string) error { return d.commit(ctx, cfg) }
func (d deadlineDevice) Reload(ctx context.Context) error             { return d.reload(ctx) }

func TestBeginApplyDeadlineRollsBack(t *testing.T) {
	adapterErr := errors.New("command interrupted")
	for _, phase := range []string{"commit", "reload", "probe"} {
		t.Run(phase, func(t *testing.T) {
			m, d := setup(t, MinWindow)
			// Starting the watchdog and syncing its PID are part of the live budget.
			// Give them room on loaded CI hosts so this test reaches its target phase.
			m.ApplyBudget = 2 * time.Second
			stopAtDeadline := func(ctx context.Context) error {
				if _, ok := ctx.Deadline(); !ok {
					return errors.New("missing apply deadline")
				}
				<-ctx.Done()
				// Commit and probe can return success just as the deadline expires.
				if phase == "reload" {
					return adapterErr
				}
				return nil
			}
			var calls []string
			var deadline time.Time
			record := func(phase string, ctx context.Context) {
				t.Helper()
				calls = append(calls, phase)
				got, ok := ctx.Deadline()
				if !ok || (!deadline.IsZero() && got != deadline) {
					t.Errorf("%s did not receive the shared apply deadline: %v", phase, got)
				}
				if !slices.Equal(uciwrite.Held(ctx), []string{"network"}) {
					t.Errorf("%s lost the transaction's lock context", phase)
				}
				deadline = got
			}
			m.Device = deadlineDevice{Device: d,
				commit: func(ctx context.Context, cfg string) error {
					record("commit", ctx)
					if phase == "commit" {
						return stopAtDeadline(ctx)
					}
					return d.Commit(ctx, cfg)
				},
				reload: func(ctx context.Context) error {
					record("reload", ctx)
					if phase == "reload" {
						return stopAtDeadline(ctx)
					}
					return d.Reload(ctx)
				},
			}
			stage := func(ctx context.Context) error {
				if _, ok := ctx.Deadline(); ok {
					t.Error("apply budget started before staging finished")
				}
				return stageSet(d, "network")(ctx)
			}
			st, err := m.Begin(ctx, "test", []string{"network"}, stage, func(ctx context.Context) error {
				record("probe", ctx)
				return stopAtDeadline(ctx)
			})
			if !errors.Is(err, context.DeadlineExceeded) || st.State != RolledBack {
				t.Fatalf("want deadline error and rolled_back, got %+v, %v", st, err)
			}
			if phase == "reload" && !errors.Is(err, adapterErr) {
				t.Fatalf("lost adapter error: %v", err)
			}
			expected := map[string][]string{
				"commit": {"commit"},
				"reload": {"commit", "reload"},
				"probe":  {"commit", "reload", "probe"},
			}
			if !slices.Equal(calls, expected[phase]) {
				t.Fatalf("calls after deadline: %v", calls)
			}
			if content(t, m, "network") != "original network\n" || guarded(t, m) {
				t.Fatal("deadline left changed config or active guard")
			}
		})
	}
}

func TestBeginCanceledAfterCommitRollsBack(t *testing.T) {
	m, d := setup(t, MinWindow)
	request, cancel := context.WithCancel(ctx)
	defer cancel()
	m.Device = deadlineDevice{Device: d,
		commit: func(ctx context.Context, cfg string) error {
			err := d.Commit(ctx, cfg)
			cancel()
			return err
		},
		reload: func(context.Context) error {
			t.Error("reload started after cancellation")
			return nil
		},
	}
	st, err := m.Begin(request, "test", []string{"network"}, stageSet(d, "network"), nil)
	if !errors.Is(err, context.Canceled) || st.State != RolledBack {
		t.Fatalf("want cancellation and rolled_back, got %+v, %v", st, err)
	}
	if content(t, m, "network") != "original network\n" || guarded(t, m) {
		t.Fatal("cancellation left changed config or active guard")
	}
}
