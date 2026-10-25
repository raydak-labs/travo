package services

import (
	"fmt"
	"os"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/guard"
)

// TestMain points every crash guard a test does not place explicitly at a temp
// dir, so no test can touch /etc/trafo.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "travo-guards-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	guard.SetDir(dir)
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
