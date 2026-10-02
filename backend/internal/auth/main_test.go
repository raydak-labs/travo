package auth

import (
	"os"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// See internal/api/main_test.go for why the suite runs at bcrypt.MinCost: the
// auth tests hash and verify passwords dozens of times, and DefaultCost makes
// their wall clock a function of the machine rather than of the code.
//
// Set before any test runs, because SetBcryptCostForTesting writes an
// unsynchronised package variable.
func TestMain(m *testing.M) {
	restore := SetBcryptCostForTesting(bcrypt.MinCost)
	code := m.Run()
	restore()
	os.Exit(code)
}
