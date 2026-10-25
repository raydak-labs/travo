package platform

import (
	"os"
)

// WriteFileAtomic writes via a sibling temp file and renames, so a concurrent
// reader never observes a truncated file.
//
// os.WriteFile truncates before writing, and /etc/crontabs/root is read by a
// running busybox crond. A reader that catches the window sees a partial
// crontab and can stop scheduling — the same defect class that produced
// "unexpected end of JSON input" on the port-forward store.
func WriteFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
