// Package execx wraps os/exec with mandatory timeouts. On an embedded router
// a hung external command (opkg on a dead uplink, ubus during a driver crash,
// speedtest on flaky Wi-Fi) otherwise pins a request handler goroutine
// forever; every shell-out in the backend must go through one of these
// helpers with an explicit timeout tier.
package execx

import (
	"bytes"
	"context"
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
//
// The streams are attached as io.Writers rather than read back through
// StdoutPipe/StderrPipe. With pipes, cmd.Wait returns as soon as the process
// exits and closes the read end, so whatever a reader had not yet drained is
// discarded — Go documents that calling Wait before reads complete is incorrect.
// That truncation is timing-dependent: it shows up as lost tail output on a busy
// machine, which for a log-tail helper means silently returning fewer lines than
// the command produced. With a Writer, os/exec owns the copying and Wait does
// not return until it has finished, so the callback sees every line.
func Stream(timeout time.Duration, logFn func(string), name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := command(ctx, name, args...)

	// One writer for both streams: logFn is shared, so a single mutex keeps a
	// line from one stream from interleaving mid-line with the other.
	shared := &lineWriter{logFn: logFn}
	cmd.Stdout = shared
	cmd.Stderr = shared

	if err := cmd.Start(); err != nil {
		return err
	}
	// Wait also waits for os/exec's copying into cmd.Stdout/cmd.Stderr, so the
	// buffers are complete once it returns. Emit any trailing partial line.
	err := cmd.Wait()
	shared.flush()
	return err
}

// maxStreamLineBytes bounds a single unterminated line so a command that emits
// bytes with no newline (binary output, a progress bar) cannot grow the buffer
// without limit.
const maxStreamLineBytes = 64 * 1024

// lineWriter splits a byte stream into lines and hands each to logFn.
type lineWriter struct {
	mu    sync.Mutex
	logFn func(string)
	buf   []byte
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		w.logFn(string(w.buf[:i]))
		w.buf = w.buf[i+1:]
	}
	// A line with no newline yet: emit it once it is big enough to be a log line
	// rather than a stream that never terminates.
	if len(w.buf) > maxStreamLineBytes {
		w.logFn(string(w.buf[:maxStreamLineBytes]))
		w.buf = w.buf[maxStreamLineBytes:]
	}
	return len(p), nil
}

// flush emits a trailing line that ended without a newline.
func (w *lineWriter) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.buf) > 0 {
		w.logFn(string(w.buf))
		w.buf = nil
	}
}
