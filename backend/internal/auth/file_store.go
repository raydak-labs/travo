package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// AuthConfig is persisted on the router to make password/JWT changes durable.
type AuthConfig struct {
	Version        int    `json:"version"`
	PasswordBcrypt string `json:"password_bcrypt,omitempty"`
	JWTSecret      string `json:"jwt_secret"`
}

// FileAuthStore persists AuthConfig to disk.
type FileAuthStore struct {
	path string
	mu   sync.Mutex
}

func NewFileAuthStore(path string) *FileAuthStore {
	return &FileAuthStore{path: path}
}

// randRead is the entropy source for generated secrets. It is a package
// variable so tests can simulate an entropy failure; production always uses
// crypto/rand.
var randRead = rand.Read

// randomSecretHex returns a fresh 256-bit hex secret. It never falls back to a
// hard-coded value: the secret is the HS256 signing key, so a predictable one
// would let anyone who has read this source forge an admin token. An entropy
// failure is returned as an error so the caller refuses to start instead of
// persisting a known key.
func randomSecretHex() (string, error) {
	b := make([]byte, 32)
	if _, err := randRead(b); err != nil {
		return "", fmt.Errorf("generating jwt secret: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func defaultAuthConfig() (AuthConfig, error) {
	secret, err := randomSecretHex()
	if err != nil {
		return AuthConfig{}, err
	}
	return AuthConfig{
		Version:   1,
		JWTSecret: secret,
	}, nil
}

func (s *FileAuthStore) LoadOrInit() (AuthConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadOrInitLocked()
}

func (s *FileAuthStore) loadOrInitLocked() (AuthConfig, error) {
	data, err := os.ReadFile(s.path)
	if err == nil && len(data) > 0 {
		var cfg AuthConfig
		if uerr := json.Unmarshal(data, &cfg); uerr == nil && cfg.JWTSecret != "" {
			if cfg.Version == 0 {
				cfg.Version = 1
			}
			return cfg, nil
		}
		// fall through to init if unreadable
	}

	cfg, derr := defaultAuthConfig()
	if derr != nil {
		return AuthConfig{}, derr
	}
	if werr := s.writeLocked(cfg); werr != nil {
		return AuthConfig{}, werr
	}
	return cfg, nil
}

func (s *FileAuthStore) writeLocked(cfg AuthConfig) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
