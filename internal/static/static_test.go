package static

import (
	"compress/gzip"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func gz(t *testing.T, s string) []byte {
	t.Helper()
	var b strings.Builder
	w := gzip.NewWriter(&b)
	_, _ = w.Write([]byte(s))
	_ = w.Close()
	return []byte(b.String())
}

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "dist")
	files := map[string][]byte{
		"index.html":                  []byte("<html>x</html>"),
		"index.html.gz":               gz(t, "<html>x</html>"),
		"index-AbCd1234.js":           []byte("console.log(1)"),
		"index-AbCd1234.js.gz":        gz(t, "console.log(1)"),
		"favicon.svg":                 []byte("<svg/>"),
		"plain.txt":                   []byte("plain"),
		"csp.json":                    []byte(`{"scriptHashes":[]}`),
		".env":                        []byte("SECRET=1"),
		"sub/inner.txt":               []byte("inner"),
		filepath.Join("..", "go.mod"): []byte("module x"),
	}
	for n, c := range files {
		p := filepath.Join(dir, n)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, c, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(root, "go.mod"), filepath.Join(dir, "link.txt")); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestHandler(t *testing.T) {
	dir := fixture(t)
	csp := CSP([]string{"sha256-abc"})
	h := Handler(dir, csp)
	const gzipEnc = "gzip, deflate"
	tests := []struct {
		name, method, path, ae string
		status                 int
		ctype, cache, encoding string
		code                   string
	}{
		{"root", "GET", "/", "", 200, "text/html; charset=utf-8", "no-store", "", ""},
		{"root gzip", "GET", "/", gzipEnc, 200, "text/html; charset=utf-8", "no-store", "gzip", ""},
		{"index.html", "GET", "/index.html", "", 200, "text/html; charset=utf-8", "no-store", "", ""},
		{"hashed asset", "GET", "/index-AbCd1234.js", "", 200, "text/javascript; charset=utf-8", "public, max-age=31536000, immutable", "", ""},
		{"hashed asset gzip", "GET", "/index-AbCd1234.js", gzipEnc, 200, "text/javascript; charset=utf-8", "public, max-age=31536000, immutable", "gzip", ""},
		{"plain no gz file", "GET", "/plain.txt", gzipEnc, 200, "text/plain; charset=utf-8", "no-cache", "", ""},
		{"svg", "GET", "/favicon.svg", "", 200, "image/svg+xml", "no-cache", "", ""},
		{"HEAD", "HEAD", "/index-AbCd1234.js", gzipEnc, 200, "text/javascript; charset=utf-8", "public, max-age=31536000, immutable", "gzip", ""},
		{"POST", "POST", "/", "", 405, "application/json", "", "", "method_not_allowed"},
		{"encoded dotdot", "GET", "/%2e%2e/go.mod", "", 404, "application/json", "", "", "not_found"},
		{"dotdot", "GET", "/../go.mod", "", 404, "application/json", "", "", "not_found"},
		{"dotfile", "GET", "/.env", "", 404, "application/json", "", "", "not_found"},
		{"two segments", "GET", "/a/b", "", 404, "application/json", "", "", "not_found"},
		{"nested real file", "GET", "/sub/inner.txt", "", 404, "application/json", "", "", "not_found"},
		{"csp.json", "GET", "/csp.json", "", 404, "application/json", "", "", "not_found"},
		{"gz direct", "GET", "/index.html.gz", "", 404, "application/json", "", "", "not_found"},
		{"symlink outside", "GET", "/link.txt", "", 404, "application/json", "", "", "not_found"},
		{"directory", "GET", "/sub", "", 404, "application/json", "", "", "not_found"},
		{"missing", "GET", "/nope.js", "", 404, "application/json", "", "", "not_found"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "/", nil)
			r.URL.Path = tc.path // mux hands the decoded path over verbatim
			if tc.ae != "" {
				r.Header.Set("Accept-Encoding", tc.ae)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, r)
			if rec.Code != tc.status {
				t.Fatalf("status %d, want %d (%s)", rec.Code, tc.status, rec.Body)
			}
			hd := rec.Header()
			if got := hd.Get("Content-Type"); got != tc.ctype {
				t.Errorf("Content-Type %q, want %q", got, tc.ctype)
			}
			if tc.cache != "" && hd.Get("Cache-Control") != tc.cache {
				t.Errorf("Cache-Control %q, want %q", hd.Get("Cache-Control"), tc.cache)
			}
			if got := hd.Get("Content-Encoding"); got != tc.encoding {
				t.Errorf("Content-Encoding %q, want %q", got, tc.encoding)
			}
			if tc.encoding == "gzip" && hd.Get("Vary") != "Accept-Encoding" {
				t.Errorf("Vary %q", hd.Get("Vary"))
			}
			if hd.Get("Content-Security-Policy") != csp {
				t.Errorf("CSP %q, want %q", hd.Get("Content-Security-Policy"), csp)
			}
			if hd.Get("X-Content-Type-Options") != "nosniff" || hd.Get("Referrer-Policy") != "no-referrer" {
				t.Errorf("security headers missing: %v", hd)
			}
			if tc.code != "" {
				var body struct{ Error, Code string }
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Code != tc.code || body.Error == "" {
					t.Errorf("body %q: %+v %v", rec.Body, body, err)
				}
			}
			if tc.method == "HEAD" && rec.Body.Len() != 0 {
				t.Errorf("HEAD body %d bytes", rec.Body.Len())
			}
		})
	}
}

