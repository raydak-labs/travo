package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/apitokens"
	"github.com/blackdark-org/travo-greenfield/internal/sshkeys"
)

const testEd25519 = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKiO5ZRy/vDVzgKRHcpeswVz7ybMJPfsbY4JBskk4YXL test-ed25519"

func sshEnv(t *testing.T) *env {
	t.Helper()
	store := apitokens.New(filepath.Join(t.TempDir(), "api-tokens.json"))
	return newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) {
		d.Tokens = store
		d.SSHKeys = &sshkeys.Service{File: d.Ubus.(sshkeys.File)}
	})
}

func keyBody(line string) string {
	b, _ := json.Marshal(map[string]string{"key": line})
	return string(b)
}

func TestSSHKeysAddListDelete(t *testing.T) {
	e := sshEnv(t)
	cookie, csrf := e.login()
	rec := e.do(req{method: "POST", path: "/api/v1/system/ssh-keys", body: keyBody(testEd25519), cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusCreated {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	id := bodyField(t, rec, "id")
	if bodyField(t, rec, "fingerprint") != "SHA256:wDQ6rSZgjr4cNqg9V83faMCsq8u5Q3JFikiOxv5Rw84" {
		t.Fatalf("%s", rec.Body)
	}
	rec = e.do(req{method: "POST", path: "/api/v1/system/ssh-keys", body: keyBody(testEd25519), cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusConflict || errCode(t, rec) != "ssh_key_exists" {
		t.Fatalf("dup: %d %s", rec.Code, rec.Body)
	}
	rec = e.do(req{method: "GET", path: "/api/v1/system/ssh-keys", cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), id) || !strings.Contains(rec.Body.String(), `"unmanaged":0`) {
		t.Fatalf("list: %d %s", rec.Code, rec.Body)
	}
	rec = e.do(req{method: "DELETE", path: "/api/v1/system/ssh-keys/" + id, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	rec = e.do(req{method: "DELETE", path: "/api/v1/system/ssh-keys/" + id, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete again: %d %s", rec.Code, rec.Body)
	}
}

func TestSSHKeysRefuseOptionsAndBadBodies(t *testing.T) {
	e := sshEnv(t)
	cookie, csrf := e.login()
	for name, c := range map[string]struct {
		body, code string
	}{
		"options":       {keyBody(`command="/bin/sh" ` + testEd25519), "ssh_key_invalid"},
		"two lines":     {keyBody(testEd25519 + "\n" + testEd25519), "ssh_key_invalid"},
		"unknown field": {`{"key":"x","extra":1}`, "invalid_request"},
		"null":          {`{"key":null}`, "invalid_request"},
	} {
		rec := e.do(req{method: "POST", path: "/api/v1/system/ssh-keys", body: c.body, cookie: cookie, csrf: csrf})
		if rec.Code != http.StatusBadRequest || errCode(t, rec) != c.code {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body)
		}
	}
}

func TestSSHKeysReadTokenListsOnly(t *testing.T) {
	e := sshEnv(t)
	_, secret := e.mint("read")
	if rec := e.do(req{method: "GET", path: "/api/v1/system/ssh-keys", auth: "Bearer " + secret}); rec.Code != http.StatusOK {
		t.Fatalf("read token list: %d %s", rec.Code, rec.Body)
	}
	_, admin := e.mint("admin")
	rec := e.do(req{method: "POST", path: "/api/v1/system/ssh-keys", body: keyBody(testEd25519), auth: "Bearer " + admin})
	if rec.Code != http.StatusForbidden || errCode(t, rec) != "session_required" {
		t.Fatalf("admin token add: %d %s", rec.Code, rec.Body)
	}
}

func TestSSHKeysUnavailable(t *testing.T) {
	e := sshEnv(t)
	e.dev.AuthorizedKeysErr = errors.New("eio")
	cookie, csrf := e.login()
	rec := e.do(req{method: "GET", path: "/api/v1/system/ssh-keys", cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusServiceUnavailable || errCode(t, rec) != "ssh_keys_unavailable" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}
