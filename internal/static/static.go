// Package static serves the built web UI (web/dist) from a directory: pre-gzipped assets,
// per-class caching and a strict Content-Security-Policy.
package static

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const cspFile = "csp.json"

var (
	// servable is one flat segment without a leading dot: no traversal, no hidden files.
	servable = regexp.MustCompile(`^/[A-Za-z0-9][A-Za-z0-9._-]*$`)
	// hashed matches Vite's content-hashed names (assets/foo-AbCd1234.js, flat layout).
	hashed = regexp.MustCompile(`-[A-Za-z0-9_-]{8}\.[a-z0-9]+$`)
)

// CSP builds the policy; scriptHashes are 'sha256-...' source values for inline scripts.
func CSP(scriptHashes []string) string {
	script := "'self'"
	for _, h := range scriptHashes {
		script += " '" + h + "'"
	}
	return "default-src 'self'; script-src " + script + "; style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'"
}

// LoadHashes reads dir/csp.json written by the web build. The caller logs an error and
// continues with no hashes: the inline script is then blocked (fails closed).
func LoadHashes(dir string) ([]string, error) {
	b, err := os.ReadFile(filepath.Join(dir, cspFile))
	if err != nil {
		return nil, err
	}
	var f struct {
		ScriptHashes []string `json:"scriptHashes"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("%s: %w", cspFile, err)
	}
	return f.ScriptHashes, nil
}

// Handler serves regular files that sit directly in dir.
func Handler(dir, csp string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		securityHeaders(w.Header(), csp)
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		name, ok := fileName(r.URL.Path)
		if !ok {
			writeError(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		path, gz := filepath.Join(dir, name), false
		if acceptsGzip(r) && isRegular(path+".gz") {
			path, gz = path+".gz", true
		}
		f, st, ok := openRegular(path)
		if !ok {
			writeError(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		defer f.Close() //nolint:errcheck // read-only
		h := w.Header()
		if gz {
			h.Set("Content-Encoding", "gzip")
			h.Set("Vary", "Accept-Encoding")
		}
		h.Set("Content-Type", contentType(name))
		h.Set("Cache-Control", cacheControl(name))
		http.ServeContent(w, r, name, st.ModTime(), f)
	})
}

func securityHeaders(h http.Header, csp string) {
	h.Set("Content-Security-Policy", csp)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
}

// openRegular opens path only if it is a regular file, not a symlink, and is still the same
// file after opening (closes the swap window between Lstat and Open).
func openRegular(path string) (*os.File, os.FileInfo, bool) {
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() {
		return nil, nil, false
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, false
	}
	after, err := f.Stat()
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) {
		f.Close() //nolint:errcheck // read-only
		return nil, nil, false
	}
	return f, after, true
}

func contentType(name string) string {
	if filepath.Ext(name) == ".html" {
		return "text/html; charset=utf-8"
	}
	if t := mime.TypeByExtension(filepath.Ext(name)); t != "" {
		return t
	}
	return "application/octet-stream"
}

// fileName maps the decoded request path to a file name in dir.
func fileName(p string) (string, bool) {
	if p == "/" {
		return "index.html", true
	}
	if !servable.MatchString(p) {
		return "", false
	}
	name := p[1:]
	if name == cspFile || strings.HasSuffix(name, ".gz") {
		return "", false
	}
	return name, true
}

// isRegular is false for symlinks, directories and missing files (Lstat does not follow).
func isRegular(path string) bool {
	st, err := os.Lstat(path)
	return err == nil && st.Mode().IsRegular()
}

func acceptsGzip(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
}

func cacheControl(name string) string {
	switch {
	case hashed.MatchString(name):
		return "public, max-age=31536000, immutable"
	case name == "index.html":
		return "no-store"
	}
	return "no-cache"
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}{msg, code})
}

// Missing answers when the UI directory does not exist: 503 on the root, 404 elsewhere.
func Missing() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		securityHeaders(w.Header(), CSP(nil))
		if r.URL.Path == "/" && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
			writeError(w, http.StatusServiceUnavailable, "ui_missing", "web UI is not installed")
			return
		}
		writeError(w, http.StatusNotFound, "not_found", "not found")
	})
}
