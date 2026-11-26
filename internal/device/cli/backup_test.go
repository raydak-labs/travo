package cli

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestBackupConfig(t *testing.T) {
	wantErr := errors.New("sysupgrade failed")
	d := &Device{Run: func(ctx context.Context, name string, args ...string) (string, int, error) {
		if name != "sysupgrade" || !reflect.DeepEqual(args, []string{"-b", "/tmp/private/config.tar.gz"}) {
			t.Fatalf("unexpected command %s %v", name, args)
		}
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) <= 0 || time.Until(deadline) > TierWrite {
			t.Fatalf("missing write timeout: %v", deadline)
		}
		return "", 1, wantErr
	}}
	if err := d.BackupConfig(context.Background(), "/tmp/private/config.tar.gz"); !errors.Is(err, wantErr) {
		t.Fatalf("got %v, want %v", err, wantErr)
	}
}

func TestBackupConfigCanceledWhileAnotherRuns(t *testing.T) {
	busy := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	d := &Device{Run: func(context.Context, string, ...string) (string, int, error) {
		close(busy)
		<-release
		return "", 0, nil
	}}
	go func() { done <- d.BackupConfig(context.Background(), "first") }()
	<-busy
	defer func() {
		close(release)
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := d.BackupConfig(ctx, "second"); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}
