package config

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func generateAndParseCert(t *testing.T) *x509.Certificate {
	t.Helper()
	dir := t.TempDir()
	certFile := filepath.Join(dir, "tls.crt")
	keyFile := filepath.Join(dir, "tls.key")
	if err := EnsureTLSCert(certFile, keyFile); err != nil {
		t.Fatalf("EnsureTLSCert failed: %v", err)
	}
	data, err := os.ReadFile(certFile)
	if err != nil {
		t.Fatalf("reading cert: %v", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		t.Fatal("no PEM block in cert file")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parsing cert: %v", err)
	}
	return cert
}

// Regenerated certificates must not reuse a serial number: browsers reject a
// second cert with the same issuer+serial (SEC_ERROR_REUSED_ISSUER_AND_SERIAL),
// which happens after a factory reset or deleted cert file.
func TestEnsureTLSCert_RandomSerial(t *testing.T) {
	first := generateAndParseCert(t)
	second := generateAndParseCert(t)

	if first.SerialNumber.Cmp(second.SerialNumber) == 0 {
		t.Errorf("expected distinct serial numbers, both are %v", first.SerialNumber)
	}
	if first.SerialNumber.BitLen() < 32 {
		t.Errorf("expected a random serial (>=32 bits), got %v", first.SerialNumber)
	}
}

func TestEnsureTLSCert_DoesNotOverwriteExisting(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "tls.crt")
	keyFile := filepath.Join(dir, "tls.key")
	if err := EnsureTLSCert(certFile, keyFile); err != nil {
		t.Fatalf("EnsureTLSCert failed: %v", err)
	}
	before, _ := os.ReadFile(certFile)
	if err := EnsureTLSCert(certFile, keyFile); err != nil {
		t.Fatalf("second EnsureTLSCert failed: %v", err)
	}
	after, _ := os.ReadFile(certFile)
	if string(before) != string(after) {
		t.Error("expected existing certificate to be left untouched")
	}
}
