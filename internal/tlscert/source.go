package tlscert

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/atomicfile"
)

// Source serves the certificate from Dir, generating it on the first handshake that needs
// one. Lazy on purpose: by then the trust provider knows the lan addresses (SANs) and the
// clock had time to sync. A failure fails that handshake only; HTTP is unaffected.
type Source struct {
	Dir    string
	Names  func() Names
	Now    func() time.Time
	Floor  time.Time // build time; zero when unstamped
	Random io.Reader

	mu   sync.Mutex
	cert *tls.Certificate
}

func (s *Source) certPath() string { return filepath.Join(s.Dir, "cert.pem") }
func (s *Source) keyPath() string  { return filepath.Join(s.Dir, "key.pem") }

// GetCertificate is the tls.Config hook.
func (s *Source) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return s.Current()
}

// Current returns a usable certificate: the cached one, the one on disk, or a new one.
func (s *Source) Current() (*tls.Certificate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	at := Effective(s.Now(), s.Floor)
	if s.cert != nil && at.Add(RenewBefore).Before(s.cert.Leaf.NotAfter) {
		return s.cert, nil
	}
	c, err := s.load(at)
	if err != nil {
		if c, err = s.generate(at, err); err != nil {
			return nil, err
		}
	}
	s.cert = &c
	return s.cert, nil
}

func (s *Source) load(at time.Time) (tls.Certificate, error) {
	certPEM, err := os.ReadFile(s.certPath())
	if err != nil {
		return tls.Certificate{}, err
	}
	keyPEM, err := os.ReadFile(s.keyPath())
	if err != nil {
		return tls.Certificate{}, err
	}
	return Parse(certPEM, keyPEM, at)
}

// generate writes the key before the certificate: a crash between the two leaves a pair
// that does not match, which fails to parse and is regenerated, never served.
func (s *Source) generate(at time.Time, why error) (tls.Certificate, error) {
	if !errors.Is(why, os.ErrNotExist) {
		slog.Warn("tls: regenerating the self-signed certificate", "reason", why)
	}
	certPEM, keyPEM, err := Generate(s.Random, s.Names(), at)
	if err != nil {
		return tls.Certificate{}, err
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return tls.Certificate{}, fmt.Errorf("tls dir: %w", err)
	}
	if err := atomicfile.Write(s.keyPath(), keyPEM, 0o600); err != nil {
		return tls.Certificate{}, fmt.Errorf("tls key: %w", err)
	}
	if err := atomicfile.Write(s.certPath(), certPEM, 0o644); err != nil {
		return tls.Certificate{}, fmt.Errorf("tls certificate: %w", err)
	}
	c, err := Parse(certPEM, keyPEM, at)
	if err == nil {
		slog.Info("tls: new self-signed certificate", "sha256", Fingerprint(c), "not_after", c.Leaf.NotAfter)
	}
	return c, err
}
