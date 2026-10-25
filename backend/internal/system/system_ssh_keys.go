package system

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

const authorizedKeysFile = "/etc/dropbear/authorized_keys"

// GetSSHKeys returns all public keys from the authorized_keys file.
func (s *SystemService) GetSSHKeys() (models.SSHKeysResponse, error) {
	data, err := os.ReadFile(s.sshKeysPath())
	if err != nil {
		if os.IsNotExist(err) {
			return models.SSHKeysResponse{Keys: []models.SSHKey{}}, nil
		}
		return models.SSHKeysResponse{}, err
	}
	var keys []models.SSHKey
	for i, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		comment := ""
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			comment = strings.Join(parts[2:], " ")
		}
		keys = append(keys, models.SSHKey{Index: i, Comment: comment, Key: line})
	}
	if keys == nil {
		keys = []models.SSHKey{}
	}
	return models.SSHKeysResponse{Keys: keys}, nil
}

// AddSSHKey appends a public key to the authorized_keys file.
func (s *SystemService) AddSSHKey(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("key must not be empty")
	}
	// The key is appended verbatim to authorized_keys: a newline would add
	// extra authorized_keys lines (i.e. grant access to another key).
	if strings.ContainsAny(key, "\n\r") {
		return fmt.Errorf("key must be a single line")
	}
	if !sshKeyRe.MatchString(key) {
		return fmt.Errorf("invalid SSH public key: expected [type] [base64] [comment]")
	}
	if err := os.MkdirAll(filepath.Dir(s.sshKeysPath()), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(s.sshKeysPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = fmt.Fprintln(f, key)
	return err
}

// DeleteSSHKey removes the key at the given line index from authorized_keys.
// The index space is byte-for-byte the same one GetSSHKeys reports (both split
// the trimmed file content on "\n"), so the number a client saw always maps to
// the same line. Negative and out-of-range indexes are rejected instead of
// silently truncating the file.
func (s *SystemService) DeleteSSHKey(index int) error {
	path := s.sshKeysPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if index < 0 || index >= len(lines) {
		return fmt.Errorf("key index %d out of range (file has %d line(s))", index, len(lines))
	}
	lines = append(lines[:index], lines[index+1:]...)
	if len(lines) == 0 {
		return os.WriteFile(path, nil, 0600)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0600)
}
