package ws

import (
	"os"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/openwrt-travel-gui/backend/internal/auth"
)

// See internal/api/main_test.go for why the suite runs at bcrypt.MinCost: the
// WebSocket tests build an AuthService per test server, and at DefaultCost that
// cost dominates an already timing-sensitive suite (the handshake tests poll
// with deadlines).
//
// Set before any test runs, because SetBcryptCostForTesting writes an
// unsynchronised package variable in the auth package.
func TestMain(m *testing.M) {
	restore := auth.SetBcryptCostForTesting(bcrypt.MinCost)
	code := m.Run()
	restore()
	os.Exit(code)
}
