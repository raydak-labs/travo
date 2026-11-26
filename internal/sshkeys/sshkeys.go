// Package sshkeys manages root's dropbear authorized_keys (invariants §3): list, append
// exactly one validated line, delete one line. Lines Travo cannot parse (options,
// comments, unknown types) are kept verbatim and never offered for deletion.
package sshkeys

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
)

// MaxKeys bounds the managed keys; foreign lines do not count.
const MaxKeys = 32

// maxLine bounds one key line (an RSA-16384 key is ~2.8 KB of base64).
const maxLine = 8192

var (
	ErrInvalid  = errors.New("ssh key: invalid")
	ErrExists   = errors.New("ssh key: already authorized")
	ErrNotFound = errors.New("ssh key: not found")
	ErrLimit    = errors.New("ssh key: limit reached")
)

// types are the key algorithms dropbear on OpenWrt 25.12 accepts in authorized_keys.
var types = map[string]bool{
	"ssh-ed25519":                        true,
	"ecdsa-sha2-nistp256":                true,
	"ecdsa-sha2-nistp384":                true,
	"ecdsa-sha2-nistp521":                true,
	"ssh-rsa":                            true,
	"sk-ssh-ed25519@openssh.com":         true,
	"sk-ecdsa-sha2-nistp256@openssh.com": true,
}

// PublicKey is one managed public key. ID is hex sha256 of the key blob (path-safe);
// Fingerprint is the OpenSSH form users compare with `ssh-keygen -lf`.
type PublicKey struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Comment     string `json:"comment"`
	Fingerprint string `json:"fingerprint"`
	blob        []byte
}

// Parse validates one `type base64 [comment]` line. Options (`command=`, `from=`, ...)
// are refused: the line would not start with a key type.
func Parse(line string) (PublicKey, error) {
	if len(line) > maxLine || strings.ContainsAny(line, "\n\r\x00") {
		return PublicKey{}, ErrInvalid
	}
	fields := strings.Fields(line)
	if len(fields) < 2 || !types[fields[0]] {
		return PublicKey{}, ErrInvalid
	}
	blob, err := base64.StdEncoding.Strict().DecodeString(fields[1])
	if err != nil || embeddedType(blob) != fields[0] {
		return PublicKey{}, ErrInvalid
	}
	comment := strings.Join(fields[2:], " ")
	for _, r := range comment {
		if r < 0x20 || r == 0x7f {
			return PublicKey{}, ErrInvalid
		}
	}
	sum := sha256.Sum256(blob)
	return PublicKey{
		ID:          hex.EncodeToString(sum[:]),
		Type:        fields[0],
		Comment:     comment,
		Fingerprint: "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:]),
		blob:        blob,
	}, nil
}

// embeddedType reads the SSH wire string at the start of the blob: it must repeat the type.
func embeddedType(blob []byte) string {
	if len(blob) < 4 {
		return ""
	}
	n := binary.BigEndian.Uint32(blob)
	if n == 0 || uint64(n) > uint64(len(blob)-4) {
		return ""
	}
	return string(blob[4 : 4+n])
}

// line is the canonical text written for a key: type, base64, comment.
func (k PublicKey) line() string {
	s := k.Type + " " + base64.StdEncoding.EncodeToString(k.blob)
	if k.Comment != "" {
		s += " " + k.Comment
	}
	return s
}

// File is the device port for root's authorized_keys. Read of an absent file is nil, nil.
type File interface {
	ReadAuthorizedKeys(context.Context) ([]byte, error)
	WriteAuthorizedKeys(context.Context, []byte) error
}

// KeyList is the managed keys plus how many lines Travo leaves alone.
type KeyList struct {
	Keys      []PublicKey `json:"keys" nullable:"false"`
	Unmanaged int         `json:"unmanaged" doc:"Lines kept verbatim that Travo cannot parse (options, comments)"`
}

// Service serialises read-modify-write of the file.
type Service struct {
	File File
	mu   sync.Mutex
}

type entry struct {
	raw string
	key *PublicKey
}

func split(data []byte) []entry {
	var out []entry
	for _, raw := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		e := entry{raw: raw}
		if k, err := Parse(raw); err == nil {
			e.key = &k
		}
		out = append(out, e)
	}
	return out
}

func listKeys(entries []entry) KeyList {
	l := KeyList{Keys: []PublicKey{}}
	for _, e := range entries {
		if e.key != nil {
			l.Keys = append(l.Keys, *e.key)
		} else if !strings.HasPrefix(strings.TrimSpace(e.raw), "#") {
			l.Unmanaged++
		}
	}
	return l
}

func join(entries []entry) []byte {
	var b bytes.Buffer
	for _, e := range entries {
		b.WriteString(e.raw)
		b.WriteByte('\n')
	}
	return b.Bytes()
}

func (s *Service) read(ctx context.Context) ([]entry, error) {
	data, err := s.File.ReadAuthorizedKeys(ctx)
	if err != nil {
		return nil, err
	}
	return split(data), nil
}

// List reads the file.
func (s *Service) List(ctx context.Context) (KeyList, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.read(ctx)
	if err != nil {
		return KeyList{}, err
	}
	return listKeys(entries), nil
}

// Add appends exactly one validated line and returns the key.
func (s *Service) Add(ctx context.Context, line string) (PublicKey, error) {
	k, err := Parse(strings.TrimSpace(line))
	if err != nil {
		return PublicKey{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.read(ctx)
	if err != nil {
		return PublicKey{}, err
	}
	l := listKeys(entries)
	for _, have := range l.Keys {
		if have.ID == k.ID {
			return PublicKey{}, ErrExists
		}
	}
	if len(l.Keys) >= MaxKeys {
		return PublicKey{}, ErrLimit
	}
	entries = append(entries, entry{raw: k.line(), key: &k})
	return k, s.File.WriteAuthorizedKeys(ctx, join(entries))
}

// Delete removes every line carrying the key with this id; foreign lines stay.
func (s *Service) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.read(ctx)
	if err != nil {
		return err
	}
	kept := entries[:0:0]
	for _, e := range entries {
		if e.key != nil && e.key.ID == id {
			continue
		}
		kept = append(kept, e)
	}
	if len(kept) == len(entries) {
		return ErrNotFound
	}
	return s.File.WriteAuthorizedKeys(ctx, join(kept))
}
