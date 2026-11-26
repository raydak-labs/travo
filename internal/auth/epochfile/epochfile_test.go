package epochfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTripAndMissing(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state", "epoch.json")
	if n, err := Load(p); err != nil || n != 0 {
		t.Fatalf("missing file: %d %v", n, err)
	}
	if err := Save(p, 42); err != nil {
		t.Fatal(err)
	}
	if n, err := Load(p); err != nil || n != 42 {
		t.Fatalf("got %d %v", n, err)
	}
	if st, _ := os.Stat(p); st.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", st.Mode())
	}
}

func TestTornFileFailsClosed(t *testing.T) {
	p := filepath.Join(t.TempDir(), "epoch.json")
	_ = os.WriteFile(p, []byte(`{"epo`), 0o600)
	if _, err := Load(p); err == nil {
		t.Fatal("a torn epoch file must not read as epoch 0")
	}
}
