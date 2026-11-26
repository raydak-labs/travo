package txn

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

// ShellSpawner runs the watchdog with /bin/sh. Start puts it in its own session so the
// daemon's death (crash, SIGKILL, procd restart) does not take it down.
type ShellSpawner struct {
	Shell string
	// RunTimeout bounds an immediate restore (reload included).
	RunTimeout time.Duration
}

func (s ShellSpawner) Start(script string, args ...string) (int, error) {
	cmd := exec.Command(s.Shell, append([]string{script}, args...)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	go func() { _ = cmd.Wait() }() // reap; the process outlives us if we die first
	return pid, nil
}

func (s ShellSpawner) Run(script string, args ...string) error {
	cmd := exec.Command(s.Shell, append([]string{script}, args...)...)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("watchdog: %w: %s", err, errb.String())
		}
		return nil
	case <-time.After(s.RunTimeout):
		_ = cmd.Process.Kill()
		return fmt.Errorf("watchdog: restore timed out after %s", s.RunTimeout)
	}
}

// ProcAlive checks /proc/<pid>/cmdline for the script path (Linux).
func ProcAlive(pid int, script string) bool {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cmdline")
	return err == nil && bytes.Contains(b, []byte(script))
}
