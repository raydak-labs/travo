package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeleteSSHKey_RejectsOutOfRangeIndex(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "authorized_keys")
	if err := os.WriteFile(path, []byte("ssh-ed25519 AAAA user1\nssh-rsa BBBB user2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := newSSHKeyTestService(t, path)

	if err := svc.DeleteSSHKey(-1); err == nil {
		t.Error("expected an error for a negative index")
	}
	if err := svc.DeleteSSHKey(2); err == nil {
		t.Error("expected an error for an index past the last line")
	}
	// Index 0 must map to the first key line, matching GetSSHKeys.
	keys, err := svc.GetSSHKeys()
	if err != nil {
		t.Fatalf("GetSSHKeys: %v", err)
	}
	if len(keys.Keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys.Keys))
	}
	if err := svc.DeleteSSHKey(0); err != nil {
		t.Fatalf("DeleteSSHKey(0): %v", err)
	}
	remaining, _ := os.ReadFile(path)
	if strings.Contains(string(remaining), "user1") {
		t.Errorf("expected user1 to be removed, file is %q", remaining)
	}
	if !strings.Contains(string(remaining), "user2") {
		t.Errorf("expected user2 to survive, file is %q", remaining)
	}
}
