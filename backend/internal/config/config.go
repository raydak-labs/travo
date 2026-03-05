package config

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all application configuration.
type Config struct {
	Port        int
	MockMode    bool
	Password    string
	JWTSecret   string
	StaticDir   string
	CorsOrigins string
}

// DefaultConfig returns config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Port:        3000,
		MockMode:    false,
		Password:    "admin",
		JWTSecret:   generateRandomSecret(),
		StaticDir:   "",
		CorsOrigins: "*",
	}
}

// LoadConfig reads configuration with priority: defaults < env vars < CLI flags.
// It accepts a slice of CLI arguments (typically os.Args[1:]).
// Returns the config, whether --version was requested, and any error.
func LoadConfig(args []string) (Config, bool, error) {
	cfg := DefaultConfig()

	// Layer 2: environment variables override defaults
	if v := os.Getenv("PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Port = p
		}
	}
	if v := os.Getenv("MOCK_MODE"); v != "" {
		cfg.MockMode = strings.EqualFold(v, "true") || v == "1"
	}
	if v := os.Getenv("PASSWORD"); v != "" {
		cfg.Password = v
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.JWTSecret = v
	}
	if v := os.Getenv("STATIC_DIR"); v != "" {
		cfg.StaticDir = v
	}
	if v := os.Getenv("CORS_ORIGINS"); v != "" {
		cfg.CorsOrigins = v
	}

	// Layer 3: CLI flags override env vars
	fs := flag.NewFlagSet("openwrt-travel-gui", flag.ContinueOnError)

	port := fs.Int("port", cfg.Port, "HTTP listen port")
	mock := fs.Bool("mock", cfg.MockMode, "Enable mock mode")
	password := fs.String("password", cfg.Password, "Admin password")
	jwtSecret := fs.String("jwt-secret", cfg.JWTSecret, "JWT signing secret")
	staticDir := fs.String("static-dir", cfg.StaticDir, "Path to static frontend files")
	corsOrigins := fs.String("cors-origins", cfg.CorsOrigins, "CORS allowed origins")
	showVersion := fs.Bool("version", false, "Print version and exit")

	if err := fs.Parse(args); err != nil {
		return Config{}, false, fmt.Errorf("parsing flags: %w", err)
	}

	cfg.Port = *port
	cfg.MockMode = *mock
	cfg.Password = *password
	cfg.JWTSecret = *jwtSecret
	cfg.StaticDir = *staticDir
	cfg.CorsOrigins = *corsOrigins

	return cfg, *showVersion, nil
}

// LoadConfigFromEnv reads configuration from environment variables with defaults.
// Deprecated: Use LoadConfig instead.
func LoadConfigFromEnv() Config {
	cfg, _, _ := LoadConfig([]string{})
	return cfg
}

func generateRandomSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "default-jwt-secret-change-me"
	}
	return hex.EncodeToString(b)
}
