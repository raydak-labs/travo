package auth

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRandomSecretHex(t *testing.T) {
	secret, err := randomSecretHex()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(secret) != 64 {
		t.Errorf("expected 64 hex chars, got %d (%q)", len(secret), secret)
	}
	other, err := randomSecretHex()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secret == other {
		t.Error("expected two distinct secrets")
	}
}

// An entropy failure must surface as an error: falling back to a hard-coded
// key would persist a publicly known HS256 signing secret and let anyone
// forge an admin token.
func TestRandomSecretHex_EntropyFailureIsAnError(t *testing.T) {
	orig := randRead
	t.Cleanup(func() { randRead = orig })
	randRead = func(b []byte) (int, error) { return 0, errors.New("no entropy available") }

	secret, err := randomSecretHex()
	if err == nil {
		t.Fatalf("expected an error instead of a secret, got %q", secret)
	}
	if secret != "" {
		t.Errorf("expected no secret on failure, got %q", secret)
	}
}

func TestDefaultAuthConfig_EntropyFailurePropagates(t *testing.T) {
	orig := randRead
	t.Cleanup(func() { randRead = orig })
	randRead = func(b []byte) (int, error) { return 0, errors.New("no entropy available") }

	cfg, err := defaultAuthConfig()
	if err == nil {
		t.Fatalf("expected an error, got config %+v", cfg)
	}
	if cfg.JWTSecret != "" {
		t.Errorf("expected no secret on failure, got %q", cfg.JWTSecret)
	}
}

// Nothing may be written to disk when the secret could not be generated —
// otherwise a restart would later read back a known/empty key.
func TestFileAuthStore_LoadOrInit_EntropyFailureWritesNothing(t *testing.T) {
	orig := randRead
	t.Cleanup(func() { randRead = orig })
	randRead = func(b []byte) (int, error) { return 0, errors.New("no entropy available") }

	path := filepath.Join(t.TempDir(), "auth.json")
	store := NewFileAuthStore(path)

	if _, err := store.LoadOrInit(); err == nil {
		t.Fatal("expected LoadOrInit to fail when entropy is unavailable")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected no auth.json to be persisted, stat err = %v", err)
	}
}

func TestFileAuthStore_LoadOrInit_PersistsAndReuses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	store := NewFileAuthStore(path)

	cfg, err := store.LoadOrInit()
	if err != nil {
		t.Fatalf("LoadOrInit: %v", err)
	}
	if cfg.JWTSecret == "" || cfg.Version != 1 {
		t.Fatalf("unexpected config: %+v", cfg)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading auth.json: %v", err)
	}
	var onDisk AuthConfig
	if err := json.Unmarshal(data, &onDisk); err != nil {
		t.Fatalf("auth.json is not valid JSON: %v", err)
	}
	if strings.Contains(string(data), "default-jwt-secret") {
		t.Error("auth.json contains the old hard-coded fallback secret")
	}

	again, err := store.LoadOrInit()
	if err != nil {
		t.Fatalf("second LoadOrInit: %v", err)
	}
	if again.JWTSecret != cfg.JWTSecret {
		t.Error("expected the persisted secret to be reused")
	}
}

// A corrupt or secret-less file must be re-initialized, not accepted.
func TestFileAuthStore_LoadOrInit_ReinitializesGarbage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"jwt_secret":""}`), 0o600); err != nil {
		t.Fatalf("seeding auth.json: %v", err)
	}

	cfg, err := NewFileAuthStore(path).LoadOrInit()
	if err != nil {
		t.Fatalf("LoadOrInit: %v", err)
	}
	if cfg.JWTSecret == "" {
		t.Error("expected a fresh secret for a config without one")
	}
}
