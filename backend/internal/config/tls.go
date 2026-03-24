package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// EnsureTLSCert generates a self-signed ECDSA certificate at certFile/keyFile
// if either file is missing. Returns nil when a usable cert already exists.
func EnsureTLSCert(certFile, keyFile string) error {
	// If both files already exist, nothing to do.
	if fileExists(certFile) && fileExists(keyFile) {
		return nil
	}

	// Generate ECDSA P-256 key.
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	// Self-signed certificate valid for 10 years.
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "openwrt-travel-gui",
		},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privKey.PublicKey, privKey)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(certFile), 0750); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(keyFile), 0750); err != nil {
		return err
	}

	// Write certificate.
	certOut, err := os.OpenFile(certFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return err
	}

	// Write private key.
	keyDER, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		return err
	}
	keyOut, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer keyOut.Close()
	return pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
