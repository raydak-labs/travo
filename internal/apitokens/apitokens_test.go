package apitokens

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var fixed = time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)

func newTest(t *testing.T) (*Service, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "api-tokens.json")
	s := New(path)
	s.now = func() time.Time { return fixed }
	return s, path
}

func TestCreateReturnsSecretOnceAndStoresOnlyTheHash(t *testing.T) {
	s, path := newTest(t)
	tok, raw, err := s.Create("  home assistant ", ScopeRead)
	if err != nil {
		t.Fatal(err)
	}
	if tok.Name != "home assistant" || tok.Scope != ScopeRead || !tok.CreatedAt.Equal(fixed) || !idPattern.MatchString(tok.ID) {
		t.Fatalf("token: %+v", tok)
	}
	if !strings.HasPrefix(raw, Prefix) || len(raw) != len(Prefix)+43 {
		t.Fatalf("raw token shape: %q", raw)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte(strings.TrimPrefix(raw, Prefix))) || !bytes.Contains(b, []byte(digest(raw))) {
		t.Fatalf("store must hold the hash only: %s", b)
	}
	if st, _ := os.Stat(path); st.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", st.Mode())
	}
	got, ok := s.Authenticate(raw)
	if !ok || got.ID != tok.ID {
		t.Fatalf("authenticate: %v %+v", ok, got)
	}
	// A fresh service reads the same file.
	again := New(path)
	if _, ok := again.Authenticate(raw); !ok {
		t.Fatal("token must survive a restart")
	}
}

func TestAuthenticateRejectsUnknownAndForeignTokens(t *testing.T) {
	s, _ := newTest(t)
	_, raw, _ := s.Create("ci", ScopeAdmin)
	for _, bad := range []string{"", "tgt_", raw + "x", strings.TrimPrefix(raw, Prefix), "tg_session_value", Prefix + strings.Repeat("a", 200)} {
		if _, ok := s.Authenticate(bad); ok {
			t.Errorf("%q authenticated", bad)
		}
	}
}

func TestDeleteRevokesImmediately(t *testing.T) {
	s, path := newTest(t)
	tok, raw, _ := s.Create("ci", ScopeAdmin)
	if err := s.Delete(tok.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Authenticate(raw); ok {
		t.Fatal("revoked token still authenticates")
	}
	if _, ok := New(path).Authenticate(raw); ok {
		t.Fatal("revocation must survive a restart")
	}
	if err := s.Delete(tok.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete: %v", err)
	}
}

func TestValidation(t *testing.T) {
	s, _ := newTest(t)
	for _, name := range []string{"", "   ", "a\nb", "tab\there", strings.Repeat("x", 65)} {
		if _, _, err := s.Create(name, ScopeRead); !errors.Is(err, ErrInvalidName) {
			t.Errorf("name %q: %v", name, err)
		}
	}
	if _, _, err := s.Create("ok", Scope("write")); !errors.Is(err, ErrInvalidScope) {
		t.Errorf("scope: %v", err)
	}
	if _, _, err := s.Create(strings.Repeat("ü", 64), ScopeRead); err != nil {
		t.Errorf("64 runes must pass: %v", err)
	}
}

func TestLimit(t *testing.T) {
	s, _ := newTest(t)
	for i := range maxTokens {
		if _, _, err := s.Create("t", ScopeRead); err != nil {
			t.Fatalf("%d: %v", i, err)
		}
	}
	if _, _, err := s.Create("t", ScopeRead); !errors.Is(err, ErrLimit) {
		t.Fatalf("over limit: %v", err)
	}
}

func TestInvalidFileFailsClosedAndIsNotOverwritten(t *testing.T) {
	for name, body := range map[string]string{
		"torn":      `{"tokens":[`,
		"null list": `{"tokens":null}`,
		"bad hash":  `{"tokens":[{"id":"0123456789abcdef0123456789abcdef","name":"x","scope":"read","created_at":"2026-01-01T00:00:00Z","hash":"zz"}]}`,
		"bad scope": `{"tokens":[{"id":"0123456789abcdef0123456789abcdef","name":"x","scope":"root","created_at":"2026-01-01T00:00:00Z","hash":"` + strings.Repeat("a", 64) + `"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			s, path := newTest(t)
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := s.List(); !errors.Is(err, ErrUnavailable) {
				t.Fatalf("list: %v", err)
			}
			if _, _, err := s.Create("x", ScopeRead); !errors.Is(err, ErrUnavailable) {
				t.Fatalf("create: %v", err)
			}
			if _, ok := s.Authenticate(Prefix + "anything"); ok {
				t.Fatal("authenticated against an invalid store")
			}
			if b, _ := os.ReadFile(path); string(b) != body {
				t.Fatalf("file rewritten: %s", b)
			}
		})
	}
}

func TestEntropyFailureCreatesNothing(t *testing.T) {
	s, path := newTest(t)
	s.random = bytes.NewReader(make([]byte, 20)) // enough for the id, not the secret
	if _, _, err := s.Create("x", ScopeRead); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("create: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("store written without a token: %v", err)
	}
}

func TestWriteFailureKeepsOldState(t *testing.T) {
	s, _ := newTest(t)
	_, raw, _ := s.Create("keep", ScopeRead)
	s.writeJSON = func(string, any, os.FileMode) error { return errors.New("disk full") }
	if _, _, err := s.Create("new", ScopeRead); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("create: %v", err)
	}
	list, err := s.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("list after failed write: %v %v", list, err)
	}
	if _, ok := s.Authenticate(raw); !ok {
		t.Fatal("old token lost")
	}
}
