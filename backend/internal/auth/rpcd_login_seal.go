package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/crypto/hkdf"
)

const rpcdSealVersion byte = 1

// rpcdLoginSealPath returns the path for the sealed rpcd login password file
// (same directory as auth.json). Not world-readable; root-only on device.
func rpcdLoginSealPath(authConfigPath string) string {
	return filepath.Join(filepath.Dir(authConfigPath), "rpcd-login.sealed")
}

func deriveRPCDSealKey(jwtSecret string) ([]byte, error) {
	salt := []byte("travo-rpcd-login-seal-v1")
	r := hkdf.New(sha256.New, []byte(jwtSecret), salt, nil)
	key := make([]byte, 32)
	if _, err := io.ReadFull(r, key); err != nil {
		return nil, err
	}
	return key, nil
}

func sealRPCDLoginPassword(jwtSecret, password string) ([]byte, error) {
	key, err := deriveRPCDSealKey(jwtSecret)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	nonce := make([]byte, ns)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	ct := gcm.Seal(nil, nonce, []byte(password), nil)
	out := make([]byte, 1+len(nonce)+len(ct))
	out[0] = rpcdSealVersion
	copy(out[1:], nonce)
	copy(out[1+len(nonce):], ct)
	return out, nil
}

func unsealRPCDLoginPassword(jwtSecret string, sealed []byte) ([]byte, error) {
	if len(sealed) < 2 {
		return nil, errors.New("sealed blob too short")
	}
	if sealed[0] != rpcdSealVersion {
		return nil, fmt.Errorf("unknown seal version %d", sealed[0])
	}
	key, err := deriveRPCDSealKey(jwtSecret)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(sealed) < 1+ns {
		return nil, errors.New("sealed blob too short")
	}
	nonce := sealed[1 : 1+ns]
	ct := sealed[1+ns:]
	return gcm.Open(nil, nonce, ct, nil)
}

// SaveSealedRPCDPassword writes the root password sealed with a key derived from jwtSecret.
// authConfigPath is the path to auth.json; the seal file lives in the same directory.
// Must not be world-readable (0600). No-op if authConfigPath is empty.
func SaveSealedRPCDPassword(authConfigPath, jwtSecret, password string) error {
	if authConfigPath == "" {
		return nil
	}
	sealed, err := sealRPCDLoginPassword(jwtSecret, password)
	if err != nil {
		return err
	}
	dir := filepath.Dir(authConfigPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path := rpcdLoginSealPath(authConfigPath)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, sealed, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadSealedRPCDPassword reads and decrypts the seal file. Returns empty string if
// the file is missing, unreadable, or decryption fails (e.g. JWT secret rotated).
func LoadSealedRPCDPassword(authConfigPath, jwtSecret string) string {
	if authConfigPath == "" {
		return ""
	}
	data, err := os.ReadFile(rpcdLoginSealPath(authConfigPath))
	if err != nil {
		return ""
	}
	plain, err := unsealRPCDLoginPassword(jwtSecret, data)
	if err != nil {
		return ""
	}
	return string(plain)
}
