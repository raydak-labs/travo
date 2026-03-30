package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

// AuthConfig is persisted on the router to make password/JWT changes durable.
type AuthConfig struct {
	Version        int    `json:"version"`
	PasswordBcrypt string `json:"password_bcrypt"`
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

func randomSecretHex() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "default-jwt-secret-change-me"
	}
	return hex.EncodeToString(b)
}

func defaultAuthConfig() (AuthConfig, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	if err != nil {
		return AuthConfig{}, err
	}
	return AuthConfig{
		Version:        1,
		PasswordBcrypt: string(hash),
		JWTSecret:      randomSecretHex(),
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
		if uerr := json.Unmarshal(data, &cfg); uerr == nil && cfg.PasswordBcrypt != "" && cfg.JWTSecret != "" {
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

func (s *FileAuthStore) SavePasswordHash(passwordBcrypt string) error {
	if passwordBcrypt == "" {
		return fmt.Errorf("password_bcrypt must not be empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg, err := s.loadOrInitLocked()
	if err != nil {
		return err
	}
	cfg.PasswordBcrypt = passwordBcrypt
	return s.writeLocked(cfg)
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
