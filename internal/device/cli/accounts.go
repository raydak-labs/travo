package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/blackdark-org/travo-greenfield/internal/auth/rootpw"
)

const shadowPath = "/etc/shadow"

// VerifyRootPassword asks rpcd through ubus. Exit 0 = valid, 250 = denied (wrong password
// and unknown user look the same); anything else is an error, never "wrong password".
func (d *Device) VerifyRootPassword(ctx context.Context, password string) (bool, error) {
	args, err := rootpw.LoginArgs(password)
	if errors.Is(err, rootpw.ErrNUL) {
		return false, nil // can never be a valid password; do not ask rpcd
	}
	if err != nil {
		return false, err
	}
	ctx, cancel := context.WithTimeout(ctx, TierRead)
	defer cancel()
	_, code, runErr := d.Run(ctx, "ubus", args...)
	switch {
	case code == 0 && runErr == nil:
		return true, nil
	case code == rootpw.ExitDenied:
		return false, nil
	}
	return false, fmt.Errorf("ubus session login: exit %d", code) // runErr omitted: it echoes argv
}

// RootPasswordSet reads shadow; an unreadable or odd file is an error so first-run stays closed.
func (d *Device) RootPasswordSet(context.Context) (bool, error) {
	read := d.ReadFile
	if read == nil {
		read = os.ReadFile
	}
	b, err := read(shadowPath)
	if err != nil {
		return false, fmt.Errorf("read shadow: %w", err)
	}
	return rootpw.ParseShadow(string(b))
}

// SetRootPassword feeds `passwd root` on stdin (twice); argv never carries the secret.
func (d *Device) SetRootPassword(ctx context.Context, password string) error {
	ctx, cancel := context.WithTimeout(ctx, TierWrite)
	defer cancel()
	run := d.RunInput
	if run == nil {
		return fmt.Errorf("passwd: no stdin runner")
	}
	if _, code, err := run(ctx, rootpw.PasswdInput(password), "passwd", "root"); err != nil || code != 0 {
		return fmt.Errorf("passwd root: exit %d", code)
	}
	return nil
}
