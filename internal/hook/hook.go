// Package hook lets root-run helpers (cron jobs, hotplug button scripts) ask the running
// daemon to perform a named action, so the daemon's locks, guards and lockout rules apply
// and no helper runs uci in shell or keeps a stored login (device-facts §8, ADR 0024).
//
// Transport: a Unix socket in a 0700 directory, mode 0600, owned by the daemon's user
// (root on the router); file permissions are the authentication. Protocol: the client
// writes one action name and a newline; the daemon answers "ok" or "error <code>" and a
// newline, then closes. It is not an HTTP route and is not reachable from the network.
package hook

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// SocketPath is the router location; /var/run is RAM.
const SocketPath = "/var/run/travo-greenfield/hook.sock"

// ActionTimeout bounds one action; ClientTimeout leaves room for the reply.
const (
	ActionTimeout = 90 * time.Second
	ClientTimeout = 120 * time.Second
)

var (
	actionName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,47}$`)
	codeName   = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,47}$`)
)

// Action performs one hook. A returned error is logged by the daemon and reported to the
// client as "error <code>" where code is Code(err).
type Action func(context.Context) error

// CodedError carries a stable code for the client's log line.
type CodedError struct {
	Code string
	Err  error
}

func (e *CodedError) Error() string { return e.Code + ": " + e.Err.Error() }
func (e *CodedError) Unwrap() error { return e.Err }

// Code is the stable code of err: its CodedError code, else "failed".
func Code(err error) string {
	var c *CodedError
	if errors.As(err, &c) && codeName.MatchString(c.Code) {
		return c.Code
	}
	return "failed"
}

// Server dispatches actions by name. Register before Serve.
type Server struct {
	Path    string
	mu      sync.Mutex
	actions map[string]Action
	running sync.Mutex // one action at a time: helpers are rare and never concurrent by design
}

// Register adds a named action; a duplicate or bad name is a programming error.
func (s *Server) Register(name string, a Action) {
	if !actionName.MatchString(name) {
		panic("hook: invalid action name " + name)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.actions == nil {
		s.actions = map[string]Action{}
	}
	if _, dup := s.actions[name]; dup {
		panic("hook: duplicate action " + name)
	}
	s.actions[name] = a
}

func (s *Server) action(name string) (Action, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.actions[name]
	return a, ok
}

// Listen creates the socket (replacing a stale one) with owner-only permissions.
func (s *Server) Listen() (net.Listener, error) {
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, err
	}
	if err := os.Remove(s.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	l, err := net.Listen("unix", s.Path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(s.Path, 0o600); err != nil {
		_ = l.Close()
		return nil, err
	}
	return l, nil
}

// Serve answers connections until ctx ends (a long-lived listener: stopped by the
// lifecycle signal, not a timeout tier).
func (s *Server) Serve(ctx context.Context, l net.Listener) {
	go func() {
		<-ctx.Done()
		_ = l.Close()
	}()
	for {
		conn, err := l.Accept()
		if err != nil {
			if ctx.Err() == nil {
				slog.Warn("hook socket", "err", err)
			}
			return
		}
		go s.handle(ctx, conn)
	}
}

func (s *Server) handle(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(ClientTimeout))
	line, err := bufio.NewReader(io.LimitReader(conn, 64)).ReadString('\n')
	if err != nil {
		return
	}
	name := strings.TrimSuffix(line, "\n")
	a, ok := s.action(name)
	if !actionName.MatchString(name) || !ok {
		slog.Warn("hook: unknown action", "action", name)
		fmt.Fprint(conn, "error unknown-action\n")
		return
	}
	reply := s.run(ctx, name, a)
	fmt.Fprint(conn, reply+"\n")
}

func (s *Server) run(ctx context.Context, name string, a Action) (reply string) {
	s.running.Lock()
	defer s.running.Unlock()
	ctx, cancel := context.WithTimeout(ctx, ActionTimeout)
	defer cancel()
	defer func() {
		if p := recover(); p != nil {
			slog.Error("hook: action panicked", "action", name, "panic", p)
			reply = "error failed"
		}
	}()
	if err := a(ctx); err != nil {
		slog.Warn("hook: action failed", "action", name, "err", err)
		return "error " + Code(err)
	}
	slog.Info("hook: action done", "action", name)
	return "ok"
}

// Call asks the daemon at path to run action and returns its error code as an error.
func Call(path, action string, timeout time.Duration) error {
	if !actionName.MatchString(action) {
		return fmt.Errorf("invalid hook action %q", action)
	}
	conn, err := net.DialTimeout("unix", path, 5*time.Second)
	if err != nil {
		return fmt.Errorf("daemon not reachable: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := fmt.Fprint(conn, action+"\n"); err != nil {
		return err
	}
	reply, err := bufio.NewReader(io.LimitReader(conn, 128)).ReadString('\n')
	if err != nil {
		return fmt.Errorf("no reply from daemon: %w", err)
	}
	reply = strings.TrimSuffix(reply, "\n")
	if reply == "ok" {
		return nil
	}
	return fmt.Errorf("%s: %s", action, strings.TrimPrefix(reply, "error "))
}
