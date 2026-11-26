package cli

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/blackdark-org/travo-greenfield/internal/atomicfile"
)

// authorizedKeysPath is root's dropbear key file (invariants §3: the only file written).
const authorizedKeysPath = "/etc/dropbear/authorized_keys"

func (d *Device) authorizedKeys() string {
	if d.AuthorizedKeysPath != "" {
		return d.AuthorizedKeysPath
	}
	return authorizedKeysPath
}

// ReadAuthorizedKeys returns nil for an absent file.
func (d *Device) ReadAuthorizedKeys(context.Context) ([]byte, error) {
	b, err := os.ReadFile(d.authorizedKeys())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return b, err
}

// WriteAuthorizedKeys replaces the file atomically with mode 0600 in a 0700 directory;
// dropbear refuses keys from a group- or world-writable file.
func (d *Device) WriteAuthorizedKeys(_ context.Context, data []byte) error {
	path := d.authorizedKeys()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return atomicfile.Write(path, data, 0o600)
}
