// Package atomicfile writes state files so a crash never leaves a torn file.
package atomicfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Write replaces path via temp file, fsync, rename and directory fsync.
func Write(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	cleanup := func(err error) error {
		_ = tmp.Close()
		_ = os.Remove(name)
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		return cleanup(err)
	}
	if _, err := tmp.Write(data); err != nil {
		return cleanup(err)
	}
	if err := tmp.Sync(); err != nil {
		return cleanup(err)
	}
	if err := tmp.Close(); err != nil {
		return cleanup(err)
	}
	if err := os.Rename(name, path); err != nil {
		return cleanup(err)
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

// WriteJSON marshals v and writes it atomically.
func WriteJSON(path string, v any, perm os.FileMode) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return Write(path, append(b, '\n'), perm)
}

// ReadJSON fails closed: a missing or torn file is an error, never "empty".
// Callers that accept absence must check errors.Is(err, os.ErrNotExist).
func ReadJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("corrupt state file %s: %w", path, err)
	}
	return nil
}
