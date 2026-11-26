package api

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const backupPath = "/api/v1/system/backup"

type backupFunc func(context.Context, string) error

func (f backupFunc) BackupConfig(ctx context.Context, path string) error { return f(ctx, path) }

func TestBackupAccess(t *testing.T) {
	e := newEnv(t)
	rec := e.do(req{method: "GET", path: backupPath})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	cookie, _ := e.login()
	rec = e.do(req{method: "GET", path: backupPath, cookie: cookie})
	if rec.Code != http.StatusServiceUnavailable || errCode(t, rec) != "backup_unavailable" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestBackupStreamsIndependentArchives(t *testing.T) {
	archive := testConfigArchive(t)
	paths := make(chan string, 2)
	start := make(chan struct{})
	e := newEnvWithBackup(t, backupFunc(func(_ context.Context, path string) error {
		paths <- path
		<-start
		return os.WriteFile(path, archive, 0600)
	}))
	cookie, _ := e.login()
	responses := make(chan *httptest.ResponseRecorder, 2)
	for range 2 {
		go func() { responses <- e.do(req{method: "GET", path: backupPath, cookie: cookie}) }()
	}
	first, second := <-paths, <-paths
	defer close(start)
	if first == second {
		t.Fatal("concurrent backups share a destination")
	}
	for _, path := range []string{first, second} {
		info, err := os.Stat(filepath.Dir(path))
		if err != nil || info.Mode().Perm() != 0700 {
			t.Fatalf("backup destination is not private: %v", err)
		}
	}
	// Release both generators before waiting for the streamed responses.
	start <- struct{}{}
	start <- struct{}{}
	for range 2 {
		rec := <-responses
		if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), archive) {
			t.Fatalf("unexpected archive response: %d, %d bytes", rec.Code, rec.Body.Len())
		}
		for name, want := range map[string]string{
			"Content-Type":        "application/gzip",
			"Content-Disposition": `attachment; filename="travo-config.tar.gz"`,
			"Cache-Control":       "no-store",
			"Content-Length":      strconv.Itoa(len(archive)),
		} {
			if got := rec.Header().Get(name); got != want {
				t.Errorf("%s: got %q, want %q", name, got, want)
			}
		}
	}
	assertBackupRemoved(t, first)
	assertBackupRemoved(t, second)
}

func TestBackupFailureAndCancellationCleanUp(t *testing.T) {
	for _, name := range []string{"generator failure", "cancellation"} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var dest string
			e := newEnvWithBackup(t, backupFunc(func(ctx context.Context, path string) error {
				dest = path
				if err := os.WriteFile(path, []byte("partial"), 0600); err != nil {
					return err
				}
				if name == "cancellation" {
					cancel()
					return ctx.Err()
				}
				return errors.New("private command output")
			}))
			cookie, _ := e.login()
			request := httptest.NewRequest(http.MethodGet, "http://192.168.1.1"+backupPath, nil).WithContext(ctx)
			request.RemoteAddr = lanAddr
			request.AddCookie(&http.Cookie{Name: cookieName, Value: cookie})
			rec := httptest.NewRecorder()
			e.h.ServeHTTP(rec, request)
			if rec.Code != http.StatusInternalServerError || errCode(t, rec) != "backup_failed" ||
				rec.Header().Get("Content-Disposition") != "" || strings.Contains(rec.Body.String(), "private command output") {
				t.Fatalf("%d %s", rec.Code, rec.Body)
			}
			assertBackupRemoved(t, dest)
		})
	}
}

func assertBackupRemoved(t *testing.T, dest string) {
	t.Helper()
	if dest == "" {
		t.Fatal("backup generator was not called")
	}
	if _, err := os.Stat(filepath.Dir(dest)); !os.IsNotExist(err) {
		t.Fatalf("backup destination not removed: %v", err)
	}
}

func testConfigArchive(t *testing.T) []byte {
	t.Helper()
	var out bytes.Buffer
	gz := gzip.NewWriter(&out)
	tarball := tar.NewWriter(gz)
	content := []byte("configuration used only to test archive streaming")
	if err := tarball.WriteHeader(&tar.Header{Name: "etc/config/test", Mode: 0600, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarball.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tarball.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
