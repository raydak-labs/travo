package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
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

// RPCDLoginHelperPath returns the path of the rpcd login argument that the
// generated wireless toggle helper passes to `ubus call session login`. It sits
// beside the sealed blob and auth.json, all root-only (0600/0700).
//
// Why a helper file exists at all: the toggle helper runs from cron and from
// the button hotplug script, with no travo process and no way to reach the
// in-memory RootPassword holder. rpcd's uci apply/confirm are session-scoped,
// so the helper must log in first, and rpcd compares a plaintext password. The
// sealed blob cannot help: unsealing needs the jwtSecret, which lives in
// auth.json in the same root-only directory, so the seal is not a barrier to
// anything that could read this plaintext file in the first place.
//
// It holds the whole JSON argument, already encoded, rather than the bare
// password: the helper then passes it to ubus byte for byte. Escaping in the
// shell instead would mean reimplementing JSON string escaping in sed, and that
// is not portable — BSD sed and BusyBox sed disagree on `s/\\/\\\\/g`, so the
// same script would escape a password containing a backslash differently
// depending on what ran it.
//
// Exported because the services package generates the helper that reads it;
// TestRPcdLoginHelperPathMatchesAuth keeps the two in agreement.
func RPCDLoginHelperPath(authConfigPath string) string {
	return filepath.Join(filepath.Dir(authConfigPath), "rpcd-login.json")
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

// BuildRPCDLoginArg returns the exact `ubus call session login` argument for
// the system root account, JSON-encoded by encoding/json. The backend uses it
// for its own in-process ubus calls and the generated toggle helper uses the
// bytes on disk, so the two cannot disagree about escaping.
//
// A struct, not a map: json.Marshal sorts map keys, so a map would emit
// "password" before "username" and the helper's bytes would stop matching the
// script's own literal, for no benefit.
func BuildRPCDLoginArg(password string) (string, error) {
	arg, err := json.Marshal(struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{Username: "root", Password: password})
	if err != nil {
		return "", err
	}
	return string(arg), nil
}

// SaveRPCDLoginHelper writes the `ubus call session login` argument for the
// generated wireless toggle helper, root-only (0600), written atomically.
// Callers must go through SaveSealedRPCDPassword so the two cannot drift.
func SaveRPCDLoginHelper(authConfigPath, password string) error {
	if authConfigPath == "" || password == "" {
		return nil
	}
	// encoding/json, not string concatenation: see BuildRPCDLoginArg.
	arg, err := BuildRPCDLoginArg(password)
	if err != nil {
		return err
	}
	dir := filepath.Dir(authConfigPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path := RPCDLoginHelperPath(authConfigPath)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(arg+"\n"), 0o600); err != nil {
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
