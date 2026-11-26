package mock

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"
)

// DefaultPassword is the mock account's password, so mock mode is never an open router.
const DefaultPassword = "mock-password"

// SetPassword scripts the root password; "" means first-run state (no password).
func (d *Device) SetPassword(pw string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.password, d.passwordInit = pw, true
}

func (d *Device) pw() string {
	if !d.passwordInit {
		return DefaultPassword
	}
	return d.password
}

func (d *Device) VerifyRootPassword(_ context.Context, password string) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.AccountsErr != nil {
		return false, d.AccountsErr
	}
	if strings.ContainsRune(password, 0) {
		return false, nil
	}
	// like rpcd: an empty shadow field accepts anything, which is why first-run reads shadow instead
	return d.pw() == "" || subtle.ConstantTimeCompare([]byte(d.pw()), []byte(password)) == 1, nil
}

func (d *Device) RootPasswordSet(context.Context) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.AccountsErr != nil {
		return false, d.AccountsErr
	}
	return d.pw() != "", nil
}

func (d *Device) SetRootPassword(_ context.Context, password string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.AccountsErr != nil {
		return d.AccountsErr
	}
	if password == "" {
		return errors.New("empty password")
	}
	d.password, d.passwordInit = password, true
	return nil
}
