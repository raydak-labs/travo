// Package execx wraps os/exec with mandatory timeouts. On an embedded router
// a hung external command (opkg on a dead uplink, ubus during a driver crash,
// speedtest on flaky Wi-Fi) otherwise pins a request handler goroutine
// forever; every shell-out in the backend must go through one of these
// helpers with an explicit timeout tier.
package execx

import (
	"bufio"
	"context"
	"io"
	"os/exec"
	"sync"
	"time"
)

// Timeout tiers. Pick the smallest tier that the slowest legitimate run fits.
const (
	// Quick is for status probes and small reads (ubus, uci, logread, init.d status).
	Quick = 30 * time.Second
	// Slow is for operations that legitimately take a while (ntpd -q,
	// speedtest runs, init.d start/stop, config backups).
	Slow = 3 * time.Minute
	// Package is for package manager operations over slow uplinks.
	Package = 10 * time.Minute
)

// waitDelay force-closes I/O pipes shortly after the context kills the
// process, so orphaned grandchildren holding the pipe cannot stall Wait.
const waitDelay = 2 * time.Second

func command(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = waitDelay
	return cmd
}

// Output runs the command and returns its stdout, killing it after timeout.
func Output(timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return command(ctx, name, args...).Output()
}

// CombinedOutput runs the command and returns stdout+stderr, killing it after timeout.
func CombinedOutput(timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return command(ctx, name, args...).CombinedOutput()
}

// Run runs the command discarding output, killing it after timeout.
func Run(timeout time.Duration, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return command(ctx, name, args...).Run()
}

// Stream runs the command and sends each merged stdout/stderr line to logFn,
// killing the command after timeout.
func Stream(timeout time.Duration, logFn func(string), name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := command(ctx, name, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	// Read stdout/stderr separately. StdoutPipe()+Stderr=Stdout races on Linux
	// under WaitDelay (CI saw empty line captures). Serialize logFn for callers.
	var mu sync.Mutex
	safeLog := func(line string) {
		mu.Lock()
		defer mu.Unlock()
		logFn(line)
	}
	scan := func(r io.Reader) {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			safeLog(scanner.Text())
		}
	}

	// Read in goroutines: if an orphaned grandchild keeps a pipe open after the
	// timeout kill, Wait's WaitDelay force-close unblocks the scanners — so Wait
	// must not sit behind the read loops.
	done := make(chan struct{})
	go func() {
		defer close(done)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); scan(stdout) }()
		go func() { defer wg.Done(); scan(stderr) }()
		wg.Wait()
	}()

	err = cmd.Wait()
	<-done
	return err
}
