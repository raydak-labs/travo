package clients

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/blackdark-org/travo-greenfield/internal/atomicfile"
)

// MaxAlias bounds an alias in runes.
const MaxAlias = 64

// MaxAliases bounds the store so one file stays small on flash.
const MaxAliases = 512

var (
	ErrAliasInvalid = errors.New("alias must be 1-64 printable characters")
	ErrAliasesFull  = errors.New("too many aliases")
	ErrStoreCorrupt = errors.New("client alias store is unreadable")
)

// AliasStore keeps admin-given names in one JSON file in the state dir (ADR 0009). An absent
// file is empty; an unreadable or invalid one fails closed and is never overwritten.
type AliasStore struct {
	Path string
	mu   sync.Mutex
}

type aliasFile struct {
	Aliases map[string]string `json:"aliases"`
}

// All returns a copy of every alias by MAC.
func (s *AliasStore) All() (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read()
}

func (s *AliasStore) read() (map[string]string, error) {
	var f aliasFile
	err := atomicfile.ReadJSON(s.Path, &f)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrStoreCorrupt, err)
	}
	for mac, alias := range f.Aliases {
		if m, ok := NormalizeMAC(mac); !ok || m != mac || validAlias(alias) != nil {
			return nil, ErrStoreCorrupt
		}
	}
	if f.Aliases == nil {
		f.Aliases = map[string]string{}
	}
	return f.Aliases, nil
}

// Set stores alias for mac; an empty alias removes it. An unchanged value writes nothing.
func (s *AliasStore) Set(mac, alias string) error {
	alias = strings.TrimSpace(alias)
	if alias != "" {
		if err := validAlias(alias); err != nil {
			return err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, err := s.read()
	if err != nil {
		return err
	}
	next := maps.Clone(cur)
	if alias == "" {
		delete(next, mac)
	} else {
		next[mac] = alias
	}
	if maps.Equal(cur, next) {
		return nil
	}
	if len(next) > MaxAliases {
		return ErrAliasesFull
	}
	return atomicfile.WriteJSON(s.Path, aliasFile{Aliases: next}, 0o600)
}

func validAlias(alias string) error {
	if alias == "" || !utf8.ValidString(alias) || utf8.RuneCountInString(alias) > MaxAlias || alias != strings.TrimSpace(alias) {
		return ErrAliasInvalid
	}
	for _, r := range alias {
		if !unicode.IsPrint(r) {
			return ErrAliasInvalid
		}
	}
	return nil
}
