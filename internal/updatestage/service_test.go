package updatestage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type verifierFunc func(context.Context, string, string, string) error

func (f verifierFunc) VerifyRelease(ctx context.Context, m, s, k string) error {
	return f(ctx, m, s, k)
}

func TestStageLifecycle(t *testing.T) {
	root := filepath.Join(t.TempDir(), "stage")
	now := time.Now()
	version := "1.2.3"
	verifier := verifierFunc(func(_ context.Context, _, _, key string) error {
		raw, err := os.ReadFile(key)
		if err != nil || string(raw) != "test key" {
			t.Fatalf("unexpected trust key: %v", err)
		}
		return nil
	})
	var pending chan struct{}
	transport := transportFunc(func(r *http.Request) (*http.Response, error) {
		if pending != nil {
			pending <- struct{}{}
			<-r.Context().Done()
			return nil, r.Context().Err()
		}
		members, signed := testPackage(version)
		var body []byte
		switch {
		case strings.HasSuffix(r.URL.Path, "checksums.txt"):
			body = []byte(signed)
		case strings.HasSuffix(r.URL.Path, "checksums.txt.sig"):
			body = []byte("signature")
		default:
			body = gzipBytes(t, tarBytes(t, members))
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(body))}, nil
	})
	client := &http.Client{Transport: transport}
	eligible := func() (string, bool) { return version, true }
	s, err := Open(context.Background(), root, "owner/repo", "test key", client, verifier, eligible, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if rival, err := Open(context.Background(), root, "owner/repo", "test key", client, verifier, eligible, time.Now); err == nil {
		rival.Close()
		t.Fatal("second service locked active staging root")
	}
	if _, err := s.Stage(context.Background(), "1.2.2"); !errors.Is(err, ErrChanged) {
		t.Fatalf("ineligible: %v", err)
	}
	if _, err := s.Stage(context.Background(), version); err != nil {
		t.Fatal(err)
	}
	if st := s.Status(); st.Status != "verified" || *st.Version != version {
		t.Fatalf("status: %+v", st)
	}
	old := s.active.dir
	version = "1.2.4"
	pending = make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := s.Stage(ctx, version); done <- err }()
	<-pending
	if st := s.Status(); st.Status != "verifying" || *st.Version != version {
		t.Fatalf("status: %+v", st)
	}
	if _, err := s.Stage(context.Background(), version); !errors.Is(err, ErrBusy) {
		t.Fatalf("concurrent: %v", err)
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	if st := s.Status(); st.Status != "verified" || *st.Version != "1.2.3" {
		t.Fatalf("prior stage: %+v", st)
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 2 {
		t.Fatalf("partial stage remained: %v, %v", entries, err)
	}
	pending = nil
	if _, err := s.Stage(context.Background(), version); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("prior stage remained: %v", err)
	}
	now = now.Add(ttl)
	s.Sweep()
	if st := s.Status(); st.Status != "idle" || st.Version != nil {
		t.Fatalf("expired status: %+v", st)
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 1 {
		t.Fatalf("expired stage remained: %v, %v", entries, err)
	}
}

func TestAssetBoundsAndRedirects(t *testing.T) {
	var calls int
	client := &http.Client{CheckRedirect: checkRedirect, Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		switch r.URL.Path {
		case "/redirect":
			return &http.Response{StatusCode: 302, Header: http.Header{"Location": {"https://other.invalid/asset"}}, Body: http.NoBody}, nil
		default:
			return &http.Response{StatusCode: 200, ContentLength: 1, Body: io.NopCloser(strings.NewReader("oversize"))}, nil
		}
	})}
	dir := t.TempDir()
	if err := download(context.Background(), client, "https://github.com/redirect", filepath.Join(dir, "redirect"), 4); err == nil || calls != 1 {
		t.Fatalf("redirect: %v (%d calls)", err, calls)
	}
	if err := download(context.Background(), client, "https://github.com/asset", filepath.Join(dir, "asset"), 4); err == nil {
		t.Fatal("streaming limit ignored")
	}
	req, _ := http.NewRequest("GET", "https://release-assets.githubusercontent.com/asset", nil)
	if err := checkRedirect(req, make([]*http.Request, 3)); err != nil {
		t.Fatal(err)
	}
	if err := checkRedirect(req, make([]*http.Request, 4)); err == nil {
		t.Fatal("redirect limit ignored")
	}
}
