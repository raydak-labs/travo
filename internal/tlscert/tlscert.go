// Package tlscert keeps the daemon's self-signed TLS certificate (ADR 0016): ECDSA P-256,
// generated on the router, stdlib only. The certificate only encrypts LAN traffic; trust
// in the browser is the operator's manual exception, so SANs and expiry are best effort.
package tlscert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/netip"
	"strings"
	"time"
)

const (
	// Validity stays within the 825 days Apple platforms accept for TLS server certificates.
	Validity = 825 * 24 * time.Hour
	// RenewBefore regenerates a certificate this close to its expiry.
	RenewBefore = 30 * 24 * time.Hour
	// backdate absorbs small clock differences between router and client.
	backdate = 24 * time.Hour
)

// Names are the subject alternative names of a new certificate.
type Names struct {
	DNS []string
	IPs []netip.Addr
}

// Effective is the clock used for validity: the router can boot at 1970 before time sync,
// and the build time is the plausibility floor (invariants §14). A zero floor is ignored.
func Effective(now, floor time.Time) time.Time {
	if now.Before(floor) {
		return floor
	}
	return now
}

// Generate returns PEM certificate and key valid from at-backdate for Validity.
func Generate(random io.Reader, names Names, at time.Time) (certPEM, keyPEM []byte, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), random)
	if err != nil {
		return nil, nil, fmt.Errorf("tls key: %w", err)
	}
	serial, err := randSerial(random)
	if err != nil {
		return nil, nil, err
	}
	notBefore := at.Add(-backdate).UTC().Truncate(time.Second)
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "travo-greenfield", Organization: []string{"Travo (self-signed)"}},
		NotBefore:             notBefore,
		NotAfter:              notBefore.Add(Validity),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              dedupe(names.DNS),
	}
	for _, ip := range names.IPs {
		tmpl.IPAddresses = append(tmpl.IPAddresses, net.IP(ip.Unmap().AsSlice()))
	}
	der, err := x509.CreateCertificate(random, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("tls certificate: %w", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), nil
}

// ErrRenew means the pair parses but expires within RenewBefore of at.
var ErrRenew = errors.New("certificate expires soon")

// Parse loads a PEM pair and checks it is still usable at the effective time at.
func Parse(certPEM, keyPEM []byte, at time.Time) (tls.Certificate, error) {
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, err
	}
	leaf := pair.Leaf
	if leaf == nil {
		if leaf, err = x509.ParseCertificate(pair.Certificate[0]); err != nil {
			return tls.Certificate{}, err
		}
		pair.Leaf = leaf
	}
	if !at.Add(RenewBefore).Before(leaf.NotAfter) {
		return tls.Certificate{}, ErrRenew
	}
	return pair, nil
}

// Fingerprint is the SHA-256 of the leaf in the colon form browsers show.
func Fingerprint(c tls.Certificate) string {
	if len(c.Certificate) == 0 {
		return ""
	}
	sum := sha256.Sum256(c.Certificate[0])
	parts := make([]string, len(sum))
	for i, b := range sum {
		parts[i] = fmt.Sprintf("%02X", b)
	}
	return strings.Join(parts, ":")
}

func randSerial(random io.Reader) (*big.Int, error) {
	n, err := randInt(random, new(big.Int).Lsh(big.NewInt(1), 127))
	if err != nil {
		return nil, fmt.Errorf("tls serial: %w", err)
	}
	return n.Add(n, big.NewInt(1)), nil // RFC 5280: positive
}

func randInt(random io.Reader, limit *big.Int) (*big.Int, error) {
	buf := make([]byte, (limit.BitLen()+7)/8)
	if _, err := io.ReadFull(random, buf); err != nil {
		return nil, err
	}
	return new(big.Int).Mod(new(big.Int).SetBytes(buf), limit), nil
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		s = strings.ToLower(strings.TrimSpace(s))
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
