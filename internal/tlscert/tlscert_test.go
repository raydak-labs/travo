package tlscert

import (
	"bytes"
	"crypto/rand"
	"crypto/x509"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

var (
	build = time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	lan   = netip.MustParseAddr("192.168.1.1")
)

func names() Names {
	return Names{DNS: []string{"localhost", "Travo.lan", "travo.lan"}, IPs: []netip.Addr{netip.MustParseAddr("127.0.0.1"), lan}}
}

func TestEffectiveUsesTheBuildTimeFloorFor1970Clocks(t *testing.T) {
	if got := Effective(time.Unix(0, 0), build); !got.Equal(build) {
		t.Fatalf("1970 clock: %v", got)
	}
	later := build.Add(48 * time.Hour)
	if got := Effective(later, build); !got.Equal(later) {
		t.Fatalf("plausible clock: %v", got)
	}
}

func TestGenerateNamesValidityAndUsage(t *testing.T) {
	certPEM, keyPEM, err := Generate(rand.Reader, names(), build)
	if err != nil {
		t.Fatal(err)
	}
	c, err := Parse(certPEM, keyPEM, build)
	if err != nil {
		t.Fatal(err)
	}
	leaf := c.Leaf
	if !slices.Equal(leaf.DNSNames, []string{"localhost", "travo.lan"}) {
		t.Fatalf("dns names = %v", leaf.DNSNames)
	}
	if len(leaf.IPAddresses) != 2 || !leaf.IPAddresses[1].Equal(lan.AsSlice()) {
		t.Fatalf("ips = %v", leaf.IPAddresses)
	}
	if !leaf.NotBefore.Equal(build.Add(-backdate)) || leaf.NotAfter.Sub(leaf.NotBefore) != Validity {
		t.Fatalf("window %v..%v", leaf.NotBefore, leaf.NotAfter)
	}
	if !slices.Equal(leaf.ExtKeyUsage, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}) || leaf.IsCA {
		t.Fatalf("usage %v ca=%v", leaf.ExtKeyUsage, leaf.IsCA)
	}
	if err := leaf.VerifyHostname("192.168.1.1"); err != nil {
		t.Fatal(err)
	}
	if len(Fingerprint(c)) != 95 {
		t.Fatalf("fingerprint %q", Fingerprint(c))
	}
}

func TestParseRefusesACertificateCloseToExpiry(t *testing.T) {
	certPEM, keyPEM, err := Generate(rand.Reader, names(), build)
	if err != nil {
		t.Fatal(err)
	}
	late := build.Add(Validity - RenewBefore)
	if _, err := Parse(certPEM, keyPEM, late); !errors.Is(err, ErrRenew) {
		t.Fatalf("err = %v", err)
	}
}

func source(t *testing.T, now *time.Time) *Source {
	t.Helper()
	return &Source{
		Dir: filepath.Join(t.TempDir(), "tls"), Names: names, Floor: build, Random: rand.Reader,
		Now: func() time.Time { return *now },
	}
}

func TestSourceGeneratesOnceAndReusesTheFiles(t *testing.T) {
	now := build.Add(time.Hour)
	s := source(t, &now)
	first, err := s.Current()
	if err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(s.keyPath()); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("key file: %v %v", st, err)
	}
	// A restart (fresh Source on the same dir) serves the same certificate.
	again := source(t, &now)
	again.Dir = s.Dir
	second, err := again.Current()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Certificate[0], second.Certificate[0]) {
		t.Fatal("certificate regenerated on restart")
	}
	// Close to expiry, the next handshake gets a new one.
	now = first.Leaf.NotAfter.Add(-RenewBefore)
	third, err := s.Current()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first.Certificate[0], third.Certificate[0]) || !third.Leaf.NotAfter.After(first.Leaf.NotAfter) {
		t.Fatal("expiring certificate not renewed")
	}
}

func TestSourceReplacesAMismatchedPair(t *testing.T) {
	now := build
	s := source(t, &now)
	if _, err := s.Current(); err != nil {
		t.Fatal(err)
	}
	_, otherKey, err := Generate(rand.Reader, names(), build)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.keyPath(), otherKey, 0o600); err != nil {
		t.Fatal(err)
	}
	fresh := source(t, &now)
	fresh.Dir = s.Dir
	if _, err := fresh.Current(); err != nil {
		t.Fatalf("a torn pair must be regenerated: %v", err)
	}
}
