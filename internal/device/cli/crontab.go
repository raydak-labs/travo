package cli

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/blackdark-org/travo-greenfield/internal/atomicfile"
)

// crontabPath is root's BusyBox crontab; /etc/cron.d does not exist (device-facts §8).
const crontabPath = "/etc/crontabs/root"

func (d *Device) crontab() string {
	if d.CrontabPath != "" {
		return d.CrontabPath
	}
	return crontabPath
}

// ReadCrontab returns "" for an absent file.
func (d *Device) ReadCrontab(context.Context) (string, error) {
	b, err := os.ReadFile(d.crontab())
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	return string(b), err
}

// WriteCrontab replaces the file atomically (mode 0600, like `crontab -e`).
func (d *Device) WriteCrontab(_ context.Context, content string) error {
	path := d.crontab()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return atomicfile.Write(path, []byte(content), 0o600)
}
