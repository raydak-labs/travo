// Package apitokens stores bearer tokens for automation (docs/adr/0008). Only
// sha256(token) is persisted; the raw token leaves the daemon once, in the create reply.
// The JSON store follows docs/adr/0009: atomic replacement, mode 0600, one process lock,
// absent = empty, and an unreadable or invalid file fails closed and is never overwritten.
package apitokens

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/blackdark-org/travo-greenfield/internal/atomicfile"
)

// Scope limits what a token may do. read = safe methods only; admin = every protected
// route except the session-only ones (token management, password, logout).
type Scope string

const (
	ScopeRead  Scope = "read"
	ScopeAdmin Scope = "admin"
)

const (
	// Prefix marks a Travo token so secret scanners and humans recognise it.
	Prefix      = "tgt_"
	maxTokens   = 16
	maxNameLen  = 64
	secretBytes = 32
	idDraws     = 3
)

var (
	ErrUnavailable  = errors.New("API tokens unavailable")
	ErrNotFound     = errors.New("API token not found")
	ErrLimit        = errors.New("API token limit reached")
	ErrInvalidName  = errors.New("token name must be 1 to 64 printable characters")
	ErrInvalidScope = errors.New("token scope must be read or admin")
	idPattern       = regexp.MustCompile("^[0-9a-f]{32}$")
	hashPattern     = regexp.MustCompile("^[0-9a-f]{64}$")
)

// Token is the public view: no secret, no hash.
type Token struct {
	ID        string    `json:"id" doc:"Stable token id, used to revoke"`
	Name      string    `json:"name"`
	Scope     Scope     `json:"scope" enum:"read,admin"`
	CreatedAt time.Time `json:"created_at" doc:"Router wall clock at creation; display only"`
}

// record is the persisted shape; Hash is hex sha256 of the raw token.
type record struct {
	Token
	Hash string `json:"hash"`
}

type stored struct {
	Tokens []record `json:"tokens"`
}

type Service struct {
	path      string
	random    io.Reader
	now       func() time.Time
	writeJSON func(string, any, os.FileMode) error
	mu        sync.Mutex
	cache     *stored // last good load or write; nil = read the file again
}

// New builds the store at path; randomness and time come from the caller's defaults.
func New(path string) *Service {
	return &Service{path: path, random: rand.Reader, now: time.Now, writeJSON: atomicfile.WriteJSON}
}

func ValidScope(s Scope) bool { return s == ScopeRead || s == ScopeAdmin }

// NormalizeName trims the name and rejects empty, overlong or non-printable names.
func NormalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > maxNameLen {
		return "", ErrInvalidName
	}
	for _, r := range name {
		if !unicode.IsPrint(r) {
			return "", ErrInvalidName
		}
	}
	return name, nil
}

func digest(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func valid(d stored) bool {
	if d.Tokens == nil || len(d.Tokens) > maxTokens {
		return false
	}
	ids, hashes := map[string]bool{}, map[string]bool{}
	for _, r := range d.Tokens {
		name, err := NormalizeName(r.Name)
		if err != nil || name != r.Name || !ValidScope(r.Scope) || !idPattern.MatchString(r.ID) ||
			!hashPattern.MatchString(r.Hash) || ids[r.ID] || hashes[r.Hash] {
			return false
		}
		ids[r.ID], hashes[r.Hash] = true, true
	}
	return true
}

// load returns the cached store or reads the file; callers hold mu.
func (s *Service) load() (stored, error) {
	if s.cache != nil {
		return *s.cache, nil
	}
	var d stored
	err := atomicfile.ReadJSON(s.path, &d)
	if errors.Is(err, os.ErrNotExist) {
		d, err = stored{Tokens: []record{}}, nil
	}
	if err != nil || !valid(d) {
		return stored{}, ErrUnavailable
	}
	s.cache = &d
	return d, nil
}

func (s *Service) write(d stored) error {
	if s.writeJSON(s.path, d, 0o600) != nil {
		s.cache = nil // the file may hold either version: read it again
		return ErrUnavailable
	}
	s.cache = &d
	return nil
}

func (s *Service) draw(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(s.random, b); err != nil {
		return nil, ErrUnavailable // no fallback: a guessable token is worse than none
	}
	return b, nil
}

func (s *Service) newID(d stored) (string, error) {
	for range idDraws {
		b, err := s.draw(16)
		if err != nil {
			return "", err
		}
		id := hex.EncodeToString(b)
		if d.index(id) < 0 {
			return id, nil
		}
	}
	return "", ErrUnavailable
}

func (d stored) index(id string) int {
	for i, r := range d.Tokens {
		if r.ID == id {
			return i
		}
	}
	return -1
}

// List returns the tokens in creation order.
func (s *Service) List() ([]Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load()
	if err != nil {
		return nil, err
	}
	out := make([]Token, 0, len(d.Tokens))
	for _, r := range d.Tokens {
		out = append(out, r.Token)
	}
	return out, nil
}

// Create stores a new token and returns its public view plus the raw secret, once.
func (s *Service) Create(name string, scope Scope) (Token, string, error) {
	name, err := NormalizeName(name)
	if err != nil {
		return Token{}, "", err
	}
	if !ValidScope(scope) {
		return Token{}, "", ErrInvalidScope
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load()
	if err != nil {
		return Token{}, "", err
	}
	if len(d.Tokens) >= maxTokens {
		return Token{}, "", ErrLimit
	}
	id, err := s.newID(d)
	if err != nil {
		return Token{}, "", err
	}
	secret, err := s.draw(secretBytes)
	if err != nil {
		return Token{}, "", err
	}
	raw := Prefix + base64.RawURLEncoding.EncodeToString(secret)
	t := Token{ID: id, Name: name, Scope: scope, CreatedAt: s.now().UTC().Truncate(time.Second)}
	next := stored{Tokens: append(append([]record{}, d.Tokens...), record{Token: t, Hash: digest(raw)})}
	if err := s.write(next); err != nil {
		return Token{}, "", err
	}
	return t, raw, nil
}

// Delete revokes a token; it stops authenticating as soon as this returns nil.
func (s *Service) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load()
	if err != nil {
		return err
	}
	i := d.index(id)
	if i < 0 {
		return ErrNotFound
	}
	next := stored{Tokens: append(append([]record{}, d.Tokens[:i]...), d.Tokens[i+1:]...)}
	return s.write(next)
}

// Authenticate maps a raw bearer token to its record. An unreadable store
// authenticates nothing (fail closed).
func (s *Service) Authenticate(raw string) (Token, bool) {
	if !strings.HasPrefix(raw, Prefix) || len(raw) > 128 {
		return Token{}, false
	}
	h := digest(raw)
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load()
	if err != nil {
		return Token{}, false
	}
	for _, r := range d.Tokens {
		if r.Hash == h {
			return r.Token, true
		}
	}
	return Token{}, false
}
