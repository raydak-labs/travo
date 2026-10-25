package api

import (
	"os"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/openwrt-travel-gui/backend/internal/auth"
	"github.com/openwrt-travel-gui/backend/internal/guard"
)

// Every test case in this package builds a fresh AuthService and then logs in,
// and each of those steps runs bcrypt. At bcrypt.DefaultCost that is ~100ms
// per operation, which made this package the slowest one in the suite and left
// the -race job on a 2-core CI runner close to its timeout: the wall clock
// tracked the runner's speed rather than the code's behaviour. MinCost keeps
// the code path (hash, compare, re-hash on password change) identical and makes
// the suite deterministic in time.
//
// The cost is set here, before any test runs, because SetBcryptCostForTesting
// writes an unsynchronised package variable. Crash guards go to a temp dir so
// no handler test can touch /etc/trafo.
func TestMain(m *testing.M) {
	restore := auth.SetBcryptCostForTesting(bcrypt.MinCost)
	dir, err := os.MkdirTemp("", "travo-guards-*")
	if err != nil {
		panic(err)
	}
	guard.SetDir(dir)
	code := m.Run()
	restore()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
