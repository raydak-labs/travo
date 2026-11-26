// Package rootpw holds the pure parts of root-password handling: the exact bytes sent
// to ubus and passwd, and shadow parsing. The Device adapters run them.
package rootpw

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ExitDenied is what `ubus call session login` exits with for a wrong password AND an
// unknown user (device capture session-login-*.txt); the two are never told apart.
const ExitDenied = 250

// ErrNUL: rpcd would truncate a password at NUL (C string), so "abc\x00x" would verify as "abc".
var ErrNUL = errors.New("password contains NUL")

// LoginArgs builds the ubus argv. The password only ever sits inside the JSON argument
// produced by json.Marshal; nothing is shell-interpolated (the runner uses argv).
// timeout 1 makes the rpcd session we do not need expire at once (`session destroy` is denied by ACL).
func LoginArgs(password string) ([]string, error) {
	if strings.ContainsRune(password, 0) {
		return nil, ErrNUL
	}
	payload, err := json.Marshal(struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Timeout  int    `json:"timeout"`
	}{"root", password, 1})
	if err != nil {
		return nil, err
	}
	return []string{"call", "session", "login", string(payload)}, nil
}

// PasswdInput is the stdin for `passwd root` (new password, then confirmation).
func PasswdInput(password string) string { return password + "\n" + password + "\n" }

// ErrShadow means /etc/shadow had no usable root entry. Callers treat it as "password set".
var ErrShadow = errors.New("no usable root entry in shadow")

// ParseShadow reports whether root has a password. An EMPTY field means none (rpcd then
// accepts any password); `!`, `*`, `x` and hashes mean set or locked, never open.
func ParseShadow(content string) (bool, error) {
	for _, line := range strings.Split(content, "\n") {
		f := strings.Split(strings.TrimSuffix(line, "\r"), ":")
		if f[0] != "root" {
			continue
		}
		if len(f) < 2 {
			return false, fmt.Errorf("%w: root line has no password field", ErrShadow)
		}
		return f[1] != "", nil
	}
	return false, fmt.Errorf("%w: no root line", ErrShadow)
}
