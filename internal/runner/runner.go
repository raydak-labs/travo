// Package runner executes external commands; production uses Exec, tests inject fakes.
package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

const commandWaitDelay = time.Second

// Runner executes a command and returns stdout and the exit code.
type Runner func(ctx context.Context, name string, args ...string) (stdout string, exit int, err error)

// InputRunner is Runner with data on stdin, for secrets that must never reach argv.
type InputRunner func(ctx context.Context, stdin string, name string, args ...string) (stdout string, exit int, err error)

// CommandError preserves stderr separately from the command and exit context.
type CommandError struct {
	Name   string
	Args   []string
	Err    error
	Stderr string
}

func (e *CommandError) Error() string {
	return fmt.Sprintf("%s %s: %v: %s", e.Name, strings.Join(e.Args, " "), e.Err, e.Stderr)
}

func (e *CommandError) Unwrap() error { return e.Err }

// Stderr returns the complete trimmed diagnostic from a command failure.
func Stderr(err error) (string, bool) {
	var commandErr *CommandError
	if !errors.As(err, &commandErr) {
		return "", false
	}
	return commandErr.Stderr, true
}

// Exec runs a real process. The caller's ctx carries the timeout tier.
func Exec(ctx context.Context, name string, args ...string) (string, int, error) {
	return RunInput(ctx, "", name, args...)
}

// RunInput runs a real process with stdin. Stderr is echoed into the error text and retained
// separately for diagnostic matching, so callers must not pass secrets in args (they would land in logs).
func RunInput(ctx context.Context, stdin string, name string, args ...string) (string, int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	// A canceled shell command can leave descendants holding its output pipes.
	// Keep commands in their own group so cancellation also stops those children.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = commandWaitDelay
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return out.String(), ee.ExitCode(), &CommandError{
			Name:   name,
			Args:   args,
			Err:    err,
			Stderr: strings.TrimSpace(errb.String()),
		}
	}
	return out.String(), 0, err
}
