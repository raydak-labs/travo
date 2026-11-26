// Package epochfile persists the session epoch (edge of internal/auth).
package epochfile

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/blackdark-org/travo-greenfield/internal/atomicfile"
)

type file struct {
	Epoch uint64 `json:"epoch"`
}

// Load returns 0 when the file does not exist yet; a torn file is an error (fail closed).
func Load(path string) (uint64, error) {
	var f file
	if err := atomicfile.ReadJSON(path, &f); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	return f.Epoch, nil
}

// Save writes atomically; callers persist BEFORE replying to the password change.
func Save(path string, epoch uint64) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return atomicfile.WriteJSON(path, file{Epoch: epoch}, 0o600)
}
