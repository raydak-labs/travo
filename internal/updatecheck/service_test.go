package updatecheck

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/atomicfile"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

const releasesJSON = `[
 {"tag_name":"v0.10.0","assets":[{"name":"travo-greenfield-0.10.0-aarch64.tar.gz"},{"name":"checksums.txt"},{"name":"checksums.txt.sig"}]},
 {"tag_name":"v0.9.9","assets":[{"name":"travo-greenfield-0.9.9-aarch64.tar.gz"},{"name":"checksums.txt"},{"name":"checksums.txt.sig"}]},
 {"tag_name":"v1.0.0","draft":true,"assets":[{"name":"travo-greenfield-1.0.0-aarch64.tar.gz"},{"name":"checksums.txt"},{"name":"checksums.txt.sig"}]},
 {"tag_name":"v2.0.0","prerelease":true,"assets":[{"name":"travo-greenfield-2.0.0-aarch64.tar.gz"},{"name":"checksums.txt"},{"name":"checksums.txt.sig"}]},
 {"tag_name":"v3.0.0","assets":[{"name":"travo-greenfield-3.0.0-aarch64.tar.gz"},{"name":"checksums.txt"}]},
 {"tag_name":"v4.0.0","assets":[{"name":"travo-greenfield-4.0.0-aarch64.tar.gz"},{"name":"checksums.txt"},{"name":"checksums.txt"},{"name":"checksums.txt.sig"}]},
 {"tag_name":"v05.0.0","assets":[{"name":"travo-greenfield-05.0.0-aarch64.tar.gz"},{"name":"checksums.txt"},{"name":"checksums.txt.sig"}]}
]`

func TestLatest(t *testing.T) {
	for _, tt := range []struct {
		name                  string
		code                  int
		body, version, status string
	}{
		{"stable assets", 200, releasesJSON, "0.10.0", ""},
		{"empty", 200, "[]", "", "no_release"},
		{"null", 200, "null", "", "unavailable"},
		{"trailing", 200, "[] []", "", "unavailable"},
		{"oversize", 200, strings.Repeat(" ", maxResponse) + "[]", "", "unavailable"},
		{"private", 404, "", "", "source_unavailable"},
		{"unauthorized", 401, "", "", "source_unavailable"},
		{"forbidden", 403, "", "", "rate_limited"},
		{"limited", 429, "", "", "rate_limited"},
		{"redirect", 302, "", "", "unavailable"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			v, status := latest(t.Context(), roundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != "GET" || r.URL.String() != "https://api.github.com/repos/"+DefaultSource+"/releases?per_page=100" || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
					t.Errorf("unexpected outbound request: %s %s", r.Method, r.URL)
				}
				res := response(tt.code, tt.body)
				res.Header.Set("Location", "https://elsewhere.example")
				return res, nil
			}), DefaultSource)
			if v != tt.version || status != tt.status || calls != 1 {
				t.Fatalf("got %q %q, calls %d", v, status, calls)
			}
		})
	}
}

func TestLatestDeadline(t *testing.T) {
	_, status := latest(t.Context(), roundTrip(func(r *http.Request) (*http.Response, error) {
		deadline, ok := r.Context().Deadline()
		if !ok || time.Until(deadline) > 5*time.Second {
			t.Error("missing total timeout")
		}
		return nil, context.DeadlineExceeded
	}), DefaultSource)
	if status != "unavailable" {
		t.Fatal(status)
	}
}

func TestCooldown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "check.json")
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var calls atomic.Int32
	transport := roundTrip(func(*http.Request) (*http.Response, error) {
		if _, err := os.Stat(path); err != nil {
			t.Error("attempt was not persisted before I/O", err)
		}
		calls.Add(1)
		return response(200, releasesJSON), nil
	})
	makeService := func(version string) *Service {
		return New(t.Context(), DefaultSource, version, path, func() time.Time { return now }, transport)
	}
	s := makeService("v0.9.0")
	check := func(s *Service, manual bool, want string, count int32) {
		t.Helper()
		if r := s.Check(t.Context(), manual); r.Status != want || calls.Load() != count {
			t.Fatalf("result %+v, calls %d, want %s and %d calls", r, calls.Load(), want, count)
		}
	}
	check(s, false, "available", 1)
	check(s, false, "available", 1)
	s = makeService("v0.9.0")
	check(s, false, "deferred", 1)
	now = now.Add(-time.Hour)
	check(s, false, "deferred", 1)
	check(s, true, "available", 2)
	now = now.Add(24 * time.Hour)
	check(s, false, "available", 3)
	check(makeService("0.10.0"), false, "up_to_date", 4)
	check(makeService("dev"), true, "disabled", 4)
	if err := os.WriteFile(path, []byte("null"), 0600); err != nil {
		t.Fatal(err)
	}
	check(makeService("v0.9.0"), true, "unavailable", 4)
}

