//go:build integration

package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The stub never restarts the container: it only records that the delayed command ran.
func TestRealRebootReturnsBeforeDelay(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "reboot-fired")
	stub := "#!/bin/sh\nprintf fired > \"$REBOOT_TEST_MARKER\"\n"
	if err := os.WriteFile(filepath.Join(dir, "reboot"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("REBOOT_TEST_MARKER", marker)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	start := time.Now()
	if err := New().Reboot(context.Background()); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed >= time.Second {
		t.Fatalf("scheduling blocked for %v", elapsed)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, err := os.Stat(marker)
		if err == nil {
			return
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			t.Fatal("delayed reboot stub did not run within 5s")
		}
		time.Sleep(50 * time.Millisecond)
	}
}
