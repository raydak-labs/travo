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

func TestLoadConfig_Defaults(t *testing.T) {
	// No env vars, no CLI flags → defaults applied
	clearConfigEnv(t)

	cfg, showVersion, err := LoadConfig([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if showVersion {
		t.Error("expected showVersion to be false")
	}
	if cfg.Port != 3000 {
		t.Errorf("expected default port 3000, got %d", cfg.Port)
	}
	if cfg.MockMode {
		t.Error("expected MockMode to be false")
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
	if cfg.CorsOrigins != "*" {
		t.Errorf("expected CorsOrigins '*', got %q", cfg.CorsOrigins)
	}
}

func TestLoadConfig_EnvOverridesDefaults(t *testing.T) {
	// Env vars should override defaults
	t.Setenv("PORT", "8080")
	t.Setenv("MOCK_MODE", "true")
	t.Setenv("PASSWORD", "secret123")
	t.Setenv("JWT_SECRET", "mysecret")
	t.Setenv("STATIC_DIR", "/var/www")
	t.Setenv("CORS_ORIGINS", "http://localhost")

	cfg, showVersion, err := LoadConfig([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if showVersion {
		t.Error("expected showVersion to be false")
	}
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
	if cfg.CorsOrigins != "http://localhost" {
		t.Errorf("expected CorsOrigins 'http://localhost', got %q", cfg.CorsOrigins)
	}
}

func TestLoadConfig_FlagsOverrideEnv(t *testing.T) {
	// CLI flags should override env vars
	t.Setenv("PORT", "8080")
	t.Setenv("PASSWORD", "envpassword")
	t.Setenv("JWT_SECRET", "envsecret")
	t.Setenv("STATIC_DIR", "/env/www")
	t.Setenv("CORS_ORIGINS", "http://env")

	args := []string{
		"--port", "9090",
		"--password", "flagpassword",
		"--jwt-secret", "flagsecret",
		"--static-dir", "/flag/www",
		"--cors-origins", "http://flag",
		"--mock",
	}

	cfg, showVersion, err := LoadConfig(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if showVersion {
		t.Error("expected showVersion to be false")
	}
	if cfg.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Port)
	}
	if !cfg.MockMode {
		t.Error("expected MockMode to be true")
	}
	if cfg.Password != "flagpassword" {
		t.Errorf("expected password 'flagpassword', got %q", cfg.Password)
	}
	if cfg.JWTSecret != "flagsecret" {
		t.Errorf("expected JWTSecret 'flagsecret', got %q", cfg.JWTSecret)
	}
	if cfg.StaticDir != "/flag/www" {
		t.Errorf("expected StaticDir '/flag/www', got %q", cfg.StaticDir)
	}
	if cfg.CorsOrigins != "http://flag" {
		t.Errorf("expected CorsOrigins 'http://flag', got %q", cfg.CorsOrigins)
	}
}

func TestLoadConfig_VersionFlag(t *testing.T) {
	clearConfigEnv(t)

	_, showVersion, err := LoadConfig([]string{"--version"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !showVersion {
		t.Error("expected showVersion to be true when --version is passed")
	}
}

func TestLoadConfig_FlagsPartialOverride(t *testing.T) {
	// Only some flags set — env should fill the rest
	t.Setenv("PORT", "8080")
	t.Setenv("PASSWORD", "envpassword")

	args := []string{"--port", "9090"}

	cfg, _, err := LoadConfig(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 9090 {
		t.Errorf("expected port 9090 (flag), got %d", cfg.Port)
	}
	if cfg.Password != "envpassword" {
		t.Errorf("expected password 'envpassword' (env), got %q", cfg.Password)
	}
}

// clearConfigEnv unsets all config-related env vars for a clean test.
func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"PORT", "MOCK_MODE", "PASSWORD", "JWT_SECRET", "STATIC_DIR", "CORS_ORIGINS"} {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}
}
