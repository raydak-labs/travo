package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSealRPCDLoginRoundTrip(t *testing.T) {
	secret := "jwt-secret-abc"
	plain := "root-pw-xyz"
	sealed, err := sealRPCDLoginPassword(secret, plain)
	if err != nil {
		t.Fatal(err)
	}
	out, err := unsealRPCDLoginPassword(secret, sealed)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != plain {
		t.Fatalf("got %q want %q", out, plain)
	}
}

func TestUnsealRPCDLoginWrongSecret(t *testing.T) {
	sealed, err := sealRPCDLoginPassword("secret-a", "pw")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unsealRPCDLoginPassword("secret-b", sealed); err == nil {
		t.Fatal("expected decrypt error for wrong JWT secret")
	}
}

func TestSaveLoadSealedRPCDPassword_FileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	authPath := filepath.Join(dir, "auth.json")
	const secret = "jwt1"
	if err := SaveSealedRPCDPassword(authPath, secret, "mypass"); err != nil {
		t.Fatal(err)
	}
	if got := LoadSealedRPCDPassword(authPath, secret); got != "mypass" {
		t.Fatalf("got %q", got)
	}
	if got := LoadSealedRPCDPassword(authPath, "wrong"); got != "" {
		t.Fatalf("wrong secret should yield empty, got %q", got)
	}
}

func TestSaveSealedRPCDPassword_EmptyPathNoOp(t *testing.T) {
	if err := SaveSealedRPCDPassword("", "s", "p"); err != nil {
		t.Fatal(err)
	}
}

// The generated wireless toggle helper logs in to rpcd from cron and hotplug,
// with no travo process to ask, so the login argument has to be on disk. It must
// be exactly what the backend itself would send — a password containing a quote
// or a backslash has to reach rpcd identically on both paths, or a toggle that
// works from the UI silently fails from the schedule.
func TestSaveRPCDLoginHelper_MatchesBackendArgument(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")

	for _, password := range []string{"", "simple", `with"quote`, `back\slash`, "sp ace", `{"nested":"json"}`} {
		if err := SaveRPCDLoginHelper(path, password); err != nil {
			t.Fatalf("SaveRPCDLoginHelper(%q): %v", password, err)
		}
		data, err := os.ReadFile(RPCDLoginHelperPath(path))
		if err != nil {
			if password == "" {
				continue // an empty password writes nothing; the helper falls back
			}
			t.Fatalf("read helper for %q: %v", password, err)
		}
		want, err := BuildRPCDLoginArg(password)
		if err != nil {
			t.Fatalf("BuildRPCDLoginArg(%q): %v", password, err)
		}
		if got := strings.TrimRight(string(data), "\n"); got != want {
			t.Errorf("helper file for %q = %s, want %s", password, got, want)
		}
	}

	info, err := os.Stat(RPCDLoginHelperPath(path))
	if err != nil {
		t.Fatalf("stat helper: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("helper file mode = %o, want 600: it carries the root password", perm)
	}
}
