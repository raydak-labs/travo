package hook

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func serve(t *testing.T, register func(*Server)) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "hk") // short: unix socket paths are limited to ~100 bytes
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	s := &Server{Path: filepath.Join(dir, "run", "hook.sock")}
	register(s)
	l, err := s.Listen()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go s.Serve(ctx, l)
	return s.Path
}

func TestCallRunsTheAction(t *testing.T) {
	ran := 0
	path := serve(t, func(s *Server) {
		s.Register("leds-off", func(context.Context) error { ran++; return nil })
	})
	if err := Call(path, "leds-off", time.Second); err != nil {
		t.Fatal(err)
	}
	if ran != 1 {
		t.Fatalf("ran %d", ran)
	}
	st, err := os.Stat(path)
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("socket mode %v %v", st.Mode(), err)
	}
	dst, _ := os.Stat(filepath.Dir(path))
	if dst.Mode().Perm() != 0o700 {
		t.Fatalf("dir mode %v", dst.Mode())
	}
}

func TestCallReportsCodes(t *testing.T) {
	path := serve(t, func(s *Server) {
		s.Register("coded", func(context.Context) error {
			return &CodedError{Code: "operation_in_progress", Err: errors.New("busy")}
		})
		s.Register("plain", func(context.Context) error { return errors.New("boom") })
		s.Register("panics", func(context.Context) error { panic("bug") })
	})
	for action, want := range map[string]string{
		"coded":   "coded: operation_in_progress",
		"plain":   "plain: failed",
		"panics":  "panics: failed",
		"missing": "missing: unknown-action",
	} {
		err := Call(path, action, time.Second)
		if err == nil || err.Error() != want {
			t.Errorf("%s: %v, want %q", action, err, want)
		}
	}
}

func TestCallRefusesBadNamesAndMissingDaemon(t *testing.T) {
	if err := Call("/nonexistent/hook.sock", "x y", time.Second); err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatal(err)
	}
	if err := Call("/nonexistent/hook.sock", "ping", time.Second); err == nil || !strings.Contains(err.Error(), "not reachable") {
		t.Fatal(err)
	}
}

func TestListenReplacesAStaleSocket(t *testing.T) {
	path := serve(t, func(s *Server) { s.Register("ping", func(context.Context) error { return nil }) })
	s := &Server{Path: path}
	s.Register("ping", func(context.Context) error { return nil })
	l, err := s.Listen()
	if err != nil {
		t.Fatal(err)
	}
	_ = l.Close()
}

func TestRegisterPanicsOnDuplicate(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("no panic")
		}
	}()
	s := &Server{}
	s.Register("a", func(context.Context) error { return nil })
	s.Register("a", func(context.Context) error { return nil })
}
