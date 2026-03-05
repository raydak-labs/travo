package config

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Port != 3000 {
		t.Errorf("expected default port 3000, got %d", cfg.Port)
	}
	if cfg.MockMode {
		t.Error("expected MockMode to be false by default")
	}
	if cfg.Password != "admin" {
		t.Errorf("expected default password 'admin', got %q", cfg.Password)
	}
	if cfg.JWTSecret == "" {
		t.Error("expected JWTSecret to be non-empty")
	}
	if cfg.StaticDir != "" {
		t.Errorf("expected empty StaticDir, got %q", cfg.StaticDir)
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	os.Setenv("PORT", "8080")
	os.Setenv("MOCK_MODE", "true")
	os.Setenv("PASSWORD", "secret123")
	os.Setenv("JWT_SECRET", "mysecret")
	os.Setenv("STATIC_DIR", "/var/www")
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("MOCK_MODE")
		os.Unsetenv("PASSWORD")
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("STATIC_DIR")
	}()

	cfg := LoadConfigFromEnv()
	if cfg.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Port)
	}
	if !cfg.MockMode {
		t.Error("expected MockMode to be true")
	}
	if cfg.Password != "secret123" {
		t.Errorf("expected password 'secret123', got %q", cfg.Password)
	}
	if cfg.JWTSecret != "mysecret" {
		t.Errorf("expected JWTSecret 'mysecret', got %q", cfg.JWTSecret)
	}
	if cfg.StaticDir != "/var/www" {
		t.Errorf("expected StaticDir '/var/www', got %q", cfg.StaticDir)
	}
}

func TestLoadConfigFromEnvDefaults(t *testing.T) {
	os.Unsetenv("PORT")
	os.Unsetenv("MOCK_MODE")
	os.Unsetenv("PASSWORD")
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("STATIC_DIR")

	cfg := LoadConfigFromEnv()
	if cfg.Port != 3000 {
		t.Errorf("expected default port 3000, got %d", cfg.Port)
	}
	if cfg.MockMode {
		t.Error("expected MockMode to be false")
	}
	if cfg.Password != "admin" {
		t.Errorf("expected default password 'admin', got %q", cfg.Password)
	}
}
