package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/runner"
)

const secret = "S3cr3t-pw!-unique"

// argvSpy records every argv and stdin; reply decides exit code.
type argvSpy struct {
	argv  []string
	stdin string
	exit  int
	err   error
}

func (s *argvSpy) run(ctx context.Context, name string, args ...string) (string, int, error) {
	if _, ok := ctx.Deadline(); !ok {
		panic("shell-out without a timeout tier")
	}
	s.argv = append(s.argv, name+" "+strings.Join(args, " "))
	return "", s.exit, s.err
}

func (s *argvSpy) runInput(ctx context.Context, stdin, name string, args ...string) (string, int, error) {
	s.stdin = stdin
	return s.run(ctx, name, args...)
}

func newSpyDevice(s *argvSpy) *Device {
	return &Device{Run: s.run, RunInput: runner.InputRunner(s.runInput)}
}

func TestVerifyExitCodes(t *testing.T) {
	cases := []struct {
		name string
		exit int
		err  error
		want bool
		fail bool
	}{
		{"valid", 0, nil, true, false},
		{"permission denied (wrong password and unknown user)", 250, errors.New("exit status 250: Permission denied"), false, false},
		{"ubus missing / not running", 4, errors.New("exit status 4: Command failed: Connection failed"), false, true},
		{"killed by timeout", -1, errors.New("signal: killed"), false, true},
		{"exec failure", 0, errors.New("fork/exec: no such file"), false, true},
	}
	for _, c := range cases {
		spy := &argvSpy{exit: c.exit, err: c.err}
		got, err := newSpyDevice(spy).VerifyRootPassword(context.Background(), secret)
		if got != c.want || (err != nil) != c.fail {
			t.Errorf("%s: got %v err %v", c.name, got, err)
		}
		if err != nil && strings.Contains(err.Error(), secret) {
			t.Errorf("%s: error leaks the password: %v", c.name, err)
		}
	}
}

func TestVerifyUsesUbusSessionLoginWithPasswordOnlyInJSON(t *testing.T) {
	spy := &argvSpy{}
	if _, err := newSpyDevice(spy).VerifyRootPassword(context.Background(), secret); err != nil {
		t.Fatal(err)
	}
	want := `ubus call session login {"username":"root","password":"` + secret + `","timeout":1}`
	if len(spy.argv) != 1 || spy.argv[0] != want {
		t.Fatalf("argv %q, want %q", spy.argv, want)
	}
}

func TestVerifyNULNeverReachesRpcd(t *testing.T) {
	spy := &argvSpy{}
	ok, err := newSpyDevice(spy).VerifyRootPassword(context.Background(), "abc\x00def")
	if ok || err != nil || len(spy.argv) != 0 {
		t.Fatalf("ok=%v err=%v calls=%v", ok, err, spy.argv)
	}
}

func TestSetRootPasswordUsesStdinNotArgv(t *testing.T) {
	spy := &argvSpy{}
	if err := newSpyDevice(spy).SetRootPassword(context.Background(), secret); err != nil {
		t.Fatal(err)
	}
	if spy.stdin != secret+"\n"+secret+"\n" {
		t.Fatalf("stdin %q", spy.stdin)
	}
	if len(spy.argv) != 1 || spy.argv[0] != "passwd root" {
		t.Fatalf("argv %q", spy.argv)
	}
	for _, a := range spy.argv {
		if strings.Contains(a, secret) {
			t.Fatal("password in argv")
		}
	}
}

func TestSetRootPasswordFailureHidesPassword(t *testing.T) {
	spy := &argvSpy{exit: 1, err: errors.New("exit status 1")}
	err := newSpyDevice(spy).SetRootPassword(context.Background(), secret)
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("got %v", err)
	}
}

func TestRootPasswordSetReadsShadow(t *testing.T) {
	read := func(content string, err error) *Device {
		return &Device{ReadFile: func(p string) ([]byte, error) {
			if p != "/etc/shadow" {
				t.Fatalf("read %s", p)
			}
			return []byte(content), err
		}}
	}
	ctx := context.Background()
	if set, err := read("root::0:0:::::\n", nil).RootPasswordSet(ctx); set || err != nil {
		t.Fatalf("empty field: %v %v", set, err)
	}
	if set, err := read("root:$5$x$y:0:0:::::\n", nil).RootPasswordSet(ctx); !set || err != nil {
		t.Fatalf("hash: %v %v", set, err)
	}
	if set, err := read("", errors.New("permission denied")).RootPasswordSet(ctx); set || err == nil {
		t.Fatalf("unreadable must be an error (fail closed): %v %v", set, err)
	}
	if _, err := read("daemon:*:0:0:::::\n", nil).RootPasswordSet(ctx); err == nil {
		t.Fatal("missing root line must be an error")
	}
}
