package runner

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestCancellationStopsCommandChildren(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  Runner
	}{
		{"Exec", Exec},
		{"RunInput", func(ctx context.Context, name string, args ...string) (string, int, error) {
			return RunInput(ctx, "input\n", name, args...)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			ready, survived := filepath.Join(dir, "ready"), filepath.Join(dir, "survived")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, _, err := tc.run(ctx, "/bin/sh", "-c",
					`(sleep 3; touch "$2"; echo child) & touch "$1"; wait`, "sh", ready, survived)
				done <- err
			}()
			deadline := time.Now().Add(5 * time.Second)
			for {
				if _, err := os.Stat(ready); err == nil {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("command did not start")
				}
				time.Sleep(10 * time.Millisecond)
			}
			cancel()
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("canceled command reported success")
				}
			case <-time.After(2 * commandWaitDelay):
				t.Fatal("canceled command waited for its child's output pipe")
			}
			time.Sleep(3100 * time.Millisecond)
			if _, err := os.Stat(survived); !os.IsNotExist(err) {
				t.Fatalf("child continued after cancellation: %v", err)
			}
		})
	}
}
func TestRunInputSeparatesStderr(t *testing.T) {
	_, code, err := RunInput(context.Background(), "", "sh", "-c", "printf 'first line\\nsecond line\\n' >&2; exit 7")
	if code != 7 || err == nil {
		t.Fatalf("exit code %d, error %v", code, err)
	}
	stderr, ok := Stderr(err)
	if !ok || stderr != "first line\nsecond line" {
		t.Fatalf("stderr %q, %v", stderr, ok)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("command cause lost: %v", err)
	}
}
