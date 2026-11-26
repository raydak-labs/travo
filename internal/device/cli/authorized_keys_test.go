package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestAuthorizedKeysFileModes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dropbear", "authorized_keys")
	d := &Device{AuthorizedKeysPath: path}
	ctx := context.Background()
	if b, err := d.ReadAuthorizedKeys(ctx); err != nil || b != nil {
		t.Fatalf("absent file: %q %v", b, err)
	}
	if err := d.WriteAuthorizedKeys(ctx, []byte("ssh-ed25519 AAAA x\n")); err != nil {
		t.Fatal(err)
	}
	b, err := d.ReadAuthorizedKeys(ctx)
	if err != nil || string(b) != "ssh-ed25519 AAAA x\n" {
		t.Fatalf("%q %v", b, err)
	}
	fi, _ := os.Stat(path)
	di, _ := os.Stat(filepath.Dir(path))
	if fi.Mode().Perm() != 0o600 || di.Mode().Perm() != 0o700 {
		t.Fatalf("file %v dir %v", fi.Mode().Perm(), di.Mode().Perm())
	}
}