func TestPersistenceFailurePreventsOutboundRequest(t *testing.T) {
	s := New(t.Context(), DefaultSource, "1.2.3", filepath.Join(t.TempDir(), "missing", "check.json"), time.Now, roundTrip(func(*http.Request) (*http.Response, error) {
		t.Error("unexpected outbound request")
		return response(500, ""), nil
	}))
	if r := s.Check(t.Context(), true); r.Status != "unavailable" || r.CheckedAt != nil {
		t.Fatalf("%+v", r)
	}
}

func TestCoalescedCheckSurvivesCallerCancellation(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	s := New(t.Context(), DefaultSource, "0.9.0", filepath.Join(t.TempDir(), "check.json"), time.Now, roundTrip(func(r *http.Request) (*http.Response, error) {
		if calls.Add(1) == 1 {
			close(entered)
		}
		select {
		case <-release:
			return response(200, releasesJSON), nil
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
	}))
	ctx, cancel := context.WithCancel(t.Context())
	first := make(chan Result, 1)
	go func() { first <- s.Check(ctx, true) }()
	<-entered
	cancel()
	if r := <-first; r.Status != "unavailable" {
		t.Fatal(r.Status)
	}
	s.mu.Lock()
	original := s.current
	s.mu.Unlock()
	if original == nil {
		t.Fatal("shared attempt ended with first caller")
	}
	second := make(chan Result, 1)
	go func() { second <- s.Check(t.Context(), false) }()
	close(release)
	if r := <-second; r.Status != "available" || calls.Load() != 1 {
		t.Fatalf("result %+v, calls %d", r, calls.Load())
	}
}

func TestValidSource(t *testing.T) {
	for _, v := range []string{DefaultSource, "Owner/a.b_c-d", "off"} {
		if !ValidSource(v) {
			t.Error(v)
		}
	}
	for _, v := range []string{"", "/repo", "owner/", "a/b/c", "a/..", "./repo", "a/b%2fc", "a/b?x", "a/bé"} {
		if ValidSource(v) {
			t.Error(v)
		}
	}
}

type observingContext struct {
	context.Context
	waiting chan struct{}
	once    sync.Once
}

func (c *observingContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.waiting) })
	return c.Context.Done()
}

func TestManualCheckBypassesConcurrentThrottledAutomaticCheck(t *testing.T) {
	now := time.Now()
	path := filepath.Join(t.TempDir(), "check.json")
	if err := atomicfile.WriteJSON(path, attemptState{DefaultSource, "0.9.0", now}, 0600); err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var clockCalls, outbound atomic.Int32
	s := New(t.Context(), DefaultSource, "0.9.0", path, func() time.Time {
		if clockCalls.Add(1) == 1 {
			close(entered)
			<-release
		}
		return now
	}, roundTrip(func(*http.Request) (*http.Response, error) {
		outbound.Add(1)
		return response(200, releasesJSON), nil
	}))
	automatic := make(chan Result, 1)
	go func() { automatic <- s.Check(t.Context(), false) }()
	<-entered
	manual := make(chan Result, 1)
	manualCtx := &observingContext{Context: t.Context(), waiting: make(chan struct{})}
	go func() { manual <- s.Check(manualCtx, true) }()
	<-manualCtx.waiting
	close(release)
	if r := <-automatic; r.Status != "deferred" {
		t.Fatal(r.Status)
	}
	if r := <-manual; r.Status != "available" || outbound.Load() != 1 {
		t.Fatalf("result %+v, outbound attempts %d", r, outbound.Load())
	}
}

func TestDisabledReason(t *testing.T) {
	never := roundTrip(func(r *http.Request) (*http.Response, error) {
		return response(200, releasesJSON), nil
	})
	for _, tt := range []struct {
		name, source, running, want string
	}{
		{"source off", "off", "1.2.3", "off"},
		{"development build", DefaultSource, "dev", "unstable_build"},
		{"both: the setting wins", "off", "dev", "off"},
		{"v prefix is stable", DefaultSource, "v1.2.3", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := New(t.Context(), tt.source, tt.running, filepath.Join(t.TempDir(), "state.json"), time.Now, never)
			got := s.Check(t.Context(), true)
			if tt.want == "" {
				if got.DisabledReason != nil {
					t.Fatalf("reason = %q, want null (status %s)", *got.DisabledReason, got.Status)
				}
				return
			}
			if got.Status != "disabled" || got.DisabledReason == nil || *got.DisabledReason != tt.want {
				t.Fatalf("got status %s reason %v, want disabled %s", got.Status, got.DisabledReason, tt.want)
			}
			if got.LatestVersion != nil || got.CheckedAt != nil || got.RunningVersion != tt.running {
				t.Fatalf("disabled result changed shape: %+v", got)
			}
		})
	}
}
