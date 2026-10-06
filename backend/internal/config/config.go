package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all application configuration.
type Config struct {
	Port           int
	MockMode       bool
	AuthConfigPath string
	StaticDir      string
	// CorsOrigins is a comma-separated allowlist of browser origins allowed to
	// call the API. Empty (the default) means SAME-ORIGIN ONLY: no
	// Access-Control-Allow-Origin header is emitted, so browsers block
	// cross-origin reads. Set it explicitly (or to "*") to opt in.
	CorsOrigins       string
	AllowedAdminCIDRs string
	// TLS options
	TLSEnabled  bool
	TLSPort     int
	TLSCertFile string
	TLSKeyFile  string
}

// DefaultConfig returns config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Port:           3000,
		MockMode:       false,
		AuthConfigPath: "/etc/travo/auth.json",
		StaticDir:      "",
		// Same-origin by default: the UI is served by this same process, so
		// there is no legitimate cross-origin caller. "*" here would let any
		// page the user visits issue credentialed requests against the router
		// API (browsers attach cookies/session state for the target host).
		CorsOrigins:       "",
		AllowedAdminCIDRs: "",
		TLSEnabled:        false,
		TLSPort:           443,
		TLSCertFile:       "/etc/travo/tls.crt",
		TLSKeyFile:        "/etc/travo/tls.key",
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
	if v := os.Getenv("AUTH_CONFIG_PATH"); v != "" {
		cfg.AuthConfigPath = v
	}
	if v := os.Getenv("STATIC_DIR"); v != "" {
		cfg.StaticDir = v
	}
	// Empty CORS_ORIGINS is treated as "unset" (same-origin only), so the
	// default stays secure when the variable is present but blank.
	if v := os.Getenv("CORS_ORIGINS"); strings.TrimSpace(v) != "" {
		cfg.CorsOrigins = v
	}
	if v := os.Getenv("ALLOWED_ADMIN_CIDRS"); v != "" {
		cfg.AllowedAdminCIDRs = v
	}
	if v := os.Getenv("TLS_ENABLED"); v != "" {
		cfg.TLSEnabled = strings.EqualFold(v, "true") || v == "1"
	}
	if v := os.Getenv("TLS_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.TLSPort = p
		}
	}
	if v := os.Getenv("TLS_CERT"); v != "" {
		cfg.TLSCertFile = v
	}
	if v := os.Getenv("TLS_KEY"); v != "" {
		cfg.TLSKeyFile = v
	}

	// Layer 3: CLI flags override env vars
	fs := flag.NewFlagSet("travo", flag.ContinueOnError)

	port := fs.Int("port", cfg.Port, "HTTP listen port")
	mock := fs.Bool("mock", cfg.MockMode, "Enable mock mode")
	authConfigPath := fs.String("auth-config-path", cfg.AuthConfigPath, "Path to auth config file (stores password hash + JWT secret)")
	staticDir := fs.String("static-dir", cfg.StaticDir, "Path to static frontend files")
	corsOrigins := fs.String("cors-origins", cfg.CorsOrigins, "CORS allowed origins (comma-separated; empty = same-origin only)")
	allowedCIDRs := fs.String("allowed-admin-cidrs", cfg.AllowedAdminCIDRs, "Comma-separated admin IP/CIDR allowlist (empty disables)")
	showVersion := fs.Bool("version", false, "Print version and exit")
	tlsEnabled := fs.Bool("tls", cfg.TLSEnabled, "Enable HTTPS/TLS listener")
	tlsPort := fs.Int("tls-port", cfg.TLSPort, "HTTPS listen port")
	tlsCert := fs.String("tls-cert", cfg.TLSCertFile, "TLS certificate file path")
	tlsKey := fs.String("tls-key", cfg.TLSKeyFile, "TLS private key file path")

	if err := fs.Parse(args); err != nil {
		return Config{}, false, fmt.Errorf("parsing flags: %w", err)
	}

	cfg.Port = *port
	cfg.MockMode = *mock
	cfg.AuthConfigPath = *authConfigPath
	cfg.StaticDir = *staticDir
	cfg.CorsOrigins = *corsOrigins
	cfg.AllowedAdminCIDRs = *allowedCIDRs
	cfg.TLSEnabled = *tlsEnabled
	cfg.TLSPort = *tlsPort
	cfg.TLSCertFile = *tlsCert
	cfg.TLSKeyFile = *tlsKey

	return cfg, *showVersion, nil
}
