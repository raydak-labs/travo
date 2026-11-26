package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCrontabRoundTrip(t *testing.T) {
	ctx := context.Background()
	d := &Device{CrontabPath: filepath.Join(t.TempDir(), "crontabs", "root")}
	if got, err := d.ReadCrontab(ctx); err != nil || got != "" {
		t.Fatalf("absent: %q %v", got, err)
	}
	if err := d.WriteCrontab(ctx, "0 1 * * * x\n"); err != nil {
		t.Fatal(err)
	}
	if got, _ := d.ReadCrontab(ctx); got != "0 1 * * * x\n" {
		t.Fatalf("%q", got)
	}
	st, _ := os.Stat(d.CrontabPath)
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", st.Mode())
	}
}