func TestHandlerGzipBodyRoundTrips(t *testing.T) {
	h := Handler(fixture(t), CSP(nil))
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 64)
	n, _ := zr.Read(buf)
	if string(buf[:n]) != "<html>x</html>" {
		t.Errorf("body %q", buf[:n])
	}
}

func TestCSP(t *testing.T) {
	want := "default-src 'self'; script-src 'self' 'sha256-a' 'sha256-b'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'"
	if got := CSP([]string{"sha256-a", "sha256-b"}); got != want {
		t.Errorf("got %q", got)
	}
	if got := CSP(nil); !strings.Contains(got, "script-src 'self'; style-src") {
		t.Errorf("no hashes: %q", got)
	}
}

func TestLoadHashes(t *testing.T) {
	write := func(content string) string {
		d := t.TempDir()
		if content != "" {
			if err := os.WriteFile(filepath.Join(d, "csp.json"), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return d
	}
	got, err := LoadHashes(write(`{"scriptHashes":["sha256-x"]}`))
	if err != nil || len(got) != 1 || got[0] != "sha256-x" {
		t.Errorf("valid: %v %v", got, err)
	}
	if _, err := LoadHashes(write("")); err == nil {
		t.Error("missing file: want error")
	}
	if _, err := LoadHashes(write("{nope")); err == nil {
		t.Error("invalid JSON: want error")
	}
}

func TestMissing(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		status       int
		code         string
	}{
		{"GET", "/", 503, "ui_missing"},
		{"GET", "/app.js", 404, "not_found"},
		{"POST", "/", 404, "not_found"},
	} {
		r := httptest.NewRequest(tc.method, "/", nil)
		r.URL.Path = tc.path
		rec := httptest.NewRecorder()
		Missing().ServeHTTP(rec, r)
		if rec.Code != tc.status || !strings.Contains(rec.Body.String(), `"`+tc.code+`"`) {
			t.Errorf("%s %s: %d %s", tc.method, tc.path, rec.Code, rec.Body)
		}
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Header().Get("Content-Security-Policy") != CSP(nil) {
			t.Errorf("%s %s: security headers missing: %v", tc.method, tc.path, rec.Header())
		}
	}
}

func TestOpenRegular(t *testing.T) {
	dir := fixture(t)
	if f, _, ok := openRegular(filepath.Join(dir, "index.html")); !ok {
		t.Fatal("regular file refused")
	} else {
		f.Close() //nolint:errcheck // test
	}
	for _, name := range []string{"link.txt", "missing.js", "sub"} {
		if _, _, ok := openRegular(filepath.Join(dir, name)); ok {
			t.Errorf("%s: opened", name)
		}
	}
}

// A .gz variant that is not a regular file must not label the 404 as gzip.
func TestGzipVariantNotRegular(t *testing.T) {
	dir := fixture(t)
	if err := os.Mkdir(filepath.Join(dir, "plain.txt.gz"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/plain.txt", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	Handler(dir, CSP(nil)).ServeHTTP(rec, r)
	if rec.Code != 200 || rec.Header().Get("Content-Encoding") != "" {
		t.Fatalf("%d encoding=%q", rec.Code, rec.Header().Get("Content-Encoding"))
	}
}
