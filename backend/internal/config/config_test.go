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
	if cfg.AuthConfigPath == "" {
		t.Error("expected AuthConfigPath to be non-empty")
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
	if cfg.AuthConfigPath == "" {
		t.Error("expected AuthConfigPath to be non-empty")
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
	t.Setenv("AUTH_CONFIG_PATH", "/tmp/auth.json")
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
	if cfg.AuthConfigPath != "/tmp/auth.json" {
		t.Errorf("expected AuthConfigPath '/tmp/auth.json', got %q", cfg.AuthConfigPath)
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
	t.Setenv("AUTH_CONFIG_PATH", "/env/auth.json")
	t.Setenv("STATIC_DIR", "/env/www")
	t.Setenv("CORS_ORIGINS", "http://env")

	args := []string{
		"--port", "9090",
		"--auth-config-path", "/flag/auth.json",
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
	if cfg.AuthConfigPath != "/flag/auth.json" {
		t.Errorf("expected AuthConfigPath '/flag/auth.json', got %q", cfg.AuthConfigPath)
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
	t.Setenv("AUTH_CONFIG_PATH", "/env/auth.json")

	args := []string{"--port", "9090"}

	cfg, _, err := LoadConfig(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 9090 {
		t.Errorf("expected port 9090 (flag), got %d", cfg.Port)
	}
	if cfg.AuthConfigPath != "/env/auth.json" {
		t.Errorf("expected AuthConfigPath '/env/auth.json' (env), got %q", cfg.AuthConfigPath)
	}
}

// clearConfigEnv unsets all config-related env vars for a clean test.
func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"PORT", "MOCK_MODE", "AUTH_CONFIG_PATH", "STATIC_DIR", "CORS_ORIGINS"} {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}
}
