package config

import (
	"crypto/rand"
	"encoding/hex"
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

// LoadConfigFromEnv reads configuration from environment variables with defaults.
func LoadConfigFromEnv() Config {
	cfg := DefaultConfig()

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

	return cfg
}

func generateRandomSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "default-jwt-secret-change-me"
	}
	return hex.EncodeToString(b)
}
