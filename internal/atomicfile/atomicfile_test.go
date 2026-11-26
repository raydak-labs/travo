package atomicfile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteReplacesAndLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	if err := WriteJSON(p, map[string]int{"a": 1}, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteJSON(p, map[string]int{"a": 2}, 0o600); err != nil {
		t.Fatal(err)
	}
	var got map[string]int
	if err := ReadJSON(p, &got); err != nil || got["a"] != 2 {
		t.Fatalf("got %v err %v", got, err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("leftover files: %v", entries)
	}
	if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", fi.Mode())
	}
}

func TestTornFileFailsClosed(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.json")
	if err := os.WriteFile(p, []byte(`{"a":`), 0o600); err != nil {
		t.Fatal(err)
	}
	var v map[string]int
	if err := ReadJSON(p, &v); err == nil || errors.Is(err, os.ErrNotExist) {
		t.Fatalf("want corrupt error, got %v", err)
	}
}

func TestMissingIsDistinguishable(t *testing.T) {
	var v map[string]int
	err := ReadJSON(filepath.Join(t.TempDir(), "nope"), &v)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("got %v", err)
	}
}
