package auth

import (
	"path/filepath"
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
